//! pmm manages physical memory.

// Physical memory frame allocation functionality.

use crate::memory::pmm::boot_info::BootInfoFrameAllocator;
use crate::Locked;
use bootloader::bootinfo::MemoryMap;
use lazy_static::lazy_static;
use x86_64::structures::paging::frame::PhysFrameRange;
use x86_64::structures::paging::{FrameAllocator, FrameDeallocator, PhysFrame, Size4KiB};

mod bitmap;
mod boot_info;

pub use crate::memory::pmm::bitmap::BitmapFrameAllocator;

lazy_static! {
    /// ALLOCATOR is the physical memory allocator. ALLOCATOR can be
    /// initialised by calling pmm::init, once the kernel's heap has
    /// been set up. To bootstrap the heap, use a BootInfoFrameAllocator,
    /// then pass that to pmm::init so ALLOCATOR can take over.
    ///
    pub static ref ALLOCATOR: Locked<BitmapFrameAllocator> = Locked::new(BitmapFrameAllocator::empty());
}

/// init sets up the physical memory manager, taking over
/// from the bootstrap BootInfoFrameAllocator.
///
pub unsafe fn init(bootstrap: BootInfoFrameAllocator) {
    let mut alloc = BitmapFrameAllocator::new(bootstrap.underlying_map());
    alloc.repossess(bootstrap);

    *ALLOCATOR.lock() = alloc;
}

/// allocate_n_frames returns n sequential physical frames,
/// or None.
///
pub fn allocate_n_frames(n: usize) -> Option<PhysFrameRange> {
    let mut allocator = ALLOCATOR.lock();
    allocator.allocate_n_frames(n)
}

/// allocate_frame returns the next available physical frame,
/// or None.
///
pub fn allocate_frame() -> Option<PhysFrame> {
    let mut allocator = ALLOCATOR.lock();
    allocator.allocate_frame()
}

/// deallocate_frame returns the given frame to the list of
/// free frames for later use.
///
/// # Safety
///
/// The caller must ensure that the given frame is unused.
///
pub unsafe fn deallocate_frame(frame: PhysFrame<Size4KiB>) {
    let mut allocator = ALLOCATOR.lock();
    allocator.deallocate_frame(frame);
}

/// debug prints debug information about the physical memory
/// manager.
///
pub fn debug() {
    let mm = ALLOCATOR.lock();
    mm.debug();
}

/// bootstrap returns an initial frame allocator, which can be
/// used to allocate the kernel's heap, so a more advanced
/// allocator can be initialised.
///
/// # Safety
///
/// This function is unsafe because the caller must guarantee
/// that the passed memory map is valid. The main requirement
/// is that all frames that are marked as USABLE in it are
/// really unused.
///
pub unsafe fn bootstrap(memory_map: &'static MemoryMap) -> BootInfoFrameAllocator {
    BootInfoFrameAllocator::new(memory_map)
}
