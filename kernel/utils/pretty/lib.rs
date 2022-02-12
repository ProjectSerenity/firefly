// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides pretty printing for various data types.
//!
//! This crate provides helper types for pretty printing units, such
//! as a number of bytes.

#![no_std]

extern crate alloc;

mod bytes;

pub use bytes::Bytes;
