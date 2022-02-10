// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Handles the kernel's internal monotonic ticker.
//!
//! The ticker functionality is captured in the static [`TICKER`],
//! which is accessed using [`tick`] and [`ticks`] to track the
//! passage of time.

use core::sync::atomic::{AtomicU64, Ordering};
use x86_64::instructions::port::Port;

// The system ticker, which is a monotonic counter.
//
static TICKER: AtomicU64 = AtomicU64::new(0);

/// Increments the system ticker.
///
pub fn tick() {
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
        Port::new(0x43).write(0x34_u8);
        Port::new(0x40).write((divisor & 0xff) as u8);
        Port::new(0x40).write((divisor >> 8) as u8);
    }
}
