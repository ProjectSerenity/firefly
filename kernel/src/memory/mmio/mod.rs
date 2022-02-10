// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides functionality for interacting with memory-mapped
//! I/O devices.
//!
//! This module provides basic but safe support for MMIO. Physical memory
//! regions can be mapped into the [`MMIO_SPACE`](super::MMIO_SPACE) virtual
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

use crate::memory;
use crate::memory::MMIO_SPACE;
use core::sync::atomic;
use physmem;
use x86_64::structures::paging::frame::PhysFrameRange;
use x86_64::structures::paging::page::Page;
use x86_64::structures::paging::page_table::PageTableFlags;
use x86_64::structures::paging::Mapper;
use x86_64::VirtAddr;

/// MMIO_START_ADDRESS is the address where the next MMIO mapping
/// will be placed.
///
static MMIO_START_ADDRESS: spin::Mutex<VirtAddr> = spin::Mutex::new(MMIO_SPACE.start());

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
fn reserve_space(size: u64) -> VirtAddr {
    let mut start_address = MMIO_START_ADDRESS.lock();
    let out = *start_address;

    // Check we haven't gone outside the bounds
    // of the reserved MMIO address space.
    if !MMIO_SPACE.contains_addr(out + size) {
        panic!("exceeded MMIO address space");
    }

    *start_address = out + size;
    out
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
    /// # Safety
    ///
    /// This function is unsafe because the caller must guarantee that the
    /// given physical memory region is not being used already for other
    /// purposes.
    ///
    pub unsafe fn map(range: PhysFrameRange) -> Self {
        let first_addr = range.start.start_address();
        let last_addr = range.end.start_address();
        let size = last_addr - first_addr;

        let mut mapper = memory::kernel_pml4();
        let mut frame_allocator = physmem::ALLOCATOR.lock();
        let start_address = reserve_space(size);
        let mut next_address = start_address;
        for frame in range {
            let flags = PageTableFlags::PRESENT
                | PageTableFlags::WRITABLE
                | PageTableFlags::WRITE_THROUGH
                | PageTableFlags::NO_CACHE
                | PageTableFlags::GLOBAL
                | PageTableFlags::NO_EXECUTE;
            let page = Page::from_start_address(next_address).expect("bad start address");
            next_address += page.size();
            mapper
                .map_to(page, frame, flags, &mut *frame_allocator)
                .expect("failed to map MMIO page")
                .flush();
        }

        Region {
            start: start_address,
            end: next_address - 1u64,
        }
    }

    /// Returns a mutable reference of the given type at the MMIO memory
    /// at `offset` into the region.
    ///
    pub fn as_mut<T: Copy>(&self, offset: u64) -> Result<&'static mut T, RegionOverflow> {
        let addr = self.start + offset;
        let size = core::mem::size_of::<T>() as u64;
        if (addr + size) > self.end {
            return Err(RegionOverflow(addr + size));
        }

        let ptr = addr.as_ptr::<T>() as *mut T;
        unsafe { Ok(ptr.as_mut().unwrap()) }
    }

    /// Returns a generic value at the given offset into the region.
    ///
    pub fn read<T: 'static + Copy>(&self, offset: u64) -> Result<T, RegionOverflow> {
        let ptr = self.as_mut(offset)?;
        Ok(*ptr)
    }

    /// Writes a generic value to the given offset into the region.
    ///
    pub fn write<T: 'static + Copy>(&mut self, offset: u64, val: T) -> Result<(), RegionOverflow> {
        let ptr = self.as_mut(offset)?;
        *ptr = val;

        Ok(())
    }
}
