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

pub mod lazy;
pub mod once;
