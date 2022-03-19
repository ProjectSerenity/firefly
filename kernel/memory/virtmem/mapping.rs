// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! mapping includes helper functionality for mapping out virtual memory, based
//! on the contents of a level 4 page table.

use alloc::vec::Vec;
use core::fmt;
use core::sync::atomic::{AtomicU64, Ordering};
use memlayout::{
    phys_to_virt_addr, VirtAddrRange, BOOT_INFO, CPU_LOCAL, KERNEL_BINARY, KERNEL_HEAP,
    KERNEL_STACK, KERNEL_STACK_GUARD, MMIO_SPACE, NULL_PAGE, PHYSICAL_MEMORY, USERSPACE,
};
use pretty::Bytes;
use x86_64::instructions;
use x86_64::structures::paging::mapper::{
    FlagUpdateError, Mapper, MapperFlushAll, OffsetPageTable, UnmapError,
};
use x86_64::structures::paging::page::Page;
use x86_64::structures::paging::{PageTable, PageTableFlags, Size1GiB, Size2MiB, Size4KiB};
use x86_64::{PhysAddr, VirtAddr};

// remap_kernel remaps all existing mappings for
// the kernel's stack as non-executable, plus unmaps
// any unknown mappings left over by the bootloader.
//
pub unsafe fn remap_kernel(mapper: &mut OffsetPageTable) {
    // Analyse and iterate through the page mappings
    // in the PML4.
    //
    // Rather than constantly flushing the TLB as we
    // go along, we do one big flush at the end.
    let mappings = level_4_table(mapper.level_4_table());
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

    // Analyse the mappings to determine their purpose,
    // which is hard to do correctly as we go along.

    // Make some constants/statics, of which we can then
    // take the address to determine where the relevant
    // sections of the kernel are mapped.
    const CONST_NUM: u64 = 1;
    const CONST_STR: &str = "Hello, kernel!";
    static STATIC_VAL: AtomicU64 = AtomicU64::new(CONST_NUM + CONST_STR.len() as u64);

    // Get example pointers.
    let const_addr1 = VirtAddr::from_ptr(&CONST_NUM);
    let const_addr2 = VirtAddr::from_ptr(&CONST_STR);
    let static_addr = VirtAddr::from_ptr(&STATIC_VAL);
    let code_addr = instructions::read_rip();
    STATIC_VAL.fetch_add(1, Ordering::Relaxed);

    let mut out = Vec::with_capacity(mappings.len());
    for map in mappings {
        let range = VirtAddrRange::new(map.virt_start, map.virt_end);
        let purpose = if NULL_PAGE.contains(&range) {
            PagePurpose::NullPage
        } else if USERSPACE.contains(&range) {
            PagePurpose::Userspace
        } else if KERNEL_BINARY.contains(&range) {
            if range.contains_addr(const_addr1) {
                PagePurpose::KernelConstants
            } else if range.contains_addr(const_addr2) {
                PagePurpose::KernelStrings
            } else if range.contains_addr(static_addr) {
                PagePurpose::KernelStatics
            } else if range.contains_addr(code_addr) {
                PagePurpose::KernelCode
            } else {
                PagePurpose::KernelBinaryUnknown
            }
        } else if BOOT_INFO.contains(&range) {
            PagePurpose::BootInfo
        } else if KERNEL_HEAP.contains(&range) {
            PagePurpose::KernelHeap
        } else if KERNEL_STACK.contains(&range) {
            PagePurpose::KernelStack
        } else if KERNEL_STACK_GUARD.contains(&range) {
            PagePurpose::KernelStackGuard
        } else if MMIO_SPACE.contains(&range) {
            PagePurpose::Mmio
        } else if CPU_LOCAL.contains(&range) {
            PagePurpose::CpuLocal
        } else if PHYSICAL_MEMORY.contains(&range) {
            PagePurpose::AllPhysicalMemory
        } else {
            PagePurpose::Unknown
        };

        out.push(Mapping::with_purpose(map, purpose));
    }

    out
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

/// PagePurpose describes the known use of a contiguous
/// set of mapped pages.
///
#[derive(Clone, Copy, PartialEq)]
pub enum PagePurpose {
    Unknown,
    NullPage,
    Userspace,
    BootInfo,
    KernelCode,
    KernelConstants,
    KernelStrings,
    KernelStatics,
    KernelBinaryUnknown,
    KernelHeap,
    KernelStack,
    KernelStackGuard,
    Mmio,
    CpuLocal,
    AllPhysicalMemory,
}

impl fmt::Display for PagePurpose {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            PagePurpose::Unknown => write!(f, ""),
            PagePurpose::NullPage => write!(f, " (null page)"),
            PagePurpose::Userspace => write!(f, " (userspace)"),
            PagePurpose::BootInfo => write!(f, " (boot info)"),
            PagePurpose::KernelCode => write!(f, " (kernel code)"),
            PagePurpose::KernelConstants => write!(f, " (kernel constants)"),
            PagePurpose::KernelStrings => write!(f, " (kernel strings)"),
            PagePurpose::KernelStatics => write!(f, " (kernel statics)"),
            PagePurpose::KernelBinaryUnknown => write!(f, " (kernel binary unknown)"),
            PagePurpose::KernelHeap => write!(f, " (kernel heap)"),
            PagePurpose::KernelStack => write!(f, " (kernel stack)"),
            PagePurpose::KernelStackGuard => write!(f, " (kernel stack guard)"),
            PagePurpose::Mmio => write!(f, " (MMIO)"),
            PagePurpose::CpuLocal => write!(f, " (CPU-local data)"),
            PagePurpose::AllPhysicalMemory => write!(f, " (all physical memory)"),
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
    pub page_count: u64,
    pub page_size: PageBytesSize,
    pub flags: PageTableFlags,
    pub purpose: PagePurpose,
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
            virt_end: virt_start + page_size.size() - 1u64,
            phys_start,
            phys_end: phys_start + page_size.size() - 1u64,
            page_count: 1,
            page_size,
            flags: flags & flags_mask,
            purpose: PagePurpose::Unknown,
        }
    }

    fn with_purpose(mapping: Mapping, purpose: PagePurpose) -> Self {
        Mapping { purpose, ..mapping }
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
                if current.virt_end + 1u64 == next.virt_start
                    && current.phys_end + 1u64 == next.phys_start
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

    /// unmap uses the given mapper to unmap the
    /// entire mapping.
    ///
    /// Note that unmap will not flush the TLB.
    ///
    pub fn unmap(&self, mapper: &mut OffsetPageTable) -> Result<(), UnmapError> {
        match self.page_size {
            PageBytesSize::Size4KiB => {
                let start = Page::<Size4KiB>::containing_address(self.virt_start);
                let end = Page::<Size4KiB>::containing_address(self.virt_end);
                let range = Page::range_inclusive(start, end);
                for page in range {
                    mapper.unmap(page)?.1.ignore();
                }
            }
            PageBytesSize::Size2MiB => {
                let start = Page::<Size2MiB>::containing_address(self.virt_start);
                let end = Page::<Size2MiB>::containing_address(self.virt_end);
                let range = Page::range_inclusive(start, end);
                for page in range {
                    mapper.unmap(page)?.1.ignore();
                }
            }
            PageBytesSize::Size1GiB => {
                let start = Page::<Size1GiB>::containing_address(self.virt_start);
                let end = Page::<Size1GiB>::containing_address(self.virt_end);
                let range = Page::range_inclusive(start, end);
                for page in range {
                    mapper.unmap(page)?.1.ignore();
                }
            }
        }

        Ok(())
    }

    /// update_flags uses the given mapper to update the
    /// flags entire mapping.
    ///
    /// Note that update_flags will not flush the TLB.
    ///
    pub fn update_flags(
        &self,
        mapper: &mut OffsetPageTable,
        flags: PageTableFlags,
    ) -> Result<(), FlagUpdateError> {
        match self.page_size {
            PageBytesSize::Size4KiB => {
                let start = Page::<Size4KiB>::containing_address(self.virt_start);
                let end = Page::<Size4KiB>::containing_address(self.virt_end);
                let range = Page::range_inclusive(start, end);
                for page in range {
                    unsafe { mapper.update_flags(page, flags) }?.ignore();
                }
            }
            PageBytesSize::Size2MiB => {
                let start = Page::<Size2MiB>::containing_address(self.virt_start);
                let end = Page::<Size2MiB>::containing_address(self.virt_end);
                let range = Page::range_inclusive(start, end);
                for page in range {
                    unsafe { mapper.update_flags(page, flags) }?.ignore();
                }
            }
            PageBytesSize::Size1GiB => {
                let start = Page::<Size1GiB>::containing_address(self.virt_start);
                let end = Page::<Size1GiB>::containing_address(self.virt_end);
                let range = Page::range_inclusive(start, end);
                for page in range {
                    unsafe { mapper.update_flags(page, flags) }?.ignore();
                }
            }
        }

        Ok(())
    }
}

impl fmt::Display for Mapping {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
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
        let write = if self.flags.contains(PageTableFlags::WRITABLE) {
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
            "{:p}-{:p} -> {:#011x}-{:#011x} {:5} x {} page = {:7} {}{}{}{}{}{}",
            self.virt_start,
            self.virt_end,
            self.phys_start,
            self.phys_end,
            self.page_count,
            Bytes::from_u64(self.page_size.size()),
            Bytes::from_u64(self.page_count * self.page_size.size()),
            global,
            user,
            read,
            write,
            execute,
            self.purpose
        )
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use alloc::format;

    #[test]
    fn test_display_mapping() {
        let offset = VirtAddr::new(0x123451000);
        let phys_start = PhysAddr::new(0x8000);
        let virt_start = offset + phys_start.as_u64();
        let page_size = PageBytesSize::Size2MiB;
        let flags = PageTableFlags::NO_EXECUTE;
        let map = Mapping::new(virt_start, phys_start, page_size, flags);
        let map = Mapping::with_purpose(map, PagePurpose::Mmio);
        assert_eq!(format!("{}", map), "0x123459000-0x123658fff -> 0x000008000-0x000207fff     1 x 2 MiB page =   2 MiB ----- (MMIO)");

        let offset = VirtAddr::new(0x1000);
        let phys_start = PhysAddr::new(0x2000);
        let virt_start = offset + phys_start.as_u64();
        let page_size = PageBytesSize::Size4KiB;
        let flags = PageTableFlags::all() - PageTableFlags::NO_EXECUTE;
        let map = Mapping::new(virt_start, phys_start, page_size, flags);
        let map = Mapping::with_purpose(map, PagePurpose::Userspace);
        assert_eq!(format!("{}", map), "0x3000-0x3fff -> 0x000002000-0x000002fff     1 x 4 KiB page =   4 KiB gurwx (userspace)");
    }
}
