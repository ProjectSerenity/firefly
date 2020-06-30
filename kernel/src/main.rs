#![no_std]
#![no_main]

use core::panic::PanicInfo;

mod vga_buffer;

// This function is called on panic.
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    println!("{}", info);
    loop {}
}

#[no_mangle]
pub extern "C" fn _start() -> ! {
    kmain();
    loop {}
}

fn kmain() {
    println!("Hello, {}!", "kernel");
}
