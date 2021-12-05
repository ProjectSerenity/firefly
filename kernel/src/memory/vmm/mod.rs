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
use x86_64::registers::control::{Cr4, Cr4Flags};
use x86_64::registers::model_specific::{Efer, EferFlags};
use x86_64::structures::paging::mapper::{MapToError, MapperFlushAll};
use x86_64::structures::paging::{
    FrameAllocator, Mapper, OffsetPageTable, Page, PageTable, PageTableFlags, Size4KiB,
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
