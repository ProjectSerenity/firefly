//! mapping includes helper functionality for mapping out virtual memory, based
//! on the contents of a level 4 page table.

use crate::memory::{
    kernel_stack_addr, phys_to_virt_addr, BOOT_INFO_START, KERNEL_HEAP_START,
    PHYSICAL_MEMORY_OFFSET,
};
use alloc::vec::Vec;
use core::fmt;
use core::sync::atomic::{AtomicU64, Ordering};
use x86_64::instructions;
use x86_64::structures::paging::{PageTable, PageTableFlags};
use x86_64::{PhysAddr, VirtAddr};

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

/// level_4_table iterates through a level 4 page table,
/// returning the sequence of contiguous mappings.
///
/// # Safety
///
/// This function is unsafe because the caller must
/// guarantee that all physical memory is mapped in
/// the given page table.
///
pub unsafe fn level_4_table(pml4: &PageTable) -> Vec<Mapping> {
    let mut mappings = Vec::new();
    let mut current: Option<Mapping> = None;
    for (i, pml4e) in pml4.iter().enumerate() {
        if pml4e.is_unused() {
            continue;
        }

        if pml4e.flags().contains(PageTableFlags::HUGE_PAGE) {
            panic!("invalid huge PML4 page");
        }

        let pdpt_addr = phys_to_virt_addr(pml4e.addr());
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
                current = Mapping::combine(&mut mappings, current, next);
                continue;
            }

            let pdt_addr = phys_to_virt_addr(pdpe.addr());
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
                    current = Mapping::combine(&mut mappings, current, next);
                    continue;
                }

                let pt_addr = phys_to_virt_addr(pde.addr());
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
                    current = Mapping::combine(&mut mappings, current, next);
                }
            }
        }
    }

    if let Some(last) = current {
        mappings.push(last);
    }

    mappings
}

/// PageBytesSize gives the size in bytes of a mapped page.
///
#[derive(Clone, Copy, PartialEq)]
pub enum PageBytesSize {
    Size1GiB,
    Size2MiB,
    Size4KiB,
}

impl PageBytesSize {
    pub fn size(&self) -> u64 {
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

/// Mapping is a helper type for grouping together
/// contiguous page mappings.
///
pub struct Mapping {
    pub virt_start: VirtAddr,
    pub virt_end: VirtAddr,
    pub phys_start: PhysAddr,
    pub phys_end: PhysAddr,
    pub page_count: usize,
    pub page_size: PageBytesSize,
    pub flags: PageTableFlags,
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
    // or will push the current mapping
    // and replace it with the next page.
    //
    pub fn combine(
        mappings: &mut Vec<Mapping>,
        got: Option<Mapping>,
        next: Mapping,
    ) -> Option<Mapping> {
        // Check we have a current mapping.
        match got {
            None => Some(next),
            Some(mut current) => {
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

                    Some(current)
                } else {
                    // Store the current mapping and
                    // replace it with the next one.
                    mappings.push(current);

                    Some(next)
                }
            }
        }
    }
}

impl fmt::Display for Mapping {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        const CONST_NUM: u64 = 1;
        const CONST_STR: &str = "Hello, kernel!";
        static STATIC_VAL: AtomicU64 = AtomicU64::new(CONST_NUM + CONST_STR.len() as u64);
        // Get example pointers.
        let const_addr1 = VirtAddr::from_ptr(&CONST_NUM);
        let const_addr2 = VirtAddr::from_ptr(&CONST_STR);
        let static_addr = VirtAddr::from_ptr(&STATIC_VAL);
        let code_addr = instructions::read_rip();
        STATIC_VAL.fetch_add(1, Ordering::Relaxed);

        // Notes suffix.
        let suffix = if self.virt_start == KERNEL_HEAP_START {
            " (kernel heap)"
        } else if kernel_stack_addr(self.virt_start) && kernel_stack_addr(self.virt_end) {
            " (kernel stack)"
        } else if self.virt_start == PHYSICAL_MEMORY_OFFSET {
            " (all physical memory)"
        } else if self.virt_start <= const_addr1 && const_addr1 <= self.virt_end {
            " (kernel constants)"
        } else if self.virt_start <= const_addr2 && const_addr2 <= self.virt_end {
            " (kernel strings)"
        } else if self.virt_start <= static_addr && static_addr <= self.virt_end {
            " (kernel statics)"
        } else if self.virt_start <= code_addr && code_addr <= self.virt_end {
            " (kernel code)"
        } else if self.virt_start == BOOT_INFO_START {
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
