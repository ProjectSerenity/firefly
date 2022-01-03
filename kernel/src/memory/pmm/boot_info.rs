// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides a basic frame allocator, using information from
//! the bootloader's memory map.

use bootloader::bootinfo::{MemoryMap, MemoryRegion, MemoryRegionType};
use core::slice::Iter;
use x86_64::structures::paging::{FrameAllocator, PhysFrame, Size4KiB};
use x86_64::PhysAddr;

/// A basic physical memory allocator.
///
/// `BootInfoFrameAllocator` is a simple FrameAllocator that
/// returns usable frames from the bootloader's memory map. It
/// cannot de-allocate frames.
///
pub struct BootInfoFrameAllocator {
    memory_map: &'static MemoryMap,
    next: usize,
}

impl BootInfoFrameAllocator {
    /// Creates a `BootInfoFrameAllocator` from the passed memory map.
    ///
    /// # Safety
    ///
    /// This function is unsafe because the caller must guarantee that the
    /// memory map is valid and complete. All frames that are marked as `USABLE`
    /// in the memory map must be unused.
    ///
    pub unsafe fn new(memory_map: &'static MemoryMap) -> Self {
        BootInfoFrameAllocator {
            memory_map,
            next: 0,
        }
    }

    /// Returns an iterator over the usable frames specified in the
    /// memory map.
    ///
    fn usable_frames(&self) -> impl Iterator<Item = PhysFrame> {
        // Get usable regions from memory map.
        let regions = self.memory_map.iter();
        let usable_regions = regions.filter(|r| r.region_type == MemoryRegionType::Usable);

        // Map each region to its address range.
        let addr_ranges = usable_regions.map(|r| r.range.start_addr()..r.range.end_addr());

        // Transform to an iterator of frame start addresses.
        let frame_addresses = addr_ranges.flat_map(|r| r.step_by(4096));

        // Create PhysFrame types from the start addresses.
        frame_addresses.map(|addr| PhysFrame::containing_address(PhysAddr::new(addr)))
    }

    /// Returns the underlying memory map.
    ///
    /// This is `pub(super)` so it can be called by [`BitmapFrameAllocator`](super::BitmapFrameAllocator)
    /// when it takes over from `BootInfoFrameallocator`.
    ///
    pub(super) fn underlying_map(&self) -> Iter<MemoryRegion> {
        self.memory_map.iter()
    }

    /// Returns an iterator of frames that have already been allocated.
    ///
    /// This is `pub(super)` so it can be called by [`BitmapFrameAllocator`](super::BitmapFrameAllocator)
    /// when it takes over from `BootInfoFrameallocator`.
    ///
    pub(super) fn used_frames(&self) -> impl Iterator<Item = PhysFrame> + '_ {
        self.usable_frames().take(self.next).into_iter()
    }
}

unsafe impl FrameAllocator<Size4KiB> for BootInfoFrameAllocator {
    /// Returns the next available physical frame, or `None`.
    ///
    fn allocate_frame(&mut self) -> Option<PhysFrame> {
        let frame = self.usable_frames().nth(self.next);
        self.next += 1;
        frame
    }
}
