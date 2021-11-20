//! constants contains useful constants for the kernel's memory layout.

use crate::Locked;
use alloc::vec::Vec;
use bootloader::bootinfo::{MemoryMap, MemoryRegion, MemoryRegionType};
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

/// PHYSICAL_MEMORY_MAP contains a map of physical memory, provided by
/// the boot info.
///
static PHYSICAL_MEMORY_MAP: Locked<Vec<MemoryRegion>> = Locked::new(Vec::new());

/// init_memory_map stores the given memory map for later queries.
///
pub fn init_memory_map(map: &MemoryMap) {
    PHYSICAL_MEMORY_MAP.lock().extend(map.iter());
}

/// in_memory_map returns whether the given region includes the given
/// region type.
///
fn in_memory_map(start: PhysAddr, end: PhysAddr, region_type: MemoryRegionType) -> bool {
    let map = PHYSICAL_MEMORY_MAP.lock();
    for region in map.iter() {
        if region.region_type != region_type {
            continue;
        }

        let region_start = PhysAddr::new(region.range.start_addr());
        let region_end = PhysAddr::new(region.range.end_addr());
        if start <= region_start && (region_end - 1u64) <= end
            || region_start <= start && (end - 1u64) <= region_end
        {
            return true;
        }
    }

    false
}

/// boot_info_region returns whether the given region includes boot
/// info data, according to the memory map.
///
pub fn boot_info_region(start: PhysAddr, end: PhysAddr) -> bool {
    in_memory_map(start, end, MemoryRegionType::BootInfo)
}

/// page_table_region returns whether the given region includes page
/// tables, according to the memory map.
///
pub fn page_table_region(start: PhysAddr, end: PhysAddr) -> bool {
    in_memory_map(start, end, MemoryRegionType::PageTable)
}

/// kernel_segment_region returns whether the given region is a kernel
/// segment.
///
pub fn kernel_segment_region(start: PhysAddr, end: PhysAddr) -> bool {
    in_memory_map(start, end, MemoryRegionType::Kernel)
}
