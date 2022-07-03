// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides helper functions for calling Firefly syscalls.

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![allow(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(deprecated_in_future)]
#![deny(unused_crate_dependencies)]
#![allow(unsafe_code)]

use core::fmt;
use core::fmt::Write;
use firefly_syscalls as syscalls;

/// Exit the current thread.
///
pub fn exit_thread() -> ! {
    // Ignore any error, as it should never happen in practice.
    let _ = syscalls::exit_thread();
    unreachable!();
}

/// Read cryptographically-secure
/// pseudorandom numbers into the
/// memory buffer.
///
/// # Panics
///
/// `read_random` will panic if `buf`
/// is not a valid byte slice.
///
pub fn read_random(buf: &mut [u8]) {
    syscalls::read_random((&mut buf[..]).as_mut_ptr(), buf.len() as u64).unwrap();
}

/// The process's standard output.
///
#[derive(Clone, Copy, Debug)]
struct StandardOutput;

impl StandardOutput {
    pub fn new() -> Self {
        StandardOutput
    }
}

impl Default for StandardOutput {
    fn default() -> Self {
        Self::new()
    }
}

impl Write for StandardOutput {
    fn write_str(&mut self, s: &str) -> fmt::Result {
        if syscalls::print_message(s.as_ptr(), s.len() as u64).is_ok() {
            Ok(())
        } else {
            Err(fmt::Error::default())
        }
    }
}

/// _print writes text to the standard
/// error output.
///
#[doc(hidden)]
pub fn _print(args: ::core::fmt::Arguments) {
    StandardOutput::new()
        .write_fmt(args)
        .expect("Printing to standard output failed");
}

/// Print to the standard output.
///
#[macro_export]
macro_rules! print {
    ($($arg:tt)*) => ($crate::_print(format_args!($($arg)*)));
}

/// Print to the standard output.
///
#[macro_export]
macro_rules! println {
    () => ($crate::print!("\n"));
    ($($arg:tt)*) => ($crate::print!("{}\n", format_args!($($arg)*)));
}

/// The process's standard error output.
///
#[derive(Clone, Copy, Debug)]
struct StandardError;

impl StandardError {
    pub fn new() -> Self {
        StandardError
    }
}

impl Default for StandardError {
    fn default() -> Self {
        Self::new()
    }
}

impl Write for StandardError {
    fn write_str(&mut self, s: &str) -> fmt::Result {
        if syscalls::print_error(s.as_ptr(), s.len() as u64).is_ok() {
            Ok(())
        } else {
            Err(fmt::Error::default())
        }
    }
}

/// _eprint writes text to the standard
/// error output.
///
#[doc(hidden)]
pub fn _eprint(args: ::core::fmt::Arguments) {
    StandardError::new()
        .write_fmt(args)
        .expect("Printing to standard error failed");
}

/// Print to the standard error output.
///
#[macro_export]
macro_rules! eprint {
    ($($arg:tt)*) => ($crate::_eprint(format_args!($($arg)*)));
}

/// Print to the standard error output.
///
#[macro_export]
macro_rules! eprintln {
    () => ($crate::eprint!("\n"));
    ($($arg:tt)*) => ($crate::eprint!("{}\n", format_args!($($arg)*)));
}
