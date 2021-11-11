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

use crate::{gdt, halt_loop, println, time};
use lazy_static::lazy_static;
use pic8259::ChainedPics;
use x86_64::registers::control::Cr2;
use x86_64::structures::idt::{InterruptDescriptorTable, InterruptStackFrame, PageFaultErrorCode};

// init loads the interrupt descriptor table and maps
// the PIC to available interrupt lines.
//
pub fn init() {
    IDT.load();
    unsafe { PICS.lock().initialize() };
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
        idt[InterruptIndex::Timer.as_usize()].set_handler_fn(timer_interrupt_handler);

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

extern "x86-interrupt" fn timer_interrupt_handler(_stack_frame: InterruptStackFrame) {
    time::TICKER.tick();

    unsafe {
        PICS.lock()
            .notify_end_of_interrupt(InterruptIndex::Timer.as_u8());
    }
}

// PIC code.

pub const PIC_1_OFFSET: u8 = 32;
pub const PIC_2_OFFSET: u8 = PIC_1_OFFSET + 8;

pub static PICS: spin::Mutex<ChainedPics> =
    spin::Mutex::new(unsafe { ChainedPics::new(PIC_1_OFFSET, PIC_2_OFFSET) });

#[derive(Debug, Clone, Copy)]
#[repr(u8)]
pub enum InterruptIndex {
    Timer = PIC_1_OFFSET,
    Keyboard,
}

impl InterruptIndex {
    fn as_u8(self) -> u8 {
        self as u8
    }

    fn as_usize(self) -> usize {
        usize::from(self.as_u8())
    }
}

// Tests

#[test_case]
fn test_breakpoint_exception() {
    // Invoke a breakpoint exception.
    x86_64::instructions::interrupts::int3();
}
