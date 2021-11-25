//! constants contains useful constants for the kernel's memory layout.

// Ignore dead code warnings for unused constants.
#![allow(dead_code)]

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
// | Region              |         Start address |          Last address |
// | ------------------- | --------------------- | --------------------- |
// | NULL page           |                   0x0 |             0x1f_ffff |
// | Userspace           |             0x20_0000 |      0x7fff_ffff_ffff |
// | Kernel binary       | 0xffff_8000_0000_0000 | 0xffff_8000_3fff_ffff |
// | Bootloader info     | 0xffff_8000_4000_0000 | 0xffff_8000_4000_0fff |
// | Kernel heap         | 0xffff_8000_4444_0000 | 0xffff_8000_444b_ffff |
// | Kernel stack guard  | 0xffff_8000_5554_f000 | 0xffff_8000_5554_ffff |
// | Kernel stack        | 0xffff_8000_5555_0000 | 0xffff_8000_555c_ffff |
// | MMIO address space  | 0xffff_8000_6666_0000 | 0xffff_8000_6675_ffff |
// | Physical memory map | 0xffff_8000_8000_0000 | 0xffff_ffff_ffff_ffff |

/// NULL_PAGE is reserved and always unmapped to ensure that null pointer
/// dereferences always result in a page fault.
///
pub const NULL_PAGE: VirtAddrRange = VirtAddrRange::new(NULL_PAGE_START, NULL_PAGE_END);
const NULL_PAGE_START: VirtAddr = VirtAddr::zero();
const NULL_PAGE_END: VirtAddr = const_virt_addr(0x1f_ffff as u64);

/// USERSPACE is the address space in which memory can be allocated for
/// userspace programs.
///
pub const USERSPACE: VirtAddrRange = VirtAddrRange::new(USERSPACE_START, USERSPACE_END);
const USERSPACE_START: VirtAddr = const_virt_addr(0x20_0000 as u64);
const USERSPACE_END: VirtAddr = const_virt_addr(0x7fff_ffff_ffff as u64);

/// KERNEL_BINARY is the virtual address at which the kernel binary
/// is loaded.
///
pub const KERNEL_BINARY: VirtAddrRange = VirtAddrRange::new(KERNEL_BINARY_START, KERNEL_BINARY_END);
const KERNEL_BINARY_START: VirtAddr = const_virt_addr(0xffff_8000_0000_0000 as u64);
const KERNEL_BINARY_END: VirtAddr = const_virt_addr(0xffff_8000_3fff_ffff as u64);

/// BOOT_INFO is the virtual address at which the boot information is
/// stored. This can be used to receive information from the bootloader
/// about the early configuration of the machine.
///
pub const BOOT_INFO: VirtAddrRange = VirtAddrRange::new(BOOT_INFO_START, BOOT_INFO_END);
const BOOT_INFO_START: VirtAddr = const_virt_addr(0xffff_8000_4000_0000 as u64);
const BOOT_INFO_END: VirtAddr = const_virt_addr(0xffff_8000_4000_0fff as u64);

/// KERNEL_HEAP is the virtual address of the kernel's heap.
///
pub const KERNEL_HEAP: VirtAddrRange = VirtAddrRange::new(KERNEL_HEAP_START, KERNEL_HEAP_END);
const KERNEL_HEAP_START: VirtAddr = const_virt_addr(0xffff_8000_4444_0000 as u64);
const KERNEL_HEAP_END: VirtAddr = const_virt_addr(0xffff_8000_444b_ffff as u64);

/// KERNEL_STACK_GUARD is the area of memory we deliberately leave unmapped
/// so we can diagnose stack overflows by spotting page faults in this region.
///
pub const KERNEL_STACK_GUARD: VirtAddrRange =
    VirtAddrRange::new(KERNEL_STACK_GUARD_START, KERNEL_STACK_GUARD_END);
const KERNEL_STACK_GUARD_START: VirtAddr = const_virt_addr(0xffff_8000_5554_f000 as u64);
const KERNEL_STACK_GUARD_END: VirtAddr = const_virt_addr(0xffff_8000_5554_ffff as u64);

/// KERNEL_STACK is the virtual address of the kernel's stack.
///
/// Note that the stack counts downwards, so the start address is larger than
/// the end address.
///
pub const KERNEL_STACK: VirtAddrRange = VirtAddrRange::new(KERNEL_STACK_END, KERNEL_STACK_START);
const KERNEL_STACK_START: VirtAddr = const_virt_addr(0xffff_8000_555c_ffff as u64);
const KERNEL_STACK_END: VirtAddr = const_virt_addr(0xffff_8000_5555_0000 as u64);

/// MMIO_SPACE is the virtual address space used for accessing
/// hardware devices via memory mapped I/O.
///
pub const MMIO_SPACE: VirtAddrRange = VirtAddrRange::new(MMIO_SPACE_START, MMIO_SPACE_END);
const MMIO_SPACE_START: VirtAddr = const_virt_addr(0xffff_8000_6666_0000 as u64);
const MMIO_SPACE_END: VirtAddr = const_virt_addr(0xffff_8000_6675_ffff as u64);

/// PHYSICAL_MEMORY_OFFSET is the virtual address at which the mapping of
/// all physical memory begins. That is, for any valid physical address,
/// that address can be reached at the same virtual address, plus
/// PHYSICAL_MEMORY_OFFSET.
///
pub const PHYSICAL_MEMORY: VirtAddrRange =
    VirtAddrRange::new(PHYSICAL_MEMORY_OFFSET, VIRTUAL_MEMORY_END);
pub const PHYSICAL_MEMORY_OFFSET: VirtAddr = const_virt_addr(0xffff_8000_8000_0000 as u64);
const VIRTUAL_MEMORY_END: VirtAddr = const_virt_addr(0xffff_ffff_ffff_ffff as u64);

/// phys_to_virt_addr returns a virtual address that is mapped to the
/// given physical address. This uses the mapping of all physical memory
/// at the virtual address PHYSICAL_MEMORY_OFFSET.
///
pub fn phys_to_virt_addr(phys: PhysAddr) -> VirtAddr {
    PHYSICAL_MEMORY_OFFSET + phys.as_u64()
}

/// VirtAddrRange represents a contiguous sequence of virtual addresses.
///
pub struct VirtAddrRange {
    first: VirtAddr,
    last: VirtAddr,
}

impl VirtAddrRange {
    /// new returns a new range, from start to end.
    ///
    /// new will panic if start is not smaller than end.
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

    /// start returns the first address in the range.
    ///
    pub const fn start(&self) -> VirtAddr {
        self.first
    }

    /// end returns the last address in the range.
    ///
    pub const fn end(&self) -> VirtAddr {
        self.last
    }

    /// size returns the number of addresses in the range.
    ///
    pub const fn size(&self) -> u64 {
        (self.last.as_u64() + 1u64) - self.first.as_u64()
    }

    /// contains returns whether the given address range
    /// exists entirely within (or is equal to) this
    /// range.
    ///
    pub const fn contains(&self, other: &VirtAddrRange) -> bool {
        self.first.as_u64() <= other.first.as_u64() && other.last.as_u64() <= self.last.as_u64()
    }

    /// contains_addr returns whether the given address
    /// exists in this range
    ///
    pub const fn contains_addr(&self, other: VirtAddr) -> bool {
        self.first.as_u64() <= other.as_u64() && other.as_u64() <= self.last.as_u64()
    }

    /// contains_range returns whether the given start and
    /// end addresses both exist within the range...
    ///
    pub const fn contains_range(&self, other_start: VirtAddr, other_end: VirtAddr) -> bool {
        other_start.as_u64() < other_end.as_u64()
            && self.contains_addr(other_start)
            && self.contains_addr(other_end)
    }
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
