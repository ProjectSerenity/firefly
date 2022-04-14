// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

use crate::{InvalidVirtAddr, PhysFrameSize, VirtAddr};
use core::cmp::{Ordering, PartialEq, PartialOrd};
use core::fmt;
use core::ops::{Add, AddAssign, Sub, SubAssign};

/// A bitmask for the largest virtual address.
///
const MAX_VIRT_ADDR: usize = 0xffff_ffff_ffff_ffff;

/// Describes the size of a page of virtual
/// memory.
///
/// The values available in `VirtPageSize` will
/// vary, depending on the target platform.
///
#[derive(Clone, Copy, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
pub enum VirtPageSize {
    Size4KiB,
    Size2MiB,
    Size1GiB,
}

impl VirtPageSize {
    /// Returns a bitmask for the start address of
    /// a page of this size.
    ///
    pub const fn start_mask(&self) -> usize {
        MAX_VIRT_ADDR & !(self.bytes() - 1)
    }

    /// Returns the page size in bytes.
    ///
    pub const fn bytes(&self) -> usize {
        match self {
            VirtPageSize::Size4KiB => 0x1000_usize,
            VirtPageSize::Size2MiB => 0x20_0000_usize,
            VirtPageSize::Size1GiB => 0x4000_0000_usize,
        }
    }

    /// Returns the corresponding physical
    /// frame size.
    ///
    pub const fn phys_frame_size(&self) -> PhysFrameSize {
        match self {
            VirtPageSize::Size4KiB => PhysFrameSize::Size4KiB,
            VirtPageSize::Size2MiB => PhysFrameSize::Size2MiB,
            VirtPageSize::Size1GiB => PhysFrameSize::Size1GiB,
        }
    }
}

impl fmt::Display for VirtPageSize {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            VirtPageSize::Size4KiB => write!(f, "4 KiB"),
            VirtPageSize::Size2MiB => write!(f, "2 MiB"),
            VirtPageSize::Size1GiB => write!(f, "1 GiB"),
        }
    }
}

impl PartialEq<PhysFrameSize> for VirtPageSize {
    fn eq(&self, other: &PhysFrameSize) -> bool {
        self.bytes() == other.bytes()
    }
}

impl PartialOrd<PhysFrameSize> for VirtPageSize {
    fn partial_cmp(&self, other: &PhysFrameSize) -> Option<Ordering> {
        self.bytes().partial_cmp(&other.bytes())
    }
}

/// A page of virtual memory.
///
#[derive(Clone, Copy, Eq, Hash, PartialEq)]
pub struct VirtPage {
    start_addr: VirtAddr,
    size: VirtPageSize,
}

impl VirtPage {
    /// Returns the page that starts at the
    /// given virtual address.
    ///
    /// Returns an error if the address is
    /// not aligned to the page size.
    ///
    #[inline]
    pub const fn from_start_address(
        start_addr: VirtAddr,
        size: VirtPageSize,
    ) -> Result<Self, InvalidVirtAddr> {
        if start_addr.is_aligned(size.bytes()) {
            Ok(VirtPage { start_addr, size })
        } else {
            Err(InvalidVirtAddr(start_addr.as_usize()))
        }
    }

    /// Returns the page that starts at the
    /// given virtual address.
    ///
    /// # Safety
    ///
    /// The address must be aligned to the
    /// page size, but this is not checked.
    ///
    #[inline]
    pub const unsafe fn from_start_address_unchecked(
        start_addr: VirtAddr,
        size: VirtPageSize,
    ) -> Self {
        VirtPage { start_addr, size }
    }

    /// Returns the page that contains the
    /// given virtual address.
    ///
    #[inline]
    pub const fn containing_address(addr: VirtAddr, size: VirtPageSize) -> Self {
        VirtPage {
            start_addr: addr.align_down(size.bytes()),
            size,
        }
    }

    /// Returns the first address in the
    /// page.
    ///
    #[inline]
    pub const fn start_address(&self) -> VirtAddr {
        self.start_addr
    }

    /// Returns the last address in the
    /// page.
    ///
    #[inline]
    pub const fn end_address(&self) -> VirtAddr {
        VirtAddr::new(self.start_addr.as_usize() | (self.size.bytes() - 1))
    }

    /// Returns the page size.
    ///
    #[inline]
    pub const fn size(&self) -> VirtPageSize {
        self.size
    }

    /// Returns whether `addr` exists within
    /// this page.
    ///
    #[inline]
    pub const fn contains(&self, addr: VirtAddr) -> bool {
        // We compare the underlying usize values
        // to remain constant.
        self.start_addr.as_usize() <= addr.as_usize()
            && addr.as_usize() <= self.end_address().as_usize()
    }

    /// Returns an exclusive page range
    /// of `[start, end)`.
    ///
    /// # Panics
    ///
    /// `range_exclusive` will panic if
    /// `start` and `end` are not of the
    /// same size.
    ///
    #[inline]
    #[track_caller]
    pub fn range_exclusive(start: Self, end: Self) -> VirtPageRange {
        assert_eq!(start.size, end.size);
        VirtPageRange {
            start,
            end: end - 1,
        }
    }

    /// Returns an inclusive page range
    /// of `[start, end]`.
    ///
    /// # Panics
    ///
    /// `range_inclusive` will panic if
    /// `start` and `end` are not of the
    /// same size.
    ///
    #[inline]
    #[track_caller]
    pub fn range_inclusive(start: Self, end: Self) -> VirtPageRange {
        assert_eq!(start.size, end.size);
        VirtPageRange { start, end }
    }
}

impl fmt::Debug for VirtPage {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        f.debug_struct("VirtPage")
            .field("size", &format_args!("{}", self.size))
            .field("start", &format_args!("{:p}", self.start_address()))
            .field("end", &format_args!("{:p}", self.end_address()))
            .finish()
    }
}

// Mathematical operators.

impl Add<usize> for VirtPage {
    type Output = Self;

    #[inline]
    #[track_caller]
    fn add(self, rhs: usize) -> Self::Output {
        VirtPage::containing_address(self.start_address() + rhs * self.size.bytes(), self.size)
    }
}

impl AddAssign<usize> for VirtPage {
    #[inline]
    #[track_caller]
    fn add_assign(&mut self, rhs: usize) {
        *self = *self + rhs;
    }
}

impl Sub<usize> for VirtPage {
    type Output = Self;

    #[inline]
    #[track_caller]
    fn sub(self, rhs: usize) -> Self::Output {
        VirtPage::containing_address(self.start_address() - rhs * self.size.bytes(), self.size)
    }
}

impl SubAssign<usize> for VirtPage {
    #[inline]
    #[track_caller]
    fn sub_assign(&mut self, rhs: usize) {
        *self = *self - rhs;
    }
}

/// A range of contiguous virtual
/// memory pages.
///
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct VirtPageRange {
    start: VirtPage,
    end: VirtPage,
}

impl VirtPageRange {
    /// Returns whether this range
    /// contains no pages.
    ///
    #[inline]
    pub fn is_empty(&self) -> bool {
        self.start.start_address() > self.end.start_address()
    }

    /// Returns the first page in
    /// the range.
    ///
    #[inline]
    pub const fn start(&self) -> VirtPage {
        self.start
    }

    /// Returns the last page in
    /// the range.
    ///
    #[inline]
    pub const fn end(&self) -> VirtPage {
        self.end
    }

    /// Returns the first address in the
    /// page range.
    ///
    #[inline]
    pub const fn start_address(&self) -> VirtAddr {
        self.start.start_address()
    }

    /// Returns the last address in the
    /// page range.
    ///
    #[inline]
    pub const fn end_address(&self) -> VirtAddr {
        self.end.end_address()
    }
}

impl Iterator for VirtPageRange {
    type Item = VirtPage;

    #[inline]
    fn next(&mut self) -> Option<Self::Item> {
        if self.is_empty() {
            None
        } else {
            let page = self.start;
            // Take care to avoid an invalid address.
            let start_addr = self.start.start_address().as_usize();
            let increase = self.start.size.bytes();
            if start_addr.checked_add(increase).is_some()
                && VirtAddr::try_new(start_addr + increase).is_ok()
            {
                self.start += 1;
            } else {
                self.end -= 1;
            }

            Some(page)
        }
    }
}

#[cfg(test)]
mod test {
    use super::*;

    #[test]
    fn test_page() {
        let addr = VirtAddr::new(0x1234_5678_9abc_usize);

        // 4 KiB.
        let page = VirtPage::containing_address(addr, VirtPageSize::Size4KiB);
        assert_eq!(page.start_address(), VirtAddr::new(0x1234_5678_9000_usize));
        assert_eq!(page.end_address(), VirtAddr::new(0x1234_5678_9fff_usize));
        assert_eq!(
            page - 1,
            VirtPage::from_start_address(
                VirtAddr::new(0x1234_5678_8000_usize),
                VirtPageSize::Size4KiB
            )
            .unwrap()
        );
        assert_eq!(
            page + 1,
            VirtPage::from_start_address(
                VirtAddr::new(0x1234_5678_a000_usize),
                VirtPageSize::Size4KiB
            )
            .unwrap()
        );

        // 2 MiB.
        let page = VirtPage::containing_address(addr, VirtPageSize::Size2MiB);
        assert_eq!(page.start_address(), VirtAddr::new(0x1234_5660_0000_usize));
        assert_eq!(page.end_address(), VirtAddr::new(0x1234_567f_ffff_usize));
        assert_eq!(
            page - 1,
            VirtPage::from_start_address(
                VirtAddr::new(0x1234_5640_0000_usize),
                VirtPageSize::Size2MiB
            )
            .unwrap()
        );
        assert_eq!(
            page + 1,
            VirtPage::from_start_address(
                VirtAddr::new(0x1234_5680_0000_usize),
                VirtPageSize::Size2MiB
            )
            .unwrap()
        );

        // 1 GiB.
        let page = VirtPage::containing_address(addr, VirtPageSize::Size1GiB);
        assert_eq!(page.start_address(), VirtAddr::new(0x1234_4000_0000_usize));
        assert_eq!(page.end_address(), VirtAddr::new(0x1234_7fff_ffff_usize));
        assert_eq!(
            page - 1,
            VirtPage::from_start_address(
                VirtAddr::new(0x1234_0000_0000_usize),
                VirtPageSize::Size1GiB
            )
            .unwrap()
        );
        assert_eq!(
            page + 1,
            VirtPage::from_start_address(
                VirtAddr::new(0x1234_8000_0000_usize),
                VirtPageSize::Size1GiB
            )
            .unwrap()
        );
    }

    struct VirtPageRangeTest {
        name: &'static str,
        want: [VirtPage; 4],
    }

    impl VirtPageRangeTest {
        fn new_upwards(name: &'static str, size: VirtPageSize, first: usize) -> Self {
            let want = [
                VirtPage::from_start_address(VirtAddr::new(first + 0 * size.bytes()), size)
                    .expect(name),
                VirtPage::from_start_address(VirtAddr::new(first + 1 * size.bytes()), size)
                    .expect(name),
                VirtPage::from_start_address(VirtAddr::new(first + 2 * size.bytes()), size)
                    .expect(name),
                VirtPage::from_start_address(VirtAddr::new(first + 3 * size.bytes()), size)
                    .expect(name),
            ];

            VirtPageRangeTest { name, want }
        }

        fn new_downwards(name: &'static str, size: VirtPageSize, last: usize) -> Self {
            let want = [
                VirtPage::from_start_address(VirtAddr::new(last - 3 * size.bytes()), size)
                    .expect(name),
                VirtPage::from_start_address(VirtAddr::new(last - 2 * size.bytes()), size)
                    .expect(name),
                VirtPage::from_start_address(VirtAddr::new(last - 1 * size.bytes()), size)
                    .expect(name),
                VirtPage::from_start_address(VirtAddr::new(last - 0 * size.bytes()), size)
                    .expect(name),
            ];

            VirtPageRangeTest { name, want }
        }
    }

    #[test]
    fn test_page_range() {
        // Check the four corner cases; ranges
        // bounding on:
        // 1. The null page.
        // 2. The largest lower half page.
        // 3. The smallest higher half page.
        // 4. The largest higher half page.
        let tests = [
            VirtPageRangeTest::new_upwards("simple", VirtPageSize::Size4KiB, 0x1000),
            VirtPageRangeTest::new_upwards("null start 4 KiB", VirtPageSize::Size4KiB, 0),
            VirtPageRangeTest::new_upwards("null start 2 MiB", VirtPageSize::Size2MiB, 0),
            VirtPageRangeTest::new_upwards("null start 1 GiB", VirtPageSize::Size1GiB, 0),
            VirtPageRangeTest::new_downwards(
                "largest lower half 4 KiB",
                VirtPageSize::Size4KiB,
                0x8000_0000_0000 - VirtPageSize::Size4KiB.bytes(),
            ),
            VirtPageRangeTest::new_downwards(
                "largest lower half 2 MiB",
                VirtPageSize::Size2MiB,
                0x8000_0000_0000 - VirtPageSize::Size2MiB.bytes(),
            ),
            VirtPageRangeTest::new_downwards(
                "largest lower half 1 GiB",
                VirtPageSize::Size1GiB,
                0x8000_0000_0000 - VirtPageSize::Size1GiB.bytes(),
            ),
            VirtPageRangeTest::new_upwards(
                "smallest higher half 4 KiB",
                VirtPageSize::Size4KiB,
                0xffff_8000_0000_0000,
            ),
            VirtPageRangeTest::new_upwards(
                "smallest higher half 2 MiB",
                VirtPageSize::Size2MiB,
                0xffff_8000_0000_0000,
            ),
            VirtPageRangeTest::new_upwards(
                "smallest higher half 1 GiB",
                VirtPageSize::Size1GiB,
                0xffff_8000_0000_0000,
            ),
            VirtPageRangeTest::new_downwards(
                "largest higher half 4 KiB",
                VirtPageSize::Size4KiB,
                0_usize.wrapping_sub(VirtPageSize::Size4KiB.bytes()),
            ),
            VirtPageRangeTest::new_downwards(
                "largest higher half 2 MiB",
                VirtPageSize::Size2MiB,
                0_usize.wrapping_sub(VirtPageSize::Size2MiB.bytes()),
            ),
            VirtPageRangeTest::new_downwards(
                "largest higher half 1 GiB",
                VirtPageSize::Size1GiB,
                0_usize.wrapping_sub(VirtPageSize::Size1GiB.bytes()),
            ),
        ];

        for (i, test) in tests.iter().enumerate() {
            // Exclusive.
            let mut range = VirtPage::range_exclusive(test.want[0], test.want[3]);
            assert_eq!(range.next(), Some(test.want[0]), "{} {}", i, test.name);
            assert_eq!(range.next(), Some(test.want[1]), "{} {}", i, test.name);
            assert_eq!(range.next(), Some(test.want[2]), "{} {}", i, test.name);
            assert_eq!(range.next(), None, "{} {}", i, test.name);

            // Inclusive.
            let mut range = VirtPage::range_inclusive(test.want[0], test.want[3]);
            assert_eq!(range.next(), Some(test.want[0]), "{} {}", i, test.name);
            assert_eq!(range.next(), Some(test.want[1]), "{} {}", i, test.name);
            assert_eq!(range.next(), Some(test.want[2]), "{} {}", i, test.name);
            assert_eq!(range.next(), Some(test.want[3]), "{} {}", i, test.name);
            assert_eq!(range.next(), None, "{} {}", i, test.name);
        }
    }

    #[test]
    fn test_page_size_masks() {
        // Check the max virt addr is the maximum 64-bit value.
        assert_eq!(MAX_VIRT_ADDR.wrapping_add(1), 0);

        // A 4 KiB page.
        assert_eq!(VirtPageSize::Size4KiB.start_mask(), 0xffff_ffff_ffff_f000);

        // A 2 MiB page.
        assert_eq!(VirtPageSize::Size2MiB.start_mask(), 0xffff_ffff_ffe0_0000);

        // A 1 GiB page.
        assert_eq!(VirtPageSize::Size1GiB.start_mask(), 0xffff_ffff_c000_0000);
    }
}
