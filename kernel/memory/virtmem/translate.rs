// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Translates virtual addresses to physical addresses.

use alloc::vec::Vec;
use memory::{PageMapping, PageTable, PhysAddr, VirtAddr, VirtPageSize};
use x86_64::instructions::interrupts::without_interrupts;
use x86_64::registers::control::Cr3;

/// Describes a single contiguous physical memory region.
///
#[derive(Clone, Copy, Debug, PartialEq, PartialOrd)]
pub struct PhysBuffer {
    pub addr: PhysAddr,
    pub len: usize,
}

/// Translates a contiguous virtual memory region into one
/// or more contiguous physical memory regions.
///
/// If any part of the virtual memory region is not mapped
/// in the given page table, then None is returned.
///
pub fn virt_to_phys_addrs(addr: VirtAddr, len: usize) -> Option<Vec<PhysBuffer>> {
    // Get the current page table.
    // This is similar to the code in kernel_pml4,
    // but will use whatever is the current page
    // table.
    //
    // This is safe, even though we use unsafe, as
    // we disable interrupts while the page tables
    // are in use and we only read, not write them.
    // This means they can't change underneath us
    // and there's no thread-safety risk.
    //
    // We also pre-allocate the vector we pass in,
    // so no allocations should need to happen.
    let (level_4_table_frame, _) = Cr3::read();
    let phys = PhysAddr::from_x86_64(level_4_table_frame.start_address());
    let page_table = unsafe { PageTable::at(phys).expect("invalid page table address") };

    // Work out the max number of buffers we could
    // get back, which is the situation where we
    // straddle a page boundary.
    let max_pages = (len / VirtPageSize::Size4KiB.bytes()) + 1;
    let buf = Vec::with_capacity(max_pages);

    without_interrupts(|| _virt_to_phys_addrs(&page_table, buf, addr, len))
}

/// This is the underlyign implementation for mapping
/// virtual addresses to physical buffers. We abstract
/// away the translator type to make testing easier.
///
fn _virt_to_phys_addrs(
    page_table: &PageTable,
    mut bufs: Vec<PhysBuffer>,
    virt_addr: VirtAddr,
    len: usize,
) -> Option<Vec<PhysBuffer>> {
    // We will allow an address with length zero
    // as a special case for a single address.
    if len == 0 {
        match page_table.translate_addr(virt_addr) {
            None => return None,
            Some(addr) => {
                bufs.push(PhysBuffer { addr, len });
                return Some(bufs);
            }
        }
    }

    // Now we pass through the buffer until we
    // have translated all of it.
    let mut virt_addr = virt_addr;
    let mut len = len;
    while len > 0 {
        match page_table.translate(virt_addr) {
            PageMapping::NotMapped => return None,
            PageMapping::InvalidPageTableAddr(_) => return None,
            PageMapping::InvalidLevel4PageTable => return None,
            PageMapping::Mapping { frame, addr, .. } => {
                // Advance the buffer by the amount of
                // physical memory we just found.
                let offset = addr - frame.start_address();
                let remaining = frame.size().bytes() - offset;

                bufs.push(PhysBuffer {
                    addr,
                    len: core::cmp::min(len, remaining),
                });
                virt_addr += remaining;
                len = len.saturating_sub(remaining);
            }
        }
    }

    // TODO(#10): Merge contiguous regions to reduce the number of buffers we return.

    Some(bufs)
}

#[cfg(test)]
mod test {
    extern crate std;
    use super::*;
    use memory::{PageTableFlags, PhysFrame, PhysFrameAllocator, PhysFrameSize, VirtPage};
    use std::boxed::Box;
    use std::vec;
    use std::vec::Vec;

    // This includes a byte array the same size as
    // a page table, aligned to frame boundaries.
    //
    // This can be allocated on the heap and should
    // have the correct alignment, allowing us to
    // use them as page tables with a physical
    // memory offset of 0.
    //
    #[derive(Clone)]
    #[repr(C)]
    #[repr(align(4096))]
    struct FakePageTable {
        entries: [u8; PhysFrameSize::Size4KiB.bytes()],
    }

    impl FakePageTable {
        fn new() -> Self {
            FakePageTable {
                entries: [0u8; PhysFrameSize::Size4KiB.bytes()],
            }
        }
    }

    // This is a "physical frame allocator" that
    // returns a virtual memory buffer that is not
    // otherwise in use. This means we can use it
    // to test page mapping with fake page tables
    // in userspace.
    //
    struct FakePhysFrameAllocator {
        buffers: Vec<Box<FakePageTable>>,
    }

    impl FakePhysFrameAllocator {
        fn new() -> Self {
            FakePhysFrameAllocator {
                buffers: Vec::new(),
            }
        }

        fn allocate(&mut self) -> PhysAddr {
            let next = Box::new(FakePageTable::new());
            let addr = PhysAddr::new(next.as_ref() as *const FakePageTable as usize);
            self.buffers.push(next);

            addr
        }
    }

    unsafe impl PhysFrameAllocator for FakePhysFrameAllocator {
        fn allocate_phys_frame(&mut self, size: PhysFrameSize) -> Option<PhysFrame> {
            if size != PhysFrameSize::Size4KiB {
                None
            } else {
                let addr = self.allocate();
                let frame = PhysFrame::from_start_address(addr, size)
                    .expect("got unaligned fake page table");
                Some(frame)
            }
        }
    }

    #[test]
    fn virt_to_phys_addrs() {
        // Start by making some mappings we can use.
        // We map as follows:
        // - page 1 => frame 3
        // - page 2 => frame 1
        // - page 3 => frame 2
        let size = PhysFrameSize::Size4KiB;
        let page1 = VirtAddr::new(1 * size.bytes());
        let page2 = VirtAddr::new(2 * size.bytes());
        let page3 = VirtAddr::new(3 * size.bytes());
        let frame1 = PhysAddr::new(1 * size.bytes());
        let frame2 = PhysAddr::new(2 * size.bytes());
        let frame3 = PhysAddr::new(3 * size.bytes());

        // We pretend that we're using physical memory by using
        // an offset of 0.
        let offset = VirtAddr::zero();

        // Make the level-4 page table.
        let mut allocator = FakePhysFrameAllocator::new();
        let pml4 = allocator
            .allocate_phys_frame(PhysFrameSize::Size4KiB)
            .unwrap();
        let pml4_addr = pml4.start_address();
        let mut page_table = unsafe { PageTable::at_offset(pml4_addr, offset) };

        fn virt_page(addr: VirtAddr) -> VirtPage {
            let size = VirtPageSize::Size4KiB;
            let page = VirtPage::from_start_address(addr, size);
            page.unwrap()
        }

        fn phys_frame(addr: PhysAddr) -> PhysFrame {
            let size = PhysFrameSize::Size4KiB;
            let frame = PhysFrame::from_start_address(addr, size);
            frame.unwrap()
        }

        unsafe {
            let flags = PageTableFlags::PRESENT;
            page_table
                .map(virt_page(page1), phys_frame(frame3), flags, &mut allocator)
                .unwrap()
                .ignore();
            page_table
                .map(virt_page(page2), phys_frame(frame1), flags, &mut allocator)
                .unwrap()
                .ignore();
            page_table
                .map(virt_page(page3), phys_frame(frame2), flags, &mut allocator)
                .unwrap()
                .ignore();
        }

        // Simple example: single address.
        let buf = Vec::with_capacity(5);
        assert_eq!(
            _virt_to_phys_addrs(&page_table, buf, page1, 0),
            Some(vec![PhysBuffer {
                addr: frame3,
                len: 0
            }])
        );

        // Simple example: within a single page.
        let buf = Vec::with_capacity(5);
        assert_eq!(
            _virt_to_phys_addrs(&page_table, buf.to_vec(), page1 + 2, 2),
            Some(vec![PhysBuffer {
                addr: frame3 + 2,
                len: 2
            }])
        );

        // Crossing a split page boundary.
        let buf = Vec::with_capacity(5);
        assert_eq!(
            _virt_to_phys_addrs(&page_table, buf.to_vec(), page1 + 4090, 12),
            Some(vec![
                PhysBuffer {
                    addr: frame3 + 4090,
                    len: 6
                },
                PhysBuffer {
                    addr: frame1,
                    len: 6
                }
            ])
        );

        // Crossing a contiguous page boundary.
        // TODO: merge contiguous regions to reduce the number of buffers we return.
        let buf = Vec::with_capacity(5);
        assert_eq!(
            _virt_to_phys_addrs(&page_table, buf.to_vec(), page2 + 4090, 12),
            Some(vec![
                PhysBuffer {
                    addr: frame1 + 4090,
                    len: 6
                },
                PhysBuffer {
                    addr: frame2,
                    len: 6
                }
            ])
        );
    }
}
