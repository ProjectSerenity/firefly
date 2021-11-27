//! interrupts handles hardware interrupts and the PIC.

// This module handles hardware interrupts and the PIC.
// ::init sets up the interrupt descriptor table and
// initialises the PIC, remapping it to available interrupt
// lines.
//
// Currently, there are 4 interrupt handlers configured:
//
// 1. Breakpoints: the breakpoint handler prints a message
//    and then continues execution.
// 2. Page faults: the page fault handler currently prints
//    and error message and enters a halt loop.
// 3. Double faults: the double fault handler switches to
//    a safe stack using the GDT and panics with an error
//    message.
// 4. Timer: the timer handler increments the counter in
//    the time module and acknowledges the interrupt.
//
// The functionality for the PIC is quite different from
// the functionality for CPU exceptions. Exceptions are
// handled directly through the IDT. The PIC's IRQs are
// instead registered using the register_irq function,
// making it easier to handle IRQs, without needing to
// know the details of the PIC.
//
// The other big difference with the IRQ handling is that
// IRQ handlers don't need to acknowledge the PIC, and
// are passed the IRQ number.

use crate::{gdt, halt_loop, println, time};
use lazy_static::lazy_static;
use pic8259::ChainedPics;
use x86_64::registers::control::Cr2;
use x86_64::structures::idt::{InterruptDescriptorTable, InterruptStackFrame, PageFaultErrorCode};

/// init loads the interrupt descriptor table and maps
/// the PIC to available interrupt lines.
///
pub fn init() {
    IDT.load();
    unsafe { PICS.lock().initialize() };
    register_irq(0, timer_interrupt_handler);
}

lazy_static! {
    static ref IDT: InterruptDescriptorTable = {
        let mut idt = InterruptDescriptorTable::new();
        idt.breakpoint.set_handler_fn(breakpoint_handler);
        idt.page_fault.set_handler_fn(page_fault_handler);
        unsafe {
            idt.double_fault
                .set_handler_fn(double_fault_handler)
                .set_stack_index(gdt::DOUBLE_FAULT_IST_INDEX);
        }

        idt[PIC_1_OFFSET + 0].set_handler_fn(irq_handler_0);
        idt[PIC_1_OFFSET + 1].set_handler_fn(irq_handler_1);
        idt[PIC_1_OFFSET + 2].set_handler_fn(irq_handler_2);
        idt[PIC_1_OFFSET + 3].set_handler_fn(irq_handler_3);
        idt[PIC_1_OFFSET + 4].set_handler_fn(irq_handler_4);
        idt[PIC_1_OFFSET + 5].set_handler_fn(irq_handler_5);
        idt[PIC_1_OFFSET + 6].set_handler_fn(irq_handler_6);
        idt[PIC_1_OFFSET + 7].set_handler_fn(irq_handler_7);
        idt[PIC_1_OFFSET + 8].set_handler_fn(irq_handler_8);
        idt[PIC_1_OFFSET + 9].set_handler_fn(irq_handler_9);
        idt[PIC_1_OFFSET + 10].set_handler_fn(irq_handler_10);
        idt[PIC_1_OFFSET + 11].set_handler_fn(irq_handler_11);
        idt[PIC_1_OFFSET + 12].set_handler_fn(irq_handler_12);
        idt[PIC_1_OFFSET + 13].set_handler_fn(irq_handler_13);
        idt[PIC_1_OFFSET + 14].set_handler_fn(irq_handler_14);
        idt[PIC_1_OFFSET + 15].set_handler_fn(irq_handler_15);

        idt
    };
}

// Interrupt handlers.

extern "x86-interrupt" fn breakpoint_handler(stack_frame: InterruptStackFrame) {
    println!("EXCEPTION: BREAKPOINT\n{:#?}", stack_frame);
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

extern "x86-interrupt" fn double_fault_handler(
    stack_frame: InterruptStackFrame,
    _error_code: u64,
) -> ! {
    panic!("EXCEPTION: DOUBLE FAULT\n{:#?}", stack_frame);
}

fn timer_interrupt_handler(_stack_frame: InterruptStackFrame, _irq: u8) {
    time::tick();
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

/// IrqHandler represents an IRQ handler function.
///
/// The irq argument is an integer between 0 and 15.
///
pub type IrqHandler = fn(frame: InterruptStackFrame, irq: u8);

/// irq_handler_none is a dummy IRQ handler, which
/// does nothing.
///
fn irq_handler_none(_frame: InterruptStackFrame, _irq: u8) {}

// IRQ handlers.

#[inline]
fn irq_handler_generic(frame: InterruptStackFrame, irq: u8) {
    let irqs = IRQS.try_lock();
    if let Some(irqs) = irqs {
        let handler = irqs[irq as usize];
        handler(frame, irq);
    }

    unsafe {
        PICS.lock()
            .notify_end_of_interrupt(irq + PIC_1_OFFSET as u8);
    }
}

extern "x86-interrupt" fn irq_handler_0(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 0u8);
}

extern "x86-interrupt" fn irq_handler_1(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 1u8);
}

extern "x86-interrupt" fn irq_handler_2(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 2u8);
}

extern "x86-interrupt" fn irq_handler_3(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 3u8);
}

extern "x86-interrupt" fn irq_handler_4(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 4u8);
}

extern "x86-interrupt" fn irq_handler_5(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 5u8);
}

extern "x86-interrupt" fn irq_handler_6(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 6u8);
}

extern "x86-interrupt" fn irq_handler_7(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 7u8);
}

extern "x86-interrupt" fn irq_handler_8(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 8u8);
}

extern "x86-interrupt" fn irq_handler_9(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 9u8);
}

extern "x86-interrupt" fn irq_handler_10(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 10u8);
}

extern "x86-interrupt" fn irq_handler_11(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 11u8);
}

extern "x86-interrupt" fn irq_handler_12(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 12u8);
}

extern "x86-interrupt" fn irq_handler_13(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 13u8);
}

extern "x86-interrupt" fn irq_handler_14(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 14u8);
}

extern "x86-interrupt" fn irq_handler_15(frame: InterruptStackFrame) {
    irq_handler_generic(frame, 15u8);
}

/// IRQS helps us to track which IRQs have been allocated.
///
static IRQS: spin::Mutex<[IrqHandler; 16]> = spin::Mutex::new([irq_handler_none; 16]);

/// register_irq sets the handler for the given IRQ.
///
/// The irq parameter must be an integer between 0 and 15.
///
/// If the given IRQ has already been assigned, register_irq
/// panics.
///
pub fn register_irq(irq: u8, handler: IrqHandler) {
    let mut irqs = IRQS.lock();
    if irq > 15 {
        panic!("invalid IRQ {} passed to register_irq", irq);
    }

    if irqs[irq as usize] != irq_handler_none {
        panic!("IRQ {} has already been registered", irq);
    }

    irqs[irq as usize] = handler;
}

// Tests

#[test_case]
fn test_breakpoint_exception() {
    // Invoke a breakpoint exception.
    x86_64::instructions::interrupts::int3();
}
