// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! kernel implements the Firefly kernel.
//!
//! This module covers the normal initialisation
//! that must always happen when the kernel starts.
//!
//! There is also various helper functionality, such
//! as the test runner and code to exit successfully
//! or with an error from QEMU to aid the testing
//! process.
//!
//! [`init`] is called when the kernel starts, setting
//! up all kernel subsystems.
//!
//! # Kernel subsystems
//!
//! Several parts of kernel functionality are provided
//! in separate crates. These are:
//!
//! - [cpu](::cpu)
//! - [drivers/pci](::pci)
//! - [drivers/serial](::serial)
//! - [drivers/virtio](::virtio)
//! - [filesystem](::filesystem)
//! - [interrupts](::interrupts)
//! - [memory](::memory)
//! - [memory/mmio](::mmio)
//! - [memory/physmem](::physmem)
//! - [memory/segmentation](::segmentation)
//! - [memory/virtmem](::virtmem)
//! - [multitasking](::multitasking)
//! - [network](::network)
//! - [random](::random)
//! - [storage](::storage)
//! - [time](::time)
//! - [utils/align](::align)
//! - [utils/bitmap_index](::bitmap_index)
//! - [utils/pretty](::pretty)
//! - [utils/spin](::spin)
//! - [utils/tar](::tar)

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![allow(unused_crate_dependencies)] // This is to allow the doc references above.
#![feature(alloc_error_handler)]
#![feature(const_btree_new)]

extern crate alloc;

pub mod syscalls;

use bootloader::BootInfo;
use core::include_str;
use interrupts::{register_irq, Irq};
use multitasking::thread::Thread;
use multitasking::{scheduler, thread};
use x86_64::structures::idt::InterruptStackFrame;

/// The Firefly license text.
///
pub const LICENSE: &str = include_str!("../LICENSE");

/// Initialise the kernel and its core components.
///
/// `init` currently performs the following steps:
///
/// - Check that the CPU supports the features we need.
/// - Initialise the [Global Descriptor Table](segmentation::SegmentData).
/// - Initialise the [Programmable Interrupt Controller](interrupts).
/// - Initialise the [Real-time clock and Programmable Interval Timer](time)
/// - Enables system interrupts.
/// - Initialise the [memory managers and kernel heap](virtmem).
/// - Initialise the [CPU-local data](cpu).
/// - Initialise the [scheduler](thread).
/// - Scan for and initialise the [PCI](drivers/pci) device drivers.
/// - Initialise the [CSPRNG](random).
///
pub fn init(boot_info: &'static BootInfo) {
    // Check the CPU has the features we need.
    // This doesn't need any extra resources
    // and there's no point going any further
    // if we dont' have the bare minimum.
    cpu::check_features();

    // Set up our bootstrap segment data. This
    // is a single global set of segment data
    // for the entire system. When we can, we
    // switch to a per-CPU set to allow each
    // core to run a different thread.
    segmentation::bootstrap();

    // Set up our interrupt handlers, including
    // the system ticker. We can then enable
    // interrupts safely.
    interrupts::init();
    time::init();
    register_irq(Irq::PID, timer_interrupt_handler);
    x86_64::instructions::interrupts::enable();

    // Set up the heap allocator.
    unsafe { virtmem::init(boot_info) };

    // Now we have a working heap, we can set
    // up the global memory region for CPU-local
    // data. With that in place, we can initialise
    // the part of the region for this CPU.
    cpu::global_init();
    cpu::per_cpu_init();

    // Now we have per-CPU data set up, we can
    // set up the other per-CPU data in other
    // kernel subsystems.
    segmentation::per_cpu_init();
    thread::per_cpu_init();
    syscalls::per_cpu_init();

    // Set up our device drivers.
    for device in pci::scan().into_iter() {
        for check in PCI_DRIVERS.iter() {
            if let Some(install) = check(&device) {
                install(device);
                break;
            }
        }
    }

    // Either we have a source of random by now,
    // or we never will. We also want to start
    // the random subsystem's helper thread to
    // keep the entropy pool fresh.
    random::init();
    Thread::start_kernel_thread(entropy_reseed_helper);

    // The kernel is now fully initialised, so we
    // freeze the kernel page mappings. This means
    // we can start making other page mappings for
    // user processes, without risking divergence
    // between the views of kernel space.
    virtmem::freeze_kernel_mappings();
}

/// This is the set of configured PCI device drivers.
///
/// For each PCI device discovered, each callback listed
/// here will be checked to determine whether the driver
/// supports the device. The first device that returns a
/// driver will then take ownership of the device.
///
const PCI_DRIVERS: &[pci::DriverSupportCheck] = &[virtio::pci_device_check];

/// The PIT's interrupt handler.
///
fn timer_interrupt_handler(_stack_frame: InterruptStackFrame, irq: Irq) {
    time::tick();

    irq.acknowledge();

    if !scheduler::ready() {
        return;
    }

    // Check whether any timers have expired.
    scheduler::timers::process();

    // Switch thread if necessary.
    scheduler::preempt();
}

/// entropy_reseed_helper is an entry point used by an
/// entropy management thread to ensure the CSPRNG
/// continues to receive entropy over time.
///
fn entropy_reseed_helper() -> ! {
    loop {
        scheduler::sleep(random::RESEED_INTERVAL);

        random::reseed();
    }
}

// The kernel's allocator error handling.
//
// Note that we disable this for Rustdoc, or it gets
// confused into thinking it's defined twice.
#[cfg(not(doc))]
#[alloc_error_handler]
fn alloc_error_handler(layout: alloc::alloc::Layout) -> ! {
    panic!("allocation error: {:?}", layout)
}

/// Halt the CPU using a loop of the `hlt` instruction.
///
pub fn halt_loop() -> ! {
    loop {
        x86_64::instructions::hlt();
    }
}

/// shutdown_qemu uses the 0x604 I/O port
/// to instruct QEMU to shut down successfully.
///
#[doc(hidden)]
pub fn shutdown_qemu() -> ! {
    unsafe {
        x86_64::instructions::port::Port::new(0x604).write(0x2000u16);
    }

    // Sometimes there's a delay before QEMU fully
    // exits, so to avoid the below panic triggering
    // unnecessarily, we loop briefly.
    let stop = time::after(time::Duration::from_secs(2));
    while stop.after(time::now()) {
        for _ in 0..1000 {
            core::hint::spin_loop();
        }
    }

    unreachable!("instruction to exit QEMU returned somehow");
}
