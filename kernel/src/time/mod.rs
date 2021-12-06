//! time handles the hardware timer for regular ticks and
//! the clock in the CMOS/RTC.

// This module focuses on time-related functionality. In
// particular, it currently implements getting the current
// wall clock time from the CMOS/RTC, along with getting
// ticks from the PIT.
//
// Currently, the ticker is used to print the uptime to
// the serial port every second (once the kernel has
// booted) and the clock is used to record and later print
// the wall clock time when the kernel booted.

mod cmos;
mod ticker;

pub use crate::time::cmos::boot_time;
pub use crate::time::ticker::{ticks, NANOSECONDS_PER_TICK, TICKS_PER_SECOND};
pub use core::time::Duration;

/// init sets up the time functionality, setting the
/// ticker frequency we expect from the PIT and recording
/// the current time from the CMOS/RTC as the kernel's
/// boot time.
///
pub fn init() {
    ticker::init();
    cmos::init();
}

/// now returns an Instant representing the current
/// time.
///
pub fn now() -> Instant {
    Instant::new(ticks())
}

/// since returns a Duration describing the amount
/// of time that has passed since the given Instant.
///
pub fn since(earlier: Instant) -> Duration {
    now().duration_since(earlier)
}

/// after returns an Instant in the future that will
/// occur after the given Duration.
///
pub fn after(wait: Duration) -> Instant {
    let now = ticks();
    let delta = wait.as_nanos() / (ticker::NANOSECONDS_PER_TICK as u128);
    Instant::new(now + delta as u64)
}

/// Instant represents a single point in the kernel's
/// monotonically nondecreasing clock. In Instant is
/// made useful by comparing with another Instant to
/// produce a Duration.
///
#[derive(Copy, Clone, PartialEq, PartialOrd)]
pub struct Instant(u64);

/// BOOT_TIME is the Instant that represents the time
/// when the kernel booted.
///
pub const BOOT_TIME: Instant = Instant(0u64);

impl Instant {
    /// new returns an Instant representing the given
    /// number of ticks.
    ///
    fn new(ticks: u64) -> Self {
        Instant(ticks)
    }

    /// duration_since returns a Duration describing the
    /// amount of time that passed between the two
    /// Instants.
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
