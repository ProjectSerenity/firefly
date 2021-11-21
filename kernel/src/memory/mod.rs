//! memory handles paging and a basic physical memory
//! frame allocator.

// This module governs management of physical memory.
// Specifically, ::init and ::active_level_4_table
// produce a page table for the level 4 page table
// (or PML4), as described on the OS Dev wiki: https://wiki.osdev.org/Paging#64-Bit_Paging
// and in the Intel x86 64 manual, volume 3A, section
// 4.5 (4-Level Paging). The functionality for mapping
// pages and translating virtual to physical addresses
// is implemented in the x86_64 crate, in the
// x86_64::structures::paging::OffsetPageTable returned
// by ::init.
//
// This crate also provides a basic physical memory frame
// allocator, which is used in the allocator module to
// build the memory manager.
//
// Although paging is covered by the x86_64 crate, the
// following high-level discussion of 4-level paging may
// be helpful:
//
// Paging maps a virtual address (referred to in the Intel manuals as a 'linear address')
// to a physical address, through a series of page tables. Different parts of the virtual
// address reference different tables, as shown below:
//
// 	       6                   5                   4
// 	 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	|            Ignored            |       PML4      |    PDPT     ~
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// 	   3                   2                   1
// 	 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	~   |       PDT       |      Table      |         Offset        |
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Ignored:     Not used during address translation.
// PML4:        Used as an index into the Page Map Level 4 table (9 bits, 0-511).
// PDPT:        Used as an index into the Page Directory Pointer table (9 bits, 0-511).
// PDT:         Used as an index into the Page Directory table (9 bits, 0-511).
// PT:          Used as an index into the Page table (9 bits, 0-511).
// Offset:      Used as an index into the page (12 bits, 4kB).
//
// A PML4 table comprises 512 64-bit entries (PML4Es)
//
// PML4 entry referencing a PDP entry:
//
// 	       6                   5                   4
// 	 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	|X|          -          |              PDPT Address             ~
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// 	   3                   2                   1
// 	 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	~             PDPT Address              |   -   |S|-|A|C|W|U|R|P|
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// X (Execute disable): Whether the memory is executable (0) or not (1).
// - (Ignored)
// PDPT Address:        The address of the entry in the Page Directory Pointer Table.
// - (Ignored)
// S (Page size):       Must be 0.
// - (Ignored)
// A (Accessed):        Whether the memory has been accessed (1) or not (0).
// C (Cache disable):   Whether the memory has caching enabled (0) or disabled (1).
// W (Write-through):   Whether the memory has write-through caching (1) or write-back (0).
// U (User):            Whether the memory is accessible to userspace.
// R (Read-only):       Whether the memory is read/write (1) or read-only (0).
// P (Present):         Whether this entry is active (1) or absent (0).
//
// A 4-KByte naturally aligned page-directory-pointer table is located at the
// physical address specified in bits 51:12 of the PML4E. A page-directory-pointer
// table comprises 512 64-bit entries (PDPTEs).
//
// PDPT entry referencing a PD entry:
//
// 	       6                   5                   4
// 	 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	|X|          -          |               PD Address              ~
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// 	   3                   2                   1
// 	 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	~               PD Address              |   -   |S|-|A|C|W|U|R|P|
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// X (Execute disable): Whether the memory is executable (0) or not (1).
// - (Ignored)
// PD Address:          The address of the entry in the Page Directory table.
// - (Ignored)
// S (Page size):       Whether the address is for a PD entry (0) or a physical address (1).
// - (Ignored)
// A (Accessed):        Whether the memory has been accessed (1) or not (0).
// C (Cache disable):   Whether the memory has caching enabled (0) or disabled (1).
// W (Write-through):   Whether the memory has write-through caching (1) or write-back (0).
// U (User):            Whether the memory is accessible to userspace.
// R (Read-only):       Whether the memory is read/write (1) or read-only (0).
// P (Present):         Whether this entry is active (1) or absent (0).
//
// Because a PDPTE is identified using bits 47:30 of the linear address, it controls
// access to a 1-GByte region of the linear-address space. Use of the PDPTE depends
// on its PS flag:
//
// - If the PDPTE’s PS flag is 1, the PDPTE maps a 1-GByte page.
// - If the PDPTE’s PS flag is 0, a 4-KByte naturally aligned page directory is
//   located at the physical address specified in bits 51:12 of the PDPTE. A page
//   directory comprises 512 64-bit entries.
//
// PD entry referencing a 2MB page:
//
// 	       6                   5                   4
// 	 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	|X|          -          |               PT Address              ~
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// 	   3                   2                   1
// 	 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	~               PT Address              |   -   |S|-|A|C|W|U|R|P|
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// X (Execute disable): Whether the memory is executable (0) or not (1).
// - (Ignored)
// PT Address:          The address of the page table.
// - (Ignored)
// S (Page size):       Whether the address is for a PT entry (0) or a physical address (1).
// - (Ignored)
// A (Accessed):        Whether the memory has been accessed (1) or not (0).
// C (Cache disable):   Whether the memory has caching enabled (0) or disabled (1).
// W (Write-through):   Whether the memory has write-through caching (1) or write-back (0).
// U (User):            Whether the memory is accessible to userspace.
// R (Read-only):       Whether the memory is read/write (1) or read-only (0).
// P (Present):         Whether this entry is active (1) or absent (0).
//
// Because a PDE is identified using bits 47:21 of the linear address, it
// controls access to a 2-MByte region of the linear-address space. Use of
// the PDE depends on its PS flag:
//
// - If the PDE's PS flag is 1, the PDE maps a 2-MByte page.
// - If the PDE’s PS flag is 0, a 4-KByte naturally aligned page table is
//   located at the physical address specified in bits 51:12 of the PDE.
//   A page table comprises 512 64-bit entries.
//
// PT entry referencing a 4kB page:
//
// 	       6                   5                   4
// 	 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	|X|          -          |              Page Address             ~
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// 	   3                   2                   1
// 	 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	~              Page Address             |  -  |G|S|-|A|C|W|U|R|P|
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// X (Execute disable): Whether the memory is executable (0) or not (1).
// - (Ignored)
// PT Address:          The address of the page table.
// - (Ignored)
// G (Global):          Whether to flush the TLB cache when changing mappings.
// S (Page size):       Whether the address is for a PT entry (0) or a physical address (1).
// D (Dirty):           Whether the memory has been written (1) or not (0).
// A (Accessed):        Whether the memory has been accessed (1) or not (0).
// C (Cache disable):   Whether the memory has caching enabled (0) or disabled (1).
// W (Write-through):   Whether the memory has write-through caching (1) or write-back (0).
// U (User):            Whether the memory is accessible to userspace.
// R (Read-only):       Whether the memory is read/write (1) or read-only (0).
// P (Present):         Whether this entry is active (1) or absent (0).
//
// Because a PTE is identified using bits 47:21 of the linear address, it
// controls access to a 4-kByte region of the linear-address space.

use bootloader::BootInfo;
use x86_64::registers::control::Cr3;
use x86_64::structures::paging::mapper::TranslateResult;
use x86_64::structures::paging::page::PageRangeInclusive;
use x86_64::structures::paging::{
    Mapper, OffsetPageTable, Page, PageTable, PageTableFlags, Size4KiB, Translate,
};
use x86_64::VirtAddr;

mod constants;
pub mod debug;
pub mod pmm;
pub mod vmm;

pub use crate::memory::constants::{
    boot_info_region, init_memory_map, kernel_heap_addr, kernel_segment_region, kernel_stack_addr,
    page_table_region, phys_to_virt_addr, KERNEL_HEAP_SIZE, KERNEL_HEAP_START, KERNEL_STACK_SIZE,
    KERNEL_STACK_START, PHYSICAL_MEMORY_OFFSET,
};

// PML4 functionality.

/// init initialises a new OffsetPageTable.
///
/// # Safety
///
/// This function is unsafe because the caller must guarantee that the
/// complete physical memory is mapped to virtual memory at the passed
/// physical_memory_offset. Also, this function must be only called once
/// to avoid aliasing &mut references (which is undefined behavior).
///
pub unsafe fn init(boot_info: &'static BootInfo) -> OffsetPageTable<'static> {
    let physical_memory_offset = VirtAddr::new(PHYSICAL_MEMORY_OFFSET as u64);
    let level_4_table = active_level_4_table(physical_memory_offset);
    let mut page_table = OffsetPageTable::new(level_4_table, physical_memory_offset);
    let mut frame_allocator = pmm::BootInfoFrameAllocator::new(&boot_info.memory_map);

    remap_kernel_stack_nx(&mut page_table);

    vmm::init(&mut page_table, &mut frame_allocator).expect("heap initialization failed");

    // Switch over to a more sophisticated physical memory manager.
    pmm::init(frame_allocator);

    // Note: We can only initialise the memory map once we
    // have initialised the heap, as extend allocates.
    //
    init_memory_map(&boot_info.memory_map);

    page_table
}

/// active_level_4_table returns a mutable reference
/// to the active level 4 table.
///
/// # Safety
///
/// This function is unsafe because the caller must
/// guarantee that all physical memory is mapped to
/// virtual memory at the passed physical_memory_offset.
///
/// active_level_4_table must only be called once to
/// avoid aliasing &mut references (which is undefined
/// behavior).
///
unsafe fn active_level_4_table(physical_memory_offset: VirtAddr) -> &'static mut PageTable {
    let (level_4_table_frame, _) = Cr3::read();

    let phys = level_4_table_frame.start_address();
    let virt = physical_memory_offset + phys.as_u64();
    let page_table_ptr: *mut PageTable = virt.as_mut_ptr();

    &mut *page_table_ptr // unsafe
}

// remap_kernel_stack_nx remaps all existing mappings for
// the kernel's stack as non-executable.
//
unsafe fn remap_kernel_stack_nx(mapper: &mut OffsetPageTable) {
    let page_range: PageRangeInclusive<Size4KiB> = {
        let top_addr = VirtAddr::new(KERNEL_STACK_START as u64);
        let bottom_addr = VirtAddr::new((KERNEL_STACK_START - KERNEL_STACK_SIZE) as u64);
        let top = Page::containing_address(top_addr);
        let bottom = Page::containing_address(bottom_addr);
        Page::range_inclusive(bottom, top)
    };

    for page in page_range {
        let res = mapper.translate(page.start_address());
        if let TranslateResult::Mapped { flags, .. } = res {
            mapper
                .update_flags(page, flags | PageTableFlags::NO_EXECUTE)
                .expect("failed to remap stack page as NO_EXECUTE")
                .flush();
        }
    }
}
