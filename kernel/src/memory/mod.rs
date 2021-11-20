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

use crate::{println, Locked};
use alloc::vec::Vec;
use bootloader::bootinfo::{MemoryRegion, MemoryRegionType};
use bootloader::BootInfo;
use core::fmt;
use x86_64::registers::control::Cr3;
use x86_64::structures::paging::mapper::TranslateResult;
use x86_64::structures::paging::page::PageRangeInclusive;
use x86_64::structures::paging::{
    Mapper, OffsetPageTable, Page, PageSize, PageTable, PageTableFlags, Size4KiB, Translate,
};
use x86_64::{PhysAddr, VirtAddr};

mod pmm;
mod vmm;

// Important addresses.
//
// Make sure the constants below keep in sync with
// the [package.metadata.bootloader] section of
// Cargo.toml.

/// PHYSICAL_MEMORY_OFFSET is the virtual address at which the mapping of
/// all physical memory begins. That is, for any valid physical address,
/// that address can be reached at the same virtual address, plus
/// PHYSICAL_MEMORY_OFFSET.
///
pub const PHYSICAL_MEMORY_OFFSET: usize = 0xffff_8000_0000_0000;

/// phys_to_virt_addr returns a virtual address that is mapped to the
/// given physical address. This uses the mapping of all physical memory
/// at the virtual address PHYSICAL_MEMORY_OFFSET.
///
pub fn phys_to_virt_addr(phys: PhysAddr) -> VirtAddr {
    VirtAddr::new(phys.as_u64() + PHYSICAL_MEMORY_OFFSET as u64)
}

/// KERNEL_HEAP_START is the virtual address where the kernel's heap begins.
pub const KERNEL_HEAP_START: usize = 0x_4444_4444_0000;

/// KERNEL_HEAP_SIZE is the size in bytes of the kernel's heap.
pub const KERNEL_HEAP_SIZE: usize = 100 * 1024; // 100 KiB

/// kernel_heap_addr returns whether addr is an address in the kernel's heap.
///
#[inline]
pub fn kernel_heap_addr(addr: VirtAddr) -> bool {
    let addr = addr.as_u64() as usize;
    KERNEL_HEAP_START <= addr && addr <= KERNEL_HEAP_START + KERNEL_HEAP_SIZE
}

/// KERNEL_STACK_START is the virtual address where the kernel's stack begins.
pub const KERNEL_STACK_START: usize = 0x_7777_7777_0000 + KERNEL_STACK_SIZE;

/// KERNEL_STACK_SIZE is the size in bytes of the kernel's stack.
///
/// Note that this includes an extra page, as the stack counts downward,
/// not upward.
///
pub const KERNEL_STACK_SIZE: usize = 513 * Size4KiB::SIZE as usize;

/// kernel_stack_addr returns whether addr is an address in the kernel's stack.
///
#[inline]
pub fn kernel_stack_addr(addr: VirtAddr) -> bool {
    let addr = addr.as_u64() as usize;
    KERNEL_STACK_START - KERNEL_STACK_SIZE <= addr && addr <= KERNEL_STACK_START
}

/// PHYSICAL_MEMORY_MAP contains a map of physical memory, provided by
/// the boot info.
///
static PHYSICAL_MEMORY_MAP: Locked<Vec<MemoryRegion>> = Locked::new(Vec::new());

/// in_memory_map returns whether the given region includes the given
/// region type.
///
fn in_memory_map(start: PhysAddr, end: PhysAddr, region_type: MemoryRegionType) -> bool {
    let map = PHYSICAL_MEMORY_MAP.lock();
    for region in map.iter() {
        if region.region_type != region_type {
            continue;
        }

        let region_start = PhysAddr::new(region.range.start_addr());
        let region_end = PhysAddr::new(region.range.end_addr());
        if start <= region_start && (region_end - 1u64) <= end
            || region_start <= start && (end - 1u64) <= region_end
        {
            return true;
        }
    }

    false
}

/// boot_info_region returns whether the given region includes boot
/// info data, according to the memory map.
///
pub fn boot_info_region(start: PhysAddr, end: PhysAddr) -> bool {
    in_memory_map(start, end, MemoryRegionType::BootInfo)
}

/// page_table_region returns whether the given region includes page
/// tables, according to the memory map.
///
pub fn page_table_region(start: PhysAddr, end: PhysAddr) -> bool {
    in_memory_map(start, end, MemoryRegionType::PageTable)
}

/// kernel_segment_region returns whether the given region is a kernel
/// segment.
///
pub fn kernel_segment_region(start: PhysAddr, end: PhysAddr) -> bool {
    in_memory_map(start, end, MemoryRegionType::Kernel)
}

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
pub unsafe fn init(
    boot_info: &'static BootInfo,
) -> (OffsetPageTable<'static>, pmm::BootInfoFrameAllocator) {
    let physical_memory_offset = VirtAddr::new(PHYSICAL_MEMORY_OFFSET as u64);
    let level_4_table = active_level_4_table(physical_memory_offset);
    let mut page_table = OffsetPageTable::new(level_4_table, physical_memory_offset);
    let mut frame_allocator = pmm::BootInfoFrameAllocator::init(&boot_info.memory_map);

    remap_kernel_stack_nx(&mut page_table);

    vmm::init(&mut page_table, &mut frame_allocator).expect("heap initialization failed");

    // Note: We can only initialise the memory map once we
    // have initialised the heap, as extend allocates.
    //
    PHYSICAL_MEMORY_MAP
        .lock()
        .extend(boot_info.memory_map.iter());

    (page_table, frame_allocator)
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

// PageBytesSize gives the size in bytes of a mapped page.
//
#[derive(Clone, Copy, PartialEq)]
enum PageBytesSize {
    Size1GiB,
    Size2MiB,
    Size4KiB,
}

impl PageBytesSize {
    fn size(&self) -> u64 {
        match self {
            PageBytesSize::Size1GiB => 0x40000000u64,
            PageBytesSize::Size2MiB => 0x200000u64,
            PageBytesSize::Size4KiB => 0x1000u64,
        }
    }
}

impl fmt::Display for PageBytesSize {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            PageBytesSize::Size1GiB => write!(f, "{}", "1GiB"),
            PageBytesSize::Size2MiB => write!(f, "{}", "2MiB"),
            PageBytesSize::Size4KiB => write!(f, "{}", "4kiB"),
        }
    }
}

// Mapping is a helper type for grouping together
// contiguous page mappings.
//
struct Mapping {
    virt_start: VirtAddr,
    virt_end: VirtAddr,
    phys_start: PhysAddr,
    phys_end: PhysAddr,
    page_count: usize,
    page_size: PageBytesSize,
    flags: PageTableFlags,
}

impl Mapping {
    pub fn new(
        virt_start: VirtAddr,
        phys_start: PhysAddr,
        page_size: PageBytesSize,
        flags: PageTableFlags,
    ) -> Self {
        let flags_mask = PageTableFlags::PRESENT
            | PageTableFlags::WRITABLE
            | PageTableFlags::USER_ACCESSIBLE
            | PageTableFlags::GLOBAL
            | PageTableFlags::NO_EXECUTE;

        Mapping {
            virt_start,
            virt_end: virt_start + page_size.size(),
            phys_start,
            phys_end: phys_start + page_size.size(),
            page_count: 1,
            page_size,
            flags: flags & flags_mask,
        }
    }

    // combine will either include the next
    // page mapping in the current mapping,
    // or will print the current mapping
    // and replace it with the next page.
    //
    pub fn combine(got: &mut Option<Mapping>, next: Mapping) {
        // Check we have a current mapping.
        match got {
            None => *got = Some(next),
            Some(current) => {
                // Check whether next extends the current
                // mapping.
                if current.virt_end == next.virt_start
                    && current.phys_end == next.phys_start
                    && current.page_size == next.page_size
                    && current.flags == next.flags
                {
                    current.virt_end = next.virt_end;
                    current.phys_end = next.phys_end;
                    current.page_count += next.page_count;
                    return;
                }

                // Print the current mapping and
                // replace it with the next one.
                println!("{}", current);
                *got = Some(next)
            }
        }
    }
}

impl fmt::Display for Mapping {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        // Notes suffix.
        let suffix = if kernel_heap_addr(self.virt_start) && kernel_heap_addr(self.virt_end) {
            " (kernel heap)"
        } else if kernel_stack_addr(self.virt_start) && kernel_stack_addr(self.virt_end) {
            " (kernel stack)"
        } else if self.virt_start.as_u64() == PHYSICAL_MEMORY_OFFSET as u64 {
            " (all physical memory)"
        } else if kernel_segment_region(self.phys_start, self.phys_end) {
            " (kernel segment)"
        } else if page_table_region(self.phys_start, self.phys_end) {
            " (page tables)"
        } else if boot_info_region(self.phys_start, self.phys_end) {
            " (boot info)"
        } else {
            ""
        };

        // Simplified flags (global, user, read, write, execute).
        let global = if self.flags.contains(PageTableFlags::GLOBAL) {
            'g'
        } else {
            '-'
        };
        let user = if self.flags.contains(PageTableFlags::USER_ACCESSIBLE) {
            'u'
        } else {
            '-'
        };
        let read = if self.flags.contains(PageTableFlags::PRESENT) {
            'r'
        } else {
            '-'
        };
        let write = if self.flags.contains(PageTableFlags::PRESENT) {
            'w'
        } else {
            '-'
        };
        let execute = if !self.flags.contains(PageTableFlags::NO_EXECUTE) {
            'x'
        } else {
            '-'
        };

        write!(
            f,
            "{:p}-{:p} -> {:p}-{:p} {}x {} page {}{}{}{}{}{}",
            self.virt_start,
            self.virt_end - 1u64,
            self.phys_start,
            self.phys_end - 1u64,
            self.page_count,
            self.page_size,
            global,
            user,
            read,
            write,
            execute,
            suffix
        )
    }
}

// indices_to_addr converts a sequence of page table
// indices into a virtual address. This is useful
// when iterating through a series of page tables,
// as the indices can be used to derive the virtual
// address that would lead to the same physical address.
//
fn indices_to_addr(l4: usize, l3: usize, l2: usize, l1: usize) -> VirtAddr {
    let l4 = (511 & l4) << 39;
    let l3 = (511 & l3) << 30;
    let l2 = (511 & l2) << 21;
    let l1 = (511 & l1) << 12;
    VirtAddr::new(l4 as u64 | l3 as u64 | l2 as u64 | l1 as u64)
}

/// debug_level_4_table iterates through a level 4 page
/// table, printing its mappings using print!.
///
/// # Safety
///
/// This function is unsafe because the caller must
/// guarantee that all physical memory is mapped in
/// the given page table.
///
pub unsafe fn debug_level_4_table(pml4: &PageTable) {
    let phys_offset = VirtAddr::new(PHYSICAL_MEMORY_OFFSET as u64);
    let mut prev: Option<Mapping> = None;
    for (i, pml4e) in pml4.iter().enumerate() {
        if pml4e.is_unused() {
            continue;
        }

        if pml4e.flags().contains(PageTableFlags::HUGE_PAGE) {
            panic!("invalid huge PML4 page");
        }

        let pdpt_addr = phys_offset + pml4e.addr().as_u64();
        let pdpt: &PageTable = &*pdpt_addr.as_mut_ptr(); // unsafe
        for (j, pdpe) in pdpt.iter().enumerate() {
            if pdpe.is_unused() {
                continue;
            }

            if pdpe.flags().contains(PageTableFlags::HUGE_PAGE) {
                let next = Mapping::new(
                    indices_to_addr(i, j, 0, 0),
                    pdpe.addr(),
                    PageBytesSize::Size1GiB,
                    pdpe.flags(),
                );
                Mapping::combine(&mut prev, next);
                continue;
            }

            let pdt_addr = phys_offset + pdpe.addr().as_u64();
            let pdt: &PageTable = &*pdt_addr.as_mut_ptr(); // unsafe
            for (k, pde) in pdt.iter().enumerate() {
                if pde.is_unused() {
                    continue;
                }

                if pde.flags().contains(PageTableFlags::HUGE_PAGE) {
                    let next = Mapping::new(
                        indices_to_addr(i, j, k, 0),
                        pde.addr(),
                        PageBytesSize::Size2MiB,
                        pde.flags(),
                    );
                    Mapping::combine(&mut prev, next);
                    continue;
                }

                let pt_addr = phys_offset + pde.addr().as_u64();
                let pt: &PageTable = &*pt_addr.as_mut_ptr(); // unsafe
                for (l, page) in pt.iter().enumerate() {
                    if page.is_unused() {
                        continue;
                    }

                    if page.flags().contains(PageTableFlags::HUGE_PAGE) {
                        panic!("invalid huge PML1 page");
                    }

                    let next = Mapping::new(
                        indices_to_addr(i, j, k, l),
                        page.addr(),
                        PageBytesSize::Size4KiB,
                        page.flags(),
                    );
                    Mapping::combine(&mut prev, next);
                }
            }
        }
    }

    if let Some(last) = prev {
        println!("{}", last);
    }
}
