//! slice includes the functionality to create and manage the time slices used
//! to determine a thread's time left on the CPU.

use crate::time::{Duration, NANOSECONDS_PER_TICK};
use core::ops::{Add, AddAssign, Sub, SubAssign};

/// TimeSlice includes the number of timer ticks a thread has left on the CPU
/// before it is next rescheduled.
///
#[derive(Clone, Copy, Debug)]
pub struct TimeSlice(u64);

impl TimeSlice {
    /// ZERO is the empty time slice.
    ///
    pub const ZERO: TimeSlice = TimeSlice(0);

    /// from_duration returns the smallest time slice no less than duration.
    ///
    /// Note that the returned TimeSlice may last slightly longer than the
    /// given duration, if limited by the frequency of the programmable
    /// interrupt timer.
    ///
    pub const fn from_duration(duration: &Duration) -> Self {
        // Round up if necessary.
        let nanos = duration.as_nanos() + (NANOSECONDS_PER_TICK - 1) as u128;
        let delta = nanos / (NANOSECONDS_PER_TICK as u128);

        TimeSlice(delta as u64)
    }

    /// tick decrements the time slice by a single tick, returning true if
    /// the time slice is now zero.
    ///
    pub fn tick(&mut self) -> bool {
        self.0 = self.0.saturating_sub(1);
        self.0 == 0u64
    }

    /// is_zero returns true if the time slice is now zero.
    ///
    pub const fn is_zero(&self) -> bool {
        self.0 == 0u64
    }
}

impl AddAssign<TimeSlice> for TimeSlice {
    #[inline]
    fn add_assign(&mut self, rhs: TimeSlice) {
        *self = *self + rhs;
    }
}

impl Add<TimeSlice> for TimeSlice {
    type Output = Self;

    #[inline]
    fn add(self, rhs: TimeSlice) -> Self::Output {
        TimeSlice(self.0 + rhs.0)
    }
}

impl SubAssign<TimeSlice> for TimeSlice {
    #[inline]
    fn sub_assign(&mut self, rhs: TimeSlice) {
        *self = *self - rhs;
    }
}

impl Sub<TimeSlice> for TimeSlice {
    type Output = Self;

    #[inline]
    fn sub(self, rhs: TimeSlice) -> Self::Output {
        TimeSlice(self.0.saturating_sub(rhs.0))
    }
}

#[test_case]
fn time_slice() {
    // We want to have 3 ticks, no matter what the
    // current tick frequency is.
    let nanos = NANOSECONDS_PER_TICK * 3;
    let duration = Duration::from_nanos(nanos);
    let mut slice = TimeSlice::from_duration(&duration);
    assert_eq!(slice.tick(), false);
    assert_eq!(slice.tick(), false);
    assert_eq!(slice.tick(), true);
    assert_eq!(slice.tick(), true);
}