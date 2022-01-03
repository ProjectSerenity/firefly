// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Handles hardware and software interrupts and the [Programmable Interrupt Controller](https://en.wikipedia.org/wiki/Programmable_interrupt_controller) (PIC).
//!
//! This module handles hardware and software interrupts and the PIC.
//! [`init`] sets up the interrupt descriptor table and initialises
//! the PIC, remapping it to available interrupt lines.
//!
//! ## CPU exceptions
//!
//! Currently, there are several [CPU exception](https://wiki.osdev.org/Exceptions) handlers configured:
//!
//! - [Breakpoints]: The breakpoint handler prints a message and then resumes execution.
//! - [Invalid opcode]: The invalid opcode handler panics with an error.
//! - [Double fault]: The double fault handler switches to a safe stack using the GDT and panics with an error message.
//! - [Segment not present]: The segment not present handler panics with an error.
//! - [General protection]: The general protection fault handler panics with an error.
//! - [Page fault]: The page fault handler prints an error message and enters a halt loop.
//!
//! ## Software interrupts
//!
//! No software interrupts are handled yet.
//!
//! ## IRQ handling and the PIC
//!
//! The functionality for the PIC is quite different from the functionality
//! for CPU exceptions. Exceptions are handled directly through the IDT.
//! The PIC's IRQs are instead registered using the [`register_irq`] function,
//! making it easier to handle IRQs, without needing to know the details of
//! the PIC.
//!
//! The other big difference with the IRQ handling is that IRQ handlers don't
//! need to acknowledge the PIC, and are passed the IRQ number. This is made
//! easier by the [`Irq`] type and its [`acknowledge`](Irq::acknowledge)
//! method.
//!
//! # Examples
//!
//! Register a simple ticker using the [Programmable Interval Timer](https://en.wikipedia.org/wiki/Programmable_interval_timer) (PIT):
//!
//! ```
//! /// TICKER stores the counter we use to measure the passage of time.
//! static TICKER: AtomicU64 = AtomicU64::new(0);
//!
//! /// Our IRQ handler, which increments the counter.
//! fn timer_interrupt_handler(_stack_frame: InterruptStackFrame, irq: Irq) {
//!     TICKER.fetch_add(1, Ordering::Relaxed);
//! }
//!
//! /// Register our IRQ handler.
//! fn init() {
//!     const PIT_IRQ: Irq::new_unsafe(0);
//!     register_irq(PIT_IRQ, timer_interrupt_handler);
//! }
//! ```
//!
//! [Breakpoints]: https://wiki.osdev.org/Exceptions#Breakpoint
//! [Invalid opcode]: https://wiki.osdev.org/Exceptions#Invalid_Opcode
//! [Double fault]: https://wiki.osdev.org/Exceptions#Double_Fault
//! [Segment not present]: https://wiki.osdev.org/Exceptions#Segment_Not_Present
//! [General protection]: https://wiki.osdev.org/Exceptions#General_Protection_Fault
//! [Page fault]: https://wiki.osdev.org/Exceptions#Page_Fault
//!

mod irq;

use crate::{gdt, halt_loop, println};
use lazy_static::lazy_static;
use pic8259::ChainedPics;
use x86_64::registers::control::Cr2;
use x86_64::structures::idt::{InterruptDescriptorTable, InterruptStackFrame, PageFaultErrorCode};

pub use irq::{register_irq, Irq};

/// Loads the [Interrupt Descriptor Table](https://en.wikipedia.org/wiki/Interrupt_descriptor_table) (IDT)
/// and the [Programmable Interrupt Controller](https://en.wikipedia.org/wiki/Programmable_interrupt_controller) (PIC).
///
/// The PIC is remapped from its initial offsets so that it
/// does not overlap with CPU exceptions. All IRQs in the PIC
/// are disabled by default. Individual IRQs can be enabled
/// by registering a handler using [`register_irq`].
///
pub fn init() {
    IDT.load();
    unsafe {
        let mut pics = PICS.lock();
        pics.initialize();
        pics.disable(); // We disable all PIC lines by default.
    }
}

lazy_static! {
    /// IDT is our interrupt descriptor table.
    ///
    /// We configure it with our CPU exception handlers,
    /// plus default exception handlers for the IRQs.
    ///
    static ref IDT: InterruptDescriptorTable = {
        let mut idt = InterruptDescriptorTable::new();
        idt.breakpoint.set_handler_fn(breakpoint_handler);
        idt.invalid_opcode.set_handler_fn(invalid_opcode_handler);
        unsafe {
            idt.double_fault
                .set_handler_fn(double_fault_handler)
                .set_stack_index(gdt::DOUBLE_FAULT_IST_INDEX);
        }
        idt.segment_not_present
            .set_handler_fn(segment_not_present_handler);
        idt.general_protection_fault
            .set_handler_fn(general_protection_fault_handler);
        idt.page_fault.set_handler_fn(page_fault_handler);

        for (i, handler) in irq::IRQ_HANDLERS.iter().enumerate() {
            idt[PIC_1_OFFSET + i].set_handler_fn(*handler);
        }

        idt
    };
}

// CPU exception handlers.

extern "x86-interrupt" fn breakpoint_handler(stack_frame: InterruptStackFrame) {
    println!("EXCEPTION: BREAKPOINT\n{:#?}", stack_frame);
}

extern "x86-interrupt" fn invalid_opcode_handler(stack_frame: InterruptStackFrame) {
    panic!("EXCEPTION: INVALID OPCODE\n{:#?}", stack_frame);
}

extern "x86-interrupt" fn double_fault_handler(
    stack_frame: InterruptStackFrame,
    _error_code: u64,
) -> ! {
    panic!("EXCEPTION: DOUBLE FAULT\n{:#?}", stack_frame);
}

extern "x86-interrupt" fn segment_not_present_handler(
    stack_frame: InterruptStackFrame,
    error_code: u64,
) {
    panic!(
        "EXCEPTION: SEGMENT NOT PRESENT: index {}\n{:#?}",
        error_code, stack_frame
    );
}

extern "x86-interrupt" fn general_protection_fault_handler(
    stack_frame: InterruptStackFrame,
    error_code: u64,
) {
    if error_code != 0 {
        panic!(
            "EXCEPTION: GENERAL PROTECTION FAULT: segment index {}\n{:#?}",
            error_code, stack_frame
        );
    } else {
        panic!("EXCEPTION: GENERAL PROTECTION FAULT:\n{:#?}", stack_frame);
    }
}

extern "x86-interrupt" fn page_fault_handler(
    stack_frame: InterruptStackFrame,
    error_code: PageFaultErrorCode,
) {
    println!("EXCEPTION: PAGE FAULT");
    println!("Accessed Address: {:?}", Cr2::read());
    println!("Error Code: {:?}", error_code);
    println!("{:#?}", stack_frame);
    halt_loop();
}

// PIC code.

const PIC_1_OFFSET: usize = 32;
const PIC_2_OFFSET: usize = PIC_1_OFFSET + 8;

/// PICS is the set of programmable interrupt controllers.
///
/// PICS can be used to acknowledge an interrupt.
///
static PICS: spin::Mutex<ChainedPics> =
    spin::Mutex::new(unsafe { ChainedPics::new(PIC_1_OFFSET as u8, PIC_2_OFFSET as u8) });

// Tests

#[test_case]
fn test_breakpoint_exception() {
    // Invoke a breakpoint exception.
    x86_64::instructions::interrupts::int3();
}
