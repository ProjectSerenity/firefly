//! ticker handles the kernel's internal monotonic ticker.

// The ticker functionality is captured in the Ticker type,
// with a static TICKER instance used with interrupts to
// track the passage of time.

use crate::interrupts::{register_irq, Irq};
use crate::multitasking::thread::scheduler;
use crate::multitasking::{cpu_local, thread};
use crate::time;
use core::mem;
use core::sync::atomic::{AtomicU64, Ordering};
use x86_64::instructions::port::Port;
use x86_64::structures::idt::InterruptStackFrame;

// Lazily initialise TICKER as a Ticker, protected
// by a spin lock.
//
static TICKER: AtomicU64 = AtomicU64::new(0);

/// tick increments the internal chronometer.
///
fn tick() {
    TICKER.fetch_add(1, Ordering::Relaxed);
}

/// ticks returns the number of ticks of the
/// internal chronometer.
///
pub fn ticks() -> u64 {
    TICKER.load(Ordering::Relaxed)
}

/// init starts the programmable interval timer,
/// setting its frequency to TICKS_PER_SECOND Hz.
///
pub fn init() {
    set_ticker_frequency(TICKS_PER_SECOND);
    register_irq(Irq::new(0).expect("invalid IRQ"), timer_interrupt_handler);
}

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
        current_thread.add_time(thread::DEFAULT_TIME_SLICE);
    }

    // Drop our reference to the current thread,
    // so the scheduler has full control.
    mem::drop(thread_id);
    mem::drop(current_thread);

    scheduler::switch();
}

pub const TICKS_PER_SECOND: u64 = 1000;
const NANOSECONDS_PER_SECOND: u64 = 1_000_000_000;
pub const NANOSECONDS_PER_TICK: u64 = NANOSECONDS_PER_SECOND / TICKS_PER_SECOND;

const MIN_FREQUENCY: u64 = 18; // See https://wiki.osdev.org/Programmable_Interval_Timer
const MAX_FREQUENCY: u64 = 1193181;

/// set_ticker_frequency initialises the hardware
/// timer, setting its frequency to `freq` Hz.
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
