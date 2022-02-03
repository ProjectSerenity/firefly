// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implement cooperative and preemptive multitasking, plus CPU-local data.
//!
//! ## Preemptive multitasking
//!
//! The [`thread`] module implements Firefly threads, each of which has its
//! own stack and execution state. This also includes the scheduler, which
//! can be used to switch from one thread to another, and for a thread to
//! sleep and be resumed. Combined with the Programmable Interval Timer
//! handler, this will pre-empt threads to allow fair sharing of the CPU.
//!
//! ## CPU-local data
//!
//! The [`cpu_local`] module allocates a memory region for each active CPU,
//! storing the address for the region in the GS register. This allows the
//! CPU to have an independent copy of its own data.

pub mod cpu_local;
pub mod thread;
