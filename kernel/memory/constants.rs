// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Contains constants describing the virtual memory layout.
//!
//! This module contains various constants for describe a [region of virtual memory](crate::VirtAddrRange)
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
//! | [`USER_BINARY`]        |             `0x20_0000` |      `0x1fff_ffff_ffff` |     randomised subset |  < 32 TiB |
//! | [`USER_HEAP`]          |      `0x2000_0000_0000` |      `0x5fff_ffff_ffff` |    randomised subsets |    64 TiB |
//! | [`USER_STACK`]         |      `0x6000_0000_0000` |      `0x7fff_ffff_ffff` |     randomised subset |    32 TiB |
//! | [`KERNELSPACE`]        | `0xffff_8000_0000_0000` | `0xffff_ffff_ffff_ffff` | higher half of memory |   128 TiB |
//! | [`KERNEL_BINARY`]      | `0xffff_8000_0000_0000` | `0xffff_8000_3fff_ffff` | up to 512x 2 MiB page |     1 GiB |
//! | [`BOOT_INFO`]          | `0xffff_8000_4000_0000` | `0xffff_8000_4000_0fff` |         1x 4 KiB page |     4 KiB |
//! | [`KERNEL_HEAP`]        | `0xffff_8000_4444_0000` | `0xffff_8000_444b_ffff` |       128x 4 KiB page |   512 KiB |
//! | [`KERNEL_STACK_GUARD`] | `0xffff_8000_5554_f000` | `0xffff_8000_5554_ffff` |            not mapped |     4 KiB |
//! | [`KERNEL_STACK_0`]     | `0xffff_8000_5555_0000` | `0xffff_8000_555c_ffff` |       128x 4 KiB page |   512 KiB |
//! | [`KERNEL_STACK`]       | `0xffff_8000_5555_0000` | `0xffff_8000_5d5c_ffff` |    32,896x 4 KiB page | 128.5 MiB |
//! | [`MMIO_SPACE`]         | `0xffff_8000_6666_0000` | `0xffff_8000_6675_ffff` |       256x 4 KiB page |     1 MiB |
//! | [`CPU_LOCAL`]          | `0xffff_8000_7777_0000` | `0xffff_8000_7f76_ffff` |    32,768x 4 KiB page |   128 MiB |
//! | [`PHYSICAL_MEMORY`]    | `0xffff_8000_8000_0000` | `0xffff_ffff_ffff_ffff` |        rest of memory | < 128 TiB |

use crate::{VirtAddr, VirtAddrRange};

/// The first virtual page, which is reserved to ensure null pointer dereferences cause a page fault.
///
pub const NULL_PAGE: VirtAddrRange = VirtAddrRange::new(NULL_PAGE_START, NULL_PAGE_END);
const NULL_PAGE_START: VirtAddr = VirtAddr::zero();
const NULL_PAGE_END: VirtAddr = VirtAddr::new(0x1f_ffff_usize);

/// The first half of virtual memory, which is used by userspace processes.
///
pub const USERSPACE: VirtAddrRange = VirtAddrRange::new(USERSPACE_START, USERSPACE_END);
const USERSPACE_START: VirtAddr = VirtAddr::new(0x20_0000_usize);
const USERSPACE_END: VirtAddr = VirtAddr::new(0x7fff_ffff_ffff_usize);

/// The user binary is mapped within this range.
///
pub const USER_BINARY: VirtAddrRange = VirtAddrRange::new(USER_BINARY_START, USER_BINARY_END);
const USER_BINARY_START: VirtAddr = VirtAddr::new(0x20_0000_usize);
const USER_BINARY_END: VirtAddr = VirtAddr::new(0x1fff_ffff_ffff_usize);

/// The region used for user processes' heaps.
///
pub const USER_HEAP: VirtAddrRange = VirtAddrRange::new(USER_HEAP_START, USER_HEAP_END);
const USER_HEAP_START: VirtAddr = VirtAddr::new(0x2000_0000_0000_usize);
const USER_HEAP_END: VirtAddr = VirtAddr::new(0x5fff_ffff_ffff_usize);

/// The region used for user processes' stacks.
///
/// Note that even though the stack counts downwards, we use the smaller address as
/// the start address and the larger address as the end address.
///
pub const USER_STACK: VirtAddrRange = VirtAddrRange::new(USER_STACK_START, USER_STACK_END);
const USER_STACK_START: VirtAddr = VirtAddr::new(0x6000_0000_0000_usize);
const USER_STACK_END: VirtAddr = VirtAddr::new(0x7fff_ffff_ffff_usize);

/// The higher half of virtual memory, which is used by the kernel.
///
pub const KERNELSPACE: VirtAddrRange = VirtAddrRange::new(KERNELSPACE_START, KERNELSPACE_END);
const KERNELSPACE_START: VirtAddr = VirtAddr::new(0xffff_8000_0000_0000_usize);
const KERNELSPACE_END: VirtAddr = VirtAddr::new(0xffff_ffff_ffff_ffff_usize);

/// The kernel binary is mapped within this range.
///
pub const KERNEL_BINARY: VirtAddrRange = VirtAddrRange::new(KERNEL_BINARY_START, KERNEL_BINARY_END);
const KERNEL_BINARY_START: VirtAddr = VirtAddr::new(0xffff_8000_0000_0000_usize);
const KERNEL_BINARY_END: VirtAddr = VirtAddr::new(0xffff_8000_3fff_ffff_usize);

/// The boot info provided by the bootloader is stored here.
///
pub const BOOT_INFO: VirtAddrRange = VirtAddrRange::new(BOOT_INFO_START, BOOT_INFO_END);
const BOOT_INFO_START: VirtAddr = VirtAddr::new(0xffff_8000_4000_0000_usize);
const BOOT_INFO_END: VirtAddr = VirtAddr::new(0xffff_8000_4000_0fff_usize);

/// The region used for the kernel's heap.
///
pub const KERNEL_HEAP: VirtAddrRange = VirtAddrRange::new(KERNEL_HEAP_START, KERNEL_HEAP_END);
const KERNEL_HEAP_START: VirtAddr = VirtAddr::new(0xffff_8000_4444_0000_usize);
const KERNEL_HEAP_END: VirtAddr = VirtAddr::new(0xffff_8000_444b_ffff_usize);

/// The page beneath the kernel's initial stack, reserved to ensure stack overflows cause a page fault.
///
pub const KERNEL_STACK_GUARD: VirtAddrRange =
    VirtAddrRange::new(KERNEL_STACK_GUARD_START, KERNEL_STACK_GUARD_END);
const KERNEL_STACK_GUARD_START: VirtAddr = VirtAddr::new(0xffff_8000_5554_f000_usize);
const KERNEL_STACK_GUARD_END: VirtAddr = VirtAddr::new(0xffff_8000_5554_ffff_usize);

/// The region used for the all kernel stacks.
///
/// Note that even though the stack counts downwards, we use the smaller address as
/// the start address and the larger address as the end address.
///
pub const KERNEL_STACK: VirtAddrRange = VirtAddrRange::new(KERNEL_STACK_START, KERNEL_STACK_END);
const KERNEL_STACK_START: VirtAddr = VirtAddr::new(0xffff_8000_5555_0000_usize);
const KERNEL_STACK_END: VirtAddr = VirtAddr::new(0xffff_8000_5d5c_ffff_usize);
/// The region used for the kernel's initial stack.
///
/// Note that even though the stack counts downwards, we use the smaller address as
/// the start address and the larger address as the end address.
///
pub const KERNEL_STACK_0: VirtAddrRange =
    VirtAddrRange::new(KERNEL_STACK_0_START, KERNEL_STACK_0_END);
const KERNEL_STACK_0_START: VirtAddr = KERNEL_STACK_START;
const KERNEL_STACK_0_END: VirtAddr = VirtAddr::new(0xffff_8000_555c_ffff_usize);
/// The bottom of the second kernel stack.
///
/// Note that even though the stack counts downwards, we use the smaller address as
/// the start address and the larger address as the end address.
///
pub const KERNEL_STACK_1_START: VirtAddr = VirtAddr::new(0xffff_8000_555d_0000_usize);

/// The region used for mapping direct memory access for memory-mapped I/O.
///
pub const MMIO_SPACE: VirtAddrRange = VirtAddrRange::new(MMIO_SPACE_START, MMIO_SPACE_END);
const MMIO_SPACE_START: VirtAddr = VirtAddr::new(0xffff_8000_6666_0000_usize);
const MMIO_SPACE_END: VirtAddr = VirtAddr::new(0xffff_8000_6675_ffff_usize);

/// The region used for storing CPU-local data.
///
/// Successive CPU cores use successive chunks of the address space.
///
pub const CPU_LOCAL: VirtAddrRange = VirtAddrRange::new(CPU_LOCAL_START, CPU_LOCAL_END);
const CPU_LOCAL_START: VirtAddr = VirtAddr::new(0xffff_8000_7777_0000_usize);
const CPU_LOCAL_END: VirtAddr = VirtAddr::new(0xffff_8000_7f76_ffff_usize);

/// The region into which all physical memory is mapped.
///
pub const PHYSICAL_MEMORY: VirtAddrRange =
    VirtAddrRange::new(PHYSICAL_MEMORY_OFFSET, VIRTUAL_MEMORY_END);
/// The offset at which all physical memory is mapped.
///
/// For any valid physical address, that address can be reached at
/// the same virtual address, plus `PHYSICAL_MEMORY_OFFSET`.
///
pub const PHYSICAL_MEMORY_OFFSET: VirtAddr = VirtAddr::new(0xffff_8000_8000_0000_usize);
const VIRTUAL_MEMORY_END: VirtAddr = VirtAddr::new(0xffff_ffff_ffff_ffff_usize);

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_memory_layout() {
        // This is the set of major memory regions.
        // There must be no overlap between regions.
        let regions = [
            (NULL_PAGE, "null page"),
            (USER_BINARY, "user binary"),
            (USER_HEAP, "user heap"),
            (USER_STACK, "user stack"),
            (KERNEL_BINARY, "kernel binary"),
            (BOOT_INFO, "boot info"),
            (KERNEL_HEAP, "kernel heap"),
            (KERNEL_STACK_GUARD, "kernel stack guard"),
            (KERNEL_STACK, "kernel stack"),
            (MMIO_SPACE, "MMIO space"),
            (CPU_LOCAL, "CPU-local"),
            (PHYSICAL_MEMORY, "physical memory"),
        ];

        // We don't need to do a quadratic search,
        // but n is small and it gives extra peace
        // of mind.
        for (i, r1) in regions.iter().enumerate() {
            for (j, r2) in regions.iter().enumerate() {
                if i == j {
                    continue;
                }

                assert!(
                    !r1.0.contains_addr(r2.0.start()),
                    "{} overlaps with {}",
                    r1.1,
                    r2.1
                );
                assert!(
                    !r1.0.contains_addr(r2.0.end()),
                    "{} overlaps with {}",
                    r1.1,
                    r2.1
                );
            }
        }

        // Check that the null page starts at address
        // zero, or it's pointless.
        assert_eq!(
            NULL_PAGE.start(),
            VirtAddr::new(0),
            "the null page does not start at 0"
        );

        // Check that userspace ends at the bottom of
        // the lower half of memory. We verify this by
        // checking that the next address is invalid.
        // If so, passing it to VirtAddr::try_new will
        // either return an error or will sign-extend
        // the address, resulting in a different numerical
        // value.
        let next_addr = USERSPACE.end().as_usize() + 1;
        assert!(VirtAddr::try_new(next_addr).is_err());

        // Likewise, we check that kernelspace begins
        // with the first valid higher half address,
        // by checking that the address before it is
        // either invalid or is coerced to a different,
        // valid, value.
        let prev_addr = KERNELSPACE.start().as_usize() - 1;
        assert!(VirtAddr::try_new(prev_addr).is_err());

        // We also check That it ends with the last
        // value by checking that incrementing the
        // last address overflows.
        let next_addr = KERNELSPACE.end().as_usize().overflowing_add(1);
        assert!(next_addr.0 == 0usize && next_addr.1);
    }
}
