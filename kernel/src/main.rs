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
use kernel::{memory, pci, println, time, CPU_ID};

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
    kernel::init(boot_info);

    #[cfg(test)]
    test_main();

    #[cfg(not(test))]
    kmain();

    kernel::halt_loop();
}

#[cfg(not(test))]
fn kmain() {
    println!("Kernel ready!");
    println!("Kernel booted at {}.", time::boot_time());
    if let Some(branding) = CPU_ID.get_processor_brand_string() {
        println!("Kernel running on {} CPU.", branding.as_str());
    } else if let Some(version) = CPU_ID.get_vendor_info() {
        println!("Kernel running on {} CPU.", version.as_str());
    } else {
        println!("Kernel running on unknown CPU.");
    }

    unsafe { memory::vmm::debug(memory::kernel_pml4().level_4_table()) };
    memory::pmm::debug();
    pci::init();
    pci::debug();

    kernel::shutdown_qemu();
}

// Testing framework.

/// This function is called on panic
/// when running tests.
#[cfg(test)]
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    kernel::test_panic_handler(info)
}
