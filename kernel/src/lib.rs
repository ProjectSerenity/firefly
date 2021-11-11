// This module covers the normal initialisation
// that must always happen when the kernel starts.
//
// There is also various helper functionality, such
// as the test runner and code to exit successfully
// or with an error from QEMU to aid the testing
// process.
//
// This also includes the Locked type, which wraps
// a spin lock to allow us to define traits and
// methods on locked types.
//
// ::init is called when the kernel starts, so it
// should only perform actions that the kernel must
// always take. In particular, it does not set up
// the heap allocator, as there are situations (tests)
// where we want to change that behaviour.

#![no_std]
#![cfg_attr(test, no_main)]
#![feature(custom_test_frameworks)]
#![feature(abi_x86_interrupt)]
#![feature(alloc_error_handler)]
#![feature(const_mut_refs)]
#![test_runner(crate::test_runner)]
#![reexport_test_harness_main = "test_main"]

extern crate alloc;

use core::panic::PanicInfo;
use lazy_static::lazy_static;
use raw_cpuid::CpuId;
use x86_64::instructions::port::Port;

pub mod allocator;
pub mod gdt;
pub mod interrupts;
pub mod memory;
pub mod serial;
pub mod time;

lazy_static! {
    pub static ref CPU_ID: CpuId = CpuId::new();
}

/// init sets up critical core functions of the kernel.
///
pub fn init() {
    gdt::init();
    interrupts::init();
    time::init();
    x86_64::instructions::interrupts::enable();
}

#[alloc_error_handler]
fn alloc_error_handler(layout: alloc::alloc::Layout) -> ! {
    panic!("allocation error: {:?}", layout)
}

/// halt_loop halts the CPU using a loop of the hlt
/// instruction.
///
pub fn halt_loop() -> ! {
    loop {
        x86_64::instructions::hlt();
    }
}

/// A wrapper around spin::Mutex to permit trait implementations.
pub struct Locked<A> {
    inner: spin::Mutex<A>,
}

impl<A> Locked<A> {
    pub const fn new(inner: A) -> Self {
        Locked {
            inner: spin::Mutex::new(inner),
        }
    }

    pub fn lock(&self) -> spin::MutexGuard<A> {
        self.inner.lock()
    }
}

/// Testable represents a test function.
///
pub trait Testable {
    fn run(&self) -> ();
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
pub fn test_runner(tests: &[&dyn Testable]) {
    println!("Running {} tests", tests.len());
    for test in tests {
        test.run();
    }

    exit_qemu(QemuExitCode::Success);
}

/// Panic handler for tests.
///
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
pub enum QemuExitCode {
    Success = 0x10,
    Failed = 0x11,
}

/// exit_qemu uses the 0xf4 I/O port to
/// instruct QEMU to exit with the given
/// exit code.
///
pub fn exit_qemu(exit_code: QemuExitCode) {
    unsafe {
        let mut port = Port::new(0xf4);
        port.write(exit_code as u32);
    }
}

#[cfg(test)]
use bootloader::{entry_point, BootInfo};

#[cfg(test)]
entry_point!(test_kernel_main);

/// test_kernel_main is the entry point for `cargo xtest`.
///
#[cfg(test)]
fn test_kernel_main(_boot_info: &'static BootInfo) -> ! {
    init();
    test_main();
    halt_loop();
}

#[cfg(test)]
#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    test_panic_handler(info)
}
