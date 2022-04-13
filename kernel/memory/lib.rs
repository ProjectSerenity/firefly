// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Contains types and functionality to represent physical and virtual memory.
//!
//! This crate provides the core types for representing physical and virtual
//! memory, plus the page table code that maps the two together. From most
//! basic to most sophisticated, the physical memory types are:
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
//! The [`PageTable`] allows the parsing and modification of page tables,
//! mapping pages of virtual memory to frames of physical memory. This uses
//! [`PageTableFlags`] to govern the behaviour of the mapped virtual memory.
//! The kernel's page tables are configured to ensure that all physical memory
//! is mapped contiguously at [`PHYSICAL_MEMORY_OFFSET`](constants::PHYSICAL_MEMORY_OFFSET).
//! This allows the convenience function [`phys_to_virt_addr`], which can be
//! used to translate any physical memory address to a virtual memory address
//! that can be used to access the physical memory. Note that this virtual
//! address cannot be used for fetching instructions.
//!
//! The [`PhysFrameAllocator`] and [`PhysFrameDeallocator`] traits can be used
//! to abstract the management of physical memory.
//!
//! The [`constants`] module contains a set of important addresses and address
//! ranges.

#![no_std]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]

pub mod constants;
mod page_table;
mod phys_addr;
mod phys_frame;
mod phys_range;
mod virt_addr;
mod virt_page;
mod virt_range;

pub use page_table::{
    PageMapping, PageMappingChange, PageMappingError, PageRemappingError, PageTable,
    PageTableEntry, PageTableFlags, PageUnmappingError,
};
pub use phys_addr::{InvalidPhysAddr, PhysAddr};
pub use phys_frame::{PhysFrame, PhysFrameRange, PhysFrameSize};
pub use phys_range::PhysAddrRange;
pub use virt_addr::{InvalidVirtAddr, VirtAddr};
pub use virt_page::{VirtPage, VirtPageRange, VirtPageSize};
pub use virt_range::VirtAddrRange;

/// Returns a virtual address that is mapped to the given physical
/// address.
///
/// This uses the mapping of all physical memory at the virtual
/// address `PHYSICAL_MEMORY_OFFSET`.
///
pub fn phys_to_virt_addr(phys: PhysAddr) -> VirtAddr {
    constants::PHYSICAL_MEMORY_OFFSET
        .checked_add(phys.as_usize())
        .expect("invalid physical address")
}

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
