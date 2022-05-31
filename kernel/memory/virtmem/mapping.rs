// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! mapping includes helper functionality for mapping out virtual memory, based
//! on the contents of a level 4 page table.

use alloc::vec::Vec;
use core::fmt;
use core::slice;
use loader::Binary;
use memory::constants::{
    BOOT_INFO, CPU_LOCAL, KERNEL_BINARY, KERNEL_HEAP, KERNEL_STACK, KERNEL_STACK_GUARD, MMIO_SPACE,
    NULL_PAGE, PHYSICAL_MEMORY, USERSPACE,
};
use memory::{
    PageTable, PageTableFlags, PageUnmappingError, PhysAddr, PhysFrame, PhysFrameRange,
    PhysFrameSize, VirtAddr, VirtAddrRange, VirtPage, VirtPageRange, VirtPageSize,
};
use pretty::Bytes;
use x86_64::instructions::tlb;

/// Ensures all page mappings meet our expectations.
///
/// This includes ensuring that the kernel is mapped
/// with the appropriate access controls and unmaps
/// any unknown mappings left over by the bootloader.
///
pub fn remap_kernel(page_table: &mut PageTable) {
    // Analyse and iterate through the page mappings
    // in the PML4.
    //
    // Rather than constantly flushing the TLB as we
    // go along, we do one big flush at the end.
    let mappings = level_4_table(page_table);
    for mapping in mappings.iter() {
        match mapping.purpose {
            // Unmap pages we no longer need.
            PagePurpose::Unknown
            | PagePurpose::NullPage
            | PagePurpose::Userspace
            | PagePurpose::KernelStackGuard
            | PagePurpose::BootInfo => {
                mapping.unmap(page_table).expect("failed to unmap page");
            }
            _ => {}
        }
    }

    // Flush the TLB.
    tlb::flush_all();
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

    // Sign-extend if necessary.
    let mut addr = l4 | l3 | l2 | l1;
    if addr >= 0x0000_8000_0000_0000 {
        addr |= 0xffff_8000_0000_0000;
    }

    VirtAddr::new(addr)
}

/// level_4_table iterates through a level 4 page table,
/// returning the sequence of contiguous mappings.
///
pub fn level_4_table(pml4: &PageTable) -> Vec<Mapping> {
    let mut mappings = Vec::new();
    let mut current: Option<Mapping> = None;
    for (i, pml4e) in pml4.iter().enumerate() {
        if !pml4e.is_present() {
            continue;
        }

        if pml4e.flags().contains(PageTableFlags::HUGE_PAGE) {
            panic!("invalid huge PML4 page");
        }

        let pdpt = unsafe { PageTable::at(pml4e.addr()).unwrap() };
        for (j, pdpe) in pdpt.iter().enumerate() {
            if !pdpe.is_present() {
                continue;
            }

            if pdpe.flags().contains(PageTableFlags::HUGE_PAGE) {
                let next = Mapping::new(
                    indices_to_addr(i, j, 0, 0),
                    pdpe.addr(),
                    VirtPageSize::Size1GiB,
                    PhysFrameSize::Size1GiB,
                    pdpe.flags(),
                );
                current = Mapping::combine(&mut mappings, current, next);
                continue;
            }

            let pdt = unsafe { PageTable::at(pdpe.addr()).unwrap() };
            for (k, pde) in pdt.iter().enumerate() {
                if !pde.is_present() {
                    continue;
                }

                if pde.flags().contains(PageTableFlags::HUGE_PAGE) {
                    let next = Mapping::new(
                        indices_to_addr(i, j, k, 0),
                        pde.addr(),
                        VirtPageSize::Size2MiB,
                        PhysFrameSize::Size2MiB,
                        pde.flags(),
                    );
                    current = Mapping::combine(&mut mappings, current, next);
                    continue;
                }

                let pt = unsafe { PageTable::at(pde.addr()).unwrap() };
                for (l, page) in pt.iter().enumerate() {
                    if !page.is_present() {
                        continue;
                    }

                    if page.flags().contains(PageTableFlags::HUGE_PAGE) {
                        panic!("invalid huge PML1 page");
                    }

                    let next = Mapping::new(
                        indices_to_addr(i, j, k, l),
                        page.addr(),
                        VirtPageSize::Size4KiB,
                        PhysFrameSize::Size4KiB,
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
    //
    // We load the kernel binary from memory, using the
    // flags for each segment to derive its purpose.

    let kernel = unsafe {
        slice::from_raw_parts(
            KERNEL_BINARY.start().as_usize() as *const u8,
            KERNEL_BINARY.size(),
        )
    };
    let kernel_binary =
        Binary::parse_kernel("kernel.bin", kernel).expect("failed to parse kernel binary");

    let kernel_code = PageTableFlags::PRESENT; // r-x
    let kernel_constants = PageTableFlags::PRESENT | PageTableFlags::NO_EXECUTE; // r--
    let kernel_statics =
        PageTableFlags::PRESENT | PageTableFlags::WRITABLE | PageTableFlags::NO_EXECUTE; // rw-

    let mut out = Vec::with_capacity(mappings.len());
    for map in mappings {
        let range = VirtAddrRange::new(map.pages.start_address(), map.pages.end_address());
        let purpose = if NULL_PAGE.contains(&range) {
            PagePurpose::NullPage
        } else if USERSPACE.contains(&range) {
            PagePurpose::Userspace
        } else if KERNEL_BINARY.contains(&range) {
            // Find the kernel binary program segment
            // containing this chunk of memory.
            //
            // When we match page mappings to segments,
            // we round out the segment bounds to the
            // page bounds, as segments cannot share a
            // page with another segment with different
            // flags.
            let segment = kernel_binary
                .iter_segments()
                .find(|&s| {
                    let size = VirtPageSize::Size4KiB.bytes();
                    s.start.align_down(size) <= range.start() && range.end() < s.end.align_up(size)
                })
                .expect("mapping in kernel binary does not exist in any segment");

            // Determine the page purpose by its flags.
            if segment.flags == kernel_code {
                PagePurpose::KernelCode
            } else if segment.flags == kernel_constants {
                PagePurpose::KernelConstants
            } else if segment.flags == kernel_statics {
                PagePurpose::KernelStatics
            } else {
                panic!(
                    "kernel binary segment has unexpected flags {:?}",
                    segment.flags
                );
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

/// PagePurpose describes the known use of a contiguous
/// set of mapped pages.
///
#[derive(Clone, Copy, Debug, PartialEq)]
pub enum PagePurpose {
    Unknown,
    NullPage,
    Userspace,
    BootInfo,
    KernelCode,
    KernelConstants,
    KernelStatics,
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
            PagePurpose::KernelStatics => write!(f, " (kernel statics)"),
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
    pub pages: VirtPageRange,
    pub page_size: VirtPageSize,
    pub frames: PhysFrameRange,
    pub frame_size: PhysFrameSize,
    pub flags: PageTableFlags,
    pub purpose: PagePurpose,
}

impl Mapping {
    pub fn new(
        virt_start: VirtAddr,
        phys_start: PhysAddr,
        page_size: VirtPageSize,
        frame_size: PhysFrameSize,
        flags: PageTableFlags,
    ) -> Self {
        let flags_mask = PageTableFlags::PRESENT
            | PageTableFlags::WRITABLE
            | PageTableFlags::USER_ACCESSIBLE
            | PageTableFlags::GLOBAL
            | PageTableFlags::NO_EXECUTE;

        let page = VirtPage::containing_address(virt_start, page_size);
        let frame = PhysFrame::containing_address(phys_start, frame_size);
        Mapping {
            pages: VirtPage::range_inclusive(page, page),
            page_size,
            frames: PhysFrame::range_inclusive(frame, frame),
            frame_size,
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
                if current.extends(&next) {
                    current.pages =
                        VirtPage::range_inclusive(current.pages.start(), next.pages.end());
                    current.frames =
                        PhysFrame::range_inclusive(current.frames.start(), next.frames.end());

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

    /// Returns whether `other` is a logical continuation
    /// of the current mapping. This is only true if the
    /// two mappings form contiguous virtual and physical
    /// memory.
    ///
    fn extends(&self, other: &Mapping) -> bool {
        // If the page sizes or frame sizes don't
        // match, then it's a simple `false`.
        if self.page_size != other.page_size || self.frame_size != other.frame_size {
            return false;
        }

        // Next, we check whether the two mappings
        // form a contiguous sequence of virtual
        // memory. This is less easy than it might
        // seem, as we have to be careful not to
        // pass non-canonical addresses to `VirtAddr::new`.
        //
        // First, we check that the addition does
        // not overflow, which would cause problems
        // before we even reach the point of calling
        // `VirtAddr::new`.
        let last_virt = self.pages.end_address().as_usize();
        if last_virt.checked_add(1).is_none() {
            return false;
        }

        // Now we can check that the next address is
        // valid and then check it matches the first
        // address in the next mapping.
        match VirtAddr::try_new(last_virt + 1) {
            Err(_) => return false,
            Ok(next_virt) => {
                if next_virt != other.pages.start_address() {
                    return false;
                }
            }
        }

        // Now we do the same for the physical
        // addresses. This is slightly simpler, as
        // we don't need to worry about a chunk of
        // invalid addresses within the address
        // space like we do with virtual memory,
        // but we might as well take the same robust
        // approach.
        let last_phys = self.frames.end_address().as_usize();
        if last_phys.checked_add(1).is_none() {
            return false;
        }

        // As before, we check that the next address
        // is valid and matches the first address in
        // the next mapping.
        match PhysAddr::try_new(last_phys + 1) {
            Err(_) => return false,
            Ok(next_phys) => {
                if next_phys != other.frames.start_address() {
                    return false;
                }
            }
        }

        true
    }

    /// unmap uses the given page table to unmap the
    /// entire mapping.
    ///
    /// Note that unmap will not flush the TLB.
    ///
    pub fn unmap(&self, page_table: &mut PageTable) -> Result<(), PageUnmappingError> {
        let range = self.pages;
        for page in range {
            unsafe { page_table.unmap(page)?.1.ignore() };
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
            self.pages.start_address(),
            self.pages.end_address(),
            self.frames.start_address(),
            self.frames.end_address(),
            (self.pages.end_address() - self.pages.start_address() + 1) / self.page_size.bytes(),
            Bytes::from_usize(self.page_size.bytes()),
            Bytes::from_usize(self.pages.end_address() - self.pages.start_address() + 1),
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
        let phys_start = PhysAddr::new(0x800000);
        let virt_start = offset + phys_start.as_usize();
        let page_size = VirtPageSize::Size2MiB;
        let frame_size = PhysFrameSize::Size2MiB;
        let flags = PageTableFlags::NO_EXECUTE;
        let map = Mapping::new(virt_start, phys_start, page_size, frame_size, flags);
        let map = Mapping::with_purpose(map, PagePurpose::Mmio);
        assert_eq!(format!("{}", map), "0x123c00000-0x123dfffff -> 0x000800000-0x0009fffff     1 x 2 MiB page =   2 MiB ----- (MMIO)");

        let offset = VirtAddr::new(0x1000);
        let phys_start = PhysAddr::new(0x2000);
        let virt_start = offset + phys_start.as_usize();
        let page_size = VirtPageSize::Size4KiB;
        let frame_size = PhysFrameSize::Size4KiB;
        let flags = PageTableFlags::all() - PageTableFlags::NO_EXECUTE;
        let map = Mapping::new(virt_start, phys_start, page_size, frame_size, flags);
        let map = Mapping::with_purpose(map, PagePurpose::Userspace);
        assert_eq!(format!("{}", map), "0x3000-0x3fff -> 0x000002000-0x000002fff     1 x 4 KiB page =   4 KiB gurwx (userspace)");
    }
}
