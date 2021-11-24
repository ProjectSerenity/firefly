//! vmm manages virtual memory and underpins Rust's heap.

// This module provides the functionality to allocate heap
// memory. This is primarily used by Rust's runtime to
// allocate heap memory for the kernel.
//
// The module includes three different allocator implementations,
// a bump allocator, a linked list allocator, and a fixed size
// block allocator. Currently the fixed size block allocator
// is used.

use crate::memory::vmm::mapping::PagePurpose;
use crate::memory::KERNEL_HEAP;
use crate::{println, Locked};
use fixed_size_block::FixedSizeBlockAllocator;
use x86_64::structures::paging::mapper::{MapToError, TranslateResult};
use x86_64::structures::paging::{
    FrameAllocator, Mapper, OffsetPageTable, Page, PageTable, PageTableFlags, Size4KiB, Translate,
};

mod bump;
mod fixed_size_block;
mod linked_list;
mod mapping;

#[global_allocator]
static ALLOCATOR: Locked<FixedSizeBlockAllocator> = Locked::new(FixedSizeBlockAllocator::new());

/// init initialises the static global allocator, using
/// the given page mapper and physical memory frame allocator.
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

    // Remap the kernel, now that the heap is set up.
    unsafe { remap_kernel(mapper) };

    Ok(())
}

// remap_kernel remaps all existing mappings for
// the kernel's stack as non-executable, plus unmaps
// any unknown mappings left over by the bootloader.
//
unsafe fn remap_kernel(mapper: &mut OffsetPageTable) {
    let mappings = mapping::level_4_table(mapper.level_4_table());
    for mapping in mappings.iter() {
        match mapping.purpose {
            // Unmap pages we no longer need.
            PagePurpose::Unknown | PagePurpose::NullPage | PagePurpose::Userspace => {
                for page in mapping.page_range() {
                    mapper
                        .unmap(page)
                        .expect("failed to unmap page")
                        .1 // This returns a tuple of the frame and the flusher.
                        .flush();
                }
            }
            PagePurpose::KernelStack => {
                for page in mapping.page_range() {
                    let res = mapper.translate(page.start_address());
                    if let TranslateResult::Mapped { flags, .. } = res {
                        mapper
                            .update_flags(page, flags | PageTableFlags::NO_EXECUTE)
                            .expect("failed to remap stack page as NO_EXECUTE")
                            .flush();
                    }
                }
            }
            _ => {}
        }
    }
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
    let mappings = mapping::level_4_table(pml4);
    for mapping in mappings.iter() {
        println!("{}", mapping);
    }
}
