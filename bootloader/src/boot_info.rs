// Forked from bootloader 0.9.22, copyright 2018 Philipp Oppermann.
//
// Use of the original source code is governed by the MIT
// license that can be found in the LICENSE.orig file.
//
// Subsequent work copyright 2022 The Firefly Authors.
//
// Use of new and modified source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

use core::slice;

use bootinfo::{E820MemoryRegion, MemoryMap, MemoryRegion, MemoryRegionType};
use usize_conversions::usize_from;
use x86_64::VirtAddr;

pub(crate) fn create_from(memory_map_addr: VirtAddr, entry_count: u64) -> MemoryMap {
    let memory_map_start_ptr = usize_from(memory_map_addr.as_u64()) as *const E820MemoryRegion;
    let e820_memory_map =
        unsafe { slice::from_raw_parts(memory_map_start_ptr, usize_from(entry_count)) };

    let mut memory_map = MemoryMap::new();
    for region in e820_memory_map {
        memory_map.add_region(MemoryRegion::from(*region));
    }

    memory_map.sort();

    let mut iter = memory_map.iter_mut().peekable();
    while let Some(region) = iter.next() {
        if let Some(next) = iter.peek() {
            if region.range.end_frame_number > next.range.start_frame_number
                && region.region_type == MemoryRegionType::Usable
            {
                region.range.end_frame_number = next.range.start_frame_number;
            }
        }
    }

    memory_map
}
