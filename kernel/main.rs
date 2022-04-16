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
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![allow(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]

extern crate alloc;

use alloc::{format, vec};
use bootloader::{entry_point, BootInfo};
use core::panic::PanicInfo;
use filesystem::{FileType, Permissions};
use memory::PageTable;
use multitasking::process::Process;
use multitasking::thread::Thread;
use multitasking::{scheduler, with_processes};
use serial::println;
use storage::block;

/// This function is called on panic.
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    println!("{}", info);

    kernel::halt_loop();
}

entry_point!(kmain);

fn kmain(boot_info: &'static BootInfo) -> ! {
    println!("Kernel booting...");

    kernel::init(boot_info);

    println!("Kernel ready!");
    println!("Kernel booted at {}.", time::boot_time());
    cpu::print_branding();

    // Set up our initial workload for when
    // we get a DHCP configuration.
    let workload = Thread::create_kernel_thread(initial_workload);
    network::register_workload(workload.waker());

    // Hand over to the scheduler.
    scheduler::start();
}

fn initial_workload() -> ! {
    println!("Starting initial workload.");

    block::iter_devices(|dev| {
        let mut reader = tar::Reader::new(dev);
        let initial_workload = (&mut reader)
            .find(|f| {
                let info = f.info();
                info.size > 0
                    && info.file_type == FileType::RegularFile
                    && info.permissions.contains(Permissions::EXECUTE)
                    && info.name == "initial-workload"
            })
            .expect("initial workload not found");

        let info = initial_workload.info();
        let mut data = vec![0u8; info.size];
        let n = reader
            .read(&initial_workload, &mut data[..])
            .expect(format!("failed to read {}", info.name).as_str());
        if n != info.size {
            panic!(
                "failed to read {}: got {} bytes, want {}",
                info.name, n, info.size
            );
        }

        match Process::create_user_process(&info.name, &data[..]) {
            Ok((_, thread_id)) => {
                thread_id.resume();
            }
            Err(s) => panic!("failed to start process: {:?}", s),
        }
    });

    // Hand off to the scheduler, until all
    // processes have exited.
    loop {
        scheduler::switch();
        if with_processes(|p| p.len()) == 0 {
            println!("All user processes have exited. Shutting down.");
            kernel::shutdown_qemu();
        }
    }
}

#[allow(dead_code)]
fn debug() {
    println!();

    // Virtual memory.
    println!("Virtual memory manager:");
    virtmem::debug(&PageTable::current());
    println!();

    // Physical memory.
    physmem::debug();
    println!();
}
