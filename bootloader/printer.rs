// Forked from bootloader 0.9.22, copyright 2018 Philipp Oppermann.
//
// Use of the original source code is governed by the MIT
// license that can be found in the LICENSE.orig file.
//
// Subsequent work copyright 2022 The Firefly Authors.
//
// Use of new and modified source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

use core::fmt::{Result, Write};
use core::sync::atomic::{AtomicUsize, Ordering};

const VGA_BUFFER: *mut u8 = 0xb8000 as *mut _;
const SCREEN_SIZE: usize = 80 * 25;

// must not be 0 so that we don't have a .bss section
pub static CURRENT_OFFSET: AtomicUsize = AtomicUsize::new(160);

pub struct Printer;

impl Printer {
    pub fn clear_screen(&mut self) {
        for i in 0..SCREEN_SIZE {
            unsafe {
                VGA_BUFFER.add(i).write_volatile(0);
            }
        }

        CURRENT_OFFSET.store(0, Ordering::Relaxed);
    }
}

impl Write for Printer {
    fn write_str(&mut self, s: &str) -> Result {
        for byte in s.bytes() {
            let index = CURRENT_OFFSET.fetch_add(2, Ordering::Relaxed) as isize;

            unsafe {
                VGA_BUFFER.offset(index).write_volatile(byte);
                VGA_BUFFER.offset(index + 1).write_volatile(0x4f);
            }
        }

        Ok(())
    }
}
