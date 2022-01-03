// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Handles the kernel's internal monotonic ticker.
//!
//! The ticker functionality is captured in the static [`TICKER`],
//! which is accessed using [`tick`] and [`ticks`] to track the
//! passage of time.

use crate::interrupts::{register_irq, Irq};
use crate::multitasking::thread::scheduler;
use crate::multitasking::{cpu_local, thread};
use crate::time;
use core::mem;
use core::sync::atomic::{AtomicU64, Ordering};
use x86_64::instructions::port::Port;
use x86_64::structures::idt::InterruptStackFrame;

// The system ticker, which is a monotonic counter.
//
static TICKER: AtomicU64 = AtomicU64::new(0);

/// Increments the system ticker.
///
fn tick() {
    TICKER.fetch_add(1, Ordering::Relaxed);
}

/// Returns the number of times the system ticker
/// has been incremented.
///
pub fn ticks() -> u64 {
    TICKER.load(Ordering::Relaxed)
}

/// Starts the [Programmable Interval Timer](https://en.wikipedia.org/wiki/Programmable_interval_timer),
/// setting its frequency to [`TICKS_PER_SECOND`] Hz.
///
pub(super) fn init() {
    set_ticker_frequency(TICKS_PER_SECOND);
    register_irq(Irq::new(0).expect("invalid IRQ"), timer_interrupt_handler);
}

/// The PIT's interrupt handler.
///
fn timer_interrupt_handler(_stack_frame: InterruptStackFrame, irq: Irq) {
    tick();

    irq.acknowledge();

    if !cpu_local::ready() || !scheduler::ready() {
        return;
    }

    // Check whether any timers have expired.
    time::timers::process();

    // Time to pre-empt the current thread.
    let current_thread = cpu_local::current_thread();
    if !current_thread.tick() {
        return;
    }

    let thread_id = current_thread.thread_id();
    if thread_id != thread::ThreadId::IDLE {
        current_thread.reset_time_slice();
    }

    // Drop our reference to the current thread,
    // so the scheduler has full control.
    mem::drop(thread_id);
    mem::drop(current_thread);

    scheduler::switch();
}

/// The number of times the system ticker will be
/// incremented per second.
///
pub const TICKS_PER_SECOND: u64 = 1000;

/// The number of nanoseconds in one second.
///
const NANOSECONDS_PER_SECOND: u64 = 1_000_000_000;

/// The number of nanoseconds that will pass between
/// each increment to the system ticker.
///
pub const NANOSECONDS_PER_TICK: u64 = NANOSECONDS_PER_SECOND / TICKS_PER_SECOND;

const MIN_FREQUENCY: u64 = 18; // See https://wiki.osdev.org/Programmable_Interval_Timer
const MAX_FREQUENCY: u64 = 1193181;

/// Initialise the hardware timer, settings its
/// frequency to `freq` Hz.
///
fn set_ticker_frequency(mut freq: u64) {
    if freq < MIN_FREQUENCY {
        freq = MIN_FREQUENCY;
    }

    if freq > MAX_FREQUENCY {
        freq = MAX_FREQUENCY;
    }

    let divisor = MAX_FREQUENCY / freq;

    // See http://kernelx.weebly.com/programmable-interval-timer.html
    unsafe {
        Port::new(0x43).write(0x34 as u8);
        Port::new(0x40).write((divisor & 0xff) as u8);
        Port::new(0x40).write((divisor >> 8) as u8);
    }
}
