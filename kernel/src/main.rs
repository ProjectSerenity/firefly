#![no_std]
#![no_main]
#![feature(asm)]

use core::panic::PanicInfo;

mod vga_buffer;

#[cfg(target_arch = "x86_64")]
fn halt() {
    unsafe {
        asm!("hlt");
    }
}

// other platforms
#[cfg(not(target_arch = "x86_64"))]
fn halt() {}

// This function is called on panic.
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    println!("{}", info);
    loop {
        halt();
    }
}

#[no_mangle]
pub extern "C" fn _start() -> ! {
    kmain();
    loop {
        halt();
    }
}

fn kmain() {
    println!("Hello, {}!", "kernel");
}
