//! bytes provides pretty printing for bytes, using base 2 for larger units.

// For more details, see https://en.wikipedia.org/wiki/Byte#Units_based_on_powers_of_2.

use core::fmt;

/// Bytes contains a number of bytes.
///
pub struct Bytes(u64);

impl Bytes {
    /// from_u64 treats n as a number of bytes.
    ///
    pub fn from_u64(n: u64) -> Self {
        Bytes(n)
    }
}

impl fmt::Display for Bytes {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        let units = ["B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"];
        let mut b = self.0;
        for unit in units.iter() {
            if b >= 1024 {
                b >>= 10;
                continue;
            }

            return write!(f, "{} {}", b, unit);
        }

        write!(f, "{} ZiB", b)
    }
}