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

pub use crate::bootinfo::BootInfo;

pub mod bootinfo;

/// Defines the entry point function.
///
/// The function must have the signature `fn(&'static BootInfo) -> !`.
///
/// This macro just creates a function named `_start`, which the linker will use as the entry
/// point. The advantage of using this macro instead of providing an own `_start` function is
/// that the macro ensures that the function and argument types are correct.
#[macro_export]
macro_rules! entry_point {
    ($path:path) => {
        #[export_name = "_start"]
        pub extern "C" fn __impl_start(boot_info: &'static $crate::bootinfo::BootInfo) -> ! {
            // validate the signature of the program entry point
            let f: fn(&'static $crate::bootinfo::BootInfo) -> ! = $path;

            f(boot_info)
        }
    };
}
