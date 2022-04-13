// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

use align::{align_down_usize, align_up_usize};
use core::fmt;
use core::ops::{Add, AddAssign, Sub, SubAssign};

/// A virtual memory address for the target architecture.
///
/// The properties of the address will vary between each
/// target architecture, but a `VirtAddr` can only store
/// an address that is valid. For example, on x86-64, a
/// `VirtAddr` is always canonical, with the top 16 bits
/// equal to bit 47.
///
#[repr(transparent)]
#[derive(Clone, Copy, Eq, Hash, Ord, PartialEq, PartialOrd)]
pub struct VirtAddr(usize);

/// An invalid virtual memory address.
///
/// If an attempt is made to create a `VirtAddr` from a
/// value that is not valid on the target platform, then
/// `InvalidVirtAddr` is returned, containing the attempted
/// value.
///
#[repr(transparent)]
#[derive(Clone, Copy, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
pub struct InvalidVirtAddr(pub usize);

impl VirtAddr {
    /// Creates a new virtual memory address.
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
            Err(_) => panic!("invalid address passed to VirtAddr::new"),
        }
    }

    /// Tries to create a new virtual memory address.
    ///
    /// If the passed value is invalid for the target
    /// platform, then an error is returned.
    ///
    #[inline]
    pub const fn try_new(addr: usize) -> Result<Self, InvalidVirtAddr> {
        // Check that the address is a 48-bit canonical address,
        // either as a a low half address (starting with 0x00007,
        // or a high half address (starting with 0xffff8).
        let top_bits = (addr & 0xffff_8000_0000_0000_usize) >> 47;
        match top_bits {
            0 | 0x1ffff => Ok(VirtAddr(addr)), // address is canonical
            _ => Err(InvalidVirtAddr(addr)),
        }
    }

    /// Creates a new virtual memory address, without any checks.
    ///
    /// ## Safety
    ///
    /// The caller must ensure `addr` describes a valid virtual
    /// address on the target platform.
    ///
    #[inline]
    pub const unsafe fn new_unchecked(addr: usize) -> Self {
        VirtAddr(addr)
    }

    /// Returns the address's numerical value.
    ///
    #[inline]
    pub const fn as_usize(self) -> usize {
        self.0
    }

    // Translation to platform-specific types.

    /// Returns the address described by the
    /// [`x86_64::VirtAddr`].
    ///
    #[inline]
    pub const fn from_x86_64(addr: x86_64::VirtAddr) -> Self {
        VirtAddr::new(addr.as_u64() as usize)
    }

    /// Returns the address as a [`x86_64::VirtAddr`],
    /// for convenience.
    ///
    #[inline]
    #[must_use]
    pub fn as_x86_64(&self) -> x86_64::VirtAddr {
        x86_64::VirtAddr::new(self.0 as u64)
    }

    // Special handling for the zero address.

    /// Returns the zero virtual memory address.
    ///
    #[inline]
    pub const fn zero() -> Self {
        VirtAddr(0)
    }

    /// Returns whether this is the zero address.
    ///
    #[inline]
    pub const fn is_zero(self) -> bool {
        self.0 == 0
    }

    // Alignment.

    /// Aligns the virtual address downwards
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
        VirtAddr(align_down_usize(self.0, align))
    }

    /// Aligns the virtual address upwards to
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
        VirtAddr(align_up_usize(self.0, align))
    }

    /// Checks whether the virtual address has
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
    /// result is not a valid virtual address.
    ///
    #[inline]
    pub const fn checked_add(self, rhs: usize) -> Option<Self> {
        if let Some(sum) = self.0.checked_add(rhs) {
            if let Ok(addr) = VirtAddr::try_new(sum) {
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
    /// result is not a valid virtual address.
    ///
    #[inline]
    pub const fn checked_sub(self, rhs: usize) -> Option<Self> {
        if let Some(sum) = self.0.checked_sub(rhs) {
            if let Ok(addr) = VirtAddr::try_new(sum) {
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

impl fmt::Binary for VirtAddr {
    #[inline]
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        fmt::Binary::fmt(&self.0, f)
    }
}

impl fmt::Debug for VirtAddr {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        f.debug_tuple("VirtAddr")
            .field(&format_args!("{:p}", self.0 as *const ()))
            .finish()
    }
}

impl fmt::LowerHex for VirtAddr {
    #[inline]
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        fmt::LowerHex::fmt(&self.0, f)
    }
}

impl fmt::Octal for VirtAddr {
    #[inline]
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        fmt::Octal::fmt(&self.0, f)
    }
}

impl fmt::Pointer for VirtAddr {
    #[inline]
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        fmt::Pointer::fmt(&(self.0 as *const ()), f)
    }
}

impl fmt::UpperHex for VirtAddr {
    #[inline]
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        fmt::UpperHex::fmt(&self.0, f)
    }
}

// Mathematical operators.

impl Add<usize> for VirtAddr {
    type Output = Self;

    #[inline]
    #[track_caller]
    fn add(self, rhs: usize) -> Self::Output {
        VirtAddr::new(self.0 + rhs)
    }
}

impl AddAssign<usize> for VirtAddr {
    #[inline]
    #[track_caller]
    fn add_assign(&mut self, rhs: usize) {
        *self = *self + rhs;
    }
}

impl Sub<usize> for VirtAddr {
    type Output = Self;

    #[inline]
    #[track_caller]
    fn sub(self, rhs: usize) -> Self::Output {
        VirtAddr::new(self.0 - rhs)
    }
}

impl SubAssign<usize> for VirtAddr {
    #[inline]
    #[track_caller]
    fn sub_assign(&mut self, rhs: usize) {
        self.0 -= rhs
    }
}

impl Sub<VirtAddr> for VirtAddr {
    type Output = usize;

    #[inline]
    #[track_caller]
    fn sub(self, rhs: VirtAddr) -> Self::Output {
        self.0.checked_sub(rhs.0).unwrap()
    }
}

#[cfg(test)]
mod test {
    use super::*;

    #[test]
    fn test_x86_64() {
        assert_eq!(VirtAddr::new(1).as_usize(), 1_usize);

        // Canonical address handling.

        // Lower half.
        assert_eq!(VirtAddr::try_new(0_usize), Ok(VirtAddr(0_usize)));
        assert_eq!(
            VirtAddr::try_new(0x7fff_ffff_ffff_usize),
            Ok(VirtAddr(0x7fff_ffff_ffff_usize))
        );
        // Non-canonical.
        assert_eq!(
            VirtAddr::try_new(0x8000_0000_0000_usize),
            Err(InvalidVirtAddr(0x8000_0000_0000_usize))
        );
        assert_eq!(
            VirtAddr::try_new(0xffff_7fff_ffff_ffff_usize),
            Err(InvalidVirtAddr(0xffff_7fff_ffff_ffff_usize))
        );
        // Higher half.
        assert_eq!(
            VirtAddr::try_new(0xffff_8000_0000_0000_usize),
            Ok(VirtAddr(0xffff_8000_0000_0000_usize))
        );
        assert_eq!(
            VirtAddr::try_new(0xffff_ffff_ffff_ffff_usize),
            Ok(VirtAddr(0xffff_ffff_ffff_ffff_usize))
        );
    }
}
