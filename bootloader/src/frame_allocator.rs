// Forked from bootloader 0.9.22, copyright 2018 Philipp Oppermann.
//
// Use of the original source code is governed by the MIT
// license that can be found in the LICENSE.orig file.
//
// Subsequent work copyright 2022 The Firefly Authors.
//
// Use of new and modified source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

use super::{frame_range, phys_frame_range};
use bootinfo::{MemoryMap, MemoryRegion, MemoryRegionType};
use core::cmp::Ordering;
use x86_64::structures::paging::{frame::PhysFrameRange, PhysFrame};

pub(crate) struct FrameAllocator<'map> {
    pub memory_map: &'map mut MemoryMap,
}

impl<'map> FrameAllocator<'map> {
    pub(crate) fn allocate_frame(&mut self, region_type: MemoryRegionType) -> Option<PhysFrame> {
        // try to find an existing region of same type that can be enlarged
        let mut iter = self.memory_map.iter_mut().peekable();
        while let Some(region) = iter.next() {
            if region.region_type == region_type {
                if let Some(next) = iter.peek() {
                    if next.range.start_frame_number == region.range.end_frame_number
                        && next.region_type == MemoryRegionType::Usable
                        && !next.range.is_empty()
                    {
                        let frame = phys_frame_range(region.range).end;
                        region.range.end_frame_number += 1;
                        iter.next().unwrap().range.start_frame_number += 1;
                        return Some(frame);
                    }
                }
            }
        }

        fn split_usable_region<'map, I>(iter: &mut I) -> Option<(PhysFrame, PhysFrameRange)>
        where
            I: Iterator<Item = &'map mut MemoryRegion>,
        {
            for region in iter {
                if region.region_type != MemoryRegionType::Usable {
                    continue;
                }
                if region.range.is_empty() {
                    continue;
                }

                let frame = phys_frame_range(region.range).start;
                region.range.start_frame_number += 1;
                return Some((frame, PhysFrame::range(frame, frame + 1)));
            }
            None
        }

        let result = if region_type == MemoryRegionType::PageTable {
            // prevent fragmentation when page tables are allocated in between
            split_usable_region(&mut self.memory_map.iter_mut().rev())
        } else {
            split_usable_region(&mut self.memory_map.iter_mut())
        };

        if let Some((frame, range)) = result {
            self.memory_map.add_region(MemoryRegion {
                range: frame_range(range),
                region_type,
            });
            Some(frame)
        } else {
            None
        }
    }

    /// Marks the passed region in the memory map.
    ///
    /// Panics if a non-usable region (e.g. a reserved region) overlaps with the passed region.
    pub(crate) fn mark_allocated_region(&mut self, region: MemoryRegion) {
        for r in self.memory_map.iter_mut() {
            if region.range.start_frame_number >= r.range.end_frame_number {
                continue;
            }
            if region.range.end_frame_number <= r.range.start_frame_number {
                continue;
            }

            if r.region_type != MemoryRegionType::Usable {
                panic!(
                    "region {:x?} overlaps with non-usable region {:x?}",
                    region, r
                );
            }

            match region
                .range
                .start_frame_number
                .cmp(&r.range.start_frame_number)
            {
                Ordering::Equal => {
                    if region.range.end_frame_number < r.range.end_frame_number {
                        // Case: (r = `r`, R = `region`)
                        // ----rrrrrrrrrrr----
                        // ----RRRR-----------
                        r.range.start_frame_number = region.range.end_frame_number;
                        self.memory_map.add_region(region);
                    } else {
                        // Case: (r = `r`, R = `region`)
                        // ----rrrrrrrrrrr----
                        // ----RRRRRRRRRRRRRR-
                        *r = region;
                    }
                }
                Ordering::Greater => {
                    if region.range.end_frame_number < r.range.end_frame_number {
                        // Case: (r = `r`, R = `region`)
                        // ----rrrrrrrrrrr----
                        // ------RRRR---------
                        let mut behind_r = *r;
                        behind_r.range.start_frame_number = region.range.end_frame_number;
                        r.range.end_frame_number = region.range.start_frame_number;
                        self.memory_map.add_region(behind_r);
                        self.memory_map.add_region(region);
                    } else {
                        // Case: (r = `r`, R = `region`)
                        // ----rrrrrrrrrrr----
                        // -----------RRRR---- or
                        // -------------RRRR--
                        r.range.end_frame_number = region.range.start_frame_number;
                        self.memory_map.add_region(region);
                    }
                }
                Ordering::Less => {
                    // Case: (r = `r`, R = `region`)
                    // ----rrrrrrrrrrr----
                    // --RRRR-------------
                    r.range.start_frame_number = region.range.end_frame_number;
                    self.memory_map.add_region(region);
                }
            }
            return;
        }
        panic!("region {:x?} is not a usable memory region", region);
    }
}
