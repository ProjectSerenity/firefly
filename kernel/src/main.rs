// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// This is the entry point for the kernel, which
// runs the kernel's standard initialisation,
// then either runs tests or starts kmain,
// where the kernel does its real work.

#![no_std]
#![no_main]

extern crate alloc;

use bootloader::{entry_point, BootInfo};
use core::panic::PanicInfo;
use kernel::multitasking::thread::{scheduler, Thread};
use kernel::{drivers, network};
use serial::println;
use virtmem::kernel_pml4;

/// This function is called on panic.
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

    kmain();

    kernel::halt_loop();
}

fn kmain() {
    println!("Kernel ready!");
    println!("Kernel booted at {}.", time::boot_time());
    cpu::print_branding();

    // Set up our initial workload for when
    // we get a DHCP configuration.
    let workload = Thread::create_kernel_thread(initial_workload);
    network::register_workload(workload.waker());

    let mut mapper = unsafe { kernel_pml4() };
    for device in pci::scan().into_iter() {
        for check in drivers::PCI_DRIVERS.iter() {
            if let Some(install) = check(&device) {
                install(device, &mut mapper);
                break;
            }
        }
    }
    drop(mapper);

    random::init();
    Thread::start_kernel_thread(kernel::entropy_reseed_helper);

    // Hand over to the scheduler.
    scheduler::start();
}

fn initial_workload() -> ! {
    println!("Starting initial workload.");

    let mut buf = [0u8; 16];
    println!("RNG before: {:02x?}", buf.to_vec());
    random::read(&mut buf[..]);
    println!("RNG after:  {:02x?}", buf.to_vec());

    kernel::shutdown_qemu();
}

#[allow(dead_code)]
fn debug() {
    println!();

    // Virtual memory.
    println!("Virtual memory manager:");
    unsafe { virtmem::debug(kernel_pml4().level_4_table()) };
    println!();

    // Physical memory.
    physmem::debug();
    println!();
}
