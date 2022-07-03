// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements a alignment of unsigned integer types to exact powers of two.

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(deprecated_in_future)]
#![deny(keyword_idents)]
#![forbid(unsafe_code)]
#![deny(unused_crate_dependencies)]

/// An internal macro to implement alignment both up and
/// down for each unsigned integer type.
///
macro_rules! align_up_and_down {
    ($t:ty, $align_up:ident, $align_down:ident) => {
        /// Aligns `value` to the smallest exact multiple of two that is no
        /// smaller than `value`.
        ///
        /// `align` must be an exact multiple of two.
        ///
        pub const fn $align_up(value: $t, align: $t) -> $t {
            assert!(
                align.is_power_of_two(),
                "`align` must be an exact multiple of two"
            );
            let mask = align - 1;
            if value & mask == 0 {
                // The value is already aligned.
                value
            } else {
                // Round up by filling the bottom bits and incrementing.
                (value | mask) + 1
            }
        }

        /// Aligns `value` to the largest exact multiple of two that is no
        /// larger than `value`.
        ///
        /// `align` must be an exact multiple of two.
        ///
        pub const fn $align_down(value: $t, align: $t) -> $t {
            assert!(
                align.is_power_of_two(),
                "`align` must be an exact multiple of two"
            );
            let mask = align - 1;

            // Round down by masking off the bottom bits.
            value & !mask
        }
    };
}

align_up_and_down! {    u8,    align_up_u8,    align_down_u8 }
align_up_and_down! {   u16,   align_up_u16,   align_down_u16 }
align_up_and_down! {   u32,   align_up_u32,   align_down_u32 }
align_up_and_down! {   u64,   align_up_u64,   align_down_u64 }
align_up_and_down! {  u128,  align_up_u128,  align_down_u128 }
align_up_and_down! { usize, align_up_usize, align_down_usize }

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_align() {
        assert_eq!(align_up_u8(0, 1), 0);
        assert_eq!(align_up_u16(0, 1), 0);
        assert_eq!(align_up_u32(0, 1), 0);
        assert_eq!(align_up_u64(0, 1), 0);
        assert_eq!(align_up_u128(0, 1), 0);
        assert_eq!(align_up_usize(0, 1), 0);

        assert_eq!(align_up_u8(1, 1), 1);
        assert_eq!(align_up_u16(1, 1), 1);
        assert_eq!(align_up_u32(1, 1), 1);
        assert_eq!(align_up_u64(1, 1), 1);
        assert_eq!(align_up_u128(1, 1), 1);
        assert_eq!(align_up_usize(1, 1), 1);

        assert_eq!(align_up_u8(2, 1), 2);
        assert_eq!(align_up_u16(2, 1), 2);
        assert_eq!(align_up_u32(2, 1), 2);
        assert_eq!(align_up_u64(2, 1), 2);
        assert_eq!(align_up_u128(2, 1), 2);
        assert_eq!(align_up_usize(2, 1), 2);

        assert_eq!(align_up_u8(3, 1), 3);
        assert_eq!(align_up_u16(3, 1), 3);
        assert_eq!(align_up_u32(3, 1), 3);
        assert_eq!(align_up_u64(3, 1), 3);
        assert_eq!(align_up_u128(3, 1), 3);
        assert_eq!(align_up_usize(3, 1), 3);

        assert_eq!(align_up_u8(1, 2), 2);
        assert_eq!(align_up_u16(1, 2), 2);
        assert_eq!(align_up_u32(1, 2), 2);
        assert_eq!(align_up_u64(1, 2), 2);
        assert_eq!(align_up_u128(1, 2), 2);
        assert_eq!(align_up_usize(1, 2), 2);

        assert_eq!(align_up_u8(2, 2), 2);
        assert_eq!(align_up_u16(2, 2), 2);
        assert_eq!(align_up_u32(2, 2), 2);
        assert_eq!(align_up_u64(2, 2), 2);
        assert_eq!(align_up_u128(2, 2), 2);
        assert_eq!(align_up_usize(2, 2), 2);

        assert_eq!(align_up_u8(3, 2), 4);
        assert_eq!(align_up_u16(3, 2), 4);
        assert_eq!(align_up_u32(3, 2), 4);
        assert_eq!(align_up_u64(3, 2), 4);
        assert_eq!(align_up_u128(3, 2), 4);
        assert_eq!(align_up_usize(3, 2), 4);

        assert_eq!(align_up_u8(4, 2), 4);
        assert_eq!(align_up_u16(4, 2), 4);
        assert_eq!(align_up_u32(4, 2), 4);
        assert_eq!(align_up_u64(4, 2), 4);
        assert_eq!(align_up_u128(4, 2), 4);
        assert_eq!(align_up_usize(4, 2), 4);

        assert_eq!(align_up_u8(2, 4), 4);
        assert_eq!(align_up_u16(2, 4), 4);
        assert_eq!(align_up_u32(2, 4), 4);
        assert_eq!(align_up_u64(2, 4), 4);
        assert_eq!(align_up_u128(2, 4), 4);
        assert_eq!(align_up_usize(2, 4), 4);

        assert_eq!(align_up_u8(4, 4), 4);
        assert_eq!(align_up_u16(4, 4), 4);
        assert_eq!(align_up_u32(4, 4), 4);
        assert_eq!(align_up_u64(4, 4), 4);
        assert_eq!(align_up_u128(4, 4), 4);
        assert_eq!(align_up_usize(4, 4), 4);

        assert_eq!(align_up_u8(5, 4), 8);
        assert_eq!(align_up_u16(5, 4), 8);
        assert_eq!(align_up_u32(5, 4), 8);
        assert_eq!(align_up_u64(5, 4), 8);
        assert_eq!(align_up_u128(5, 4), 8);
        assert_eq!(align_up_usize(5, 4), 8);

        assert_eq!(align_up_u8(2, 128), 128);
        assert_eq!(align_up_u16(2, 128), 128);
        assert_eq!(align_up_u32(2, 128), 128);
        assert_eq!(align_up_u64(2, 128), 128);
        assert_eq!(align_up_u128(2, 128), 128);
        assert_eq!(align_up_usize(2, 128), 128);

        assert_eq!(align_up_u16(2, 512), 512);
        assert_eq!(align_up_u32(2, 512), 512);
        assert_eq!(align_up_u64(2, 512), 512);
        assert_eq!(align_up_u128(2, 512), 512);
        assert_eq!(align_up_usize(2, 512), 512);

        assert_eq!(align_up_u16(512, 512), 512);
        assert_eq!(align_up_u32(512, 512), 512);
        assert_eq!(align_up_u64(512, 512), 512);
        assert_eq!(align_up_u128(512, 512), 512);
        assert_eq!(align_up_usize(512, 512), 512);

        assert_eq!(align_up_u16(513, 512), 1024);
        assert_eq!(align_up_u32(513, 512), 1024);
        assert_eq!(align_up_u64(513, 512), 1024);
        assert_eq!(align_up_u128(513, 512), 1024);
        assert_eq!(align_up_usize(513, 512), 1024);

        assert_eq!(align_up_u64(2, 0x8000_0000_0000), 0x8000_0000_0000);
        assert_eq!(align_up_u128(2, 0x8000_0000_0000), 0x8000_0000_0000);
        assert_eq!(align_up_usize(2, 0x8000_0000_0000), 0x8000_0000_0000);

        assert_eq!(align_down_u8(0, 1), 0);
        assert_eq!(align_down_u16(0, 1), 0);
        assert_eq!(align_down_u32(0, 1), 0);
        assert_eq!(align_down_u64(0, 1), 0);
        assert_eq!(align_down_u128(0, 1), 0);
        assert_eq!(align_down_usize(0, 1), 0);

        assert_eq!(align_down_u8(1, 1), 1);
        assert_eq!(align_down_u16(1, 1), 1);
        assert_eq!(align_down_u32(1, 1), 1);
        assert_eq!(align_down_u64(1, 1), 1);
        assert_eq!(align_down_u128(1, 1), 1);
        assert_eq!(align_down_usize(1, 1), 1);

        assert_eq!(align_down_u8(2, 1), 2);
        assert_eq!(align_down_u16(2, 1), 2);
        assert_eq!(align_down_u32(2, 1), 2);
        assert_eq!(align_down_u64(2, 1), 2);
        assert_eq!(align_down_u128(2, 1), 2);
        assert_eq!(align_down_usize(2, 1), 2);

        assert_eq!(align_down_u8(3, 1), 3);
        assert_eq!(align_down_u16(3, 1), 3);
        assert_eq!(align_down_u32(3, 1), 3);
        assert_eq!(align_down_u64(3, 1), 3);
        assert_eq!(align_down_u128(3, 1), 3);
        assert_eq!(align_down_usize(3, 1), 3);

        assert_eq!(align_down_u8(1, 2), 0);
        assert_eq!(align_down_u16(1, 2), 0);
        assert_eq!(align_down_u32(1, 2), 0);
        assert_eq!(align_down_u64(1, 2), 0);
        assert_eq!(align_down_u128(1, 2), 0);
        assert_eq!(align_down_usize(1, 2), 0);

        assert_eq!(align_down_u8(2, 2), 2);
        assert_eq!(align_down_u16(2, 2), 2);
        assert_eq!(align_down_u32(2, 2), 2);
        assert_eq!(align_down_u64(2, 2), 2);
        assert_eq!(align_down_u128(2, 2), 2);
        assert_eq!(align_down_usize(2, 2), 2);

        assert_eq!(align_down_u8(3, 2), 2);
        assert_eq!(align_down_u16(3, 2), 2);
        assert_eq!(align_down_u32(3, 2), 2);
        assert_eq!(align_down_u64(3, 2), 2);
        assert_eq!(align_down_u128(3, 2), 2);
        assert_eq!(align_down_usize(3, 2), 2);

        assert_eq!(align_down_u8(4, 2), 4);
        assert_eq!(align_down_u16(4, 2), 4);
        assert_eq!(align_down_u32(4, 2), 4);
        assert_eq!(align_down_u64(4, 2), 4);
        assert_eq!(align_down_u128(4, 2), 4);
        assert_eq!(align_down_usize(4, 2), 4);

        assert_eq!(align_down_u8(2, 4), 0);
        assert_eq!(align_down_u16(2, 4), 0);
        assert_eq!(align_down_u32(2, 4), 0);
        assert_eq!(align_down_u64(2, 4), 0);
        assert_eq!(align_down_u128(2, 4), 0);
        assert_eq!(align_down_usize(2, 4), 0);

        assert_eq!(align_down_u8(4, 4), 4);
        assert_eq!(align_down_u16(4, 4), 4);
        assert_eq!(align_down_u32(4, 4), 4);
        assert_eq!(align_down_u64(4, 4), 4);
        assert_eq!(align_down_u128(4, 4), 4);
        assert_eq!(align_down_usize(4, 4), 4);

        assert_eq!(align_down_u8(5, 4), 4);
        assert_eq!(align_down_u16(5, 4), 4);
        assert_eq!(align_down_u32(5, 4), 4);
        assert_eq!(align_down_u64(5, 4), 4);
        assert_eq!(align_down_u128(5, 4), 4);
        assert_eq!(align_down_usize(5, 4), 4);

        assert_eq!(align_down_u8(2, 128), 0);
        assert_eq!(align_down_u16(2, 128), 0);
        assert_eq!(align_down_u32(2, 128), 0);
        assert_eq!(align_down_u64(2, 128), 0);
        assert_eq!(align_down_u128(2, 128), 0);
        assert_eq!(align_down_usize(2, 128), 0);

        assert_eq!(align_down_u16(2, 512), 0);
        assert_eq!(align_down_u32(2, 512), 0);
        assert_eq!(align_down_u64(2, 512), 0);
        assert_eq!(align_down_u128(2, 512), 0);
        assert_eq!(align_down_usize(2, 512), 0);

        assert_eq!(align_down_u16(512, 512), 512);
        assert_eq!(align_down_u32(512, 512), 512);
        assert_eq!(align_down_u64(512, 512), 512);
        assert_eq!(align_down_u128(512, 512), 512);
        assert_eq!(align_down_usize(512, 512), 512);

        assert_eq!(align_down_u16(513, 512), 512);
        assert_eq!(align_down_u32(513, 512), 512);
        assert_eq!(align_down_u64(513, 512), 512);
        assert_eq!(align_down_u128(513, 512), 512);
        assert_eq!(align_down_usize(513, 512), 512);

        assert_eq!(align_down_u64(2, 0x8000_0000_0000), 0);
        assert_eq!(align_down_u128(2, 0x8000_0000_0000), 0);
        assert_eq!(align_down_usize(2, 0x8000_0000_0000), 0);
    }
}
