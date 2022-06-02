// Forked from bootloader 0.9.22, copyright 2018 Philipp Oppermann.
//
// Use of the original source code is governed by the MIT
// license that can be found in the LICENSE.orig file.
//
// Subsequent work copyright 2022 The Firefly Authors.
//
// Use of new and modified source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! This library part of the bootloader allows kernels to retrieve information from the bootloader.
//!
//! To combine your kernel with the bootloader crate you need a tool such
//! as [`bootimage`](https://github.com/rust-osdev/bootimage). See the
//! [_Writing an OS in Rust_](https://os.phil-opp.com/minimal-rust-kernel/#creating-a-bootimage)
//! blog for an explanation.

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]
#![forbid(unsafe_code)]
