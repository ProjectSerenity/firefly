// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Handles the hardware timer for regular ticks and the clock in the [Real-time clock](https://en.wikipedia.org/wiki/Real-time_clock) (RTC).
//!
//! This crate focuses on time-related functionality. In particular, it
//! currently implements getting the current wall clock time from the RTC,
//! along with getting ticks from the PIT.
//!
//! The clock is used to [record](boot_time) the wall clock time when the kernel booted.
//!
//! The [`Duration`] and [`Instant`] types can be used to measure and compare
//! points in time.

#![no_std]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]

mod rtc;
mod slice;
mod ticker;

pub use core::time::Duration;
pub use rtc::boot_time;
pub use slice::TimeSlice;
pub use ticker::{tick, ticks, NANOSECONDS_PER_TICK, TICKS_PER_SECOND};

/// Initialise the time functionality.
///
/// `init` sets the [Programmable Interval Timer](https://en.wikipedia.org/wiki/Programmable_interval_timer)'s
/// timer frequency and records the current clock time from
/// the RTC as the kernel's [boot time](boot_time).
///
pub fn init() {
    ticker::init();
    rtc::init();
}

/// Returns an Instant representing the current time.
///
pub fn now() -> Instant {
    Instant::new(ticks())
}

/// Returns a Duration describing the amount of time
/// that has passed since the given `Instant`.
///
pub fn since(earlier: Instant) -> Duration {
    now().duration_since(earlier)
}

/// Returns an Instant in the future that will occur
/// after the given `Duration`.
///
pub fn after(wait: Duration) -> Instant {
    let now = ticks();
    let delta = wait.as_nanos() / (ticker::NANOSECONDS_PER_TICK as u128);
    Instant::new(now + delta as u64)
}

/// Represents a single point in the kernel's monotonically
/// nondecreasing clock.
///
/// An `Instant` is made useful by comparing it with another
/// `Instant` to produce a `Duration`.
///
#[derive(Copy, Clone, Debug, Eq, Ord, PartialEq, PartialOrd)]
pub struct Instant(u64);

/// The `Instant` that represents the time when the kernel
/// booted.
///
pub const BOOT_TIME: Instant = Instant(0u64);

impl Instant {
    /// Returns an `Instant` representing the given number
    /// of system ticks.
    ///
    fn new(ticks: u64) -> Self {
        Instant(ticks)
    }

    /// Returns the number of microseconds since the system
    /// booted.
    ///
    pub const fn system_micros(&self) -> u64 {
        (self.0 * ticker::NANOSECONDS_PER_TICK) / 1000
    }

    /// Returns whether this instant occurred after the
    /// other.
    ///
    pub fn after(self, other: Self) -> bool {
        self.0 > other.0
    }

    /// Returns whether this instant occurred before the
    /// other.
    ///
    pub fn before(self, other: Self) -> bool {
        self.0 < other.0
    }

    /// Returns a `Duration` describing the amount of time
    /// that passed between the two `Instant`s.
    ///
    /// # Panics
    ///
    /// This function will panic if `earlier` is later than `self`.
    ///
    pub fn duration_since(&self, earlier: Instant) -> Duration {
        if self.0 < earlier.0 {
            panic!("duration_since called with later instant");
        }

        if self.0 == earlier.0 {
            Duration::ZERO
        } else {
            let ticks = self.0 - earlier.0;
            let secs = ticks / ticker::TICKS_PER_SECOND;
            let rem = ticks % ticker::TICKS_PER_SECOND;
            let nanos = rem * ticker::NANOSECONDS_PER_TICK;
            Duration::new(secs, nanos as u32)
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn instant() {
        let a = Instant::new(4 * ticker::TICKS_PER_SECOND);
        let b = Instant::new(6 * ticker::TICKS_PER_SECOND);
        assert_eq!(a < b, true);
        assert_eq!(b.duration_since(a), Duration::from_secs(2));
    }
}
