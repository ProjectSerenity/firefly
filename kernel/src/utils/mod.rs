// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements various utilities and data structures used elsewhere in the kernel.
//!
//! ## Lazy initialisation
//!
//! The [`Lazy`](lazy::Lazy) type can be used to create an uninitialised value,
//! which is later initialised before use.
//!
//! ## Single initialisation
//!
//! The [`Once`](once::Once) type can be used to create an uninitialised value,
//! which is later initialised exactly once.
//!
//! ## Pretty printing
//!
//! The [`pretty`] module provides helper types for pretty printing units, such
//! as a number of bytes.
//!
//! ## Reading TAR archives
//!
//! The [`tar`] module provides functionality to read [TAR](https://en.wikipedia.org/wiki/Tar_(computing))
//! archives from a block device.

pub mod lazy;
pub mod once;
pub mod pretty;
pub mod tar;
