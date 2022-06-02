// Forked from bootloader 0.9.22, copyright 2018 Philipp Oppermann.
//
// Use of the original source code is governed by the MIT
// license that can be found in the LICENSE.orig file.
//
// Subsequent work copyright 2022 The Firefly Authors.
//
// Use of new and modified source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

#![no_std]
#![no_main]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]
#![allow(unsafe_code)]

use bootloader as _;
use core::fmt::Write;
use core::panic::PanicInfo;
use uart_16550::SerialPort;

#[panic_handler]
#[no_mangle]
pub fn panic(info: &PanicInfo) -> ! {
    unsafe {
        // Write the panic info to COM1.
        let mut COM1 = SerialPort::new(0x3f8);
        write!(COM1, "{}", info);
    }

    loop {}
}

#[no_mangle]
pub extern "C" fn _Unwind_Resume() {
    loop {}
}
