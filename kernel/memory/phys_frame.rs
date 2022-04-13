// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

use crate::{InvalidPhysAddr, PhysAddr, VirtPageSize};
use core::cmp::{Ordering, PartialEq, PartialOrd};
use core::fmt;
use core::ops::{Add, AddAssign, Sub, SubAssign};

/// A bitmask for the largest physical address.
///
const MAX_PHYS_ADDR: usize = 0x000f_ffff_ffff_ffff;

/// Describes the size of a frame of physical
/// memory.
///
/// The values available in `PhysFrameSize` will
/// vary, depending on the target platform.
///
#[derive(Clone, Copy, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
pub enum PhysFrameSize {
    Size4KiB,
    Size2MiB,
    Size1GiB,
}

impl PhysFrameSize {
    /// Returns a bitmask for the start address of
    /// a frame of this size.
    ///
    pub const fn start_mask(&self) -> usize {
        MAX_PHYS_ADDR & !(self.bytes() - 1)
    }

    /// Returns the frame size in bytes.
    ///
    pub const fn bytes(&self) -> usize {
        match self {
            PhysFrameSize::Size4KiB => 0x1000_usize,
            PhysFrameSize::Size2MiB => 0x20_0000_usize,
            PhysFrameSize::Size1GiB => 0x4000_0000_usize,
        }
    }

    /// Returns the corresponding virtual
    /// page size.
    ///
    pub const fn virt_page_size(&self) -> VirtPageSize {
        match self {
            PhysFrameSize::Size4KiB => VirtPageSize::Size4KiB,
            PhysFrameSize::Size2MiB => VirtPageSize::Size2MiB,
            PhysFrameSize::Size1GiB => VirtPageSize::Size1GiB,
        }
    }
}

impl fmt::Display for PhysFrameSize {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            PhysFrameSize::Size4KiB => write!(f, "4 KiB"),
            PhysFrameSize::Size2MiB => write!(f, "2 MiB"),
            PhysFrameSize::Size1GiB => write!(f, "1 GiB"),
        }
    }
}

impl PartialEq<VirtPageSize> for PhysFrameSize {
    fn eq(&self, other: &VirtPageSize) -> bool {
        self.bytes() == other.bytes()
    }
}

impl PartialOrd<VirtPageSize> for PhysFrameSize {
    fn partial_cmp(&self, other: &VirtPageSize) -> Option<Ordering> {
        self.bytes().partial_cmp(&other.bytes())
    }
}

/// A frame of physical memory.
///
#[derive(Clone, Copy, Eq, Hash, PartialEq)]
pub struct PhysFrame {
    start_addr: PhysAddr,
    size: PhysFrameSize,
}

impl PhysFrame {
    /// Returns the frame that starts at the
    /// given physical address.
    ///
    /// Returns an error if the address is
    /// not aligned to the frame size.
    ///
    #[inline]
    pub const fn from_start_address(
        start_addr: PhysAddr,
        size: PhysFrameSize,
    ) -> Result<Self, InvalidPhysAddr> {
        if start_addr.is_aligned(size.bytes()) {
            Ok(PhysFrame { start_addr, size })
        } else {
            Err(InvalidPhysAddr(start_addr.as_usize()))
        }
    }

    /// Returns the frame that starts at the
    /// given physical address.
    ///
    /// # Safety
    ///
    /// The address must be aligned to the
    /// frame size, but this is not checked.
    ///
    #[inline]
    pub const unsafe fn from_start_address_unchecked(
        start_addr: PhysAddr,
        size: PhysFrameSize,
    ) -> Self {
        PhysFrame { start_addr, size }
    }

    /// Returns the frame that contains the
    /// given physical address.
    ///
    #[inline]
    pub const fn containing_address(addr: PhysAddr, size: PhysFrameSize) -> Self {
        PhysFrame {
            start_addr: addr.align_down(size.bytes()),
            size,
        }
    }

    /// Returns the first address in the
    /// frame.
    ///
    #[inline]
    pub const fn start_address(&self) -> PhysAddr {
        self.start_addr
    }

    /// Returns the last address in the
    /// frame.
    ///
    #[inline]
    pub const fn end_address(&self) -> PhysAddr {
        PhysAddr::new(self.start_addr.as_usize() | (self.size.bytes() - 1))
    }

    /// Returns the frame size.
    ///
    #[inline]
    pub const fn size(&self) -> PhysFrameSize {
        self.size
    }

    /// Returns an exclusive frame range
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
    pub fn range_exclusive(start: Self, end: Self) -> PhysFrameRange {
        assert_eq!(start.size, end.size);
        PhysFrameRange {
            start,
            end: end - 1,
        }
    }

    /// Returns an inclusive frame range
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
    pub fn range_inclusive(start: Self, end: Self) -> PhysFrameRange {
        assert_eq!(start.size, end.size);
        PhysFrameRange { start, end }
    }
}

impl fmt::Debug for PhysFrame {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        f.debug_struct("PhysFrame")
            .field("size", &format_args!("{}", self.size))
            .field("start", &format_args!("{:p}", self.start_address()))
            .field("end", &format_args!("{:p}", self.end_address()))
            .finish()
    }
}

// Mathematical operators.

impl Add<usize> for PhysFrame {
    type Output = Self;

    #[inline]
    #[track_caller]
    fn add(self, rhs: usize) -> Self::Output {
        PhysFrame::containing_address(self.start_address() + rhs * self.size.bytes(), self.size)
    }
}

impl AddAssign<usize> for PhysFrame {
    #[inline]
    #[track_caller]
    fn add_assign(&mut self, rhs: usize) {
        *self = *self + rhs;
    }
}

impl Sub<usize> for PhysFrame {
    type Output = Self;

    #[inline]
    #[track_caller]
    fn sub(self, rhs: usize) -> Self::Output {
        PhysFrame::containing_address(self.start_address() - rhs * self.size.bytes(), self.size)
    }
}

impl SubAssign<usize> for PhysFrame {
    #[inline]
    #[track_caller]
    fn sub_assign(&mut self, rhs: usize) {
        *self = *self - rhs;
    }
}

/// A range of contiguous physical
/// memory frames.
///
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct PhysFrameRange {
    start: PhysFrame,
    end: PhysFrame,
}

impl PhysFrameRange {
    /// Returns whether this range
    /// contains no frames.
    ///
    #[inline]
    pub fn is_empty(&self) -> bool {
        self.start.start_address() > self.end.start_address()
    }

    /// Returns the first page in
    /// the range.
    ///
    #[inline]
    pub const fn start(&self) -> PhysFrame {
        self.start
    }

    /// Returns the last page in
    /// the range.
    ///
    #[inline]
    pub const fn end(&self) -> PhysFrame {
        self.end
    }

    /// Returns the first address in the
    /// frame range.
    ///
    #[inline]
    pub const fn start_address(&self) -> PhysAddr {
        self.start.start_address()
    }

    /// Returns the last address in the
    /// frame range.
    ///
    #[inline]
    pub const fn end_address(&self) -> PhysAddr {
        self.end.end_address()
    }
}

impl Iterator for PhysFrameRange {
    type Item = PhysFrame;

    #[inline]
    fn next(&mut self) -> Option<Self::Item> {
        if self.is_empty() {
            None
        } else {
            let frame = self.start;
            // Take care to avoid an invalid address.
            let start_addr = self.start.start_address().as_usize();
            let increase = self.start.size.bytes();
            if start_addr.checked_add(increase).is_some()
                && PhysAddr::try_new(start_addr + increase).is_ok()
            {
                self.start += 1;
            } else {
                self.end -= 1;
            }

            Some(frame)
        }
    }
}

#[cfg(test)]
mod test {
    use super::*;

    #[test]
    fn test_frame() {
        let addr = PhysAddr::new(0x1234_5678_9abc_usize);

        // 4 KiB.
        let frame = PhysFrame::containing_address(addr, PhysFrameSize::Size4KiB);
        assert_eq!(frame.start_address(), PhysAddr::new(0x1234_5678_9000_usize));
        assert_eq!(frame.end_address(), PhysAddr::new(0x1234_5678_9fff_usize));
        assert_eq!(
            frame - 1,
            PhysFrame::from_start_address(
                PhysAddr::new(0x1234_5678_8000_usize),
                PhysFrameSize::Size4KiB
            )
            .unwrap()
        );
        assert_eq!(
            frame + 1,
            PhysFrame::from_start_address(
                PhysAddr::new(0x1234_5678_a000_usize),
                PhysFrameSize::Size4KiB
            )
            .unwrap()
        );

        // 2 MiB.
        let frame = PhysFrame::containing_address(addr, PhysFrameSize::Size2MiB);
        assert_eq!(frame.start_address(), PhysAddr::new(0x1234_5660_0000_usize));
        assert_eq!(frame.end_address(), PhysAddr::new(0x1234_567f_ffff_usize));
        assert_eq!(
            frame - 1,
            PhysFrame::from_start_address(
                PhysAddr::new(0x1234_5640_0000_usize),
                PhysFrameSize::Size2MiB
            )
            .unwrap()
        );
        assert_eq!(
            frame + 1,
            PhysFrame::from_start_address(
                PhysAddr::new(0x1234_5680_0000_usize),
                PhysFrameSize::Size2MiB
            )
            .unwrap()
        );

        // 1 GiB.
        let frame = PhysFrame::containing_address(addr, PhysFrameSize::Size1GiB);
        assert_eq!(frame.start_address(), PhysAddr::new(0x1234_4000_0000_usize));
        assert_eq!(frame.end_address(), PhysAddr::new(0x1234_7fff_ffff_usize));
        assert_eq!(
            frame - 1,
            PhysFrame::from_start_address(
                PhysAddr::new(0x1234_0000_0000_usize),
                PhysFrameSize::Size1GiB
            )
            .unwrap()
        );
        assert_eq!(
            frame + 1,
            PhysFrame::from_start_address(
                PhysAddr::new(0x1234_8000_0000_usize),
                PhysFrameSize::Size1GiB
            )
            .unwrap()
        );
    }

    struct PhysFrameRangeTest {
        name: &'static str,
        want: [PhysFrame; 4],
    }

    impl PhysFrameRangeTest {
        fn new_upwards(name: &'static str, size: PhysFrameSize, first: usize) -> Self {
            let want = [
                PhysFrame::from_start_address(PhysAddr::new(first + 0 * size.bytes()), size)
                    .expect(name),
                PhysFrame::from_start_address(PhysAddr::new(first + 1 * size.bytes()), size)
                    .expect(name),
                PhysFrame::from_start_address(PhysAddr::new(first + 2 * size.bytes()), size)
                    .expect(name),
                PhysFrame::from_start_address(PhysAddr::new(first + 3 * size.bytes()), size)
                    .expect(name),
            ];

            PhysFrameRangeTest { name, want }
        }

        fn new_downwards(name: &'static str, size: PhysFrameSize, last: usize) -> Self {
            let want = [
                PhysFrame::from_start_address(PhysAddr::new(last - 3 * size.bytes()), size)
                    .expect(name),
                PhysFrame::from_start_address(PhysAddr::new(last - 2 * size.bytes()), size)
                    .expect(name),
                PhysFrame::from_start_address(PhysAddr::new(last - 1 * size.bytes()), size)
                    .expect(name),
                PhysFrame::from_start_address(PhysAddr::new(last - 0 * size.bytes()), size)
                    .expect(name),
            ];

            PhysFrameRangeTest { name, want }
        }
    }

    #[test]
    fn test_frame_range() {
        // Check the two corner cases; ranges
        // bounding on:
        // 1. The zero frame.
        // 2. The largest physical frame.
        let tests = [
            PhysFrameRangeTest::new_upwards("simple", PhysFrameSize::Size4KiB, 0x1000),
            PhysFrameRangeTest::new_upwards("zero start 4 KiB", PhysFrameSize::Size4KiB, 0),
            PhysFrameRangeTest::new_upwards("zero start 2 MiB", PhysFrameSize::Size2MiB, 0),
            PhysFrameRangeTest::new_upwards("zero start 1 GiB", PhysFrameSize::Size1GiB, 0),
            PhysFrameRangeTest::new_downwards(
                "largest phys addr 4 KiB",
                PhysFrameSize::Size4KiB,
                (MAX_PHYS_ADDR + 1) - PhysFrameSize::Size4KiB.bytes(),
            ),
            PhysFrameRangeTest::new_downwards(
                "largest phys addr 2 MiB",
                PhysFrameSize::Size2MiB,
                (MAX_PHYS_ADDR + 1) - PhysFrameSize::Size2MiB.bytes(),
            ),
            PhysFrameRangeTest::new_downwards(
                "largest phys addr 1 GiB",
                PhysFrameSize::Size1GiB,
                (MAX_PHYS_ADDR + 1) - PhysFrameSize::Size1GiB.bytes(),
            ),
        ];

        for (i, test) in tests.iter().enumerate() {
            // Exclusive.
            let mut range = PhysFrame::range_exclusive(test.want[0], test.want[3]);
            assert_eq!(range.next(), Some(test.want[0]), "{} {}", i, test.name);
            assert_eq!(range.next(), Some(test.want[1]), "{} {}", i, test.name);
            assert_eq!(range.next(), Some(test.want[2]), "{} {}", i, test.name);
            assert_eq!(range.next(), None, "{} {}", i, test.name);

            // Inclusive.
            let mut range = PhysFrame::range_inclusive(test.want[0], test.want[3]);
            assert_eq!(range.next(), Some(test.want[0]), "{} {}", i, test.name);
            assert_eq!(range.next(), Some(test.want[1]), "{} {}", i, test.name);
            assert_eq!(range.next(), Some(test.want[2]), "{} {}", i, test.name);
            assert_eq!(range.next(), Some(test.want[3]), "{} {}", i, test.name);
            assert_eq!(range.next(), None, "{} {}", i, test.name);
        }
    }

    /// The maximum number of bits in a physical address.
    ///
    /// We cannot know statically what this value will be,
    /// but it cannot be larger than 52, according to the
    /// Intel x86_64 manual, volume 3A, section 4.5.
    ///
    const MAX_PHYS_ADDR_BITS: usize = 52;

    fn phys_addr_mask_from(min: usize) -> usize {
        let mut mask = 0;
        for i in min..MAX_PHYS_ADDR_BITS {
            mask |= 1 << i;
        }

        mask
    }

    #[test]
    fn test_frame_size_masks() {
        // Check the mask constants.

        // Any physical address.
        assert_eq!(MAX_PHYS_ADDR, phys_addr_mask_from(0));
        // A 4 KiB frame.
        assert_eq!(
            PhysFrameSize::Size4KiB.start_mask(),
            phys_addr_mask_from(12)
        );

        // A 2 MiB frame.
        assert_eq!(
            PhysFrameSize::Size2MiB.start_mask(),
            phys_addr_mask_from(21)
        );

        // A 1 GiB frame.
        assert_eq!(
            PhysFrameSize::Size1GiB.start_mask(),
            phys_addr_mask_from(30)
        );
    }
}
