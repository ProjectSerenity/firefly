// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements the SHA-256 cryptographic hash algorithm.
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
//! # Examples
//!
//! Calculate the SHA-256 digest of a single slice of bytes:
//!
//! ```
//! use sha256::digest;
//!
//! let got = digest(b"The quick brown fox jumps over the lazy dog.");
//!
//! let want = hex!("ef537f25c895bfa782526529a9b63d97aa631564d5d789c2b765448c8635fb6c");
//! assert_eq!(&digest, &want);
//! ```
//!
//! Build a SHA-256 digest gradually:
//!
//! ```
//! use sha256::{Sha256, SIZE};
//!
//! // Create the hash state.
//! let hash = Sha256::new();
//!
//! // Write data to update the hash state.
//! hash.update(b"The quick brown fo");
//! hash.update(b"x jumps over the lazy dog.");
//!
//! // Produce the hash digest.
//! let mut got = [0u8; SIZE];
//! hash.sum(&mut got);
//!
//! let want = hex!("ef537f25c895bfa782526529a9b63d97aa631564d5d789c2b765448c8635fb6c");
//! assert_eq!(&got, &want);
//! ```

#![no_std]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]

use core::cmp::min;
use core::default::Default;

/// The size of a SHA-256 checksum in bytes.
///
pub const SIZE: usize = 256 / 8;

/// The state size of SHA-256 in bytes.
///
const STATE_SIZE: usize = 8;

/// The chunk size of SHA-256 in bytes.
///
const CHUNK_SIZE: usize = 64;

/// The SHA-256 initial state.
///
const INITIAL_STATE: [u32; STATE_SIZE] = [
    0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19,
];

/// The SHA-256 round constants.
///
const K: [u32; 64] = [
    0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
    0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
    0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
    0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
    0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
    0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
    0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
    0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
];

/// Computes and returns the SHA-256
/// digest of the input.
///
/// # Examples
///
/// Calculate the SHA-256 digest of a single slice of bytes:
///
/// ```
/// use sha256::digest;
///
/// let got = digest(b"The quick brown fox jumps over the lazy dog.");
///
/// let want = hex!("ef537f25c895bfa782526529a9b63d97aa631564d5d789c2b765448c8635fb6c");
/// assert_eq!(&digest, &want);
/// ```
///
pub fn digest(data: &[u8]) -> [u8; SIZE] {
    let mut hash = Sha256::new();
    hash.update(data);
    let mut out = [0u8; SIZE];
    hash.sum(&mut out);
    out
}

/// A SHA-256 hash state.
///
/// # Examples
///
/// Build a SHA-256 digest gradually:
///
/// ```
/// use sha256::{Sha256, SIZE};
///
/// // Create the hash state.
/// let hash = Sha256::new();
///
/// // Write data to update the hash state.
/// hash.update(b"The quick brown fo");
/// hash.update(b"x jumps over the lazy dog.");
///
/// // Produce the hash digest.
/// let mut got = [0u8; SIZE];
/// hash.sum(&mut got);
///
/// let want = hex!("ef537f25c895bfa782526529a9b63d97aa631564d5d789c2b765448c8635fb6c");
/// assert_eq!(&got, &want);
/// ```
///
#[derive(Clone, PartialEq)]
pub struct Sha256 {
    // The current hash state.
    state: [u32; STATE_SIZE],

    // The current chunk of partial
    // data that has not been processed
    // yet.
    chunk: [u8; CHUNK_SIZE],

    // The number of bytes in `self.chunk`.
    chunk_bytes: usize,

    // The total number of bits hashed
    // into the current state, `l`.
    total_bits: u64,
}

impl Sha256 {
    /// Builds a fresh SHA-256 state, which
    /// is immediately ready for use.
    ///
    pub fn new() -> Self {
        Sha256 {
            state: INITIAL_STATE, // Copy.
            chunk: [0u8; CHUNK_SIZE],
            chunk_bytes: 0,
            total_bits: 0,
        }
    }

    /// Resets to a fresh state.
    ///
    pub fn reset(&mut self) {
        *self = Self::new();
    }

    /// Updates the state with the given
    /// input data.
    ///
    pub fn update(&mut self, data: &[u8]) {
        // Make data a mutable variable in case
        // we need to update it after processing
        // any incomplete chunks.
        let mut data = data;

        // Record how much data we've mixed in.
        // This is expressed as a 64-bit integer,
        // but the value may exceed that.
        self.total_bits = self
            .total_bits
            .wrapping_add((data.len() as u64).wrapping_mul(8));

        // Finish any pending data in the chunk.
        if self.chunk_bytes > 0 {
            // Populate as much of the initial
            // chunk as we can without overflowing
            // it.
            let copied = min(CHUNK_SIZE - self.chunk_bytes, data.len());
            self.chunk[self.chunk_bytes..self.chunk_bytes + copied]
                .copy_from_slice(&data[..copied]);
            self.chunk_bytes += copied; // Cannot overflow.

            // If we now have a complete chunk,
            // we process it.
            if self.chunk_bytes == CHUNK_SIZE {
                let chunk = self.chunk; // Copy.
                self.process_chunks(&chunk[..]);
                self.chunk_bytes = 0;
            }

            // Skip over the bytes we've just
            // processed.
            data = &data[copied..];
        }

        // Process any complete chunks we have,
        // directly from the input slice.
        if data.len() >= CHUNK_SIZE {
            // Trim the number of bytes down to
            // an exact multiple of the chunk
            // size.
            let processed = data.len() & !(CHUNK_SIZE - 1);
            self.process_chunks(&data[..processed]);

            // Skip over the bytes we've just
            // processed.
            data = &data[processed..];
        }

        // Add any remaining data to the chunk
        // cache.
        if !data.is_empty() {
            self.chunk[..data.len()].copy_from_slice(data);
            self.chunk_bytes = data.len();
        }
    }

    /// Processes the given data, which must be
    /// an exact multiple of [`CHUNK_SIZE`].
    ///
    fn process_chunks(&mut self, data: &[u8]) {
        // Make data a mutable variable in case
        // we need to update it after processing
        // any incomplete chunks.
        let mut data = data;

        // For more context, see https://en.wikipedia.org/wiki/SHA-2#Pseudocode

        // Create the message schedule.
        let mut w = [0u32; 64];

        while data.len() >= CHUNK_SIZE {
            // Copy the chunk data into
            // the first 16 words of the
            // message schedule.
            for (i, w) in w.iter_mut().enumerate().take(16) {
                let d = &data[i * 4..i * 4 + 4];
                *w = u32::from_be_bytes([d[0], d[1], d[2], d[3]]);
            }

            // Extend the first 16 words
            // into the remaining 48 words
            // of the message schedule array.
            for i in 16..64 {
                let s0 = w[i - 15].rotate_right(7) ^ w[i - 15].rotate_right(18) ^ (w[i - 15] >> 3);
                let s1 = w[i - 2].rotate_right(17) ^ w[i - 2].rotate_right(19) ^ (w[i - 2] >> 10);
                w[i] = w[i - 16]
                    .wrapping_add(s0)
                    .wrapping_add(w[i - 7])
                    .wrapping_add(s1);
            }

            // Initialise current hash value
            // into working variables.
            let mut a = self.state[0];
            let mut b = self.state[1];
            let mut c = self.state[2];
            let mut d = self.state[3];
            let mut e = self.state[4];
            let mut f = self.state[5];
            let mut g = self.state[6];
            let mut h = self.state[7];

            // Compression function main loop.
            for i in 0..64 {
                let s1 = e.rotate_right(6) ^ e.rotate_right(11) ^ e.rotate_right(25);
                let ch = (e & f) ^ (!e & g);
                let t1 = h
                    .wrapping_add(s1)
                    .wrapping_add(ch)
                    .wrapping_add(K[i])
                    .wrapping_add(w[i]);
                let s0 = a.rotate_right(2) ^ a.rotate_right(13) ^ a.rotate_right(22);
                let ma = (a & b) ^ (a & c) ^ (b & c);
                let t2 = s0.wrapping_add(ma);

                h = g;
                g = f;
                f = e;
                e = d.wrapping_add(t1);
                d = c;
                c = b;
                b = a;
                a = t1.wrapping_add(t2);
            }

            // Add the compressed chunk to
            // the current hash value.
            self.state[0] = self.state[0].wrapping_add(a);
            self.state[1] = self.state[1].wrapping_add(b);
            self.state[2] = self.state[2].wrapping_add(c);
            self.state[3] = self.state[3].wrapping_add(d);
            self.state[4] = self.state[4].wrapping_add(e);
            self.state[5] = self.state[5].wrapping_add(f);
            self.state[6] = self.state[6].wrapping_add(g);
            self.state[7] = self.state[7].wrapping_add(h);

            // Skip over the bytes we've just
            // processed.
            data = &data[CHUNK_SIZE..];
        }
    }

    /// Produce the final hash output,
    /// writing it into out.
    ///
    pub fn sum(&self, out: &mut [u8; SIZE]) {
        // Make a copy of the state so we
        // don't modify it when we write
        // in the length data. This allows
        // the state to be reused in later
        // work, extending the current
        // state.
        let mut state = self.clone();

        // Calculate the suffix, which
        // pads the data written so far
        // to the smallest number of
        // bytes that is an exact multiple
        // of [`CHUNK_SIZE`] minus 8 bytes.
        // The padding contains all zeros,
        // except the first bit, which is
        // a 1. Finally, we append the
        // number of data bytes written,
        // in big-endian form. This suffix,
        // plus any remaining pending data
        // in self.chunk, takes the written
        // data to an exact multiple of
        // [`CHUNK_SIZE`].
        let mut suffix = [0u8; 2 * CHUNK_SIZE];
        suffix[0] = 0x80; // Set the first bit.

        let offset = if state.chunk_bytes < (CHUNK_SIZE - 8) {
            // There's enough space in one chunk.
            (CHUNK_SIZE - 8) - state.chunk_bytes
        } else {
            // We need two chunks.
            (2 * CHUNK_SIZE - 8) - state.chunk_bytes
        };

        // Write the number of bits written
        // so far.
        suffix[offset..offset + 8].copy_from_slice(&state.total_bits.to_be_bytes());

        // Write the suffix to the final
        // state.
        state.update(&suffix[..offset + 8]);

        // Extract the state into the output,
        // as a sequence of big-endian words.
        for (i, w) in state.state.iter().enumerate() {
            out[i * 4..i * 4 + 4].copy_from_slice(&w.to_be_bytes());
        }
    }
}

impl Default for Sha256 {
    /// Builds a fresh SHA-256 state, which
    /// is immediately ready for use.
    ///
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use hex_literal::hex;

    /// Macro to test the implementation using
    /// a test vector. We use a macro so that
    /// any panic uses the line number of the
    /// vector that failed.
    ///
    macro_rules! test_vector {
        ($in:expr, $want:expr) => {
            // Check the helper function works.
            let mut got = super::digest($in);
            assert_eq!(
                &got, &$want,
                "sha256::digest():\nGot:  {:02x?}\nWant: {:02x?}",
                &got, &$want
            );

            // Check we can reuse the hash state
            // and can split the input into
            // chunks and still get the right
            // digest.
            let mut hash = Sha256::new();
            for i in 0..3 {
                if i < 2 {
                    // Write the whole input in one go.
                    hash.update(&$in[..]);
                } else {
                    // Write the input in two chunks,
                    // calculating (and discarding)
                    // the digest in between, to check
                    // the running state remains viable.
                    hash.update(&$in[..$in.len() / 2]);
                    hash.sum(&mut got);
                    hash.update(&$in[$in.len() / 2..]);
                }

                // Check we get the right output each
                // time.
                hash.sum(&mut got);
                assert_eq!(
                    &got, &$want,
                    "sha256::sum(): loop {}\nGot:  {:02x?}\nWant: {:02x?}",
                    i, &got, &$want
                );
                hash.reset();
            }
        };
    }

    #[test]
    fn test_vectors() {
        // Check each test vector. Vectors copied from Go's "crypto/sha256".
        test_vector!(
            b"",
            hex!("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
        );
        test_vector!(
            b"a",
            hex!("ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb")
        );
        test_vector!(
            b"ab",
            hex!("fb8e20fc2e4c3f248c60c39bd652f3c1347298bb977b8b4d5903b85055620603")
        );
        test_vector!(
            b"abc",
            hex!("ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")
        );
        test_vector!(
            b"abcd",
            hex!("88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589")
        );
        test_vector!(
            b"abcde",
            hex!("36bbe50ed96841d10443bcb670d6554f0a34b761be67ec9c4a8ad2c0c44ca42c")
        );
        test_vector!(
            b"abcdef",
            hex!("bef57ec7f53a6d40beb640a780a639c83bc29ac8a9816f1fc6c5c6dcd93c4721")
        );
        test_vector!(
            b"abcdefg",
            hex!("7d1a54127b222502f5b79b5fb0803061152a44f92b37e23c6527baf665d4da9a")
        );
        test_vector!(
            b"abcdefgh",
            hex!("9c56cc51b374c3ba189210d5b6d4bf57790d351c96c47c02190ecf1e430635ab")
        );
        test_vector!(
            b"abcdefghi",
            hex!("19cc02f26df43cc571bc9ed7b0c4d29224a3ec229529221725ef76d021c8326f")
        );
        test_vector!(
            b"abcdefghij",
            hex!("72399361da6a7754fec986dca5b7cbaf1c810a28ded4abaf56b2106d06cb78b0")
        );
        test_vector!(
            b"Discard medicine more than two years old.",
            hex!("a144061c271f152da4d151034508fed1c138b8c976339de229c3bb6d4bbb4fce")
        );
        test_vector!(
            b"He who has a shady past knows that nice guys finish last.",
            hex!("6dae5caa713a10ad04b46028bf6dad68837c581616a1589a265a11288d4bb5c4")
        );
        test_vector!(
            b"I wouldn't marry him with a ten foot pole.",
            hex!("ae7a702a9509039ddbf29f0765e70d0001177914b86459284dab8b348c2dce3f")
        );
        test_vector!(
            b"Free! Free!/A trip/to Mars/for 900/empty jars/Burma Shave",
            hex!("6748450b01c568586715291dfa3ee018da07d36bb7ea6f180c1af6270215c64f")
        );
        test_vector!(
            b"The days of the digital watch are numbered.  -Tom Stoppard",
            hex!("14b82014ad2b11f661b5ae6a99b75105c2ffac278cd071cd6c05832793635774")
        );
        test_vector!(
            b"Nepal premier won't resign.",
            hex!("7102cfd76e2e324889eece5d6c41921b1e142a4ac5a2692be78803097f6a48d8")
        );
        test_vector!(
            b"For every action there is an equal and opposite government program.",
            hex!("23b1018cd81db1d67983c5f7417c44da9deb582459e378d7a068552ea649dc9f")
        );
        test_vector!(
            b"His money is twice tainted: 'taint yours and 'taint mine.",
            hex!("8001f190dfb527261c4cfcab70c98e8097a7a1922129bc4096950e57c7999a5a")
        );
        test_vector!(
            b"There is no reason for any individual to have a computer in their home. -Ken Olsen, 1977",
            hex!("8c87deb65505c3993eb24b7a150c4155e82eee6960cf0c3a8114ff736d69cad5")
        );
        test_vector!(
            b"It's a tiny change to the code and not completely disgusting. - Bob Manchek",
            hex!("bfb0a67a19cdec3646498b2e0f751bddc41bba4b7f30081b0b932aad214d16d7")
        );
        test_vector!(
            b"size:  a.out:  bad magic",
            hex!("7f9a0b9bf56332e19f5a0ec1ad9c1425a153da1c624868fda44561d6b74daf36")
        );
        test_vector!(
            b"The major problem is with sendmail.  -Mark Horton",
            hex!("b13f81b8aad9e3666879af19886140904f7f429ef083286195982a7588858cfc")
        );
        test_vector!(
            b"Give me a rock, paper and scissors and I will move the world.  CCFestoon",
            hex!("b26c38d61519e894480c70c8374ea35aa0ad05b2ae3d6674eec5f52a69305ed4")
        );
        test_vector!(
            b"If the enemy is within range, then so are you.",
            hex!("049d5e26d4f10222cd841a119e38bd8d2e0d1129728688449575d4ff42b842c1")
        );
        test_vector!(
            b"It's well we cannot hear the screams/That we create in others' dreams.",
            hex!("0e116838e3cc1c1a14cd045397e29b4d087aa11b0853fc69ec82e90330d60949")
        );
        test_vector!(
            b"You remind me of a TV show, but that's all right: I watch it anyway.",
            hex!("4f7d8eb5bcf11de2a56b971021a444aa4eafd6ecd0f307b5109e4e776cd0fe46")
        );
        test_vector!(
            b"C is as portable as Stonehedge!!",
            hex!("61c0cc4c4bd8406d5120b3fb4ebc31ce87667c162f29468b3c779675a85aebce")
        );
        test_vector!(
            b"Even if I could be Shakespeare, I think I should still choose to be Faraday. - A. Huxley",
            hex!("1fb2eb3688093c4a3f80cd87a5547e2ce940a4f923243a79a2a1e242220693ac")
        );
        test_vector!(
            b"The fugacity of a constituent in a mixture of gases at a given temperature is proportional to its mole fraction.  Lewis-Randall Rule",
            hex!("395585ce30617b62c80b93e8208ce866d4edc811a177fdb4b82d3911d8696423")
        );
        test_vector!(
            b"How can you write a big system without C++?  -Paul Glick",
            hex!("4f9b189a13d030838269dce846b16a1ce9ce81fe63e65de2f636863336a98fe6")
        );
    }
}
