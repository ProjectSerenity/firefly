// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

#![no_std]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]
#![deny(unsafe_code)]

use firefly::println;
use firefly::syscalls::read_random;

/// The application entry point.
///
#[inline]
pub fn main() {
    let mut buf = [0u8; 8];
    read_random((&mut buf[..]).as_mut_ptr(), buf.len() as u64);
    println!("Hello from userland: {:x?}!", &buf[..]);
}
