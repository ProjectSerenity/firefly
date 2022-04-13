// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements the priority queue of system timers.
//!
//! This is a fairly simple design to get us started. Each timer consists of
//! the [`Instant`](time::Instant) at which the timer will fire, and the
//! [`KernelThreadId`](crate::thread::KernelThreadId) of the thread we should resume.

use crate::thread::KernelThreadId;
use alloc::collections::binary_heap::BinaryHeap;
use core::cmp::{Ordering, PartialEq, PartialOrd};
use lazy_static::lazy_static;
use spin::{lock, Mutex};
use time::{now, Instant};
use x86_64::instructions::interrupts::without_interrupts;

lazy_static! {
    /// The priority queue of pending timers.
    ///
    static ref TIMERS: Mutex<BinaryHeap<Timer>> = Mutex::new(BinaryHeap::new());
}

/// Creates a new timer and adds it to the priority
/// queue of timers.
///
/// The given thread will be resumed once wakeup is
/// no longer in the future.
///
pub fn add(thread_id: KernelThreadId, wakeup: Instant) -> Timer {
    let timer = Timer::new(thread_id, wakeup);
    without_interrupts(|| lock!(TIMERS).push(timer));

    timer
}

/// Cancel all timers for the given thread.
///
/// Returns whether any timers were cancelled without
/// having fired.
///
pub fn cancel_all_for_thread(thread_id: KernelThreadId) -> bool {
    let mut timers = lock!(TIMERS);
    let len1 = timers.len();
    timers.retain(|x| x.thread_id != thread_id);
    let len2 = timers.len();

    len1 != len2
}

/// Processes the set of system timers, waking threads
/// as necessary.
///
/// `process` iterates through the priority queue of
/// timers, removing any timers that have expired and
/// marking the corresponding threads as runnable.
///
#[allow(clippy::missing_panics_doc)] // Will only panic if the timer state is inconsistent.
pub fn process() {
    let now = now();
    let mut timers = lock!(TIMERS);
    loop {
        if let Some(next) = timers.peek() {
            if next.wakeup > now {
                // Nothing more ready.
                return;
            }

            let next = timers.pop().unwrap();
            next.thread_id.resume();
        } else {
            // Nothing left to do.
            return;
        }
    }
}

/// Represents a system time when a thread
/// should be woken.
///
#[derive(Clone, Copy, Eq)]
pub struct Timer {
    wakeup: Instant,
    thread_id: KernelThreadId,
}

impl Timer {
    /// new creates a timer that will wake the
    /// given thread at or after the given time.
    ///
    fn new(thread_id: KernelThreadId, wakeup: Instant) -> Self {
        Timer { wakeup, thread_id }
    }

    /// Cancels the timer, ensuring that it will
    /// not fire from this point onward.
    ///
    /// Returns whether the timer has expired,
    /// and therefore may have fired already.
    ///
    pub fn cancel(self) -> bool {
        let expired = self.wakeup <= now();
        let mut timers = lock!(TIMERS);
        timers.retain(|x| *x != self);

        expired
    }
}

impl PartialEq for Timer {
    fn eq(&self, other: &Timer) -> bool {
        self.wakeup == other.wakeup
    }
}

// Describe how timers are ordered, which
// is the reverse of what you'd expect.
// That is, a timer with a smaller ticks
// has a higher priority and therefore
// compares as 'larger'.
//
impl PartialOrd for Timer {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl Ord for Timer {
    fn cmp(&self, other: &Self) -> Ordering {
        match self.wakeup.cmp(&other.wakeup) {
            Ordering::Equal => Ordering::Equal,
            Ordering::Less => Ordering::Greater,
            _ => Ordering::Less,
        }
    }
}
