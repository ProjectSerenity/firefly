// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides a helper type for representing a range of contiguous
//! virtual addresses.

use x86_64::VirtAddr;

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

    /// Returns whether the given address range has any
    /// overlap with this range.
    ///
    pub const fn overlaps_with(&self, other: &VirtAddrRange) -> bool {
        self.contains_addr(other.first) || self.contains_addr(other.last) || other.contains(self)
    }
}

#[cfg(test)]
mod tests {
    use super::super::const_virt_addr;
    use super::*;

    #[test]
    fn test_virt_addr_range() {
        let start = const_virt_addr(12);
        let end = const_virt_addr(15);
        let range = VirtAddrRange::new(start, end);
        let subset_start = VirtAddrRange::new(start, end - 1u64);
        let subset_middle = VirtAddrRange::new(start + 1u64, end - 1u64);
        let subset_end = VirtAddrRange::new(start + 1u64, end);
        let overlap_start = VirtAddrRange::new(start - 1u64, end);
        let overlap_end = VirtAddrRange::new(start, end + 1u64);
        let superset = VirtAddrRange::new(start - 1u64, end + 1u64);

        // Check the range contains the values
        // we expect.
        assert_eq!(range.start(), start);
        assert_eq!(range.end(), end);
        assert_eq!(range.size(), 4u64); // VirtAddrRange is inclusive.

        // Check range union works properly.
        assert!(range.contains(&range));
        assert!(range.contains(&subset_start));
        assert!(range.contains(&subset_middle));
        assert!(range.contains(&subset_end));
        assert!(!range.contains(&overlap_start));
        assert!(!range.contains(&overlap_end));
        assert!(!range.contains(&superset));

        // Check whether overlap works properly.
        assert!(range.overlaps_with(&range));
        assert!(range.overlaps_with(&subset_start));
        assert!(range.overlaps_with(&subset_middle));
        assert!(range.overlaps_with(&subset_end));
        assert!(range.overlaps_with(&overlap_start));
        assert!(range.overlaps_with(&overlap_end));
        assert!(range.overlaps_with(&superset));
    }
}
