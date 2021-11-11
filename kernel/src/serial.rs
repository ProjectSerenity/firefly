// This module handles interactions with serial ports.
// In particular, this is used for early kernel logs,
// which are written to the serial port.

use core::fmt::Write;
use lazy_static::lazy_static;
use spin::Mutex;
use uart_16550::SerialPort;
use x86_64::instructions::interrupts;

// SERIAL1 is used to read or write data to
// the first serial port, sometimes referred
// to as COM1.
//
lazy_static! {
    static ref SERIAL1: Mutex<SerialPort> = {
        let mut serial_port = unsafe { SerialPort::new(0x3F8) };
        serial_port.init();
        Mutex::new(serial_port)
    };
}

/// _print writes text to the serial port by
/// acquiring SERIAL1 using a spin lock.
///
#[doc(hidden)]
pub fn _print(args: ::core::fmt::Arguments) {
    interrupts::without_interrupts(|| {
        SERIAL1
            .lock()
            .write_fmt(args)
            .expect("Printing to serial failed");
    });
}

/// print! is the standard printing macro, implemented
/// using the _print function, which acquires SERIAL1
/// using a spin lock and writes the message to the
/// serial port.
///
#[macro_export]
macro_rules! print {
    ($($arg:tt)*) => ($crate::serial::_print(format_args!($($arg)*)));
}

/// println! is the standard printing macro, implemented
/// using the _print function, which acquires WRITER
/// using a spin lock and writes the message to the
/// serial port.
///
#[macro_export]
macro_rules! println {
    () => ($crate::print!("\n"));
    ($($arg:tt)*) => ($crate::print!("{}\n", format_args!($($arg)*)));
}
