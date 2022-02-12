// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Contains constants describing the virtual memory layout.
//!
//! This module contains various constants for describe a [region of virtual memory](VirtAddrRange)
//! that is used for a prescribed purpose:
//!
//! - [`NULL_PAGE`]: The first virtual page, which is reserved to ensure null pointer dereferences cause a page fault.
//! - [`USERSPACE`]: The first half of virtual memory, which is used by userspace processes.
//! - [`KERNEL_BINARY`]: The kernel binary is mapped within this range.
//! - [`BOOT_INFO`]: The boot info provided by the bootloader is stored here.
//! - [`KERNEL_HEAP`]: The region used for the kernel's heap.
//! - [`KERNEL_STACK_GUARD`]: The page beneath the kernel's initial stack, reserved to ensure stack overflows cause a page fault.
//! - [`KERNEL_STACK`]: The region used for the all kernel stacks.
//! - [`KERNEL_STACK_0`]: The region used for the kernel's initial stack.
//! - [`MMIO_SPACE`]: The region used for mapping direct memory access for memory-mapped I/O.
//! - [`CPU_LOCAL`]: The region used for storing CPU-local data.
//! - [`PHYSICAL_MEMORY`]: The region into which all physical memory is mapped.
//!
//! There are also the following address constants:
//!
//! - [`KERNEL_STACK_1_START`]: The bottom of the second kernel stack.
//! - [`PHYSICAL_MEMORY_OFFSET`]: The offset at which all physical memory is mapped.
//!
//! The memory layout is summarised below:
//!
//! | Region                 |           Start address |            Last address |                 Pages |      Size |
//! | ---------------------- | ----------------------: | ----------------------: | --------------------: | --------: |
//! | [`NULL_PAGE`]          |                   `0x0` |             `0x1f_ffff` |            not mapped |     2 MiB |
//! | [`USERSPACE`]          |             `0x20_0000` |      `0x7fff_ffff_ffff` |        rest of memory | < 128 TiB |
//! | [`KERNEL_BINARY`]      | `0xffff_8000_0000_0000` | `0xffff_8000_3fff_ffff` | up to 512x 2 MiB page |     1 GiB |
//! | [`BOOT_INFO`]          | `0xffff_8000_4000_0000` | `0xffff_8000_4000_0fff` |         1x 4 KiB page |     4 KiB |
//! | [`KERNEL_HEAP`]        | `0xffff_8000_4444_0000` | `0xffff_8000_444b_ffff` |       128x 4 KiB page |   512 KiB |
//! | [`KERNEL_STACK_GUARD`] | `0xffff_8000_5554_f000` | `0xffff_8000_5554_ffff` |            not mapped |     4 KiB |
//! | [`KERNEL_STACK_0`]     | `0xffff_8000_5555_0000` | `0xffff_8000_555c_ffff` |       128x 4 KiB page |   512 KiB |
//! | [`KERNEL_STACK`]       | `0xffff_8000_5555_0000` | `0xffff_8000_5d5c_ffff` |    32,896x 4 KiB page | 128.5 MiB |
//! | [`MMIO_SPACE`]         | `0xffff_8000_6666_0000` | `0xffff_8000_6675_ffff` |       256x 4 KiB page |     1 MiB |
//! | [`CPU_LOCAL`]          | `0xffff_8000_7777_0000` | `0xffff_8000_7f76_ffff` |    32,768x 4 KiB page |   128 MiB |
//! | [`PHYSICAL_MEMORY`]    | `0xffff_8000_8000_0000` | `0xffff_ffff_ffff_ffff` |        rest of memory | < 128 TiB |

#![no_std]

use x86_64::{PhysAddr, VirtAddr};

/// The first virtual page, which is reserved to ensure null pointer dereferences cause a page fault.
///
pub const NULL_PAGE: VirtAddrRange = VirtAddrRange::new(NULL_PAGE_START, NULL_PAGE_END);
const NULL_PAGE_START: VirtAddr = VirtAddr::zero();
const NULL_PAGE_END: VirtAddr = const_virt_addr(0x1f_ffff_u64);

/// The first half of virtual memory, which is used by userspace processes.
///
pub const USERSPACE: VirtAddrRange = VirtAddrRange::new(USERSPACE_START, USERSPACE_END);
const USERSPACE_START: VirtAddr = const_virt_addr(0x20_0000_u64);
const USERSPACE_END: VirtAddr = const_virt_addr(0x7fff_ffff_ffff_u64);

/// The kernel binary is mapped within this range.
///
pub const KERNEL_BINARY: VirtAddrRange = VirtAddrRange::new(KERNEL_BINARY_START, KERNEL_BINARY_END);
const KERNEL_BINARY_START: VirtAddr = const_virt_addr(0xffff_8000_0000_0000_u64);
const KERNEL_BINARY_END: VirtAddr = const_virt_addr(0xffff_8000_3fff_ffff_u64);

/// The boot info provided by the bootloader is stored here.
///
pub const BOOT_INFO: VirtAddrRange = VirtAddrRange::new(BOOT_INFO_START, BOOT_INFO_END);
const BOOT_INFO_START: VirtAddr = const_virt_addr(0xffff_8000_4000_0000_u64);
const BOOT_INFO_END: VirtAddr = const_virt_addr(0xffff_8000_4000_0fff_u64);

/// The region used for the kernel's heap.
///
pub const KERNEL_HEAP: VirtAddrRange = VirtAddrRange::new(KERNEL_HEAP_START, KERNEL_HEAP_END);
const KERNEL_HEAP_START: VirtAddr = const_virt_addr(0xffff_8000_4444_0000_u64);
const KERNEL_HEAP_END: VirtAddr = const_virt_addr(0xffff_8000_444b_ffff_u64);

/// The page beneath the kernel's initial stack, reserved to ensure stack overflows cause a page fault.
///
pub const KERNEL_STACK_GUARD: VirtAddrRange =
    VirtAddrRange::new(KERNEL_STACK_GUARD_START, KERNEL_STACK_GUARD_END);
const KERNEL_STACK_GUARD_START: VirtAddr = const_virt_addr(0xffff_8000_5554_f000_u64);
const KERNEL_STACK_GUARD_END: VirtAddr = const_virt_addr(0xffff_8000_5554_ffff_u64);

/// The region used for the all kernel stacks.
///
/// Note that even though the stack counts downwards, we use the smaller address as
/// the start address and the larger address as the end address.
///
pub const KERNEL_STACK: VirtAddrRange = VirtAddrRange::new(KERNEL_STACK_START, KERNEL_STACK_END);
const KERNEL_STACK_START: VirtAddr = const_virt_addr(0xffff_8000_5555_0000_u64);
const KERNEL_STACK_END: VirtAddr = const_virt_addr(0xffff_8000_5d5c_ffff_u64);
/// The region used for the kernel's initial stack.
///
/// Note that even though the stack counts downwards, we use the smaller address as
/// the start address and the larger address as the end address.
///
pub const KERNEL_STACK_0: VirtAddrRange =
    VirtAddrRange::new(KERNEL_STACK_0_START, KERNEL_STACK_0_END);
const KERNEL_STACK_0_START: VirtAddr = KERNEL_STACK_START;
const KERNEL_STACK_0_END: VirtAddr = const_virt_addr(0xffff_8000_555c_ffff_u64);
/// The bottom of the second kernel stack.
///
/// Note that even though the stack counts downwards, we use the smaller address as
/// the start address and the larger address as the end address.
///
pub const KERNEL_STACK_1_START: VirtAddr = const_virt_addr(0xffff_8000_555d_0000_u64);

/// The region used for mapping direct memory access for memory-mapped I/O.
///
pub const MMIO_SPACE: VirtAddrRange = VirtAddrRange::new(MMIO_SPACE_START, MMIO_SPACE_END);
const MMIO_SPACE_START: VirtAddr = const_virt_addr(0xffff_8000_6666_0000_u64);
const MMIO_SPACE_END: VirtAddr = const_virt_addr(0xffff_8000_6675_ffff_u64);

/// The region used for storing CPU-local data.
///
/// Successive CPU cores use successive chunks of the address space.
///
pub const CPU_LOCAL: VirtAddrRange = VirtAddrRange::new(CPU_LOCAL_START, CPU_LOCAL_END);
const CPU_LOCAL_START: VirtAddr = const_virt_addr(0xffff_8000_7777_0000_u64);
const CPU_LOCAL_END: VirtAddr = const_virt_addr(0xffff_8000_7f76_ffff_u64);

/// The region into which all physical memory is mapped.
///
pub const PHYSICAL_MEMORY: VirtAddrRange =
    VirtAddrRange::new(PHYSICAL_MEMORY_OFFSET, VIRTUAL_MEMORY_END);
/// The offset at which all physical memory is mapped.
///
/// For any valid physical address, that address can be reached at
/// the same virtual address, plus `PHYSICAL_MEMORY_OFFSET`.
///
pub const PHYSICAL_MEMORY_OFFSET: VirtAddr = const_virt_addr(0xffff_8000_8000_0000_u64);
const VIRTUAL_MEMORY_END: VirtAddr = const_virt_addr(0xffff_ffff_ffff_ffff_u64);

/// Returns a virtual address that is mapped to the given physical
/// address.
///
/// This uses the mapping of all physical memory at the virtual
/// address `PHYSICAL_MEMORY_OFFSET`.
///
pub fn phys_to_virt_addr(phys: PhysAddr) -> VirtAddr {
    PHYSICAL_MEMORY_OFFSET + phys.as_u64()
}

/// Represents a contiguous sequence of virtual addresses.
///
pub struct VirtAddrRange {
    first: VirtAddr,
    last: VirtAddr,
}

impl VirtAddrRange {
    /// Returns a new range, from `start` to `end`.
    ///
    /// # Panics
    ///
    /// `new` will panic if `start` is not smaller than `end`.
    ///
    pub const fn new(start: VirtAddr, end: VirtAddr) -> Self {
        if start.as_u64() >= end.as_u64() {
            panic!("invalid virtual address range: start is not smaller than end");
        }

        VirtAddrRange {
            first: start,
            last: end,
        }
    }

    /// Returns the first address in the range.
    ///
    pub const fn start(&self) -> VirtAddr {
        self.first
    }

    /// Returns the last address in the range.
    ///
    pub const fn end(&self) -> VirtAddr {
        self.last
    }

    /// Returns the number of addresses in the range.
    ///
    pub const fn size(&self) -> u64 {
        (self.last.as_u64() + 1u64) - self.first.as_u64()
    }

    /// Returns whether the given address range exists
    /// entirely within (or is equal to) this range.
    ///
    pub const fn contains(&self, other: &VirtAddrRange) -> bool {
        self.first.as_u64() <= other.first.as_u64() && other.last.as_u64() <= self.last.as_u64()
    }

    /// Returns whether the given address exists in this
    /// range.
    ///
    pub const fn contains_addr(&self, other: VirtAddr) -> bool {
        self.first.as_u64() <= other.as_u64() && other.as_u64() <= self.last.as_u64()
    }

    /// Returns whether the given `start` and `end`
    /// addresses both exist within this range.
    ///
    pub const fn contains_range(&self, other_start: VirtAddr, other_end: VirtAddr) -> bool {
        other_start.as_u64() < other_end.as_u64()
            && self.contains_addr(other_start)
            && self.contains_addr(other_end)
    }
}

/// A `const fn` that returns the given virtual address.
///
const fn const_virt_addr(addr: u64) -> VirtAddr {
    // Check that the address is a 48-bit canonical address,
    // either as a a low half address (starting with 0x00007,
    // or a high half address (starting with 0xffff8).
    let top_bits = (addr & 0xffff_8000_0000_0000u64) >> 47;
    match top_bits {
        0 => unsafe { VirtAddr::new_unsafe(addr) }, // Canonical low half address.
        0x1ffff => unsafe { VirtAddr::new_unsafe(addr) }, // Canonical high half address.
        1 => VirtAddr::new_truncate(addr),          // Noncanonical address we can sign-extend.
        _ => panic!("invalid virtual address"),     // Invalid address.
    }
}
