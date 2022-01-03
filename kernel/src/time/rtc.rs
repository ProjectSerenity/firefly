// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements the functionality to read the [Real-time clock](https://en.wikipedia.org/wiki/Real-time_clock) (RTC).
//!
//! The clock functionality is captured in the Time type,
//! which can be produced by reading the current time from
//! the CMOS/RTC. A static BOOT_TIME instance is initialsed
//! with the time when ::init is called, which we treat as
//! the time when the kernel booted.

use core::fmt;
use spin::Mutex;
use x86_64::instructions::port::Port;

// Store the wall clock value for the boot time, protected
// by a spin lock.
//
static BOOT_TIME: Mutex<Time> = Mutex::new(Time::new());

/// Returns the clock time when the kernel booted.
///
pub fn boot_time() -> Time {
    *BOOT_TIME.lock()
}

/// Sets the boot time by reading the RTC.
///
pub(super) fn init() {
    let time = read_cmos();
    let mut boot = BOOT_TIME.lock();
    boot.year = time.year;
    boot.month = time.month;
    boot.day = time.day;
    boot.hour = time.hour;
    boot.minute = time.minute;
    boot.second = time.second;
}

/// Stores a low-precision wall clock time.
///
#[derive(Clone, Copy)]
pub struct Time {
    year: u16,
    month: u8,
    day: u8,
    hour: u8,
    minute: u8,
    second: u8,
}

impl Time {
    /// Creates a time representing the zero time.
    ///
    pub const fn new() -> Self {
        Time {
            year: 0,
            month: 0,
            day: 0,
            hour: 0,
            minute: 0,
            second: 0,
        }
    }
}

impl fmt::Display for Time {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(
            f,
            "{:02}:{:02}:{:02} {:02}/{:02}/{:04}",
            self.hour, self.minute, self.second, self.day, self.month, self.year
        )
    }
}

const CMOS_ADDRESS: u16 = 0x70;
const CMOS_DATA: u16 = 0x71;
const CMOS_REGISTERS: usize = 16;

const CMOS_SECOND: usize = 0; // Range: 0-59.
const CMOS_MINUTE: usize = 2; // Range: 0-59.
const CMOS_HOUR: usize = 4; // Range: 0-23 or 1-12, with top bit set for PM.
const CMOS_DAY: usize = 7; // Range: 1-31.
const CMOS_MONTH: usize = 8; // Range: 1-12.
const CMOS_YEAR: usize = 9; // Range: 0-99.
const CMOS_REGISTER_B: usize = 0xb;

/// Returns whether the CMOS is currently updating
/// and therefore should be left alone.
///
fn cmos_updating() -> bool {
    unsafe {
        Port::new(CMOS_ADDRESS).write(0x0a as u8);
        Port::<u8>::new(CMOS_DATA).read() & 0x80 != 0
    }
}

/// Populates values with the RTC's current register
/// values.
///
fn read_cmos_values(values: &mut [u8; CMOS_REGISTERS]) {
    for i in 0..CMOS_REGISTERS {
        unsafe {
            Port::new(CMOS_ADDRESS).write(i as u8);
            values[i] = Port::new(CMOS_DATA).read();
        }
    }
}

/// Translates val from the semi-textual BCD format
/// into binary values, as described in [here](https://wiki.osdev.org/CMOS#Format_of_Bytes).
///
#[inline]
fn from_bcd(val: u8) -> u8 {
    ((val & 0xf0) >> 1) + ((val & 0xF0) >> 3) + (val & 0xf)
}

/// Returns the current time.
///
fn read_cmos() -> Time {
    let mut values = [0u8; CMOS_REGISTERS];
    let mut prev_values: [u8; CMOS_REGISTERS];

    // Wait for the CMOS to be stable.
    while cmos_updating() {}

    // Read CMOS values until we get the
    // same values twice in a row, meaning
    // they must be consistent.
    read_cmos_values(&mut values);
    loop {
        prev_values = values.clone();
        while cmos_updating() {}
        read_cmos_values(&mut values);

        // If all the values match, we're done.
        if prev_values[CMOS_SECOND] == values[CMOS_SECOND]
            && prev_values[CMOS_MINUTE] == values[CMOS_MINUTE]
            && prev_values[CMOS_HOUR] == values[CMOS_HOUR]
            && prev_values[CMOS_DAY] == values[CMOS_DAY]
            && prev_values[CMOS_MONTH] == values[CMOS_MONTH]
            && prev_values[CMOS_YEAR] == values[CMOS_YEAR]
            && prev_values[CMOS_REGISTER_B] == values[CMOS_REGISTER_B]
        {
            break;
        }
    }

    // Convert values to binary if necessary.
    if values[CMOS_REGISTER_B] & 4 == 0 {
        values[CMOS_SECOND] = from_bcd(values[CMOS_SECOND]);
        values[CMOS_MINUTE] = from_bcd(values[CMOS_MINUTE]);
        values[CMOS_HOUR] = from_bcd(values[CMOS_HOUR]);
        values[CMOS_DAY] = from_bcd(values[CMOS_DAY]);
        values[CMOS_MONTH] = from_bcd(values[CMOS_MONTH]);
        values[CMOS_YEAR] = from_bcd(values[CMOS_YEAR]);
    }

    // Convert 12 hour clock to 24 hour clock if necessary.
    if values[CMOS_REGISTER_B] & 2 == 0 && values[CMOS_HOUR] & 0x80 != 0 {
        values[CMOS_HOUR] = ((values[CMOS_HOUR] & 0x7f) + 12) % 24;
    }

    // TODO: sort out the year more properly.
    let year = 2000 + values[CMOS_YEAR] as u16;

    Time {
        year: year,
        month: values[CMOS_MONTH],
        day: values[CMOS_DAY],
        hour: values[CMOS_HOUR],
        minute: values[CMOS_MINUTE],
        second: values[CMOS_SECOND],
    }
}
