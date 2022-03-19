// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Translates virtual addresses to physical addresses.

use alloc::vec::Vec;
use memlayout::{phys_to_virt_addr, PHYSICAL_MEMORY_OFFSET};
use x86_64::instructions::interrupts::without_interrupts;
use x86_64::registers::control::Cr3;
use x86_64::structures::paging::mapper::{MappedFrame, TranslateResult};
use x86_64::structures::paging::{OffsetPageTable, PageSize, PageTable, Size4KiB, Translate};
use x86_64::{PhysAddr, VirtAddr};

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
    let phys = level_4_table_frame.start_address();
    let virt = phys_to_virt_addr(phys);
    let page_table_ptr: *mut PageTable = virt.as_mut_ptr();
    let page_table = unsafe { &mut *page_table_ptr };
    let pml4 = unsafe { OffsetPageTable::new(page_table, PHYSICAL_MEMORY_OFFSET) };

    // Work out the max number of buffers we could
    // get back, which is the situation where we
    // straddle a page boundary.
    let max_pages = (len / (Size4KiB::SIZE as usize)) + 1;
    let buf = Vec::with_capacity(max_pages);

    without_interrupts(|| _virt_to_phys_addrs(&pml4, buf, addr, len))
}

/// This is the underlyign implementation for mapping
/// virtual addresses to physical buffers. We abstract
/// away the translator type to make testing easier.
///
fn _virt_to_phys_addrs<T: Translate>(
    page_table: &T,
    mut bufs: Vec<PhysBuffer>,
    addr: VirtAddr,
    len: usize,
) -> Option<Vec<PhysBuffer>> {
    // We will allow an address with length zero
    // as a special case for a single address.
    if len == 0 {
        match page_table.translate_addr(addr) {
            None => return None,
            Some(addr) => {
                bufs.push(PhysBuffer { addr, len });
                return Some(bufs);
            }
        }
    }

    // Now we pass through the buffer until we
    // have translated all of it.
    let mut addr = addr;
    let mut len = len;
    while len > 0 {
        match page_table.translate(addr) {
            TranslateResult::NotMapped => return None,
            TranslateResult::InvalidFrameAddress(_) => return None,
            TranslateResult::Mapped { frame, offset, .. } => {
                // Advance the buffer by the amount of
                // physical memory we just found.
                let found = (frame.size() - offset) as usize;
                let phys_addr = match frame {
                    MappedFrame::Size4KiB(frame) => frame.start_address() + offset,
                    MappedFrame::Size2MiB(frame) => frame.start_address() + offset,
                    MappedFrame::Size1GiB(frame) => frame.start_address() + offset,
                };

                bufs.push(PhysBuffer {
                    addr: phys_addr,
                    len: core::cmp::min(len, found),
                });
                addr += found;
                len = len.saturating_sub(found);
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
    use align::align_down_u64;
    use alloc::collections::BTreeMap;
    use alloc::vec;
    use x86_64::structures::paging::{PageTableFlags, PhysFrame};

    /// DebugPageTable is a helper type for testing code that
    /// uses page tables. It emulates the behaviour for a level
    /// 4 page table using heap memory, without modifying the
    /// system page tables.
    ///
    pub struct DebugPageTable {
        mappings: BTreeMap<VirtAddr, PhysFrame>,
    }

    impl DebugPageTable {
        pub fn new() -> Self {
            DebugPageTable {
                mappings: BTreeMap::new(),
            }
        }

        pub fn map(&mut self, addr: VirtAddr, frame: PhysFrame) {
            // Check the virtual address is at a page boundary,
            // to simplify things.
            assert_eq!(addr.as_u64(), align_down_u64(addr.as_u64(), Size4KiB::SIZE));

            self.mappings.insert(addr, frame);
        }
    }

    impl Translate for DebugPageTable {
        fn translate(&self, addr: VirtAddr) -> TranslateResult {
            let truncated = VirtAddr::new(align_down_u64(addr.as_u64(), Size4KiB::SIZE));
            match self.mappings.get(&truncated) {
                None => return TranslateResult::NotMapped,
                Some(frame) => TranslateResult::Mapped {
                    frame: MappedFrame::Size4KiB(*frame),
                    offset: addr - truncated,
                    flags: PageTableFlags::PRESENT,
                },
            }
        }
    }

    #[test]
    fn debug_page_table() {
        // Check that the debug page table works
        // correctly.
        let mut page_table = DebugPageTable::new();
        fn phys_frame(addr: u64) -> PhysFrame {
            let addr = PhysAddr::new(addr);
            let frame = PhysFrame::from_start_address(addr);
            frame.unwrap()
        }

        assert_eq!(page_table.translate_addr(VirtAddr::new(4096)), None);
        page_table.map(VirtAddr::new(4096), phys_frame(4096));
        assert_eq!(
            page_table.translate_addr(VirtAddr::new(4096)),
            Some(PhysAddr::new(4096))
        );
        assert_eq!(
            page_table.translate_addr(VirtAddr::new(4097)),
            Some(PhysAddr::new(4097))
        );
    }

    #[test]
    fn virt_to_phys_addrs() {
        // Start by making some mappings we can use.
        // We map as follows:
        // - page 1 => frame 3
        // - page 2 => frame 1
        // - page 3 => frame 2
        let page1 = VirtAddr::new(1 * Size4KiB::SIZE);
        let page2 = VirtAddr::new(2 * Size4KiB::SIZE);
        let page3 = VirtAddr::new(3 * Size4KiB::SIZE);
        let frame1 = PhysAddr::new(1 * Size4KiB::SIZE);
        let frame2 = PhysAddr::new(2 * Size4KiB::SIZE);
        let frame3 = PhysAddr::new(3 * Size4KiB::SIZE);

        let mut page_table = DebugPageTable::new();
        fn phys_frame(addr: PhysAddr) -> PhysFrame {
            let frame = PhysFrame::from_start_address(addr);
            frame.unwrap()
        }

        page_table.map(page1, phys_frame(frame3));
        page_table.map(page2, phys_frame(frame1));
        page_table.map(page3, phys_frame(frame2));

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
            _virt_to_phys_addrs(&page_table, buf.to_vec(), page1 + 2u64, 2),
            Some(vec![PhysBuffer {
                addr: frame3 + 2u64,
                len: 2
            }])
        );

        // Crossing a split page boundary.
        let buf = Vec::with_capacity(5);
        assert_eq!(
            _virt_to_phys_addrs(&page_table, buf.to_vec(), page1 + 4090u64, 12),
            Some(vec![
                PhysBuffer {
                    addr: frame3 + 4090u64,
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
            _virt_to_phys_addrs(&page_table, buf.to_vec(), page2 + 4090u64, 12),
            Some(vec![
                PhysBuffer {
                    addr: frame1 + 4090u64,
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
