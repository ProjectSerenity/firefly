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
//
// The ticker functionality is captured in the Ticker type,
// with a static TICKER instance used with interrupts to
// track the passage of time.
//
// The cloc functionality is captured in the Time type,
// which can be produced by reading the current time from
// the CMOS/RTC. A static BOOT_TIME instance is initialsed
// with the time when ::init is called, which we treat as
// the time when the kernel booted.

mod cmos;
mod ticker;

pub use crate::time::cmos::boot_time;
pub use crate::time::ticker::{tick, ticks};

/// init sets up the time functionality, setting the
/// ticker frequency we expect from the PIT and recording
/// the current time from the CMOS/RTC as the kernel's
/// boot time.
///
pub fn init() {
    ticker::init();
    cmos::init();
}
