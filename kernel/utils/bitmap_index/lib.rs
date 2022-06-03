// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements a bitmap structure, backed by a sequence of `u64`s.
//!
//! This crate can be used to a binary state for an arbitrary number
//! of sequential items efficently. For example, a bitmap could track whether
//! each 4 KiB frame in 2 GiB of physical memory with only 64 KiB of overhead.
//!
//! # Examples
//!
//! ```
//! let mut data = bitmap::new_unset(64);
//! assert_eq!(data.num_set(), 0);
//! assert_eq!(data.num_unset(), 64);
//!
//! data.set(8);
//! data.set(9);
//! assert_eq!(data.num_set(), 2);
//! assert_eq!(data.num_unset(), 62);
//! assert_eq!(data.next_set(), 8);
//! assert_eq!(data.next_n_set(3), None);
//! assert_eq!(data.next_n_set(2), Some(8));
//! ```

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![allow(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]
#![forbid(unsafe_code)]

extern crate alloc;

use align::align_up_usize;
use alloc::vec;
use alloc::vec::Vec;

/// A simple bitmap implementation, backed by a sequence of `u64`s.
///
#[derive(Debug, PartialEq, Eq)]
pub struct Bitmap {
    num: usize,
    bits: Vec<u64>,
}

impl Bitmap {
    /// Returns a new bitmap with all bits set to `true`.
    ///
    pub fn new_set(num: usize) -> Self {
        // Make sure we only set the bits we
        // have.
        let mut bitmap = Bitmap::new_unset(num);
        for i in 0..num {
            bitmap.set(i);
        }

        bitmap
    }

    /// Returns a new bitmap with all bits set to `false`.
    ///
    pub fn new_unset(num: usize) -> Self {
        Bitmap {
            num,
            bits: vec![0u64; (num + 63) / 64],
        }
    }

    /// Extends the bitmap to add `bits` bits to the capacity,
    /// with all new bits unset.
    ///
    pub fn add_unset(&mut self, bits: usize) {
        let new_num = self.num + bits;
        while self.bits.len() * 64 < new_num {
            self.bits.push(0u64);
        }

        self.num = new_num;
    }

    /// Extends the bitmap to add `bits` bits to the capacity,
    /// with all new bits set.
    pub fn add_set(&mut self, bits: usize) {
        let start = self.num;
        self.add_unset(bits);
        for i in start..(start + bits) {
            self.set(i);
        }
    }

    /// Returns the number of bits in the bitmap that are
    /// set.
    ///
    pub fn num_set(&self) -> usize {
        self.bits
            .iter()
            .map(|&x| x.count_ones())
            .reduce(|accum, item| accum + item)
            .unwrap_or(0) as usize
    }

    /// Returns the number of bits in the bitmap that are
    /// not set.
    ///
    pub fn num_unset(&self) -> usize {
        // We need to ignore the zero
        // bits in the final u64 after
        // our actual data ends.
        let ignore = align_up_usize(self.num, 64) - self.num;
        self.bits
            .iter()
            .map(|&x| x.count_zeros())
            .reduce(|accum, item| accum + item)
            .unwrap_or(0) as usize
            - ignore
    }

    /// Returns whether bit `n` is set.
    ///
    /// # Panics
    ///
    /// `get` will panic if `n` exceeds the bitmap's size
    /// in bits.
    ///
    pub fn get(&self, n: usize) -> bool {
        if n >= self.num {
            panic!("cannot call get({}) on Bitmap of size {}", n, self.num);
        }

        let i = n / 64;
        let j = n % 64;
        let mask = 1u64 << (j as u64);

        self.bits[i] & mask == mask
    }

    /// Sets bit `n` to `true`.
    ///
    /// # Panics
    ///
    /// `set` will panic if `n` exceeds the bitmap's size
    /// in bits.
    ///
    pub fn set(&mut self, n: usize) {
        if n >= self.num {
            panic!("cannot call set({}) on Bitmap of size {}", n, self.num);
        }

        let i = n / 64;
        let j = n % 64;
        let mask = 1u64 << (j as u64);

        self.bits[i] |= mask;
    }

    /// Sets bit `n` to `false`.
    ///
    /// # Panics
    ///
    /// `unset` will panic if `n` exceeds the bitmap's
    /// size in bits.
    ///
    pub fn unset(&mut self, n: usize) {
        if n >= self.num {
            panic!("cannot call unset({}) on Bitmap of size {}", n, self.num);
        }

        let i = n / 64;
        let j = n % 64;
        let mask = 1u64 << (j as u64);

        self.bits[i] &= !mask;
    }

    /// Returns the smallest `n`, such that bit `n` is
    /// set, or `None` if all bits are unset.
    ///
    pub fn next_set(&self) -> Option<usize> {
        for (i, values) in self.bits.iter().enumerate() {
            for j in 0..64 {
                let mask = 1u64 << (j as u64);
                if values & mask == mask {
                    return Some(i * 64 + j);
                }
            }
        }

        None
    }

    /// Returns the smallest `i`, such that bits `i` to
    /// to `i + (n-1)` are set, or `None` if no such `i`
    /// could be found.
    ///
    pub fn next_n_set(&self, n: usize) -> Option<usize> {
        let mut mask = 0u64;
        for i in 0..n {
            mask |= 1 << i;
        }

        for (i, values) in self.bits.iter().enumerate() {
            for j in 0..64 - n {
                if values & mask << j == mask << j {
                    return Some(i * 64 + j);
                }
            }
        }

        None
    }

    /// Returns the smallest `n`, such that bit `n` is
    /// unset, or `None` if all bits are set.
    ///
    pub fn next_unset(&self) -> Option<usize> {
        for (i, values) in self.bits.iter().enumerate() {
            for j in 0..64 {
                let mask = 1u64 << (j as u64);
                if values & mask == 0 {
                    return Some(i * 64 + j);
                }
            }
        }

        None
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn bitmap() {
        let mut bitmap = Bitmap::new_unset(7);
        for i in 0..7 {
            // Check it's false by default.
            assert_eq!(bitmap.get(i), false);
            assert_eq!(bitmap.next_set(), None);
            assert_eq!(bitmap.num_set(), 0);
            assert_eq!(bitmap.num_unset(), 7);

            // Check set.
            bitmap.set(i);
            assert_eq!(bitmap.get(i), true);
            assert_eq!(bitmap.next_set(), Some(i));
            assert_eq!(bitmap.num_set(), 1);
            assert_eq!(bitmap.num_unset(), 6);

            // Check unset.
            bitmap.unset(i);
            assert_eq!(bitmap.get(i), false);
            assert_eq!(bitmap.num_set(), 0);
            assert_eq!(bitmap.num_unset(), 7);
        }

        bitmap = Bitmap::new_unset(67);
        for i in 0..67 {
            // Check it's false by default.
            assert_eq!(bitmap.get(i), false);
            assert_eq!(bitmap.next_set(), None);
            assert_eq!(bitmap.num_set(), 0);
            assert_eq!(bitmap.num_unset(), 67);

            // Check set.
            bitmap.set(i);
            assert_eq!(bitmap.get(i), true);
            assert_eq!(bitmap.next_set(), Some(i));
            assert_eq!(bitmap.num_set(), 1);
            assert_eq!(bitmap.num_unset(), 66);

            // Check unset.
            bitmap.unset(i);
            assert_eq!(bitmap.get(i), false);
            assert_eq!(bitmap.num_set(), 0);
            assert_eq!(bitmap.num_unset(), 67);
        }

        bitmap = Bitmap::new_set(7);
        for i in 0..7 {
            // Check it's true by default.
            assert_eq!(bitmap.get(i), true);
            assert_eq!(bitmap.num_set(), 7);
            assert_eq!(bitmap.num_unset(), 0);

            // Check unset.
            bitmap.unset(i);
            assert_eq!(bitmap.get(i), false);
            assert_eq!(bitmap.next_unset(), Some(i));
            assert_eq!(bitmap.num_set(), 6);
            assert_eq!(bitmap.num_unset(), 1);

            // Check set.
            bitmap.set(i);
            assert_eq!(bitmap.get(i), true);
            assert_eq!(bitmap.num_set(), 7);
            assert_eq!(bitmap.num_unset(), 0);
        }

        bitmap = Bitmap::new_set(67);
        for i in 0..67 {
            // Check it's true by default.
            assert_eq!(bitmap.get(i), true);
            assert_eq!(bitmap.num_set(), 67);
            assert_eq!(bitmap.num_unset(), 0);

            // Check unset.
            bitmap.unset(i);
            assert_eq!(bitmap.get(i), false);
            assert_eq!(bitmap.next_unset(), Some(i));
            assert_eq!(bitmap.num_set(), 66);
            assert_eq!(bitmap.num_unset(), 1);

            // Check set.
            bitmap.set(i);
            assert_eq!(bitmap.get(i), true);
            assert_eq!(bitmap.num_set(), 67);
            assert_eq!(bitmap.num_unset(), 0);
        }

        // Increase the size and continue.
        bitmap.add_set(3);
        for i in 0..70 {
            // Check it's true by default.
            assert_eq!(bitmap.get(i), true);
            assert_eq!(bitmap.num_set(), 70);
            assert_eq!(bitmap.num_unset(), 0);

            // Check unset.
            bitmap.unset(i);
            assert_eq!(bitmap.get(i), false);
            assert_eq!(bitmap.next_unset(), Some(i));
            assert_eq!(bitmap.num_set(), 69);
            assert_eq!(bitmap.num_unset(), 1);

            // Check set.
            bitmap.set(i);
            assert_eq!(bitmap.get(i), true);
            assert_eq!(bitmap.num_set(), 70);
            assert_eq!(bitmap.num_unset(), 0);
        }

        // Increase the size and continue.
        bitmap.add_set(60);
        for i in 0..130 {
            // Check it's true by default.
            assert_eq!(bitmap.get(i), true);
            assert_eq!(bitmap.num_set(), 130);
            assert_eq!(bitmap.num_unset(), 0);

            // Check unset.
            bitmap.unset(i);
            assert_eq!(bitmap.get(i), false);
            assert_eq!(bitmap.next_unset(), Some(i));
            assert_eq!(bitmap.num_set(), 129);
            assert_eq!(bitmap.num_unset(), 1);

            // Check set.
            bitmap.set(i);
            assert_eq!(bitmap.get(i), true);
            assert_eq!(bitmap.num_set(), 130);
            assert_eq!(bitmap.num_unset(), 0);
        }
    }
}
