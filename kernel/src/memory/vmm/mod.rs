//! vmm manages virtual memory and underpins Rust's heap.

// This module provides the functionality to allocate heap
// memory. This is primarily used by Rust's runtime to
// allocate heap memory for the kernel.
//
// The module includes three different allocator implementations,
// a bump allocator, a linked list allocator, and a fixed size
// block allocator. Currently the fixed size block allocator
// is used.

use crate::memory::{KERNEL_HEAP_SIZE, KERNEL_HEAP_START};
use crate::{memory, Locked};
use fixed_size_block::FixedSizeBlockAllocator;
use x86_64::structures::paging::mapper::MapToError;
use x86_64::structures::paging::{
    FrameAllocator, Mapper, Page, PageTable, PageTableFlags, Size4KiB,
};

mod bump;
mod debug;
mod fixed_size_block;
mod linked_list;

#[global_allocator]
static ALLOCATOR: Locked<FixedSizeBlockAllocator> = Locked::new(FixedSizeBlockAllocator::new());

/// init initialises the static global allocator, using
/// the given page mapper and physical memory frame allocator.
///
pub fn init(
    mapper: &mut impl Mapper<Size4KiB>,
    frame_allocator: &mut impl FrameAllocator<Size4KiB>,
) -> Result<(), MapToError<Size4KiB>> {
    let page_range = {
        let heap_end = KERNEL_HEAP_START + memory::KERNEL_HEAP_SIZE - 1u64;
        let heap_start_page = Page::containing_address(KERNEL_HEAP_START);
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
        ALLOCATOR
            .lock()
            .init(KERNEL_HEAP_START.as_u64() as usize, KERNEL_HEAP_SIZE);
    }

    Ok(())
}

/// align_up aligns the given address upwards to alignment align.
///
/// Requires that align is a power of two.
///
fn align_up(addr: usize, align: usize) -> usize {
    (addr + align - 1) & !(align - 1)
}

/// debug iterates through a level 4 page
/// table, printing its mappings using print!.
///
/// # Safety
///
/// This function is unsafe because the caller must
/// guarantee that all physical memory is mapped in
/// the given page table.
///
pub unsafe fn debug(pml4: &PageTable) {
    debug::level_4_table(pml4);
}
