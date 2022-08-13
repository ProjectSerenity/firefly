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

use alloc::string::ToString;
use alloc::{format, vec};
use bootinfo::{entry_point, BootInfo};
use core::panic::PanicInfo;
use filesystem::{FileType, Permissions};
use memory::PageTable;
use multitasking::process::Process;
use multitasking::thread::Thread;
use multitasking::{scheduler, thread};
use serial::println;
use storage::block;
use x86_64::instructions::{hlt, interrupts};

/// This function is called on panic.
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    println!("{}", info);

    loop {
        interrupts::disable();
        hlt();
    }
}

entry_point!(kmain);

fn kmain(boot_info: &'static BootInfo) -> ! {
    println!("Kernel booting...");

    kernel::init(boot_info);

    println!("Kernel ready!");
    println!("Kernel booted at {}.", time::boot_time());
    cpu::print_branding();

    // Check that we have the devices necessary
    // to do something useful.
    if network::with_interfaces(|i| i.is_empty()) {
        println!("No network interfaces identified. Shutting down.");
        power::shutdown();
    }

    if block::with_devices(|d| d.is_empty()) {
        println!("No storage devices identified. Shutting down.");
        power::shutdown();
    }

    // Set up our initial workload for when
    // we get a DHCP configuration.
    let workload =
        Thread::create_kernel_thread(initial_workload, "initial workload starter".to_string());
    network::register_workload(workload.waker());

    // Hand over to the scheduler.
    scheduler::start();
}

fn initial_workload() -> ! {
    println!("Starting initial workload.");

    block::iter_devices(|dev| {
        // Read the first block to see whether we
        // need to advance the device.
        if dev.segment_size() != 512 {
            return;
        }

        let mut block = [0u8; 512];
        if let Ok(n) = dev.read(0, &mut block) {
            if n != 512 {
                return;
            }
        } else {
            return;
        }

        let offset = if block[511] == 0xaa && block[510] == 0x55 {
            let offset = u32::from_le_bytes([block[506], block[507], block[508], block[509]]);
            offset as usize / 512
        } else {
            0
        };

        // Read the archive.
        let mut reader = tar::Reader::new_at_offset(dev, offset);
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

    // We've started any processes, so we
    // can stop here and leave the rest
    // to the scheduler.
    thread::exit();
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
