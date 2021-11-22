//! debug includes helper functionality for debugging memory usage.

// This module includes functionality for debugging memory usage. In particular,
// the level_4_table function can be used to iterate through the paging data,
// printing contiguous mappings and their known use.

use crate::memory::vmm::mapping;
use crate::println;
use x86_64::structures::paging::PageTable;

/// level_4_table iterates through a level 4 page
/// table, printing its mappings using print!.
///
/// # Safety
///
/// This function is unsafe because the caller must
/// guarantee that all physical memory is mapped in
/// the given page table.
///
pub unsafe fn level_4_table(pml4: &PageTable) {
    let mappings = mapping::level_4_table(pml4);
    for mapping in mappings.iter() {
        println!("{}", mapping);
    }
}
