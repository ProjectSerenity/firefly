// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

#![no_std]
#![allow(clippy::float_arithmetic)] // Allowed in userspace.
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(deprecated_in_future)]
#![deny(keyword_idents)]
#![deny(macro_use_extern_crate)]
#![deny(missing_abi)]
#![forbid(unsafe_code)]
#![deny(unused_crate_dependencies)]

use firefly::{current_thread_id, println, read_random};

/// The application entry point.
///
#[inline]
pub fn main() {
    let mut buf = [0u8; 8];
    read_random(&mut buf[..]);
    let thread_id = current_thread_id();
    println!("Hello from userland {:?}: {:x?}!", thread_id, &buf[..]);
    println!("main() is at {:p}", main as *const u8);
}
