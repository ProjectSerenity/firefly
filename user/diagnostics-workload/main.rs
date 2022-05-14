// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// This is a simple user application.

#![no_std]
#![no_main]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]

use core::panic::PanicInfo;
use firefly::{eprintln, exit_thread};
use diagnostics_workload::main;

/// This function is called on panic.
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    eprintln!("{}", info);
    exit_thread();
}

/// The application's entry point.
#[no_mangle]
pub extern "sysv64" fn _start() -> ! {
    main();
    exit_thread();
}
