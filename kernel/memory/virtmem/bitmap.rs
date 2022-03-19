// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! A fixed-size bitmap to track level-4 kernel page mappings.
//!
//! Once the kernel is initialised, the kernel's page mappings are
//! frozen, meaning that no changes can be made to the page mappings
//! affecting kernel space (the top half of memory), if they would
//! result in a change to the level 4 page table. This is to ensure
//! that we can safely create new page tables by making a copy of
//! the level 4 page table only. This means that new page mappings
//! can be made in lower-level page tables, as those tables are
//! shared across all level 4 page tables.
//!
//! Level 4 page tables can (and will) differ in their mappings in
//! userspace, but they should all agree on kernelspace.
//!
//! To ensure this is true, once the kernel page mappings have been
//! frozen, we check that each new mapping that is in kernelspace
//! would use a level 4 page mapping that is already present. To
//! make this check efficient, we store a copy of the level 4 mappings
//! for kernelspace when we freeze the page mappings, storing a
//! bitmap for each page in kernelspace, indicating whether each page
//! is present or not.
//!
//! We could use the existing bitmap implementation in `bitmap_index`,
//! as by the time the page mappings are frozen, the kernel's heap
//! will have been initialised. However, it uses a vector to allow
//! a dynamic size, which is unnecessary for this use case. Here,
//! the bitmap will always represent the top half of memory, which
//! has a fixed size. This means we can use an array, rather than
//! a vector and a constant constructor.
//!
//! Page tables are indexed by 9 bits of the virtual address, which
//! means each table contains 512 entries. Since we only care about
//! the top half for this case, we need 256 entries.

use memlayout::KERNELSPACE;
use x86_64::structures::paging::Page;
use x86_64::VirtAddr;

/// The number of page table entries covering the higher half of
/// memory.
///
const NUM_BITS: usize = 512 / 2;

/// The number of u64 values we need to store the bitmap.
///
const NUM_U64S: usize = NUM_BITS / 64;

/// Returns the index into the bitmap and the `u64` mask for
/// the given page, or the start address if the page is not
/// in kernelspace.
///
fn get_index(page: &Page) -> Result<(usize, u64), VirtAddr> {
    let start = page.start_address();
    if !KERNELSPACE.contains_addr(start) {
        Err(start)
    } else {
        // Extract the level 4 page table index.
        let pml4_index = 511 & ((start.as_u64() as usize) >> 39);

        // Drop it down to the lower half.
        let half_index = pml4_index - 256;

        // Work out which u64 we should be using.
        let u64_index = half_index / 64;

        // Work out the mask into the u64 we should use.
        let u64_mask = 1u64 << (half_index % 64) as u64;

        Ok((u64_index, u64_mask))
    }
}

/// A simple bitmap implementation, specialised to tracking whether
/// each page is mapped in kernelspace in the level 4 page table.
///
pub struct BitmapLevel4KernelMappings {
    bits: [u64; NUM_U64S],
}

impl BitmapLevel4KernelMappings {
    /// Returns a new bitmap with all pages marked unmapped.
    ///
    pub const fn new() -> Self {
        BitmapLevel4KernelMappings {
            bits: [0u64; NUM_U64S],
        }
    }

    /// Mark the given page as mapped in the bitmap.
    ///
    /// # Panics
    ///
    /// `map` will panic if the page is not in kernelspace.
    ///
    pub fn map(&mut self, page: &Page) {
        match get_index(page) {
            Ok((index, mask)) => self.bits[index] |= mask,
            Err(start) => panic!("page at {:p} is not in kernelspace", start),
        }
    }

    /// Returns whether the given page is mapped.
    ///
    /// # Panics
    ///
    /// `mapped` will panic if the page is not in kernelspace.
    ///
    pub fn mapped(&self, page: &Page) -> bool {
        match get_index(page) {
            Ok((index, mask)) => (self.bits[index] & mask) == mask,
            Err(start) => panic!("page at {:p} is not in kernelspace", start),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn bitmap_level4_kernel_mappings() {
        // Helper function to speed up making pages.
        fn page_for(addr: u64) -> Page {
            let start_addr = VirtAddr::new(addr);
            Page::from_start_address(start_addr).unwrap()
        }

        const NUL_PAGE: u64 = 0x0000_0000_0000_0000_u64;
        const LOW_PAGE: u64 = 0x0000_0000_0000_4000_u64;
        const HALF_PAGE: u64 = 0xffff_8000_0000_0000_u64;
        const NEXT_PAGE: u64 = 0xffff_8080_0000_0000_u64;
        const LAST_PAGE: u64 = 0xffff_ffff_ffff_f000_u64;

        // First, check that `get_index` works correctly.
        assert_eq!(get_index(&page_for(NUL_PAGE)), Err(VirtAddr::new(NUL_PAGE)));
        assert_eq!(get_index(&page_for(LOW_PAGE)), Err(VirtAddr::new(LOW_PAGE)));
        assert_eq!(get_index(&page_for(HALF_PAGE)), Ok((0, 1)));
        assert_eq!(get_index(&page_for(NEXT_PAGE)), Ok((0, 2)));
        assert_eq!(get_index(&page_for(LAST_PAGE)), Ok((NUM_U64S - 1, 1 << 63)));

        // Next, check that `map` and `mapped` agree.
        let mut bitmap = BitmapLevel4KernelMappings::new();
        assert!(!bitmap.mapped(&page_for(HALF_PAGE)));
        bitmap.map(&page_for(HALF_PAGE));
        assert!(bitmap.mapped(&page_for(HALF_PAGE)));

        assert!(!bitmap.mapped(&page_for(LAST_PAGE)));
        bitmap.map(&page_for(LAST_PAGE));
        assert!(bitmap.mapped(&page_for(LAST_PAGE)));
    }
}
