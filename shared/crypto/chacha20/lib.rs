// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements the ChaCha20 stream cipher.
//!
//! This is deliberately quite a simple, conservative implementation.
//! The priorities for this crate, in order, are:
//!
//! 1. Correctness
//! 2. Readability
//! 3. Performance
//!
//! As a result, the crate is currently implemented in pure Rust.
//!
//! # ⚠️ Security warning
//!
//! This cipher does not provide integrity protection on the
//! ciphertexts it produces. This makes it vulnerable to
//! attacks using attacker-controlled ciphertexts. Most use
//! cases for encryption should use a higher-level primitive,
//! such as ChaCha20-Poly1305 or AES-GCM.
//!
//! # Examples
//!
//! Encrypt a stream of data:
//!
//! ```
//! use chacha20::ChaCha20;
//!
//! // Prepare the key, counter, and nonce.
//! let key = hex!(
//!     "0001020304050607 08090a0b0c0d0e0f"
//!     "1011121314151617 18191a1b1c1d1e1f"
//! );
//!
//! let counter = 1;
//! let nonce = hex!("000000000000004a00000000");
//!
//! // Create the cipher state.
//! let mut cipher = ChaCha20::new(&key, counter, &nonce);
//!
//! // Have some plaintext data.
//! let mut data = hex!(
//!     "4c61646965732061 6e642047656e746c"
//!     "656d656e206f6620 74686520636c6173"
//!     "73206f6620273939 3a20496620492063"
//!     "6f756c64206f6666 657220796f75206f"
//!     "6e6c79206f6e6520 74697020666f7220"
//!     "7468652066757475 72652c2073756e73"
//!     "637265656e20776f 756c642062652069"
//!     "742e"
//! );
//!
//! // Encrypt the data.
//! cipher.xor_key_stream(&mut data).unwrap();
//! ```

#![no_std]

use core::cmp::min;

/// The size of a ChaCha20 key in bytes.
///
pub const KEY_SIZE: usize = 32;

/// The size of a ChaCha20 nonce in bytes.
///
/// Note that this is too short to be
/// safely generated at random if the
/// same key is reused more than 2³²
/// times.
///
pub const NONCE_SIZE: usize = 12;

/// The block used within the ChaCha20
/// state machine in bytes.
///
const BLOCK_SIZE: usize = 64;

/// The first 4 words of the ChaCha20
/// state.
///
const WORDS: [u32; 4] = [
    0x61707865, // expa
    0x3320646e, // nd 3
    0x79622d32, // 2-by
    0x6b206574, // te k
];

/// Represents an error that can occur
/// while using ChaCha20.
///
#[derive(Debug, PartialEq)]
pub enum Error {
    /// The internal counter has overflowed,
    /// exhausting the key stream.
    KeyStreamExhausted,
}

/// The ChaCha20 cipher state.
///
pub struct ChaCha20 {
    key: [u32; 8],
    counter: u32,
    nonce: [u32; 3],

    // Cached partial block, for if we have
    // key stream left over after a call to
    // `xor_key_stream`.
    block: [u8; BLOCK_SIZE],
    block_bytes: usize,

    // This is set when the counter overflows.
    // Once `overflow` is set, no more blocks
    // can be generated and the next attempt
    // to do so will panic.
    overflow: bool,
}

impl ChaCha20 {
    /// Returns a ChaCha20 cipher state,
    /// which can be used to produce a
    /// key stream.
    ///
    pub fn new(key: &[u8; KEY_SIZE], counter: u32, nonce: &[u8; NONCE_SIZE]) -> Self {
        ChaCha20 {
            key: [
                u32::from_le_bytes([key[0x00], key[0x01], key[0x02], key[0x03]]),
                u32::from_le_bytes([key[0x04], key[0x05], key[0x06], key[0x07]]),
                u32::from_le_bytes([key[0x08], key[0x09], key[0x0a], key[0x0b]]),
                u32::from_le_bytes([key[0x0c], key[0x0d], key[0x0e], key[0x0f]]),
                u32::from_le_bytes([key[0x10], key[0x11], key[0x12], key[0x13]]),
                u32::from_le_bytes([key[0x14], key[0x15], key[0x16], key[0x17]]),
                u32::from_le_bytes([key[0x18], key[0x19], key[0x1a], key[0x1b]]),
                u32::from_le_bytes([key[0x1c], key[0x1d], key[0x1e], key[0x1f]]),
            ],
            counter,
            nonce: [
                u32::from_le_bytes([nonce[0x00], nonce[0x01], nonce[0x02], nonce[0x03]]),
                u32::from_le_bytes([nonce[0x04], nonce[0x05], nonce[0x06], nonce[0x07]]),
                u32::from_le_bytes([nonce[0x08], nonce[0x09], nonce[0x0a], nonce[0x0b]]),
            ],
            block: [0u8; BLOCK_SIZE],
            block_bytes: 0,
            overflow: false,
        }
    }

    /// Returns the current key state.
    ///
    fn key_state(&self) -> KeyState {
        KeyState {
            s: [
                WORDS[0],
                WORDS[1],
                WORDS[2],
                WORDS[3],
                self.key[0],
                self.key[1],
                self.key[2],
                self.key[3],
                self.key[4],
                self.key[5],
                self.key[6],
                self.key[7],
                self.counter,
                self.nonce[0],
                self.nonce[1],
                self.nonce[2],
            ],
        }
    }

    /// Fills the given slice with the
    /// next block of key stream, advancing
    /// the key state.
    ///
    fn next_block(&mut self, block: &mut [u8; BLOCK_SIZE]) -> Result<(), Error> {
        if self.overflow {
            return Err(Error::KeyStreamExhausted);
        }

        let next = self.key_state().advance();
        for (i, word) in next.s.iter().enumerate() {
            block[i * 4..i * 4 + 4].copy_from_slice(&word.to_le_bytes());
        }

        (self.counter, self.overflow) = self.counter.overflowing_add(1);

        Ok(())
    }

    /// Performs the ChaCha20 crypt
    /// operation by generating the
    /// next `data.len()` bytes of
    /// key stream and XORing them
    /// with `data`.
    ///
    /// If the key stream has been
    /// exhausted by the counter
    /// overflowing, then `xor_key_stream`
    /// will return an error.
    ///
    pub fn xor_key_stream(&mut self, data: &mut [u8]) -> Result<(), Error> {
        // Make data mutable within the
        // function so we can update the
        // slice as we make progress.
        let mut data = data;

        // Start by using up any leftover
        // key stream in the block cache.
        if self.block_bytes > 0 {
            let n = min(data.len(), self.block_bytes);
            for (d, b) in data
                .iter_mut()
                .zip(self.block[BLOCK_SIZE - self.block_bytes..].iter())
            {
                *d ^= b;
            }

            data = &mut data[n..];
            self.block_bytes -= n;
        }

        // Iterate through the key stream
        // while we have complete blocks
        // of data.
        let mut block = [0u8; BLOCK_SIZE];
        while data.len() >= BLOCK_SIZE {
            self.next_block(&mut block)?;
            for (d, b) in data.iter_mut().zip(block.iter()) {
                *d ^= b;
            }

            data = &mut data[BLOCK_SIZE..];
        }

        // We may need to generate another
        // block for the remaining data,
        // storing the leftover key stream
        // into the block cache.
        if !data.is_empty() {
            self.next_block(&mut block)?;
            let copied = data.len();
            for (d, b) in data.iter_mut().zip(block[..copied].iter()) {
                *d ^= b;
            }

            self.block = block;
            self.block_bytes = BLOCK_SIZE - copied;
        }

        Ok(())
    }
}

/// Represents the ChaCha20 internal key
/// state.
///
#[derive(Clone, Debug, PartialEq)]
struct KeyState {
    s: [u32; 16],
}

impl KeyState {
    /// Performs the ChaCha20 block function.
    /// This duplicates the key state, then
    /// advances it the full 20 rounds,
    /// returning the final processed state.
    ///
    fn advance(self) -> KeyState {
        let mut state = self.clone();

        // Perform the 20 rounds, by doing
        // 10 double rounds, as described in
        // RFC 7539, section 2.3.
        for _ in 0..10 {
            state.quarter_round(0x0, 0x4, 0x8, 0xc);
            state.quarter_round(0x1, 0x5, 0x9, 0xd);
            state.quarter_round(0x2, 0x6, 0xa, 0xe);
            state.quarter_round(0x3, 0x7, 0xb, 0xf);

            state.quarter_round(0x0, 0x5, 0xa, 0xf);
            state.quarter_round(0x1, 0x6, 0xb, 0xc);
            state.quarter_round(0x2, 0x7, 0x8, 0xd);
            state.quarter_round(0x3, 0x4, 0x9, 0xe);
        }

        // Add the original state to the
        // result, using vector addition.
        for (new, old) in state.s.iter_mut().zip(self.s.iter()) {
            *new = new.wrapping_add(*old);
        }

        state
    }

    /// Performs a quarter round operation
    /// on the given indices into the key
    /// state, as described in RFC 7539,
    /// section 2.2.
    ///
    /// Each of the indices must be in the
    /// range [0, 16).
    ///
    fn quarter_round(&mut self, a: usize, b: usize, c: usize, d: usize) {
        (self.s[a], self.s[b], self.s[c], self.s[d]) =
            quarter_round(self.s[a], self.s[b], self.s[c], self.s[d]);
    }
}

/// Performs a quarter round operation, as
/// described in RFC 7539, section 2.1:
///
/// ```c
/// a += b; d ^= a; d <<<= 16;
/// c += d; b ^= c; b <<<= 12;
/// a += b; d ^= a; d <<<= 8;
/// c += d; b ^= c; b <<<= 7;
/// ```
///
fn quarter_round(a: u32, b: u32, c: u32, d: u32) -> (u32, u32, u32, u32) {
    // Make the four values mutable within the function.
    let mut a = a;
    let mut b = b;
    let mut c = c;
    let mut d = d;

    a = a.wrapping_add(b);
    d ^= a;
    d = d.rotate_left(16);

    c = c.wrapping_add(d);
    b ^= c;
    b = b.rotate_left(12);

    a = a.wrapping_add(b);
    d ^= a;
    d = d.rotate_left(8);

    c = c.wrapping_add(d);
    b ^= c;
    b = b.rotate_left(7);

    (a, b, c, d)
}

#[cfg(test)]
mod test {
    use super::*;
    use hex_literal::hex;

    #[test]
    fn test_cipher_xor_key_stream() {
        // Test vector from RFC 7539, section
        // 2.4.2.
        let key = hex!(
            "0001020304050607 08090a0b0c0d0e0f"
            "1011121314151617 18191a1b1c1d1e1f"
        );
        let counter = 1;
        let nonce = hex!("000000000000004a00000000");

        let mut cipher = ChaCha20::new(&key, counter, &nonce);

        let mut data = hex!(
            "4c61646965732061 6e642047656e746c"
            "656d656e206f6620 74686520636c6173"
            "73206f6620273939 3a20496620492063"
            "6f756c64206f6666 657220796f75206f"
            "6e6c79206f6e6520 74697020666f7220"
            "7468652066757475 72652c2073756e73"
            "637265656e20776f 756c642062652069"
            "742e"
        );

        cipher.xor_key_stream(&mut data).unwrap();

        let want = hex!(
            "6e2e359a2568f980 41ba0728dd0d6981"
            "e97e7aec1d4360c2 0a27afccfd9fae0b"
            "f91b65c5524733ab 8f593dabcd62b357"
            "1639d624e65152ab 8f530c359f0861d8"
            "07ca0dbf500d6a61 56a38e088a22b65e"
            "52bc514d16ccf806 818ce91ab7793736"
            "5af90bbf74a35be6 b40b8eedf2785e42"
            "874d"
        );

        assert_eq!(data, want);
    }

    #[test]
    fn test_cipher_next_block() {
        // Test vector from RFC 7539, section
        // 2.3.2.
        let key = hex!(
            "0001020304050607 08090a0b0c0d0e0f"
            "1011121314151617 18191a1b1c1d1e1f"
        );
        let counter = 1;
        let nonce = hex!("000000090000004a00000000");

        let mut cipher = ChaCha20::new(&key, counter, &nonce);

        let mut got = [0u8; BLOCK_SIZE];
        cipher.next_block(&mut got).unwrap();

        let want = hex!(
            "10f1e7e4d13b5915 500fdd1fa32071c4"
            "c7d1f4c733c06803 0422aa9ac3d46c4e"
            "d2826446079faa09 14c2d705d98b02a2"
            "b5129cd1de164eb9 cbd083e8a2503c4e"
        );

        assert_eq!(got, want);
    }

    #[test]
    fn test_key_state_advance() {
        // Test vector from RFC 7539, section
        // 2.3.2.
        let initial = KeyState {
            s: [
                0x61707865, 0x3320646e, 0x79622d32, 0x6b206574, 0x03020100, 0x07060504, 0x0b0a0908,
                0x0f0e0d0c, 0x13121110, 0x17161514, 0x1b1a1918, 0x1f1e1d1c, 0x00000001, 0x09000000,
                0x4a000000, 0x00000000,
            ],
        };

        let second = initial.advance();

        let want = KeyState {
            s: [
                0xe4e7f110, 0x15593bd1, 0x1fdd0f50, 0xc47120a3, 0xc7f4d1c7, 0x0368c033, 0x9aaa2204,
                0x4e6cd4c3, 0x466482d2, 0x09aa9f07, 0x05d7c214, 0xa2028bd9, 0xd19c12b5, 0xb94e16de,
                0xe883d0cb, 0x4e3c50a2,
            ],
        };

        assert_eq!(second, want);
    }

    #[test]
    fn test_key_state_quarter_round() {
        // Test vector from RFC 7539, section
        // 2.2.1.
        let mut state = KeyState {
            s: [
                0x879531e0, 0xc5ecf37d, 0x516461b1, 0xc9a62f8a, 0x44c20ef3, 0x3390af7f, 0xd9fc690b,
                0x2a5f714c, 0x53372767, 0xb00a5631, 0x974c541a, 0x359e9963, 0x5c971061, 0x3d631689,
                0x2098d9d6, 0x91dbd320,
            ],
        };

        state.quarter_round(2, 7, 8, 13);

        let want = KeyState {
            s: [
                0x879531e0, 0xc5ecf37d, 0xbdb886dc, 0xc9a62f8a, 0x44c20ef3, 0x3390af7f, 0xd9fc690b,
                0xcfacafd2, 0xe46bea80, 0xb00a5631, 0x974c541a, 0x359e9963, 0x5c971061, 0xccc07c79,
                0x2098d9d6, 0x91dbd320,
            ],
        };

        assert_eq!(state, want);
    }

    #[test]
    fn test_quarter_round() {
        // Test vector from RFC 7539, section
        // 2.1.1.
        let (a, b, c, d) = quarter_round(0x11111111, 0x01020304, 0x9b8d6f43, 0x01234567);
        assert_eq!(a, 0xea2a92f4, "wrong value for `a` after quarter round");
        assert_eq!(b, 0xcb1cf8ce, "wrong value for `b` after quarter round");
        assert_eq!(c, 0x4581472e, "wrong value for `c` after quarter round");
        assert_eq!(d, 0x5881c4bb, "wrong value for `d` after quarter round");
    }
}
