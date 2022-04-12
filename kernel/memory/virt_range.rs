// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

use crate::VirtAddr;

/// Represents a contiguous sequence of virtual addresses.
///
/// A range has no alignment requirements, such as adhering
/// to page boundaries.
///
pub struct VirtAddrRange {
    start: VirtAddr,
    end: VirtAddr,
}

impl VirtAddrRange {
    /// Returns a new inclusive range, from `start` to `end`.
    ///
    /// # Panics
    ///
    /// `new` will panic if `start` is greater than `end`.
    ///
    #[inline]
    #[track_caller]
    pub const fn new(start: VirtAddr, end: VirtAddr) -> Self {
        if start.as_usize() > end.as_usize() {
            panic!("invalid virtual address range: start is greather than end");
        }

        VirtAddrRange { start, end }
    }

    /// Returns the first address in the range.
    ///
    #[inline]
    pub const fn start(&self) -> VirtAddr {
        self.start
    }

    /// Returns the last address in the range.
    ///
    #[inline]
    pub const fn end(&self) -> VirtAddr {
        self.end
    }

    /// Returns the number of addresses in the range.
    ///
    #[inline]
    pub const fn size(&self) -> usize {
        // We already know `end` is not smaller than
        // `start`, so the subtraction is safe. We add
        // one afterwards to make sure we avoid overflow.
        (self.end.as_usize() - self.start.as_usize()) + 1
    }

    /// Returns whether the given address range exists
    /// entirely within (or is equal to) this range.
    ///
    pub const fn contains(&self, other: &Self) -> bool {
        self.start.as_usize() <= other.start.as_usize()
            && other.end.as_usize() <= self.end.as_usize()
    }

    /// Returns whether the given address exists in this
    /// range.
    ///
    pub const fn contains_addr(&self, other: VirtAddr) -> bool {
        self.start.as_usize() <= other.as_usize() && other.as_usize() <= self.end.as_usize()
    }

    /// Returns whether the given `start` and `end`
    /// addresses both exist within this range.
    ///
    pub const fn contains_range(&self, other_start: VirtAddr, other_end: VirtAddr) -> bool {
        other_start.as_usize() < other_end.as_usize()
            && self.contains_addr(other_start)
            && self.contains_addr(other_end)
    }

    /// Returns whether the given address range has any
    /// overlap with this range.
    ///
    pub const fn overlaps_with(&self, other: &VirtAddrRange) -> bool {
        self.contains_addr(other.start) || self.contains_addr(other.end) || other.contains(self)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_virt_addr_range() {
        let start = VirtAddr::new(12);
        let end = VirtAddr::new(15);
        let range = VirtAddrRange::new(start, end);
        let subset_start = VirtAddrRange::new(start, end - 1usize);
        let subset_middle = VirtAddrRange::new(start + 1usize, end - 1usize);
        let subset_end = VirtAddrRange::new(start + 1usize, end);
        let overlap_start = VirtAddrRange::new(start - 1usize, end);
        let overlap_end = VirtAddrRange::new(start, end + 1usize);
        let superset = VirtAddrRange::new(start - 1usize, end + 1usize);

        // Check the range contains the values
        // we expect.
        assert_eq!(range.start(), start);
        assert_eq!(range.end(), end);
        assert_eq!(range.size(), 4usize); // VirtAddrRange is inclusive.

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
