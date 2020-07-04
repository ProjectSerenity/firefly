use crate::{print, Locked};
use lazy_static::lazy_static;

pub fn init() {
    set_ticker_frequency(TICKS_PER_SECOND);
}

// Lazily initialise TICKER as a Ticker, protected
// by a spin lock.
//
lazy_static! {
    pub static ref TICKER: Locked<Ticker> = Locked::new(Ticker::new());
}

const TICKS_PER_SECOND: usize = 512;

const MIN_FREQUENCY: usize = 18; // See https://wiki.osdev.org/Programmable_Interval_Timer
const MAX_FREQUENCY: usize = 1193181;

pub fn set_ticker_frequency(mut freq: usize) {
    use x86_64::instructions::port::Port;

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

pub struct Ticker {
    counter: usize,
}

impl Ticker {
    // new creates a new ticker, with a zero
    // counter.
    //
    pub const fn new() -> Self {
        Ticker { counter: 0 }
    }
}

impl Locked<Ticker> {
    // tick increments the counter, printing the
    // system uptime if a whole number of seconds
    // have passed since boot..
    //
    pub fn tick(&self) {
        let mut ticker = self.lock();
        ticker.counter += 1;
        if ticker.counter % TICKS_PER_SECOND == 0 {
            print!("\rUptime: {} seconds.", ticker.counter / TICKS_PER_SECOND);
        }
    }
}
