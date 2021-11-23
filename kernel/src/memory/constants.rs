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
pub const PHYSICAL_MEMORY_OFFSET: VirtAddr =
    unsafe { VirtAddr::new_unsafe(0xffff_8000_6000_0000 as u64) };
/// phys_to_virt_addr returns a virtual address that is mapped to the
/// given physical address. This uses the mapping of all physical memory
/// at the virtual address PHYSICAL_MEMORY_OFFSET.
///
pub fn phys_to_virt_addr(phys: PhysAddr) -> VirtAddr {
    PHYSICAL_MEMORY_OFFSET + phys.as_u64()
}

/// BOOT_INFO_START is the virtual address at which the boot information
/// is stored. This can be used to receive information from the bootloader
/// about the early configuration of the machine.
///
pub const BOOT_INFO_START: VirtAddr = unsafe { VirtAddr::new_unsafe(0xffff_8000_4000_0000 as u64) };

/// KERNEL_HEAP_START is the virtual address where the kernel's heap begins.
pub const KERNEL_HEAP_START: VirtAddr =
    unsafe { VirtAddr::new_unsafe(0xffff_8000_4444_0000 as u64) };

/// KERNEL_HEAP_SIZE is the size in bytes of the kernel's heap.
pub const KERNEL_HEAP_SIZE: u64 = 128 * Size4KiB::SIZE; // 512 kiB

/// kernel_heap_addr returns whether addr is an address in the kernel's heap.
///
#[inline]
pub fn kernel_heap_addr(addr: VirtAddr) -> bool {
    KERNEL_HEAP_START <= addr && addr <= (KERNEL_HEAP_START + KERNEL_HEAP_SIZE)
}

/// KERNEL_STACK_START is the virtual address where the kernel's stack begins.
pub const KERNEL_STACK_START: VirtAddr =
    unsafe { VirtAddr::new_unsafe(0xffff_8000_5555_1000 as u64 + KERNEL_STACK_SIZE) };

/// KERNEL_STACK_SIZE is the size in bytes of the kernel's stack.
///
/// Note that this includes an extra page, as the stack counts downward,
/// not upward.
///
pub const KERNEL_STACK_SIZE: u64 = 129 * Size4KiB::SIZE; // 512 kiB

/// kernel_stack_addr returns whether addr is an address in the kernel's stack.
///
#[inline]
pub fn kernel_stack_addr(addr: VirtAddr) -> bool {
    KERNEL_STACK_START - KERNEL_STACK_SIZE <= addr && addr <= KERNEL_STACK_START
}

/// KERNEL_BINARY_START is the virtual address at which the kernel binary
/// is loaded.
///
pub const KERNEL_BINARY_START: VirtAddr =
    unsafe { VirtAddr::new_unsafe(0xffff_8000_0000_0000 as u64) };

/// KERNEL_BINARY_SIZE is the maximum size of the kernel binary.
///
pub const KERNEL_BINARY_SIZE: u64 = 262143 * Size4KiB::SIZE; // 1 GiB - 1 page (for boot info)

// Simple macro to simplify checking address constants.
//
macro_rules! check_const_addr {
    ($addr:expr) => {
        VirtAddr::try_new($addr.as_u64()).expect("bad constant");
        if $addr < KERNEL_BINARY_START {
            panic!("$addr is in the lower half of the virtual address space");
        }
    };
}

/// check ensures that the unsafe constant assignments in the constants
/// above are indeed valid.
///
pub fn check() {
    check_const_addr!(KERNEL_BINARY_START);
    check_const_addr!(BOOT_INFO_START);
    check_const_addr!(KERNEL_HEAP_START);
    check_const_addr!(KERNEL_STACK_START);
    check_const_addr!(PHYSICAL_MEMORY_OFFSET);
}
