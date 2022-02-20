// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Virtual memory management and allocation, plus kernel heap management.
//!
//! This module provides the functionality to allocate heap memory. This is
//! primarily used by Rust's runtime to allocate heap memory for the kernel.
//!
//! Most of the module's functionality is internal. The two external APIs
//! are:
//!
//! 1. [`init`]: Use a page table and physical memory allocator to initialise the kernel heap.
//! 2. [`debug`]: Print debug info about the page tables and the virtual address spaces in use.
//!
//! ## Heap initialisation
//!
//! The [`init`] function starts by mapping the entirety of the kernel heap
//! address space ([`KERNEL_HEAP`](memlayout::KERNEL_HEAP)) using the physical
//! frame allocator provided. This virtual memory is then used to initialise
//! the heap allocator.
//!
//! The module includes three different heap allocator implementations:
//!
//! - [`BumpAllocator`]
//! - [`LinkedListAllocator`]
//! - [`FixedSizeBlockAllocator`]
//!
//! Currently, we use `FixedSizeBlockAllocator`.
//!
//! With the heap initialised, `init` enables global page mappings and the
//! no-execute permission bit and then remaps virtual memory. This ensures
//! that unexpected page mappings are removed and the remaining page mappings
//! have the correct flags. For example, the kernel stack is mapped with the
//! no-execute permission bit set.

#![no_std]
#![feature(const_mut_refs)]

extern crate alloc;

mod bump;
mod fixed_size_block;
mod linked_list;
mod mapping;

use alloc::vec::Vec;
use bootloader::BootInfo;
use mapping::PagePurpose;
use memlayout::{phys_to_virt_addr, KERNEL_HEAP, PHYSICAL_MEMORY_OFFSET};
use serial::println;
use spin::{Mutex, MutexGuard};
use x86_64::instructions::interrupts::without_interrupts;
use x86_64::registers::control::{Cr3, Cr4, Cr4Flags};
use x86_64::registers::model_specific::{Efer, EferFlags};
use x86_64::structures::paging::frame::PhysFrame;
use x86_64::structures::paging::mapper::{
    MapToError, MappedFrame, MapperFlushAll, TranslateResult,
};
use x86_64::structures::paging::{
    FrameAllocator, Mapper, OffsetPageTable, Page, PageSize, PageTable, PageTableFlags, Size4KiB,
    Translate,
};
use x86_64::{PhysAddr, VirtAddr};

// PML4 functionality.

/// KERNEL_PML4_ADDRESS contains the virtual address of the kernel's
/// level 4 page table. This enables the kernel_pml4 function to
/// construct the structured data.
///
static KERNEL_PML4_ADDRESS: Mutex<VirtAddr> = Mutex::new(VirtAddr::zero());

/// Initialise the kernel's memory, including setting up the
/// heap.
///
/// - The bootstrap physical memory manager is initialised using the memory map included in the boot info we are passed by the bootloader.
/// - The bootstrap allocator is used to initialise the virtual memory management (including for allocating page tables).
/// - The global heap allocator is initialised to set up the kernel's heap.
/// - The heap and the bootstrap physical memory manager are used to initialse the second stage physical memory manager.
///
/// # Safety
///
/// This function is unsafe because the caller must guarantee
/// that the complete physical memory is mapped to virtual memory
/// at [`PHYSICAL_MEMORY_OFFSET`].
///
/// `init` must be called only once to avoid aliasing &mut
/// references (which is undefined behavior).
///
pub unsafe fn init(boot_info: &'static BootInfo) {
    // Prepare the kernel's PML4.
    let (level_4_table_frame, _) = Cr3::read();
    let phys = level_4_table_frame.start_address();
    *KERNEL_PML4_ADDRESS.lock() = phys_to_virt_addr(phys);

    let mut frame_allocator = physmem::bootstrap(&boot_info.memory_map);

    init_heap(&mut frame_allocator).expect("heap initialization failed");

    // Switch over to a more sophisticated physical memory manager.
    physmem::init(frame_allocator);
}

/// Allows the caller to modify the page mappings
/// without multiple mutable references existing
/// at the same time.
///
/// If just mapping a set of memory pages, prefer
/// [`map_pages`] instead, which provides additional
/// checks on the memory being mapped.
///
pub fn with_page_tables<F, R>(f: F) -> R
where
    F: FnOnce(&mut OffsetPageTable) -> R,
{
    let virt = KERNEL_PML4_ADDRESS.lock();
    let page_table_ptr: *mut PageTable = virt.as_mut_ptr();

    // This bit is unsafe if we're not using
    // the currently-active page tables.
    let page_table = unsafe { &mut *page_table_ptr };
    let mut mapper = unsafe { OffsetPageTable::new(page_table, PHYSICAL_MEMORY_OFFSET) };

    f(&mut mapper)
}

/// Map the given page range, which can be inclusive or exclusive.
///
pub fn map_pages<R, A>(
    page_range: R,
    allocator: &mut A,
    flags: PageTableFlags,
) -> Result<(), MapToError<Size4KiB>>
where
    R: Iterator<Item = Page>,
    A: FrameAllocator<Size4KiB> + ?Sized,
{
    with_page_tables(|mapper| {
        for page in page_range {
            let frame = allocator
                .allocate_frame()
                .ok_or(MapToError::FrameAllocationFailed)?;
            unsafe {
                mapper.map_to(page, frame, flags, allocator)?.flush();
            }
        }

        Ok(())
    })
}

/// Map the given frame range to the page range, either of which
/// can be inclusive or exclusive.
///
pub fn map_frames_to_pages<F, P, A>(
    mut frame_range: F,
    page_range: P,
    allocator: &mut A,
    flags: PageTableFlags,
) -> Result<(), MapToError<Size4KiB>>
where
    F: Iterator<Item = PhysFrame>,
    P: Iterator<Item = Page>,
    A: FrameAllocator<Size4KiB> + ?Sized,
{
    with_page_tables(|mapper| {
        for page in page_range {
            let frame = frame_range
                .next()
                .ok_or(MapToError::FrameAllocationFailed)?;
            unsafe {
                mapper.map_to(page, frame, flags, allocator)?.flush();
            }
        }

        Ok(())
    })
}

// Re-export the heap allocators. We don't need to do this, but it's useful
// to expose their documentation to aid future development.
pub use bump::BumpAllocator;
pub use fixed_size_block::FixedSizeBlockAllocator;
pub use linked_list::LinkedListAllocator;

#[cfg(not(test))]
#[global_allocator]
static ALLOCATOR: Locked<FixedSizeBlockAllocator> = Locked::new(FixedSizeBlockAllocator::new());

/// Initialise the static global allocator, enabling the kernel heap.
///
/// The given page mapper and physical memory frame allocator are used to
/// map the entirety of the kernel heap address space ([`KERNEL_HEAP`](memlayout::KERNEL_HEAP)).
///
/// With the heap initialised, `init` enables global page mappings and the
/// no-execute permission bit and then remaps virtual memory. This ensures
/// that unexpected page mappings are removed and the remaining page mappings
/// have the correct flags. For example, the kernel stack is mapped with the
/// no-execute permission bit set.
///
fn init_heap(
    frame_allocator: &mut impl FrameAllocator<Size4KiB>,
) -> Result<(), MapToError<Size4KiB>> {
    let page_range = {
        let heap_end = KERNEL_HEAP.end();
        let heap_start_page = Page::containing_address(KERNEL_HEAP.start());
        let heap_end_page = Page::containing_address(heap_end);
        Page::range_inclusive(heap_start_page, heap_end_page)
    };

    let flags = PageTableFlags::PRESENT | PageTableFlags::WRITABLE | PageTableFlags::NO_EXECUTE;

    map_pages(page_range, frame_allocator, flags).expect("failed to map kernel heap");

    #[cfg(not(test))]
    unsafe {
        ALLOCATOR.lock().init(
            KERNEL_HEAP.start().as_u64() as usize,
            KERNEL_HEAP.size() as usize,
        );
    }

    // Set the CR4 fields, so we can then use the global
    // page flag when we remap the kernel.
    let mut flags = Cr4::read();
    flags |= Cr4Flags::PAGE_GLOBAL; // Enable the global flag in page tables.
    unsafe { Cr4::write(flags) };

    // Set the EFER fields, so we can use the no-execute
    // page flag when we remap the kernel.
    let mut flags = Efer::read();
    flags |= EferFlags::NO_EXECUTE_ENABLE; // Enable the no-execute flag in page tables.
    unsafe { Efer::write(flags) };

    // Remap the kernel, now that the heap is set up.
    with_page_tables(|mapper| unsafe { remap_kernel(mapper) });

    Ok(())
}

// remap_kernel remaps all existing mappings for
// the kernel's stack as non-executable, plus unmaps
// any unknown mappings left over by the bootloader.
//
unsafe fn remap_kernel(mapper: &mut OffsetPageTable) {
    // Analyse and iterate through the page mappings
    // in the PML4.
    //
    // Rather than constantly flushing the TLB as we
    // go along, we do one big flush at the end.
    let mappings = mapping::level_4_table(mapper.level_4_table());
    for mapping in mappings.iter() {
        match mapping.purpose {
            // Unmap pages we no longer need.
            PagePurpose::Unknown
            | PagePurpose::NullPage
            | PagePurpose::Userspace
            | PagePurpose::KernelStackGuard => {
                mapping.unmap(mapper).expect("failed to unmap page");
            }
            // Global and read-write (kernel stack, heap, data, physical memory).
            PagePurpose::KernelStack
            | PagePurpose::KernelHeap
            | PagePurpose::KernelStatics
            | PagePurpose::AllPhysicalMemory => {
                let flags = PageTableFlags::GLOBAL
                    | PageTableFlags::PRESENT
                    | PageTableFlags::WRITABLE
                    | PageTableFlags::NO_EXECUTE;
                mapping
                    .update_flags(mapper, flags)
                    .expect("failed to update page flags");
            }
            // Global read only (kernel constants, boot info).
            PagePurpose::KernelConstants | PagePurpose::KernelStrings | PagePurpose::BootInfo => {
                let flags =
                    PageTableFlags::GLOBAL | PageTableFlags::PRESENT | PageTableFlags::NO_EXECUTE;
                mapping
                    .update_flags(mapper, flags)
                    .expect("failed to update page flags");
            }
            // Global read execute (kernel code).
            PagePurpose::KernelCode => {
                let flags = PageTableFlags::GLOBAL | PageTableFlags::PRESENT;
                mapping
                    .update_flags(mapper, flags)
                    .expect("failed to update page flags");
            }
            // This means a segment spans multiple pages
            // and the page we got in our constants was
            // not in this one, so we don't know which
            // segment this is.
            PagePurpose::KernelBinaryUnknown => {
                // Leave with the default flags. They might have more
                // permissions than we'd like, but removing permissions
                // could easily break things.
            }
            // MMIO and CPU-local data won't have been
            // mapped yet, so this shouldn't happen,
            // but if it does, we just leave it as is.
            PagePurpose::Mmio | PagePurpose::CpuLocal => {
                // Nothing to do.
            }
        }
    }

    // Flush the TLB.
    MapperFlushAll::new().flush_all();
}

/// Prints debug info about the passed level 4 page table, including
/// its mappings.
///
/// # Safety
///
/// This function is unsafe because the caller must
/// guarantee that all physical memory is mapped in
/// the given page table.
///
pub unsafe fn debug(pml4: &PageTable) {
    let mappings = mapping::level_4_table(pml4);
    for mapping in mappings.iter() {
        println!("{}", mapping);
    }
}

/// Describes a single contiguous physical memory region.
///
#[derive(Clone, Copy, Debug, PartialEq, PartialOrd)]
pub struct PhysBuffer {
    pub addr: PhysAddr,
    pub len: usize,
}

/// Translates a contiguous virtual memory region into one
/// or more contiguous physical memory regions.
///
/// If any part of the virtual memory region is not mapped
/// in the given page table, then None is returned.
///
pub fn virt_to_phys_addrs(addr: VirtAddr, len: usize) -> Option<Vec<PhysBuffer>> {
    // Get the current page table.
    // This is similar to the code in kernel_pml4,
    // but will use whatever is the current page
    // table.
    //
    // This is safe, even though we use unsafe, as
    // we disable interrupts while the page tables
    // are in use and we only read, not write them.
    // This means they can't change underneath us
    // and there's no thread-safety risk.
    //
    // We also pre-allocate the vector we pass in,
    // so no allocations should need to happen.
    let (level_4_table_frame, _) = Cr3::read();
    let phys = level_4_table_frame.start_address();
    let virt = phys_to_virt_addr(phys);
    let page_table_ptr: *mut PageTable = virt.as_mut_ptr();
    let page_table = unsafe { &mut *page_table_ptr };
    let pml4 = unsafe { OffsetPageTable::new(page_table, PHYSICAL_MEMORY_OFFSET) };

    // Work out the max number of buffers we could
    // get back, which is the situation where we
    // straddle a page boundary.
    let max_pages = (len / (Size4KiB::SIZE as usize)) + 1;
    let buf = Vec::with_capacity(max_pages);

    without_interrupts(|| _virt_to_phys_addrs(&pml4, buf, addr, len))
}

/// This is the underlyign implementation for mapping
/// virtual addresses to physical buffers. We abstract
/// away the translator type to make testing easier.
///
fn _virt_to_phys_addrs<T: Translate>(
    page_table: &T,
    mut bufs: Vec<PhysBuffer>,
    addr: VirtAddr,
    len: usize,
) -> Option<Vec<PhysBuffer>> {
    // We will allow an address with length zero
    // as a special case for a single address.
    if len == 0 {
        match page_table.translate_addr(addr) {
            None => return None,
            Some(addr) => {
                bufs.push(PhysBuffer { addr, len });
                return Some(bufs);
            }
        }
    }

    // Now we pass through the buffer until we
    // have translated all of it.
    let mut addr = addr;
    let mut len = len;
    while len > 0 {
        match page_table.translate(addr) {
            TranslateResult::NotMapped => return None,
            TranslateResult::InvalidFrameAddress(_) => return None,
            TranslateResult::Mapped { frame, offset, .. } => {
                // Advance the buffer by the amount of
                // physical memory we just found.
                let found = (frame.size() - offset) as usize;
                let phys_addr = match frame {
                    MappedFrame::Size4KiB(frame) => frame.start_address() + offset,
                    MappedFrame::Size2MiB(frame) => frame.start_address() + offset,
                    MappedFrame::Size1GiB(frame) => frame.start_address() + offset,
                };

                bufs.push(PhysBuffer {
                    addr: phys_addr,
                    len: core::cmp::min(len, found),
                });
                addr += found;
                len = len.saturating_sub(found);
            }
        }
    }

    // TODO(#10): Merge contiguous regions to reduce the number of buffers we return.

    Some(bufs)
}

/// Wrap a type in a [`spin::Mutex`] so we can
/// implement traits on a locked type.
///
struct Locked<A> {
    inner: Mutex<A>,
}

impl<A> Locked<A> {
    #[allow(dead_code)]
    pub const fn new(inner: A) -> Self {
        Locked {
            inner: Mutex::new(inner),
        }
    }

    pub fn lock(&self) -> MutexGuard<A> {
        self.inner.lock()
    }
}

#[cfg(test)]
mod test {
    extern crate std;
    use super::*;
    use align::align_down_u64;
    use alloc::collections::BTreeMap;
    use alloc::vec;
    use x86_64::structures::paging::PhysFrame;

    /// DebugPageTable is a helper type for testing code that
    /// uses page tables. It emulates the behaviour for a level
    /// 4 page table using heap memory, without modifying the
    /// system page tables.
    ///
    pub struct DebugPageTable {
        mappings: BTreeMap<VirtAddr, PhysFrame>,
    }

    impl DebugPageTable {
        pub fn new() -> Self {
            DebugPageTable {
                mappings: BTreeMap::new(),
            }
        }

        pub fn map(&mut self, addr: VirtAddr, frame: PhysFrame) {
            // Check the virtual address is at a page boundary,
            // to simplify things.
            assert_eq!(addr.as_u64(), align_down_u64(addr.as_u64(), Size4KiB::SIZE));

            self.mappings.insert(addr, frame);
        }
    }

    impl Translate for DebugPageTable {
        fn translate(&self, addr: VirtAddr) -> TranslateResult {
            let truncated = VirtAddr::new(align_down_u64(addr.as_u64(), Size4KiB::SIZE));
            match self.mappings.get(&truncated) {
                None => return TranslateResult::NotMapped,
                Some(frame) => TranslateResult::Mapped {
                    frame: MappedFrame::Size4KiB(*frame),
                    offset: addr - truncated,
                    flags: PageTableFlags::PRESENT,
                },
            }
        }
    }

    #[test]
    fn debug_page_table() {
        // Check that the debug page table works
        // correctly.
        let mut page_table = DebugPageTable::new();
        fn phys_frame(addr: u64) -> PhysFrame {
            let addr = PhysAddr::new(addr);
            let frame = PhysFrame::from_start_address(addr);
            frame.unwrap()
        }

        assert_eq!(page_table.translate_addr(VirtAddr::new(4096)), None);
        page_table.map(VirtAddr::new(4096), phys_frame(4096));
        assert_eq!(
            page_table.translate_addr(VirtAddr::new(4096)),
            Some(PhysAddr::new(4096))
        );
        assert_eq!(
            page_table.translate_addr(VirtAddr::new(4097)),
            Some(PhysAddr::new(4097))
        );
    }

    #[test]
    fn virt_to_phys_addrs() {
        // Start by making some mappings we can use.
        // We map as follows:
        // - page 1 => frame 3
        // - page 2 => frame 1
        // - page 3 => frame 2
        let page1 = VirtAddr::new(1 * Size4KiB::SIZE);
        let page2 = VirtAddr::new(2 * Size4KiB::SIZE);
        let page3 = VirtAddr::new(3 * Size4KiB::SIZE);
        let frame1 = PhysAddr::new(1 * Size4KiB::SIZE);
        let frame2 = PhysAddr::new(2 * Size4KiB::SIZE);
        let frame3 = PhysAddr::new(3 * Size4KiB::SIZE);

        let mut page_table = DebugPageTable::new();
        fn phys_frame(addr: PhysAddr) -> PhysFrame {
            let frame = PhysFrame::from_start_address(addr);
            frame.unwrap()
        }

        page_table.map(page1, phys_frame(frame3));
        page_table.map(page2, phys_frame(frame1));
        page_table.map(page3, phys_frame(frame2));

        // Simple example: single address.
        let buf = Vec::with_capacity(5);
        assert_eq!(
            _virt_to_phys_addrs(&page_table, buf, page1, 0),
            Some(vec![PhysBuffer {
                addr: frame3,
                len: 0
            }])
        );

        // Simple example: within a single page.
        let buf = Vec::with_capacity(5);
        assert_eq!(
            _virt_to_phys_addrs(&page_table, buf.to_vec(), page1 + 2u64, 2),
            Some(vec![PhysBuffer {
                addr: frame3 + 2u64,
                len: 2
            }])
        );

        // Crossing a split page boundary.
        let buf = Vec::with_capacity(5);
        assert_eq!(
            _virt_to_phys_addrs(&page_table, buf.to_vec(), page1 + 4090u64, 12),
            Some(vec![
                PhysBuffer {
                    addr: frame3 + 4090u64,
                    len: 6
                },
                PhysBuffer {
                    addr: frame1,
                    len: 6
                }
            ])
        );

        // Crossing a contiguous page boundary.
        // TODO: merge contiguous regions to reduce the number of buffers we return.
        let buf = Vec::with_capacity(5);
        assert_eq!(
            _virt_to_phys_addrs(&page_table, buf.to_vec(), page2 + 4090u64, 12),
            Some(vec![
                PhysBuffer {
                    addr: frame1 + 4090u64,
                    len: 6
                },
                PhysBuffer {
                    addr: frame2,
                    len: 6
                }
            ])
        );
    }
}