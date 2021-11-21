//! boot_info provides a basic frame allocator, using information from
//! the bootloader's memory map.

use bootloader::bootinfo::{MemoryMap, MemoryRegion, MemoryRegionType};
use core::slice::Iter;
use x86_64::structures::paging::{FrameAllocator, PhysFrame, Size4KiB};
use x86_64::PhysAddr;

/// BootInfoFrameAllocator is a FrameAllocator that returns
/// usable frames from the bootloader's memory map.
///
pub struct BootInfoFrameAllocator {
    memory_map: &'static MemoryMap,
    next: usize,
}

impl BootInfoFrameAllocator {
    /// new creates a FrameAllocator from the passed memory map.
    ///
    /// # Safety
    ///
    /// This function is unsafe because the caller must guarantee
    /// that the passed memory map is valid. The main requirement
    /// is that all frames that are marked as USABLE in it are
    /// really unused.
    ///
    pub unsafe fn new(memory_map: &'static MemoryMap) -> Self {
        BootInfoFrameAllocator {
            memory_map,
            next: 0,
        }
    }

    /// usable_frames returns an iterator over the usable frames
    /// specified in the memory map.
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

    /// underlying_map returns the underlying memory map. This is
    /// pub(crate) so it can be called by BitmapFrameAllocator when
    /// it takes over from BootInfoFrameallocator.
    ///
    pub(crate) fn underlying_map(&self) -> Iter<MemoryRegion> {
        self.memory_map.iter()
    }

    /// used_frames returns an iterator of frames that have already
    /// been allocated. This is pub(crate) so it can be called by
    /// BitmapFrameAllocator when it takes over from BootInfoFrameallocator.
    ///
    pub(crate) fn used_frames(&self) -> impl Iterator<Item = PhysFrame> + '_ {
        self.usable_frames().take(self.next).into_iter()
    }
}

unsafe impl FrameAllocator<Size4KiB> for BootInfoFrameAllocator {
    fn allocate_frame(&mut self) -> Option<PhysFrame> {
        let frame = self.usable_frames().nth(self.next);
        self.next += 1;
        frame
    }
}
