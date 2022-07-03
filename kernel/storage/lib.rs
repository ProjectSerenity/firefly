// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements the kernel's storage subsystem.

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(deprecated_in_future)]
#![deny(unused_crate_dependencies)]
#![forbid(unsafe_code)]

extern crate alloc;

pub mod block;
