// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

use align::{align_down_usize, align_up_usize};
use core::fmt;
use core::ops::{Add, AddAssign, Sub, SubAssign};

/// A physical memory address for the target architecture.
///
/// The properties of the address will vary between each
/// target architecture, but a `PhysAddr` can only store
/// an address that is valid. For example, on x86-64, a
/// `PhysAddr` the top 12 bits are always zero.
///
#[repr(transparent)]
#[derive(Clone, Copy, Eq, Hash, Ord, PartialEq, PartialOrd)]
pub struct PhysAddr(usize);

/// An invalid physical memory address.
///
/// If an attempt is made to create a `PhysAddr` from a
/// value that is not valid on the target platform, then
/// `InvalidPhysAddr` is returned, containing the attempted
/// value.
///
#[repr(transparent)]
#[derive(Clone, Copy, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
pub struct InvalidPhysAddr(pub usize);

impl PhysAddr {
    /// Creates a new physical memory address.
    ///
    /// ## Panics
    ///
    /// `new` will panic if `addr` is not valid for the
    /// target platform.
    ///
    #[inline]
    #[track_caller]
    pub const fn new(addr: usize) -> Self {
        match Self::try_new(addr) {
            Ok(addr) => addr,
            Err(_) => panic!("invalid address passed to PhysAddr::new"),
        }
    }

    /// Tries to create a new physical memory address.
    ///
    /// If the passed value is invalid for the target
    /// platform, then an error is returned.
    ///
    #[inline]
    pub const fn try_new(addr: usize) -> Result<Self, InvalidPhysAddr> {
        // Check that the top 12 bits (52 - 63) are
        // unset (see Intel x86_64 manual, volume 1,
        // section 3.2.1).
        let top_bits = (addr & 0xfff0_0000_0000_0000_usize) >> 52;
        if top_bits == 0 {
            Ok(PhysAddr(addr))
        } else {
            Err(InvalidPhysAddr(addr))
        }
    }

    /// Creates a new physical memory address, without any checks.
    ///
    /// ## Safety
    ///
    /// The caller must ensure `addr` describes a valid physical
    /// address on the target platform.
    ///
    #[inline]
    pub const unsafe fn new_unchecked(addr: usize) -> Self {
        PhysAddr(addr)
    }

    /// Returns the address's numerical value.
    ///
    #[inline]
    pub const fn as_usize(self) -> usize {
        self.0
    }

    // Translation to platform-specific types.

    /// Returns the address described by the
    /// [`x86_64::PhysAddr`].
    ///
    #[inline]
    pub const fn from_x86_64(addr: x86_64::PhysAddr) -> Self {
        PhysAddr::new(addr.as_u64() as usize)
    }

    /// Returns the address as a [`x86_64::PhysAddr`],
    /// for convenience.
    ///
    #[inline]
    #[must_use]
    pub fn as_x86_64(&self) -> x86_64::PhysAddr {
        x86_64::PhysAddr::new(self.0 as u64)
    }

    // Special handling for the zero address.

    /// Returns the zero physical memory address.
    ///
    #[inline]
    pub const fn zero() -> Self {
        PhysAddr(0)
    }

    /// Returns whether this is the zero address.
    ///
    #[inline]
    pub const fn is_zero(self) -> bool {
        self.0 == 0
    }

    // Alignment.

    /// Aligns the physical address downwards
    /// to the largest exact multiple of `align`
    /// that is no larger than the address.
    ///
    /// `align` must be an exact multiple of
    /// two.
    ///
    #[inline]
    #[must_use]
    pub const fn align_down(self, align: usize) -> Self {
        // A change of alignment cannot make a valid
        // address invalid, so we can skip the checks
        // in the constructor and return the result
        // directly.
        PhysAddr(align_down_usize(self.0, align))
    }

    /// Aligns the physical address upwards to
    /// the smallest exact multiple of `align`
    /// that is no smaller than the address.
    ///
    /// `align` must be an exact multiple of
    /// two.
    ///
    #[inline]
    #[must_use]
    pub const fn align_up(self, align: usize) -> Self {
        // A change of alignment cannot make a valid
        // address invalid, so we can skip the checks
        // in the constructor and return the result
        // directly.
        PhysAddr(align_up_usize(self.0, align))
    }

    /// Checks whether the physical address has
    /// the given alignment.
    ///
    /// `align` must be an exact multiple of
    /// two.
    ///
    #[inline]
    pub const fn is_aligned(self, align: usize) -> bool {
        self.align_down(align).0 == self.0
    }

    // Overflow-safe mathematical operations.

    /// Checked integer addition. Computes `self + rhs`,
    /// returning `None` if overflow occurred or if the
    /// result is not a valid physical address.
    ///
    #[inline]
    pub const fn checked_add(self, rhs: usize) -> Option<Self> {
        if let Some(sum) = self.0.checked_add(rhs) {
            if let Ok(addr) = PhysAddr::try_new(sum) {
                Some(addr)
            } else {
                None
            }
        } else {
            None
        }
    }

    /// Checked integer subtraction. Computes `self - rhs`,
    /// returning `None` if overflow occurred or if the
    /// result is not a valid physical address.
    ///
    #[inline]
    pub const fn checked_sub(self, rhs: usize) -> Option<Self> {
        if let Some(sum) = self.0.checked_sub(rhs) {
            if let Ok(addr) = PhysAddr::try_new(sum) {
                Some(addr)
            } else {
                None
            }
        } else {
            None
        }
    }
}

// Formatting.

impl fmt::Binary for PhysAddr {
    #[inline]
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        fmt::Binary::fmt(&self.0, f)
    }
}

impl fmt::Debug for PhysAddr {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        f.debug_tuple("PhysAddr")
            .field(&format_args!("{:p}", self.0 as *const ()))
            .finish()
    }
}

impl fmt::LowerHex for PhysAddr {
    #[inline]
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        fmt::LowerHex::fmt(&self.0, f)
    }
}

impl fmt::Octal for PhysAddr {
    #[inline]
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        fmt::Octal::fmt(&self.0, f)
    }
}

impl fmt::Pointer for PhysAddr {
    #[inline]
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        fmt::Pointer::fmt(&(self.0 as *const ()), f)
    }
}

impl fmt::UpperHex for PhysAddr {
    #[inline]
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        fmt::UpperHex::fmt(&self.0, f)
    }
}

// Mathematical operators.

impl Add<usize> for PhysAddr {
    type Output = Self;

    #[inline]
    #[track_caller]
    fn add(self, rhs: usize) -> Self::Output {
        PhysAddr::new(self.0 + rhs)
    }
}

impl AddAssign<usize> for PhysAddr {
    #[inline]
    #[track_caller]
    fn add_assign(&mut self, rhs: usize) {
        *self = *self + rhs;
    }
}

impl Sub<usize> for PhysAddr {
    type Output = Self;

    #[inline]
    #[track_caller]
    fn sub(self, rhs: usize) -> Self::Output {
        PhysAddr::new(self.0 - rhs)
    }
}

impl SubAssign<usize> for PhysAddr {
    #[inline]
    #[track_caller]
    fn sub_assign(&mut self, rhs: usize) {
        self.0 -= rhs
    }
}

impl Sub<PhysAddr> for PhysAddr {
    type Output = usize;

    #[inline]
    #[track_caller]
    fn sub(self, rhs: PhysAddr) -> Self::Output {
        self.0.checked_sub(rhs.0).unwrap()
    }
}

#[cfg(test)]
mod test {
    use super::*;

    #[test]
    fn test_x86_64() {
        assert_eq!(PhysAddr::new(1).as_usize(), 1_usize);

        // Physical address space.

        // Valid.
        assert_eq!(PhysAddr::try_new(0_usize), Ok(PhysAddr(0_usize)));
        assert_eq!(
            PhysAddr::try_new(0x000f_ffff_ffff_ffff_usize),
            Ok(PhysAddr(0x000f_ffff_ffff_ffff_usize))
        );
        // Invalid.
        assert_eq!(
            PhysAddr::try_new(0xfff0_0000_0000_0000_usize),
            Err(InvalidPhysAddr(0xfff0_0000_0000_0000_usize))
        );
        assert_eq!(
            PhysAddr::try_new(0xffff_ffff_ffff_ffff_usize),
            Err(InvalidPhysAddr(0xffff_ffff_ffff_ffff_usize))
        );
    }
}
