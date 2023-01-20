// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides pretty printing for bytes, using [powers of 2 for larger units](https://en.wikipedia.org/wiki/Byte#Units_based_on_powers_of_2).
//!
//! # Examples
//!
//! ```
//! println!("{}", Bytes::from_u64(2)); // Prints "2 B"
//! println!("{}", Bytes::from_u64(4096)); // Prints "4 KiB"
//! ```

use core::fmt;

/// Contains a number of bytes.
///
pub struct Bytes(usize);

impl Bytes {
    /// Wraps a number of bytes.
    ///
    pub fn from_u64(n: u64) -> Self {
        Bytes(n as usize)
    }

    /// Wraps a number of bytes.
    ///
    pub fn from_usize(n: usize) -> Self {
        Bytes(n)
    }
}

impl fmt::Display for Bytes {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        let units = ["B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"];
        let mut b = self.0;
        let width = f.width();
        for unit in units.iter() {
            if b >= 1024 {
                b >>= 10;
                continue;
            }

            return match width {
                None => write!(f, "{b} {unit}"),
                Some(width) => write!(
                    f,
                    "{:width$} {}",
                    b,
                    unit,
                    width = width.saturating_sub(1 + unit.len())
                ),
            };
        }

        match width {
            None => write!(f, "{b} ZiB"),
            Some(width) => write!(f, "{:width$} ZiB", b, width = width.saturating_sub(4)),
        }
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use alloc::format;

    #[test]
    fn test_bytes() {
        assert_eq!(format!("{}", Bytes::from_usize(1)), "1 B");
        assert_eq!(format!("{}", Bytes::from_usize(2)), "2 B");
        assert_eq!(format!("{}", Bytes::from_usize(1000)), "1000 B");
        assert_eq!(format!("{}", Bytes::from_usize(1023)), "1023 B");
        assert_eq!(format!("{}", Bytes::from_usize(1024)), "1 KiB");
        assert_eq!(format!("{}", Bytes::from_usize(2 * 1024)), "2 KiB");
        assert_eq!(format!("{}", Bytes::from_usize(1000 * 1024)), "1000 KiB");
        assert_eq!(format!("{}", Bytes::from_usize(1023 * 1024)), "1023 KiB");
        assert_eq!(format!("{}", Bytes::from_usize(1024 * 1024)), "1 MiB");
    }
}
