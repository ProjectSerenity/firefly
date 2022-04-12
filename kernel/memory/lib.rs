// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Contains types and functionality to represent physical and virtual memory.
//!
//! This crate provides the core types for representing physical and virtual
//! memory. From most basic to most sophisticated, the physical memory types
//! are:
//!
//! - [`PhysAddr`]: An address in the 52-bit physical address space.
//! - [`PhysAddrRange`]: An arbitrary sequence of contiguous physical addresses.
//! - [`PhysFrameSize`]: The size of a frame of physical memory.
//! - [`PhysFrame`]: A frame of physical memory, including its size.
//! - [`PhysFrameRange`]: An arbitrary sequence of contiguous physical memory frames.
//!
//! The corresponding virtual memory types, from most basic to most sophisticated
//! are:
//!
//! - [`VirtAddr`]: A canonical address in the 48-bit virtual address space.
//! - [`VirtAddrRange`]: An arbitrary sequence of contiguous physical addresses.
//! - [`VirtPageSize`]: The size of a page of virtual memory.
//! - [`VirtPage`]: A page of virtual memory, including its size.
//! - [`VirtPageRange`]: An arbitrary sequence of contiguous virtual memory pages.
//!
//! The [`PhysFrameAllocator`] and [`PhysFrameDeallocator`] traits can be used
//! to abstract the management of physical memory.

#![no_std]

mod phys_addr;
mod phys_frame;
mod phys_range;
mod virt_addr;
mod virt_page;
mod virt_range;

pub use phys_addr::{InvalidPhysAddr, PhysAddr};
pub use phys_frame::{PhysFrame, PhysFrameRange, PhysFrameSize};
pub use phys_range::PhysAddrRange;
pub use virt_addr::{InvalidVirtAddr, VirtAddr};
pub use virt_page::{VirtPage, VirtPageRange, VirtPageSize};
pub use virt_range::VirtAddrRange;

/// A trait for types that can
/// allocate a frame of physical
/// memory.
///
/// # Safety
///
/// This trait is unsafe, as each
/// implementation must ensure
/// that it always returns unused
/// frames of the correct size.
///
pub unsafe trait PhysFrameAllocator {
    /// Allocate a physical frame of
    /// the requested size and return
    /// it, if possible.
    ///
    fn allocate_phys_frame(&mut self, size: PhysFrameSize) -> Option<PhysFrame>;
}

/// A trait for types that can
/// deallocate a frame of physical
/// memory.
///
pub trait PhysFrameDeallocator {
    /// Deallocate the given physical
    /// frame of memory.
    ///
    /// # Safety
    ///
    /// The caller must ensure that
    /// the given frame is unused.
    ///
    unsafe fn deallocate_phys_frame(&mut self, frame: PhysFrame);
}
