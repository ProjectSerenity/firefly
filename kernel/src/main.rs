#![no_std]
#![no_main]
#![feature(abi_x86_interrupt)]
#![feature(custom_test_frameworks)]
#![test_runner(kernel::test_runner)]
#![reexport_test_harness_main = "test_main"]

extern crate alloc;

use bootloader::{entry_point, BootInfo};
use core::panic::PanicInfo;
use kernel::time::BOOT_TIME;
use kernel::{allocator, memory, println, CPU_ID};
use x86_64::VirtAddr;

/// This function is called on panic.
#[cfg(not(test))]
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    println!("{}", info);

    kernel::halt_loop();
}

entry_point!(kernel_main);

#[allow(unused_variables)]
fn kernel_main(boot_info: &'static BootInfo) -> ! {
    println!("Kernel booting...");
    kernel::init();

    #[cfg(test)]
    test_main();

    #[cfg(not(test))]
    kmain(boot_info);

    kernel::halt_loop();
}

#[cfg(not(test))]
fn kmain(boot_info: &'static BootInfo) {
    // Set up the heap allocator.
    let phys_mem_offset = VirtAddr::new(boot_info.physical_memory_offset);
    let mut mapper = unsafe { memory::init(phys_mem_offset) };
    let mut frame_allocator =
        unsafe { memory::BootInfoFrameAllocator::init(&boot_info.memory_map) };
    allocator::init_heap(&mut mapper, &mut frame_allocator).expect("heap initialization failed");

    println!("Kernel ready!");
    println!("Kernel booted at {}.", *BOOT_TIME.lock());
    if let Some(branding) = CPU_ID.get_processor_brand_string() {
        println!("Kernel running on {} CPU.", branding.as_str());
    } else if let Some(version) = CPU_ID.get_vendor_info() {
        println!("Kernel running on {} CPU.", version.as_str());
    } else {
        println!("Kernel running on unknown CPU.");
    }
}

// Testing framework.

/// This function is called on panic
/// when running tests.
#[cfg(test)]
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    kernel::test_panic_handler(info)
}
