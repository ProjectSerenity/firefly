// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides functionality for interacting with memory-mapped
//! I/O devices.
//!
//! This module provides basic but safe support for MMIO. Physical memory
//! regions can be mapped into the [`MMIO_SPACE`](memory::constants::MMIO_SPACE) virtual
//! memory region. This mapping disables the use of caching to ensure correct
//! interaction with the underlying device memory.
//!
//! Mapped regions perform bounds checking to ensure that overflows do not
//! occur.
//!
//! # Examples
//!
//! ```
//! let phys_memory = PhysFrame::range(/* ... */);
//! let mut mapped = unsafe { mmio::Range::map(phys_memory) };
//! let first_short: u16 = mapped.read(0);
//! let second_short: u16 = 0x1234;
//! mapped.write(2, second_short);
//! ```
//!
//! The [`read_volatile`] and [`write_volatile`] macros  can be used to read
//! and write MMIO memory without seemingly idempotent accesses being removed
//! or rearranged by the compiler.
//!
//! ```
//! // Helper type describing the MMIO region data.
//! #[repr(C, packed)]
//! struct Config {
//!    field1: u32,
//!    field2: u32,
//! }
//!
//! // Map an MMIO region.
//! let mut region = unsafe { mmio::Range::map(/* ... */) };
//!
//! // Load the config data at the start of the region.
//! let mut config = region.as_mut::<Config>(0);
//!
//! // Read the first field twice and write the second
//! // field once, without the compiler removing or
//! // re-ordering any of the accesses.
//! unsafe {
//!     let _first = mmio::read_volatile!(config.field1);
//!     let _second = mmio::read_volatile!(config.field1);
//!     mmio::write_volatile!(config.field2, 1u16);
//! }
//! ```

#![no_std]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]
#![feature(decl_macro)]

use core::sync::atomic;
use memory::constants::MMIO_SPACE;
use memory::{PageTableFlags, PhysFrameRange, VirtAddr, VirtPage, VirtPageRange, VirtPageSize};
use physmem::ALLOCATOR;
use spin::{lock, Mutex};
use virtmem::map_frames_to_pages;

/// MMIO_START_ADDRESS is the address where the next MMIO mapping
/// will be placed.
///
static MMIO_START_ADDRESS: Mutex<VirtAddr> = Mutex::new(MMIO_SPACE.start());

/// Ensures the compiler will not rearrange any reads or
/// writes from one side of the barrier to the other.
///
#[inline]
pub fn access_barrier() {
    atomic::compiler_fence(atomic::Ordering::SeqCst);
}

/// reserve_space reserves the given amount of MMIO address space,
/// returning the virtual address where the reservation begins.
///
fn reserve_space(size: usize) -> VirtPageRange {
    let mut start_address = lock!(MMIO_START_ADDRESS);
    let start = *start_address;

    // Check we haven't gone outside the bounds
    // of the reserved MMIO address space.
    if !MMIO_SPACE.contains_addr(start + size) {
        panic!("exceeded MMIO address space");
    }

    let end = start + size;
    *start_address = end;

    let start_page = VirtPage::from_start_address(start, VirtPageSize::Size4KiB)
        .expect("bad MMIO region start virt address");
    let end_page = VirtPage::from_start_address(end, VirtPageSize::Size4KiB)
        .expect("bad MMIO region end virt address");

    VirtPage::range_exclusive(start_page, end_page)
}

/// Returns the referenced field in a way that will not be removed
/// by the compiler or reordered at runtime.
///
pub macro read_volatile($typ:ident.$field:ident) {
    core::ptr::read_volatile(core::ptr::addr_of!($typ.$field))
}

/// Writes to the referenced field in a way that will not be removed
/// by the compiler or reordered at runtime.
///
/// # Safety
///
/// `write_volatile!` is unsafe, as it bypasses the type system, and
/// could be used to write to non-mutable data. Using write_volatile
/// on a `const` value may also result in a general protection fault.
///
pub macro write_volatile($typ:ident.$field:ident, $value:expr) {
    core::ptr::write_volatile(core::ptr::addr_of_mut!($typ.$field), $value)
}

/// Indicates that a read or write in an MMIO region exceeded the
/// bounds of the region.
///
/// The address that exceeded the MMIO region bounds is included.
///
#[derive(Debug)]
pub struct RegionOverflow(pub VirtAddr);

/// Describes a set of memory allocated for memory-mapped I/O.
///
/// # Examples
///
/// ```
/// let phys_memory = PhysFrame::range(/* ... */);
/// let mut mapped = unsafe { mmio::Range::map(phys_memory) };
/// let first_short: u16 = mapped.read(0);
/// let second_short: u16 = 0x1234;
/// mapped.write(2, second_short);
/// ```
///
pub struct Region {
    // start is the first valid address in the region.
    start: VirtAddr,

    // end is the last valid address in the region.
    end: VirtAddr,
}

impl Region {
    /// Maps the given physical address region into the MMIO address
    /// space, returning a range through which the region can be
    /// accessed safely.
    ///
    pub fn map(frame_range: PhysFrameRange) -> Self {
        let first_addr = frame_range.start_address();
        let last_addr = frame_range.end_address();
        let size = (last_addr - first_addr) + 1;

        let page_range = reserve_space(size);
        let flags = PageTableFlags::PRESENT
            | PageTableFlags::WRITABLE
            | PageTableFlags::WRITE_THROUGH
            | PageTableFlags::NO_CACHE
            | PageTableFlags::GLOBAL
            | PageTableFlags::NO_EXECUTE;

        map_frames_to_pages(frame_range, page_range, &mut *lock!(ALLOCATOR), flags)
            .expect("failed to map MMIO region");

        Region {
            start: page_range.start_address(),
            end: page_range.end_address(),
        }
    }

    /// Returns a mutable reference of the given type at the MMIO memory
    /// at `offset` into the region.
    ///
    pub fn as_mut<T: Copy>(&self, offset: usize) -> Result<&'static mut T, RegionOverflow> {
        let addr = self.start + offset;
        let size = core::mem::size_of::<T>();
        if (addr + size) > self.end {
            return Err(RegionOverflow(addr + size));
        }

        let ptr = addr.as_usize() as *mut T;
        unsafe { Ok(ptr.as_mut().unwrap()) }
    }

    /// Returns a generic value at the given offset into the region.
    ///
    pub fn read<T: 'static + Copy>(&self, offset: usize) -> Result<T, RegionOverflow> {
        let ptr = self.as_mut(offset)?;
        Ok(*ptr)
    }

    /// Writes a generic value to the given offset into the region.
    ///
    pub fn write<T: 'static + Copy>(
        &mut self,
        offset: usize,
        val: T,
    ) -> Result<(), RegionOverflow> {
        let ptr = self.as_mut(offset)?;
        *ptr = val;

        Ok(())
    }
}
