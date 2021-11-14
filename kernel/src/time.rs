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

use crate::{print, Locked};
use core::fmt;
use lazy_static::lazy_static;
use x86_64::instructions::port::Port;

/// init sets up the time functionality, setting the
/// ticker frequency we expect from the PIT and recording
/// the current time from the CMOS/RTC as the kernel's
/// boot time.
///
pub fn init() {
    set_ticker_frequency(TICKS_PER_SECOND);
    BOOT_TIME.update_from(read_cmos());
}

// Ticker functionality.

/// Ticker contains a counter, which is used
/// to track the passage of time by a regular
/// sequence of ticks.
///
pub struct Ticker {
    counter: usize,
}

// Lazily initialise TICKER as a Ticker, protected
// by a spin lock.
//
lazy_static! {
    static ref TICKER: Locked<Ticker> = Locked::new(Ticker::new());
}

/// tick increments the internal chronometer.
///
pub fn tick() {
    TICKER.tick();
}

impl Ticker {
    /// new creates a new ticker, with a zero
    /// counter.
    ///
    pub const fn new() -> Self {
        Ticker { counter: 0 }
    }
}

impl Locked<Ticker> {
    /// tick increments the counter, printing the
    /// system uptime if a whole number of seconds
    /// have passed since boot.
    ///
    pub fn tick(&self) {
        let mut ticker = self.lock();
        ticker.counter += 1;
        if ticker.counter % TICKS_PER_SECOND == 0 {
            print!("\rUptime: {} seconds.", ticker.counter / TICKS_PER_SECOND);
        }
    }
}

const TICKS_PER_SECOND: usize = 512;

const MIN_FREQUENCY: usize = 18; // See https://wiki.osdev.org/Programmable_Interval_Timer
const MAX_FREQUENCY: usize = 1193181;

pub fn set_ticker_frequency(mut freq: usize) {
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

// Wall clock functionality.

/// Time stores a low-precision wall clock
/// time.
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

// Lazily initialise the boot time, protected
// by a spin lock.
//
lazy_static! {
    static ref BOOT_TIME: Locked<Time> = Locked::new(Time::new());
}

/// boot_time returns the clock time when the
/// kernel booted.
///
pub fn boot_time() -> Time {
    *BOOT_TIME.lock()
}

impl Time {
    /// new creates the zero time.
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

impl Locked<Time> {
    /// update_from sets the time from the
    /// given value.
    ///
    pub fn update_from(&self, value: Time) {
        let mut time = self.lock();
        time.year = value.year;
        time.month = value.month;
        time.day = value.day;
        time.hour = value.hour;
        time.minute = value.minute;
        time.second = value.second;
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

// cmos_updating returns whether the CMOS is
// currently updating and therefore should be
// left alone.
//
fn cmos_updating() -> bool {
    unsafe {
        Port::new(CMOS_ADDRESS).write(0x0a as u8);
        Port::<u8>::new(CMOS_DATA).read() & 0x80 != 0
    }
}

// read_cmos_values populates values with the
// CMOS's current register values.
//
fn read_cmos_values(values: &mut [u8; CMOS_REGISTERS]) {
    for i in 0..CMOS_REGISTERS {
        unsafe {
            Port::new(CMOS_ADDRESS).write(i as u8);
            values[i] = Port::new(CMOS_DATA).read();
        }
    }
}

// from_bcd translates val from the semi-textual
// BCD format into binary values, as described in
// https://wiki.osdev.org/CMOS#Format_of_Bytes.
//
#[inline]
fn from_bcd(val: u8) -> u8 {
    ((val & 0xf0) >> 1) + ((val & 0xF0) >> 3) + (val & 0xf)
}

// read_cmos returns the current time.
//
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
