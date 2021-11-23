//! constants contains useful constants for the kernel's memory layout.

// Ignore dead code warnings for unused constants.
#![allow(dead_code)]

use x86_64::structures::paging::{PageSize, Size4KiB};
use x86_64::{PhysAddr, VirtAddr};

// Important addresses.
//
// Make sure the constants below keep in sync with
// the [package.metadata.bootloader] section of
// Cargo.toml.
//
// Reminder of the memory layout (documented in more
// detail in README.md):
//
// | Region              |         Start address |            Last address |
// | ------------------- | --------------------- | --------------------- |
// | Kernel binary       | 0xffff_8000_0000_0000 | 0xffff_8000_3fff_ffff |
// | Bootloader info     | 0xffff_8000_4000_0000 | 0xffff_8000_4000_0fff |
// | Kernel heap         | 0xffff_8000_4444_0000 | 0xffff_8000_444b_ffff |
// | Kernel stack        | 0xffff_8000_5555_1000 | 0xffff_8000_555d_0fff |
// | Physical memory map | 0xffff_8000_6000_0000 | 0xffff_ffff_ffff_ffff |

/// KERNEL_BINARY_START is the virtual address at which the kernel binary
/// is loaded.
///
pub const KERNEL_BINARY_START: VirtAddr = const_virt_addr(0xffff_8000_0000_0000 as u64);
pub const KERNEL_BINARY_END: VirtAddr = const_virt_addr(0xffff_8000_3fff_ffff as u64);

/// BOOT_INFO_START is the virtual address at which the boot information
/// is stored. This can be used to receive information from the bootloader
/// about the early configuration of the machine.
///
pub const BOOT_INFO_START: VirtAddr = const_virt_addr(0xffff_8000_4000_0000 as u64);
pub const BOOT_INFO_END: VirtAddr = const_virt_addr(0xffff_8000_4000_0fff as u64);

/// KERNEL_HEAP_START is the virtual address where the kernel's heap begins.
///
pub const KERNEL_HEAP_START: VirtAddr = const_virt_addr(0xffff_8000_4444_0000 as u64);
pub const KERNEL_HEAP_END: VirtAddr = const_virt_addr(0xffff_8000_444b_ffff as u64);
pub const KERNEL_HEAP_SIZE: u64 = 128 * Size4KiB::SIZE; // 512 kiB

/// kernel_heap_addr returns whether addr is an address in the kernel's heap.
///
#[inline]
pub fn kernel_heap_addr(addr: VirtAddr) -> bool {
    KERNEL_HEAP_START <= addr && addr <= KERNEL_HEAP_END
}

/// KERNEL_STACK_START is the virtual address where the kernel's stack begins.
///
/// Note that the stack counts downwards, so the start address is larger than
/// the end address.
///
pub const KERNEL_STACK_START: VirtAddr = const_virt_addr(0xffff_8000_555d_0fff as u64);
pub const KERNEL_STACK_END: VirtAddr = const_virt_addr(0xffff_8000_5555_1000 as u64);
pub const KERNEL_STACK_SIZE: u64 = 129 * Size4KiB::SIZE; // 512 kiB

/// kernel_stack_addr returns whether addr is an address in the kernel's stack.
///
#[inline]
pub fn kernel_stack_addr(addr: VirtAddr) -> bool {
    KERNEL_STACK_END <= addr && addr <= KERNEL_STACK_START
}

/// PHYSICAL_MEMORY_OFFSET is the virtual address at which the mapping of
/// all physical memory begins. That is, for any valid physical address,
/// that address can be reached at the same virtual address, plus
/// PHYSICAL_MEMORY_OFFSET.
///
pub const PHYSICAL_MEMORY_OFFSET: VirtAddr = const_virt_addr(0xffff_8000_6000_0000 as u64);
pub const VIRTUAL_MEMORY_END: VirtAddr = const_virt_addr(0xffff_ffff_ffff_ffff as u64);

/// phys_to_virt_addr returns a virtual address that is mapped to the
/// given physical address. This uses the mapping of all physical memory
/// at the virtual address PHYSICAL_MEMORY_OFFSET.
///
pub fn phys_to_virt_addr(phys: PhysAddr) -> VirtAddr {
    PHYSICAL_MEMORY_OFFSET + phys.as_u64()
}

/// const_virt_addr is a const fn that returns the given virtual
/// address.
///
const fn const_virt_addr(addr: u64) -> VirtAddr {
    // Check that the address is a 48-bit canonical address,
    // either as a a low half address (starting with 0x00007,
    // or a high half address (starting with 0xffff8).
    if addr & 0xffff_8000_0000_0000 == 0 {
        // Canonical low half address.
        unsafe { VirtAddr::new_unsafe(addr) }
    } else if addr & 0xffff_8000_0000_0000 == 0xffff_8000_0000_0000 {
        // Canonical high half address.
        unsafe { VirtAddr::new_unsafe(addr) }
    } else if addr & 0x0000_8000_0000_0000 == 0x0000_8000_0000_0000 {
        // Noncanonical address we can sign-extend.
        VirtAddr::new_truncate(addr)
    } else {
        // Invalid address.
        panic!("invalid virtual address")
    }
}
