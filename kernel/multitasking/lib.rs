// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements preemptive multitasking, using independent threads of execution.
//!
//! ## Shared state
//!
//! The multitasking subsystem contains various shared state, such as the
//! set of threads, the idle thread for each CPU, and the scheduler. This
//! state is stored in the crate root so that it can easily be shared with
//! all modules in the crate.
//!
//! ## Preemptive multitasking
//!
//! The [`thread`] module implements Firefly threads, each of which has its
//! own stack and execution state. This also includes the scheduler, which
//! can be used to switch from one thread to another, and for a thread to
//! sleep and be resumed. Combined with the Programmable Interval Timer
//! handler, this will pre-empt threads to allow fair sharing of the CPU.

#![no_std]
#![feature(binary_heap_retain)]
#![feature(const_btree_new)]

extern crate alloc;

pub mod scheduler;
mod switch;
pub mod thread;

use crate::scheduler::Scheduler;
use crate::thread::{KernelThreadId, Thread};
use alloc::collections::BTreeMap;
use alloc::sync::Arc;
use alloc::vec::Vec;
use lazy_static::lazy_static;
use spin::Mutex;

// State shared throughout the crate.

lazy_static! {
    /// SCHEDULER is the thread scheduler.
    ///
    static ref SCHEDULER: Mutex<Scheduler> = Mutex::new(Scheduler::new());
}

type ThreadTable = BTreeMap<KernelThreadId, Arc<Thread>>;

/// THREADS stores all living threads, referencing them by
/// their thread id. Note that THREADS does not contain
/// the idle thread, as there will be a separate instance
/// for each CPU, but a single shared THREADS structure.
///
static THREADS: Mutex<ThreadTable> = Mutex::new(BTreeMap::new());

lazy_static! {
    /// The currently executing thread for each CPU.
    ///
    static ref CURRENT_THREADS: Mutex<Vec<Arc<Thread>>> = Mutex::new(Vec::with_capacity(cpu::max_cores()));

    /// The idle thread for each CPU.
    ///
    static ref IDLE_THREADS: Mutex<Vec<Arc<Thread>>> = Mutex::new(Vec::with_capacity(cpu::max_cores()));
}
