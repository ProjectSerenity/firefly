// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides pretty printing for various data types.
//!
//! This crate provides helper types for pretty printing units, such
//! as a number of bytes.

#![no_std]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]

extern crate alloc;

mod bytes;

pub use bytes::Bytes;
