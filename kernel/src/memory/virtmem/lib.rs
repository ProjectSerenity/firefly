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

use alloc::vec;
use alloc::vec::Vec;
use mapping::PagePurpose;
use memlayout::KERNEL_HEAP;
use serial::println;
use spin::{Mutex, MutexGuard};
use x86_64::registers::control::{Cr4, Cr4Flags};
use x86_64::registers::model_specific::{Efer, EferFlags};
use x86_64::structures::paging::mapper::{
    MapToError, MappedFrame, MapperFlushAll, TranslateResult,
};
use x86_64::structures::paging::{
    FrameAllocator, Mapper, OffsetPageTable, Page, PageTable, PageTableFlags, Size4KiB, Translate,
};
use x86_64::{PhysAddr, VirtAddr};

// Re-export the heap allocators. We don't need to do this, but it's useful
// to expose their documentation to aid future development.
pub use bump::BumpAllocator;
pub use fixed_size_block::FixedSizeBlockAllocator;
pub use linked_list::LinkedListAllocator;

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
pub fn init(
    mapper: &mut OffsetPageTable,
    frame_allocator: &mut impl FrameAllocator<Size4KiB>,
) -> Result<(), MapToError<Size4KiB>> {
    let page_range = {
        let heap_end = KERNEL_HEAP.end();
        let heap_start_page = Page::containing_address(KERNEL_HEAP.start());
        let heap_end_page = Page::containing_address(heap_end);
        Page::range_inclusive(heap_start_page, heap_end_page)
    };

    for page in page_range {
        let frame = frame_allocator
            .allocate_frame()
            .ok_or(MapToError::FrameAllocationFailed)?;
        let flags = PageTableFlags::PRESENT | PageTableFlags::WRITABLE | PageTableFlags::NO_EXECUTE;
        unsafe { mapper.map_to(page, frame, flags, frame_allocator)?.flush() };
    }

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
    unsafe { remap_kernel(mapper) };

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
pub fn virt_to_phys_addrs<T: Translate>(
    page_table: &T,
    addr: VirtAddr,
    len: usize,
) -> Option<Vec<PhysBuffer>> {
    // We will allow an address with length zero
    // as a special case for a single address.
    if len == 0 {
        match page_table.translate_addr(addr) {
            None => return None,
            Some(addr) => return Some(vec![PhysBuffer { addr, len }]),
        }
    }

    // Now we pass through the buffer until we
    // have translated all of it.
    let mut bufs = Vec::new();
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
    use super::*;
    use align::align_down_u64;
    use alloc::collections::BTreeMap;
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
        assert_eq!(
            virt_to_phys_addrs(&page_table, page1, 0),
            Some(vec![PhysBuffer {
                addr: frame3,
                len: 0
            }])
        );

        // Simple example: within a single page.
        assert_eq!(
            virt_to_phys_addrs(&page_table, page1 + 2u64, 2),
            Some(vec![PhysBuffer {
                addr: frame3 + 2u64,
                len: 2
            }])
        );

        // Crossing a split page boundary.
        assert_eq!(
            virt_to_phys_addrs(&page_table, page1 + 4090u64, 12),
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
        assert_eq!(
            virt_to_phys_addrs(&page_table, page2 + 4090u64, 12),
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
