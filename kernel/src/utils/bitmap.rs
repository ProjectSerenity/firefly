//! bitmaps implement a bitmap structure, backed by an array of u64s.

use alloc::vec;
use alloc::vec::Vec;
use x86_64::align_up;

/// Bitmap is a simple bitmap implementation.
///
#[derive(Debug, PartialEq)]
pub struct Bitmap {
    num: usize,
    bits: Vec<u64>,
}

impl Bitmap {
    /// new_set returns a new bitmap with all
    /// bits set to true.
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

    /// new_unset returns a new bitmap with all
    /// bits set to false.
    ///
    pub fn new_unset(num: usize) -> Self {
        Bitmap {
            num,
            bits: vec![0u64; (num + 63) / 64],
        }
    }

    /// num_set returns the number of bits
    /// in the bitmap that are set.
    ///
    pub fn num_set(&self) -> usize {
        self.bits
            .iter()
            .map(|&x| x.count_ones())
            .reduce(|accum, item| accum + item)
            .unwrap() as usize
    }

    /// num_unset returns the number of bits
    /// in the bitmap that are not set.
    ///
    pub fn num_unset(&self) -> usize {
        // We need to ignore the zero
        // bits in the final u64 after
        // our actual data ends.
        let ignore = (align_up(self.num as u64, 64 as u64) - self.num as u64) as usize;
        self.bits
            .iter()
            .map(|&x| x.count_zeros())
            .reduce(|accum, item| accum + item)
            .unwrap() as usize
            - ignore
    }

    /// get returns whether bit n is set.
    ///
    /// get will panic if n exceeds the bitmap's
    /// size in bits.
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

    /// set sets bit n to true.
    ///
    /// set will panic if n exceeds the bitmap's
    /// size in bits.
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

    /// unset sets bit n to false.
    ///
    /// unset will panic if n exceeds the bitmap's
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

    /// next_set returns the smallest n, such that
    /// bit n is set (true), or None if all bits
    /// are false.
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

    /// next_n_set returns the smallest i, such that
    /// bits i to i+n-1 are set (true), or None if
    /// no such i can be found..
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

    /// next_unset returns the smallest n, such that
    /// bit n is unset (false), or None if all bits
    /// are true.
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

#[test_case]
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
}