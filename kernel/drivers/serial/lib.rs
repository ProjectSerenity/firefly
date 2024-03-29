// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides access to serial ports and implements the `print` and `println` macros.
//!
//! This module provides functionality to write text to a serial port device. Each
//! of the four devices is provided ([`COM1`], [`COM2`], [`COM3`], and [`COM4`]),
//! protected with a spin lock.
//!
//! This module also implements the [`print`] and [`println`] macros, both of which
//! write their output to [`COM1`].
//!
//! # Examples
//!
//! ```
//! println!("This is written to serial port COM{}!", 1);
//! ```
//!
//! # Safety
//!
//! The [`print`] and [`println`] macros both disable interrupts while running, to
//! prevent deadlocks when locking [`COM1`]. Direct access to the individual serial
//! ports without disabling interrupts could lead to deadlocks.

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
#![deny(macro_use_extern_crate)]
#![deny(missing_abi)]
#![allow(unsafe_code)]
#![deny(unused_crate_dependencies)]

use core::fmt::Write;
use spin::{lock, Mutex};
use uart_16550::SerialPort;
use x86_64::instructions::interrupts::without_interrupts;

/// COM1 is the first serial port device.
///
pub static COM1: Mutex<SerialPort> = unsafe { Mutex::new(SerialPort::new(0x3f8)) };

/// COM2 is the second serial port device.
///
pub static COM2: Mutex<SerialPort> = unsafe { Mutex::new(SerialPort::new(0x2f8)) };

/// COM3 is the third serial port device.
///
pub static COM3: Mutex<SerialPort> = unsafe { Mutex::new(SerialPort::new(0x3e8)) };

/// COM4 is the fourth serial port device.
///
pub static COM4: Mutex<SerialPort> = unsafe { Mutex::new(SerialPort::new(0x2e8)) };

/// Write a string to the first serial port,
/// COM1.
///
pub fn write_str(s: &str) -> core::fmt::Result {
    without_interrupts(|| lock!(COM1).write_str(s))
}

/// _print writes text to the serial port by
/// acquiring COM1 using a spin lock.
///
#[doc(hidden)]
pub fn _print(args: ::core::fmt::Arguments) {
    without_interrupts(|| {
        lock!(COM1)
            .write_fmt(args)
            .expect("Printing to COM1 failed");
    });
}

/// Print to the first serial port, COM1.
///
#[macro_export]
macro_rules! print {
    ($($arg:tt)*) => ($crate::_print(format_args!($($arg)*)));
}

/// Print to the first serial port, COM1.
///
#[macro_export]
macro_rules! println {
    () => ($crate::print!("\n"));
    ($($arg:tt)*) => ($crate::print!("{}\n", format_args!($($arg)*)));
}
