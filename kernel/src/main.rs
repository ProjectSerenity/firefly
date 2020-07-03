#![no_std]
#![no_main]
#![feature(abi_x86_interrupt)]
#![feature(custom_test_frameworks)]
#![test_runner(kernel::test_runner)]
#![reexport_test_harness_main = "test_main"]

use core::panic::PanicInfo;
use kernel::println;

#[cfg(target_arch = "x86_64")]
fn halt() {
    x86_64::instructions::interrupts::disable();
    x86_64::instructions::hlt();
}

// other platforms
#[cfg(not(target_arch = "x86_64"))]
fn halt() {}

// This function is called on panic.
#[cfg(not(test))]
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    println!("{}", info);
    loop {
        halt();
    }
}

#[no_mangle]
pub extern "C" fn _start() -> ! {
    kernel::init();

    #[cfg(test)]
    test_main();

    #[cfg(not(test))]
    kmain();

    loop {
        halt();
    }
}

#[cfg(not(test))]
fn kmain() {
    println!("Kernel ready!");
}

// Testing framework.

// This function is called on panic
// when running tests.
#[cfg(test)]
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    kernel::test_panic_handler(info)
}
