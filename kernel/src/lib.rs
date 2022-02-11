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
//! [`init`] is called when the kernel starts, so it
//! should only perform actions that the kernel must
//! always take. In particular, it does not set up
//! device drivers.
//!
//! # Kernel subsystems
//!
//! Several parts of kernel functionality are provided
//! in separate crates. These are:
//!
//! - [cpuid](::cpuid)
//! - [drivers/pci](::pci)
//! - [drivers/serial](::serial)
//! - [filesystem](::filesystem)
//! - [interrupts](::interrupts)
//! - [memory/memlayout](::memlayout)
//! - [memory/mmio](::mmio)
//! - [memory/physmem](::physmem)
//! - [memory/virtmem](::virtmem)
//! - [random](::random)
//! - [segmentation](::segmentation)
//! - [storage](::storage)
//! - [time](::time)
//! - [utils/align](::align)
//! - [utils/bitmap_index](::bitmap_index)
//! - [utils/pretty](::pretty)
//! - [utils/tar](::tar)

#![no_std]
#![cfg_attr(test, no_main)]
#![feature(alloc_error_handler)]
#![feature(asm)]
#![feature(binary_heap_retain)]
#![feature(const_btree_new)]
#![feature(custom_test_frameworks)]
#![feature(global_asm)]
#![test_runner(crate::test_runner)]
#![reexport_test_harness_main = "test_main"]

extern crate alloc;

pub mod drivers;
pub mod memory;
pub mod multitasking;
pub mod network;
pub mod syscalls;

use crate::multitasking::cpu_local;
use crate::multitasking::thread::scheduler;
use bootloader::BootInfo;
use core::panic::PanicInfo;
use core::pin::Pin;
use interrupts::{register_irq, Irq};
use memlayout::KERNEL_STACK_0;
use segmentation::SegmentData;
use serial::{print, println};
use x86_64::instructions::port::Port;
use x86_64::structures::idt::InterruptStackFrame;

/// Initialise the kernel and its core components.
///
/// `init` currently performs the following steps:
///
/// - Check that the CPU supports the features we need.
/// - Initialise the [Global Descriptor Table](segmentation::SegmentData).
/// - Initialise the [Programmable Interrupt Controller](interrupts).
/// - Initialise the [Real-time clock and Programmable Interval Timer](time)
/// - Enables system interrupts.
/// - Initialise the [memory managers and kernel heap](memory).
/// - Initialise the [CPU-local data](multitasking/cpu_local).
/// - Initialise the [scheduler](multitasking/thread).
///
pub fn init(boot_info: &'static BootInfo) {
    cpuid::check_features();

    // Make sure we shadow the initial segment
    // data so we can't circumvent the pin later.
    let mut segment_data = SegmentData::new_uninitialised();
    let mut segment_data = unsafe { Pin::new_unchecked(&mut segment_data) };
    segment_data.init();
    segment_data.activate();

    interrupts::init();
    time::init();
    register_irq(Irq::PID, timer_interrupt_handler);
    x86_64::instructions::interrupts::enable();

    // Set up the heap allocator.
    unsafe { memory::init(boot_info) };
    cpu_local::init(cpu_local::CpuId::new(), &KERNEL_STACK_0, segment_data);
    syscalls::init();
}

/// The PIT's interrupt handler.
///
fn timer_interrupt_handler(_stack_frame: InterruptStackFrame, irq: Irq) {
    time::tick();

    irq.acknowledge();

    if !cpu_local::ready() || !scheduler::ready() {
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
pub fn entropy_reseed_helper() -> ! {
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

// Test helpers.

/// Testable represents a test function.
///
#[doc(hidden)]
pub trait Testable {
    fn run(&self);
}

/// Wrap tests with debug statements.
///
impl<T> Testable for T
where
    T: Fn(),
{
    fn run(&self) {
        print!("{}...\t", core::any::type_name::<T>());
        self();
        println!("[ok]");
    }
}

/// Entry point for the set of unit
/// tests.
///
#[doc(hidden)]
pub fn test_runner(tests: &[&dyn Testable]) {
    println!("Running {} tests", tests.len());
    for test in tests {
        test.run();
    }

    exit_qemu(QemuExitCode::Success);
}

/// Panic handler for tests.
///
#[doc(hidden)]
pub fn test_panic_handler(info: &PanicInfo) -> ! {
    println!("[failed]\n");
    println!("Error: {}\n", info);
    exit_qemu(QemuExitCode::Failed);
    halt_loop();
}

/// QemuExitCode represents the two valid
/// values for exiting QEMU.
///
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u32)]
#[doc(hidden)]
pub enum QemuExitCode {
    Success = 0x10,
    Failed = 0x11,
}

/// exit_qemu uses the 0xf4 I/O port to
/// instruct QEMU to exit with the given
/// exit code.
///
#[doc(hidden)]
pub fn exit_qemu(exit_code: QemuExitCode) {
    unsafe {
        let mut port = Port::new(0xf4);
        port.write(exit_code as u32);
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
    for _ in 0..10000 {}

    unreachable!("instruction to exit QEMU returned somehow");
}

#[cfg(test)]
use bootloader::entry_point;

#[cfg(test)]
entry_point!(test_kernel_main);

/// test_kernel_main is the entry point for `cargo xtest`.
///
#[cfg(test)]
fn test_kernel_main(boot_info: &'static BootInfo) -> ! {
    init(boot_info);
    test_main();
    halt_loop();
}

#[cfg(test)]
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    test_panic_handler(info)
}
