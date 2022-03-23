// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! rng implements the cryptographically secure pseudo-random number generator (CSPRNG)
//! used in the Firefly kernel.

// The design here is directly inspired by Fuchsia's (https://fuchsia.dev/fuchsia-src/concepts/kernel/cprng).
//
// There are two main methods on the RNG: add_entropy and read. Calling add_entropy mixes
// bytes into the current entropy pool, which is a 256-bit buffer. The bytes are mixed in
// by replacing the buffer's contents with the SHA-256 hash of the new data, followed by
// the buffer's previous contents:
//
//     buffer = SHA-256(new-entropy || buffer);
//
// Calling read populates the supplied buffer with random bytes by encrypting it using
// ChaCha20, where the key is the entropy buffer and the nonce is a monotonically incrementing
// 96-bit integer, which starts at 0. Note that the nonce is never reset, and if it reaches
// 2^95, the kernel panics. This feels like a reasonable limit for now, but may change in
// the future.
//
// To make the CSPRNG usable, seed must be called at least once before read is called.
// seed is a specialised version of add_entropy, which requires exactly 256 bits of entropy.

use chacha20::ChaCha20;
use sha256::Sha256;

/// ENTROPY_BITS is the number of bits in the entropy pool.
///
const ENTROPY_BITS: usize = 256;

/// ENTROPY_BYTES is the number of bytes in the entropy pool.
///
const ENTROPY_BYTES: usize = ENTROPY_BITS / 8;

/// NONCE_OVERFLOW is the nonce value at which we panic and
/// give up.
///
const NONCE_OVERFLOW: u128 = 1 << 95;

/// Csprng is a cryptographically secure pseudo-random number generator.
pub struct Csprng {
    // entropy is our 256-bit pool of current entropy, which
    // is used as the key in ChaCha20.
    entropy: [u8; ENTROPY_BYTES],

    // counter is our 96-bit counter, which is used as the
    // nonce in ChaCha20.
    counter: u128,

    // seeded notes whether the entropy pool has been seeded
    // with initial data. The CSPRNG must not be used (and will
    // panic) if read is called while seeded is false.
    seeded: bool,
}

impl Csprng {
    pub const fn new() -> Self {
        Csprng {
            entropy: [0u8; ENTROPY_BYTES],
            counter: 0,
            seeded: false,
        }
    }

    /// add_entropy can be called to provide entropy to the
    /// CSPRNG.
    ///
    pub fn add_entropy(&mut self, entropy: &[u8]) {
        let mut sha256 = Sha256::new();
        sha256.update(entropy);
        sha256.update(&self.entropy);
        sha256.sum(&mut self.entropy);
    }

    /// seed is a specialised version of add_entropy, which
    /// must be called before read can be called without it
    /// panicking.
    ///
    pub fn seed(&mut self, entropy: &[u8; ENTROPY_BYTES]) {
        self.add_entropy(&entropy[..]);
        self.seeded = true;
    }

    /// read draws entropy from the CSPRNG by encrypting the
    /// passed buffer. read will always encrypt the entire
    /// buffer, so no length is returned.
    ///
    /// read will panic if seed has not been called, or if
    /// the counter reaches 2^95.
    ///
    pub fn read(&mut self, buf: &mut [u8]) {
        // Check we've been seeded with at least
        // 256 bits of entropy.
        if !self.seeded {
            panic!("CSPRNG::read called without being seeded");
        }

        // Increment the nonce. We don't need
        // to worry about it wrapping, as we
        // cap the value at 2^95.
        self.counter += 1;
        if self.counter >= NONCE_OVERFLOW {
            panic!("CSPRNG nonce overflowed");
        }

        // Encrypt the input buffer.
        // nonce is the first 96 bits of nonce, in
        // little-endian format.
        let nonce = [
            self.counter as u8,
            (self.counter >> 8) as u8,
            (self.counter >> 16) as u8,
            (self.counter >> 24) as u8,
            (self.counter >> 32) as u8,
            (self.counter >> 40) as u8,
            (self.counter >> 48) as u8,
            (self.counter >> 56) as u8,
            (self.counter >> 64) as u8,
            (self.counter >> 72) as u8,
            (self.counter >> 80) as u8,
            (self.counter >> 88) as u8,
        ];

        let mut cipher = ChaCha20::new(&self.entropy, 0, &nonce);
        cipher
            .xor_key_stream(buf)
            .expect("failed to generate random data");
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn csprng() {
        use hex_literal::hex;

        let mut csprng = Csprng::new();
        let mut mixin = hex!("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f");
        let entropy = hex!("5576ce645abbf23973c63a02b3cdb0efc8ed3c9bd7dac3845f6b9ad6820b4bde");
        let random1 = hex!("5064209e4d5aabe42c7deb96ed27955b29dbb87e69b4c083ebe45935a0325150");
        let random2 = hex!("fa0b31ee61c5a7ff2fdbe3d85586e262ac9e69c6f84702c9bea1cb73df836bda");

        // Check seed works correctly.
        csprng.seed(&mixin);
        assert_eq!(csprng.entropy, entropy);

        // Check read works correctly.
        let mut buf = &mut mixin[..];
        csprng.read(&mut buf);
        assert_eq!(buf, random1);

        csprng.read(&mut buf);
        assert_eq!(buf, random2);

        // The test vectors above were generated
        // by running the following Go program:
        //
        //     package main
        //
        //     import (
        //         "crypto/sha256"
        //         "fmt"
        //
        //         "golang.org/x/crypto/chacha20"
        //     )
        //
        //     var mixin = [32]byte{
        //         0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
        //         0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
        //         0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
        //         0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
        //     }
        //
        //     func random(key, nonce, buf []byte) {
        //         cipher, err := chacha20.NewUnauthenticatedCipher(key, nonce)
        //         if err != nil {
        //             panic(err.Error())
        //         }
        //
        //         cipher.XORKeyStream(buf, buf)
        //     }
        //
        //     func main() {
        //         entropy := make([]byte, sha256.Size)
        //         hash := sha256.New()
        //         hash.Write(mixin[:])
        //         hash.Write(entropy)
        //         entropy = hash.Sum(entropy[:0])
        //         fmt.Printf("Entropy:  %x\n", entropy)
        //
        //         key := entropy
        //         nonce := make([]byte, 96/8)
        //         buf := mixin[:]
        //
        //         nonce[0] = 1
        //         random(key, nonce, buf)
        //         fmt.Printf("Random 1: %x\n", buf)
        //
        //         nonce[0] = 2
        //         random(key, nonce, buf)
        //         fmt.Printf("Random 2: %x\n", buf)
        //     }
    }
}
