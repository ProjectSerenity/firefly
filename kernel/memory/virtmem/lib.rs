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

mod bitmap;
mod bump;
mod fixed_size_block;
mod linked_list;
mod mapping;
mod translate;

use self::bitmap::BitmapLevel4KernelMappings;
pub use self::translate::{virt_to_phys_addrs, PhysBuffer};
use bootloader::BootInfo;
use core::slice;
use core::sync::atomic::{AtomicBool, Ordering};
use mapping::PagePurpose;
use memlayout::{phys_to_virt_addr, KERNELSPACE, KERNEL_HEAP, PHYSICAL_MEMORY_OFFSET};
use serial::println;
use spin::Mutex;
use x86_64::registers::control::{Cr3, Cr4, Cr4Flags};
use x86_64::registers::model_specific::{Efer, EferFlags};
use x86_64::structures::paging::mapper::{MapToError, MapperFlushAll};
use x86_64::structures::paging::{
    FrameAllocator, Mapper, OffsetPageTable, Page, PageTable, PageTableFlags, PhysFrame, Size4KiB,
};
use x86_64::{PhysAddr, VirtAddr};

// PML4 functionality.

/// KERNEL_LEVEL4_PAGE_TABLE contains the physical frame where the kernel's
/// level 4 page table resides. This is used as a template for new
/// page tables and as a safe page table when exiting a process.
///
/// This is a mutable static value as it is only assigned to once, while
/// the kernel is being initialised.
///
static mut KERNEL_LEVEL4_PAGE_TABLE: PhysFrame<Size4KiB> =
    unsafe { PhysFrame::from_start_address_unchecked(PhysAddr::zero()) };

/// Returns the kernel's level 4 page table. This must only be used
/// as a readable page table to switch to, without being modified.
///
pub fn kernel_level4_page_table() -> PhysFrame<Size4KiB> {
    unsafe { KERNEL_LEVEL4_PAGE_TABLE }
}

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
    KERNEL_LEVEL4_PAGE_TABLE = level_4_table_frame;

    let mut frame_allocator = physmem::bootstrap(&boot_info.memory_map);

    init_heap(&mut frame_allocator).expect("heap initialization failed");

    // Switch over to a more sophisticated physical memory manager.
    physmem::init(frame_allocator);
}

/// Indicates whether the kernel page mappings have
/// been frozen because the kernel's initialisation
/// is complete.
///
/// Once the page mappings are frozen, any attempts
/// to map memory in kernel space where the level 4
/// page entry is not already mapped will result in
/// an error. This is because we may have multiple
/// sets of page tables, so a change to the level 4
/// page table for kernel space could lead to
/// inconsistencies.
///
static KERNEL_MAPPINGS_FROZEN: AtomicBool = AtomicBool::new(false);

/// Stores the set of level 4 page mappings for
/// kernelspace for once the kernel mappings have
/// been frozen.
///
/// When the page mappings are frozen, we store a
/// bitmap of which level 4 mappings in kernelspace
/// are present. Any future mappings that would
/// affect the level 4 mappings for kernelspace
/// must already be mapped. See [`bitmap`] for more
/// details.
///
/// This is a mutable static, so we have to use
/// unsafe to access it, but it's safe in practice,
/// as we only modify it once, when the page mappings
/// are frozen.
///
static mut KERNEL_MAPPINGS: BitmapLevel4KernelMappings = BitmapLevel4KernelMappings::new();

/// Freeze the kernel page mappings at the top-most
/// level.
///
/// Once the page mappings are frozen, any attempts
/// to map memory in kernel space where the level 4
/// page entry is not already mapped will result in
/// a panic. This is because we may have multiple
/// sets of page tables, so a change to the level 4
/// page table for kernel space could lead to
/// inconsistencies.
///
/// The page mappings cannot be unfrozen once frozen.
///
pub fn freeze_kernel_mappings() {
    let prev = KERNEL_MAPPINGS_FROZEN.fetch_or(true, Ordering::SeqCst);
    if prev {
        panic!("virtmem::freeze_kernel_mappings() called more than once");
    }

    // Set the page mappings.
    with_page_tables(|mapper| {
        let pml4 = mapper.level_4_table();

        // Skip the lower half (userspace) mappings.
        for (idx, entry) in pml4.iter().skip(256).enumerate() {
            if entry.flags().contains(PageTableFlags::PRESENT) {
                let half_idx = 256 + idx as u64; // Bring back to higher half.
                let pml4_idx = half_idx << 39; // Bring back to an address.
                let start_addr = VirtAddr::new(pml4_idx);
                let page = Page::from_start_address(start_addr).unwrap();
                unsafe { KERNEL_MAPPINGS.map(&page) };
            }
        }
    });
}

/// Returns whether the kernel page mappings have
/// been frozen.
///
/// See [`freeze_kernel_mappings`] for more details.
///
#[inline(always)]
pub fn kernel_mappings_frozen() -> bool {
    KERNEL_MAPPINGS_FROZEN.load(Ordering::Relaxed)
}

/// Creates a new level-4 page table, mirroring the
/// kernel's.
///
/// # Panics
///
/// This will panic if the kernel page mappings have
/// not yet been frozen.
///
pub fn new_page_table() -> PhysFrame<Size4KiB> {
    if !kernel_mappings_frozen() {
        panic!("new_page_table() called without having frozen the kernel page mappings.");
    }

    // Allocate the frame, then copy from the
    // kernel mapping.
    let frame = physmem::allocate_frame().expect("failed to allocate new page table");
    let new_virt = phys_to_virt_addr(frame.start_address());
    let old_phys = unsafe { KERNEL_LEVEL4_PAGE_TABLE };
    let old_virt = phys_to_virt_addr(old_phys.start_address());
    let new_buf: &mut [u8] =
        unsafe { slice::from_raw_parts_mut(new_virt.as_mut_ptr(), frame.size() as usize) };
    let old_buf: &[u8] =
        unsafe { slice::from_raw_parts(old_virt.as_mut_ptr(), frame.size() as usize) };
    new_buf.copy_from_slice(old_buf);

    frame
}

/// Check that the kernel mappings are not yet frozen,
/// the proposed mapping is in user space, or the
/// proposed mappings would not modify the level 4
/// page table.
///
/// Note that for performance reasons, this function
/// does not check whether the page tables are frozen.
/// The caller should do so and skip calling `check_mapping`
/// if the page tables are not frozen.
///
fn check_mapping(page: Page) {
    let start_addr = page.start_address();
    if !KERNELSPACE.contains_addr(start_addr) {
        return;
    }

    if unsafe { !KERNEL_MAPPINGS.mapped(&page) } {
        panic!(
            "cannot map page {:p}: kernel mappings frozen and page entry unmapped",
            start_addr
        );
    }
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
    let (level_4_table_frame, _) = Cr3::read();
    let phys = level_4_table_frame.start_address();
    let virt = phys_to_virt_addr(phys);
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
    let frozen = kernel_mappings_frozen();
    with_page_tables(|mapper| {
        for page in page_range {
            if frozen {
                check_mapping(page);
            }

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
    let frozen = kernel_mappings_frozen();
    with_page_tables(|mapper| {
        for page in page_range {
            if frozen {
                check_mapping(page);
            }

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
        use spin::lock;
        lock!(ALLOCATOR.lock).init(
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

/// Wrap a type in a [`spin::Mutex`] so we can
/// implement traits on a locked type.
///
struct Locked<A> {
    lock: Mutex<A>,
}

impl<A> Locked<A> {
    #[allow(dead_code)]
    pub const fn new(inner: A) -> Self {
        Locked {
            lock: Mutex::new(inner),
        }
    }
}
