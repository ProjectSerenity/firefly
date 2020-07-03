#![no_std]
#![no_main]
#![feature(abi_x86_interrupt)]
#![feature(custom_test_frameworks)]
#![test_runner(kernel::test_runner)]
#![reexport_test_harness_main = "test_main"]

use bootloader::{entry_point, BootInfo};
use core::panic::PanicInfo;
use kernel::println;

// This function is called on panic.
#[cfg(not(test))]
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    println!("{}", info);

    kernel::halt_loop();
}

entry_point!(kernel_main);

fn kernel_main(boot_info: &'static BootInfo) -> ! {
    kernel::init();

    #[cfg(test)]
    test_main();

    #[cfg(not(test))]
    kmain();

    kernel::halt_loop();
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
