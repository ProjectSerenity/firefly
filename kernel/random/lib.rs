// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides a cryptographically secure pseudo-random number generator (CSPRNG).
//!
//! ## Design and algorithm selection
//!
//! The design here is directly inspired by [Fuchsia's](https://fuchsia.dev/fuchsia-src/concepts/kernel/cprng).
//!
//! There are two main methods on the RNG: `add_entropy` and `read`. Calling `add_entropy`
//! mixes bytes into the current entropy pool, which is a 256-bit buffer. The bytes are
//! mixed in by replacing the buffer's contents with the SHA-256 hash of the new data,
//! followed by the buffer's previous contents:
//!
//! ```
//! buffer = SHA-256(new-entropy || buffer);
//! ```
//!
//! Calling `read` populates the supplied buffer with random bytes by encrypting it using
//! ChaCha20, where the key is the entropy buffer and the nonce is a monotonically incrementing
//! 96-bit integer, which starts at 0. Note that the nonce is never reset, and if it reaches
//! 2^95, the kernel panics. This feels like a reasonable limit for now, but may change in
//! the future.
//!
//! To make the CSPRNG usable, `seed` must be called at least once before `read` is called.
//! `seed` is a specialised version of `add_entropy`, which requires exactly 256 bits of entropy.
//!
//! ## Initialisation
//!
//! To prepare the CSPRNG, at least one entropy source must be registered by calling [`register_entropy_source`]
//! (or the host must support the [`RDRAND`](https://en.wikipedia.org/wiki/RDRAND) instruction),
//! then [`init`] must be called to seed the CSPRNG and start the companion thread. This thread
//! re-seeds the CSPRNG with more entropy every 30 seconds.
//!
//! Once the CSPRNG has been initialised with `init`, memory buffers can be filled with random
//! data by calling [`read`].
//!
//! # Examples
//!
//! Fill a buffer with random data.
//!
//! ```
//! let mut buf = [0u8; 16];
//! random::read(&mut buf[..]);
//! ```

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![allow(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(deprecated_in_future)]
#![deny(keyword_idents)]
#![deny(macro_use_extern_crate)]
#![deny(missing_abi)]
#![forbid(unsafe_code)]
#![deny(unused_crate_dependencies)]

extern crate alloc;

mod csprng;
mod rdrand;

use alloc::boxed::Box;
use alloc::vec::Vec;
use spin::{lock, Mutex};
use time::Duration;

/// CSPRNG is the kernel's cryptographically secure pseudo-random number generator.
///
/// CSPRNG must be seeded by at least one source of entropy before use. The kernel
/// will panic if read is called before CSPRNG has been seeded. We also start a
/// companion kernel thread to add more entropy from each available source every
/// 30 seconds.
///
/// We do not currently expose add_entropy more widely, but that may change in the
/// future.
///
static CSPRNG: Mutex<csprng::Csprng> = Mutex::new(csprng::Csprng::new());

/// Fills the given buffer with random data.
///
/// # Panics
///
/// `read` will panic if the CSPRNG has not been seeded by registering at least one
/// entropy source, then calling [`init`].
///
pub fn read(buf: &mut [u8]) {
    lock!(CSPRNG).read(buf);
}

/// EntropySource is a trait we use to simplify the process of collecting sources
/// of entropy.
///
pub trait EntropySource: Send {
    /// get_entropy fills the given buffer with entropy.
    ///
    fn get_entropy(&mut self, buf: &mut [u8; 32]);
}

/// ENTROPY_SOURCES is our set of entropy sources, supplied using register_entropy_source.
///
static ENTROPY_SOURCES: Mutex<Vec<Box<dyn EntropySource>>> = Mutex::new(Vec::new());

/// Provide an ongoing source of entropy to the kernel for use in seeding the CSPRNG.
///
/// `register_entropy_source` should be called before calling [`init`] to ensure it
/// provides entropy when the CSPRNG is first set up. However, provided at least one
/// entropy source was available when `init` was called, subsequent calls to `register_entropy_source`
/// will still result in the entropy source being used during the periodic re-seeding
/// of the CSPRNG.
///
pub fn register_entropy_source(src: Box<dyn EntropySource>) {
    lock!(ENTROPY_SOURCES).push(src);
}

/// Initialise the CSPNRG using the entropy sources that have been registered.
///
/// `init` also starts a companion kernel thread to ensure the CSPRNG gets a steady
/// feed of entropy  over time.
///
/// # Panics
///
/// `init` will panic if no sources of entropy are available.
///
pub fn init() {
    // Detect RDRAND support before we lock ENTROPY_SOURCES.
    rdrand::init();

    let mut csprng = lock!(CSPRNG);
    let mut sources = lock!(ENTROPY_SOURCES);
    if sources.is_empty() {
        panic!("random::init called without any entropy sources registered");
    }

    let mut buf = [0u8; 32];
    for source in sources.iter_mut() {
        source.get_entropy(&mut buf);
        csprng.seed(&buf);
    }
}

/// RESEED_INTERVAL is the maximum amount of time after the entropy pool was last
/// reseeded before it should be reseeded again.
///
pub const RESEED_INTERVAL: Duration = Duration::from_secs(30);

/// Reseed the CSPRNG's entropy pool. This should be performed periodically, no
/// less frequently than [`RESEED_INTERVAL`].
///
/// # Panics
///
/// `reseed` will panic if no sources of entropy are available.
///
pub fn reseed() {
    let mut buf = [0u8; 32];
    let mut csprng = lock!(CSPRNG);
    let mut sources = lock!(ENTROPY_SOURCES);
    if sources.is_empty() {
        panic!("all entropy sources removed");
    }

    for source in sources.iter_mut() {
        source.get_entropy(&mut buf);
        csprng.seed(&buf);
    }
}
