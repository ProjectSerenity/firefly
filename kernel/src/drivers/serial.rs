//! serial implements the `print` and `println` macros,
//! writing their output to the first serial port, COM1.

// This module handles interactions with serial ports.
// In particular, this is used for early kernel logs,
// which are written to the serial port.

use core::fmt::Write;
use uart_16550::SerialPort;
use x86_64::instructions::interrupts;

/// COM1 is the first serial port device.
///
pub const COM1: spin::Mutex<SerialPort> = unsafe { spin::Mutex::new(SerialPort::new(0x3f8)) };

/// COM2 is the second serial port device.
///
pub const COM2: spin::Mutex<SerialPort> = unsafe { spin::Mutex::new(SerialPort::new(0x2f8)) };

/// COM3 is the third serial port device.
///
pub const COM3: spin::Mutex<SerialPort> = unsafe { spin::Mutex::new(SerialPort::new(0x3e8)) };

/// COM4 is the fourth serial port device.
///
pub const COM4: spin::Mutex<SerialPort> = unsafe { spin::Mutex::new(SerialPort::new(0x2e8)) };

/// _print writes text to the serial port by
/// acquiring COM1 using a spin lock.
///
#[doc(hidden)]
pub fn _print(args: ::core::fmt::Arguments) {
    interrupts::without_interrupts(|| {
        COM1.lock()
            .write_fmt(args)
            .expect("Printing to COM1 failed");
    });
}

/// print! is the standard printing macro, which writes
/// its output to COM1.
///
#[macro_export]
macro_rules! print {
    ($($arg:tt)*) => ($crate::drivers::serial::_print(format_args!($($arg)*)));
}

/// println! is the standard printing macro, which writes
/// its output to COM1.
///
#[macro_export]
macro_rules! println {
    () => ($crate::print!("\n"));
    ($($arg:tt)*) => ($crate::print!("{}\n", format_args!($($arg)*)));
}
