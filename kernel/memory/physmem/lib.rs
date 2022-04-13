// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Physical memory management and allocation.
//!
//! This module consists of two physical memory allocators:
//!
//! 1. [`BootInfoFrameAllocator`], which is used to initialise the kernel heap.
//! 2. [`BitmapFrameAllocator`], which takes over from the bootstrap allocator for subsequent use.
//!
//! The bootstrap allocator (constructed using [`bootstrap`])
//! uses the memory map provided by the bootloader to identify
//! a series of available physical memory frames and allocate
//! them sequentially. This is only intended for early use and
//! cannot de-allocate the frames it allocates.
//!
//! Once the kernel heap is initialised, we switch over to the
//! second-phase allocator in [`init`], which takes ownership
//! of the memory map from the bootstrap allocator, including
//! the frames it has already allocated. From that point onwards,
//! we only use the bitmap allocator, which can de-allocate pages.
//!
//! ## Helper functions
//!
//! While the bitmap allocator can be used directly via [`ALLOCATOR`](struct@ALLOCATOR),
//! the [`allocate_phys_frame`], [`allocate_n_frames`], and [`deallocate_phys_frame`]
//! helper functions are typically easier to use. The [`debug`]
//! function can be used to print debug information about the bitmap
//! allocator's state.
//!
//! # Examples
//!
//! ```
//! // Allocate a physical memory frame.
//! let frame = physmem::allocate_frame().unwrap();
//!
//! // Write to the frame.
//! let virt_addr = memory::phys_to_virt_addr(frame.start_address());
//! let buf: &mut [u8] =
//!     unsafe { slice::from_raw_parts_mut(virt_addr.as_usize() as *mut u8, frame.size().bytes()) };
//! buf[0] = 0xff;
//!
//! // Drop the virtual memory and de-allocate the frame.
//! drop(buf);
//! unsafe { physmem::deallocate_frame(frame) };
//! ```

#![no_std]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]

extern crate alloc;

mod bitmap;
mod boot_info;

pub use crate::bitmap::{ArenaFrameAllocator, BitmapFrameAllocator, BitmapFrameTracker};
pub use crate::boot_info::BootInfoFrameAllocator;
use bootloader::bootinfo::MemoryMap;
use lazy_static::lazy_static;
use memory::{PhysFrame, PhysFrameAllocator, PhysFrameDeallocator, PhysFrameRange, PhysFrameSize};
use spin::{lock, Mutex};

lazy_static! {
    /// The second-phase physical memory allocator.
    ///
    /// `ALLOCATOR` can be initialised by calling [`init`], once the kernel's heap has
    /// been set up. To bootstrap the heap, use [`bootstrap`] to build a [`BootInfoFrameAllocator`],
    /// then pass that to [`init`] so `ALLOCATOR` can take over.
    ///
    pub static ref ALLOCATOR: Mutex<BitmapFrameAllocator> = Mutex::new(BitmapFrameAllocator::empty());
}

/// Sets up the second-phase physical memory manager, taking over
/// from the bootstrap allocator.
///
/// # Safety
///
/// The `bootstrap` allocator passed to `init` must have sole control
/// over all physical memory it describes. If any physical memory is
/// being used but is marked as available in `bootstrap`, then undefined
/// behaviour may ensue.
///
pub unsafe fn init(bootstrap: BootInfoFrameAllocator) {
    let mut alloc = BitmapFrameAllocator::new(bootstrap.underlying_map());
    alloc.repossess(bootstrap);

    *lock!(ALLOCATOR) = alloc;
}

/// Returns the next available physical frame, or `None`.
///
/// If `allocate_frame` is called before [`init`], it will return `None`.
///
pub fn allocate_phys_frame(size: PhysFrameSize) -> Option<PhysFrame> {
    let mut allocator = lock!(ALLOCATOR);
    allocator.allocate_phys_frame(size)
}

/// Returns `n` sequential physical frames, or `None`.
///
/// It's possible that `n` frames may be available, but `allocate_n_frames`
/// still return `None`. The bitmap allocator must be able to return `n`
/// frames in a single contiguous sequence for it to succeed.
///
/// If `allocate_n_frames` is called before [`init`], it will return `None`.
///
pub fn allocate_n_frames(n: usize) -> Option<PhysFrameRange> {
    let mut allocator = lock!(ALLOCATOR);
    allocator.allocate_n_frames(n)
}

/// Marks the given physical memory frame as unused and returns it to the
/// list of free frames for later use.
///
/// # Safety
///
/// The caller must ensure that `frame` is unused.
///
pub unsafe fn deallocate_phys_frame(frame: PhysFrame) {
    let mut allocator = lock!(ALLOCATOR);
    allocator.deallocate_phys_frame(frame);
}

/// Prints debug information about the physical memory manager.
///
pub fn debug() {
    let mm = lock!(ALLOCATOR);
    mm.debug();
}

/// Returns an initial frame allocator, which can be used to allocate the
/// the kernel's heap.
///
/// Once the kernel's heap has been initialised, the kernel should switch
/// over to a more advanced allocator, by calling [`init`].
///
/// # Safety
///
/// This function is unsafe because the caller must guarantee that the
/// memory map is valid and complete. All frames that are marked as `USABLE`
/// in the memory map must be unused.
///
/// `bootstrap` must be called at most once, and must not be called after
/// a call to [`init`].
///
pub unsafe fn bootstrap(memory_map: &'static MemoryMap) -> BootInfoFrameAllocator {
    BootInfoFrameAllocator::new(memory_map)
}
