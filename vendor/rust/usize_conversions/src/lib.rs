//! Conversion traits and functions for conversions between `usize` and fixed sized integers.
//!
//! Warning: The traits are conditionally implemented based on the target pointer width,
//! so they can make your crate less portable.

#![feature(i128_type)]
#![warn(missing_docs)]

#![no_std]

/// Create an usize from a fixed sized integer.
pub fn usize_from<T>(value: T) -> usize where T: IntoUsize {
    IntoUsize::into_usize(value)
}

/// Convert a type to `usize`.
pub trait IntoUsize: Sized {
    /// Performs the conversion.
    fn into_usize(self) -> usize;
}

/// Convert a type from `usize`.
pub trait FromUsize: Sized {
    /// Performs the conversion.
    fn from_usize(usize) -> Self;
}

macro_rules! impl_into_usize {
    ($x:ty) => {
        impl IntoUsize for $x {
            fn into_usize(self) -> usize {
                self as usize
            }
        }
    }
}

macro_rules! impl_from_usize {
    ($x:ty) => {
        impl FromUsize for $x {
            fn from_usize(value: usize) -> Self {
                value as Self
            }
        }
    }
}

#[cfg(target_pointer_width = "128")] impl_from_usize!(u128);
#[cfg(target_pointer_width = "128")] impl_into_usize!(u128);
#[cfg(target_pointer_width = "128")] impl_into_usize!(u64);
#[cfg(target_pointer_width = "128")] impl_into_usize!(u32);
#[cfg(target_pointer_width = "128")] impl_into_usize!(u16);
#[cfg(target_pointer_width = "128")] impl_into_usize!(u8);

#[cfg(target_pointer_width = "64")] impl_from_usize!(u128);
#[cfg(target_pointer_width = "64")] impl_from_usize!(u64);
#[cfg(target_pointer_width = "64")] impl_into_usize!(u64);
#[cfg(target_pointer_width = "64")] impl_into_usize!(u32);
#[cfg(target_pointer_width = "64")] impl_into_usize!(u16);
#[cfg(target_pointer_width = "64")] impl_into_usize!(u8);

#[cfg(target_pointer_width = "32")] impl_from_usize!(u128);
#[cfg(target_pointer_width = "32")] impl_from_usize!(u64);
#[cfg(target_pointer_width = "32")] impl_from_usize!(u32);
#[cfg(target_pointer_width = "32")] impl_into_usize!(u32);
#[cfg(target_pointer_width = "32")] impl_into_usize!(u16);
#[cfg(target_pointer_width = "32")] impl_into_usize!(u8);

#[cfg(target_pointer_width = "16")] impl_from_usize!(u128);
#[cfg(target_pointer_width = "16")] impl_from_usize!(u64);
#[cfg(target_pointer_width = "16")] impl_from_usize!(u32);
#[cfg(target_pointer_width = "16")] impl_from_usize!(u16);
#[cfg(target_pointer_width = "16")] impl_into_usize!(u16);
#[cfg(target_pointer_width = "16")] impl_into_usize!(u8);

#[cfg(target_pointer_width = "8")] impl_from_usize!(u128);
#[cfg(target_pointer_width = "8")] impl_from_usize!(u64);
#[cfg(target_pointer_width = "8")] impl_from_usize!(u32);
#[cfg(target_pointer_width = "8")] impl_from_usize!(u16);
#[cfg(target_pointer_width = "8")] impl_from_usize!(u8);
#[cfg(target_pointer_width = "8")] impl_into_usize!(u8);
