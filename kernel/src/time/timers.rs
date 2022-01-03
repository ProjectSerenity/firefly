// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements the priority queue of system timers.
//!
//! This is a fairly simple design to get us started. Each timer consists of
//! the [`Instant`](super::Instant) at which the timer will fire, and the [`ThreadId`](crate::multitasking::thread::ThreadId)
//! of the thread we should resume.

use crate::multitasking::thread;
use crate::time;
use crate::utils::lazy::Lazy;
use alloc::collections::binary_heap::BinaryHeap;
use core::cmp::{Ordering, PartialEq, PartialOrd};
use core::mem;
use x86_64::instructions::interrupts;

/// The priority queue of pending timers.
///
static TIMERS: spin::Mutex<Lazy<BinaryHeap<Timer>>> = spin::Mutex::new(Lazy::new());

/// Initialise the system timers.
///
pub(super) fn init() {
    TIMERS.lock().set(BinaryHeap::new());
}

/// Creates a new timer and adds it to the priority
//// queue of timers.
///
/// The given thread will be resumed once wakeup is
/// no longer in the future.
///
pub fn add(thread_id: thread::ThreadId, wakeup: time::Instant) {
    interrupts::without_interrupts(|| TIMERS.lock().push(Timer::new(thread_id, wakeup)));
}

/// Processes the set of system timers, waking threads
/// as necessary.
///
/// `process` iterates through the priority queue of
/// timers, removing any timers that have expired and
/// marking the corresponding threads as runnable.
///
pub fn process() {
    let now = time::now();
    let mut timers = TIMERS.lock();
    loop {
        let next = timers.peek();
        if !next.is_some() {
            // Nothing left to do.
            return;
        }

        let next = next.unwrap();
        if next.wakeup > now {
            // Nothing more ready.
            return;
        }

        mem::drop(next);
        let next = timers.pop().unwrap();
        next.thread_id.resume();
    }
}

/// Represents a system time when a thread
/// should be woken.
///
#[derive(Clone, Copy, Eq, Ord)]
struct Timer {
    wakeup: time::Instant,
    thread_id: thread::ThreadId,
}

impl Timer {
    /// new creates a timer that will wake the
    /// given thread at or after the given time.
    ///
    fn new(thread_id: thread::ThreadId, wakeup: time::Instant) -> Self {
        Timer { wakeup, thread_id }
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
    fn partial_cmp(&self, other: &Timer) -> Option<Ordering> {
        if self.wakeup == other.wakeup {
            Some(Ordering::Equal)
        } else if self.wakeup < other.wakeup {
            Some(Ordering::Greater)
        } else {
            Some(Ordering::Less)
        }
    }
}

#[test_case]
fn timers_ordering() {
    let foo = Timer::new(thread::ThreadId::IDLE, time::Instant::new(2));
    let bar = Timer::new(thread::ThreadId::IDLE, time::Instant::new(3));
    let baz = Timer::new(thread::ThreadId::IDLE, time::Instant::new(3));
    assert_eq!(foo < bar, false);
    assert_eq!(bar == baz, true);
    assert_eq!(bar < foo, true);
}
