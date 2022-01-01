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
use kernel::drivers::pci;
use kernel::multitasking::thread::{scheduler, Thread};
use kernel::{memory, network, println, random};

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
    use kernel::{time, CPU_ID};

    println!("Kernel ready!");
    println!("Kernel booted at {}.", time::boot_time());
    if let Some(branding) = CPU_ID.get_processor_brand_string() {
        println!("Kernel running on {} CPU.", branding.as_str());
    } else if let Some(version) = CPU_ID.get_vendor_info() {
        println!("Kernel running on {} CPU.", version.as_str());
    } else {
        println!("Kernel running on unknown CPU.");
    }

    // Set up our initial workload for when
    // we get a DHCP configuration.
    network::register_workload(Thread::create_kernel_thread(initial_workload));

    pci::init();
    random::init();

    // Hand over to the scheduler.
    scheduler::start();
}

fn initial_workload() -> ! {
    println!("Starting initial workload.");

    let mut array = [0u8; 16];
    let buf = &mut array[..];
    println!("RNG before: {:02x?}", buf.to_vec());
    random::read(buf);
    println!("RNG after:  {:02x?}", buf.to_vec());

    kernel::shutdown_qemu();
}

#[allow(dead_code)]
fn debug() {
    println!("");

    // Virtual memory.
    println!("Virtual memory manager:");
    unsafe { memory::vmm::debug(memory::kernel_pml4().level_4_table()) };
    println!("");

    // Physical memory.
    memory::pmm::debug();
    println!("");

    // Unclaimed PCI devices.
    println!("Unclaimed PCI devices:");
    pci::debug();
    println!("");
}

// Testing framework.

/// This function is called on panic
/// when running tests.
#[cfg(test)]
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    kernel::test_panic_handler(info)
}
