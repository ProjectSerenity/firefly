//! constants contains useful constants for the kernel's memory layout.

use x86_64::structures::paging::{PageSize, Size4KiB};
use x86_64::{PhysAddr, VirtAddr};

// Important addresses.
//
// Make sure the constants below keep in sync with
// the [package.metadata.bootloader] section of
// Cargo.toml.

/// PHYSICAL_MEMORY_OFFSET is the virtual address at which the mapping of
/// all physical memory begins. That is, for any valid physical address,
/// that address can be reached at the same virtual address, plus
/// PHYSICAL_MEMORY_OFFSET.
///
pub const PHYSICAL_MEMORY_OFFSET: usize = 0xffff_8000_0000_0000;

/// phys_to_virt_addr returns a virtual address that is mapped to the
/// given physical address. This uses the mapping of all physical memory
/// at the virtual address PHYSICAL_MEMORY_OFFSET.
///
pub fn phys_to_virt_addr(phys: PhysAddr) -> VirtAddr {
    VirtAddr::new(phys.as_u64() + PHYSICAL_MEMORY_OFFSET as u64)
}

/// BOOT_INFO_START is the virtual address at which the boot information
/// is stored. This can be used to receive information from the bootloader
/// about the early configuration of the machine.
///
pub const BOOT_INFO_START: usize = 0x4444_4443_3000;

/// KERNEL_HEAP_START is the virtual address where the kernel's heap begins.
pub const KERNEL_HEAP_START: usize = 0x_4444_4444_0000;

/// KERNEL_HEAP_SIZE is the size in bytes of the kernel's heap.
pub const KERNEL_HEAP_SIZE: usize = 100 * 1024; // 100 KiB

/// kernel_heap_addr returns whether addr is an address in the kernel's heap.
///
#[inline]
pub fn kernel_heap_addr(addr: VirtAddr) -> bool {
    let addr = addr.as_u64() as usize;
    KERNEL_HEAP_START <= addr && addr <= KERNEL_HEAP_START + KERNEL_HEAP_SIZE
}

/// KERNEL_STACK_START is the virtual address where the kernel's stack begins.
pub const KERNEL_STACK_START: usize = 0x_7777_7777_0000 + KERNEL_STACK_SIZE;

/// KERNEL_STACK_SIZE is the size in bytes of the kernel's stack.
///
/// Note that this includes an extra page, as the stack counts downward,
/// not upward.
///
pub const KERNEL_STACK_SIZE: usize = 513 * Size4KiB::SIZE as usize;

/// kernel_stack_addr returns whether addr is an address in the kernel's stack.
///
#[inline]
pub fn kernel_stack_addr(addr: VirtAddr) -> bool {
    let addr = addr.as_u64() as usize;
    KERNEL_STACK_START - KERNEL_STACK_SIZE <= addr && addr <= KERNEL_STACK_START
}
