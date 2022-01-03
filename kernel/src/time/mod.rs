//! Handles the hardware timer for regular ticks and the clock in the [Real-time clock](https://en.wikipedia.org/wiki/Real-time_clock) (RTC).
//!
//! This module focuses on time-related functionality. In particular, it
//! currently implements getting the current wall clock time from the RTC,
//! along with getting ticks from the PIT. It also interacts with the
//! [scheduler](crate::multitasking::thread::scheduler) to perform thread
//! preemption and provide timers to allow threads to sleep.
//!
//! The clock is used to [record](boot_time) the wall clock time when the kernel booted.
//!
//! The [`Duration`] and [`Instant`] types can be used to measure and compare
//! points in time.
//!
//!

use crate::multitasking::cpu_local;
use crate::multitasking::thread::{scheduler, ThreadState};
use core::mem;

mod cmos;
mod slice;
mod ticker;
pub mod timers;

pub use crate::time::cmos::boot_time;
pub use crate::time::slice::TimeSlice;
pub use crate::time::ticker::{ticks, NANOSECONDS_PER_TICK, TICKS_PER_SECOND};
pub use core::time::Duration;

/// Initialise the time functionality.
///
/// `init` sets the [Programmable Interval Timer](https://en.wikipedia.org/wiki/Programmable_interval_timer)'s
/// timer frequency and records the current clock time from
/// the RTC as the kernel's [boot time](boot_time).
///
pub fn init() {
    ticker::init();
    timers::init();
    cmos::init();
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

/// Sleep the current thread for the given `duration`.
///
pub fn sleep(duration: Duration) {
    let stop = after(duration);
    let current = cpu_local::current_thread();

    // Create a timer to wake us up.
    timers::add(current.thread_id(), stop);

    // Put ourselves to sleep.
    current.set_state(ThreadState::Sleeping);
    mem::drop(current);

    // Switch to the next thread.
    scheduler::switch();
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

#[test_case]
fn instant() {
    let a = Instant::new(4 * ticker::TICKS_PER_SECOND);
    let b = Instant::new(6 * ticker::TICKS_PER_SECOND);
    assert_eq!(a < b, true);
    assert_eq!(b.duration_since(a), Duration::from_secs(2));
}
