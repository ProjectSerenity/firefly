// This is the entry point for the kernel, which
// runs the kernel's standard initialisation,
// then either runs tests or starts kmain,
// where the kernel does its real work.

#![no_std]
#![no_main]
#![feature(abi_x86_interrupt)]
#![feature(custom_test_frameworks)]
#![test_runner(kernel::test_runner)]
#![reexport_test_harness_main = "test_main"]

extern crate alloc;

use bootloader::{entry_point, BootInfo};
use core::panic::PanicInfo;
use kernel::{memory, println, time, CPU_ID};

/// This function is called on panic.
#[cfg(not(test))]
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    println!("{}", info);

    kernel::halt_loop();
}

entry_point!(kernel_main);

// kernel_main is the entry point for the kernel,
// so it starts the kernel's initialisation and
// then either starts the tests by calling test_main,
// or starts the kernel's real work by calling kmain.
//
#[allow(unused_variables)]
fn kernel_main(boot_info: &'static BootInfo) -> ! {
    println!("Kernel booting...");
    kernel::init();

    #[cfg(test)]
    test_main();

    #[cfg(not(test))]
    kmain(boot_info);

    kernel::shutdown_qemu();
    kernel::halt_loop();
}

#[cfg(not(test))]
fn kmain(boot_info: &'static BootInfo) {
    // Set up the heap allocator.
    let mut mapper = unsafe { memory::init(boot_info) };

    println!("Kernel ready!");
    println!("Kernel booted at {}.", time::boot_time());
    if let Some(branding) = CPU_ID.get_processor_brand_string() {
        println!("Kernel running on {} CPU.", branding.as_str());
    } else if let Some(version) = CPU_ID.get_vendor_info() {
        println!("Kernel running on {} CPU.", version.as_str());
    } else {
        println!("Kernel running on unknown CPU.");
    }

    unsafe { memory::debug::level_4_table(mapper.level_4_table()) };
}

// Testing framework.

/// This function is called on panic
/// when running tests.
#[cfg(test)]
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    kernel::test_panic_handler(info)
}
