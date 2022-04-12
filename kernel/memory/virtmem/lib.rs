// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Virtual memory management and allocation, plus kernel heap management.
//!
//! This module provides the functionality to allocate heap memory. This is
//! primarily used by Rust's runtime to allocate heap memory for the kernel.
//!
//! The two basic APIs are:
//!
//! 1. [`init`]: Use a page table and physical memory allocator to initialise the kernel heap.
//! 2. [`debug`]: Print debug info about the page tables and the virtual address spaces in use.
//!
//! It also keeps track of the kernel's page tables, along with functionality
//! to create new page tables safely shared with the kernel. This set of APIs
//! consists of:
//!
//! - [`freeze_kernel_mappings`]: Mark the kernel's top-level page tables as finished.
//! - [`kernel_mappings_frozen`]: Check whether the kernel's top-level page tables are frozen.
//! - [`kernel_level4_page_table`]: Returns the kernel's level 4 page table.
//! - [`new_page_table`]: Creates a new set of page tables for a user process.
//!
//! Finally, the crate provides functionality to modify the current page
//! tables, such as to map virtual to physical memory:
//!
//! - [`map_pages`]: Map virtual pages to arbitrary physical memory.
//! - [`map_frames_to_pages`]: Map virtual pages to chosen physical memory.
//! - [`with_page_tables`]: Allows access to the current page tables.
//! - [`virt_to_phys_addrs`]: Translate a virtual memory region to the underlying physical memory region(s).
//!
//! ## Heap initialisation
//!
//! The [`init`] function starts by mapping the entirety of the kernel heap
//! address space ([`KERNEL_HEAP`](memlayout::KERNEL_HEAP)) using the physical
//! frame allocator provided. This virtual memory is then used to initialise
//! the heap allocator.
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
mod heap;
mod mapping;
mod translate;

use self::bitmap::BitmapLevel4KernelMappings;
pub use self::translate::{virt_to_phys_addrs, PhysBuffer};
use bootloader::BootInfo;
use core::slice;
use core::sync::atomic::{AtomicBool, Ordering};
use memlayout::{phys_to_virt_addr, KERNELSPACE, PHYSICAL_MEMORY_OFFSET};
use serial::println;
use x86_64::registers::control::Cr3;
use x86_64::structures::paging::mapper::MapToError;
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

    heap::init(&mut frame_allocator).expect("heap initialization failed");

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
