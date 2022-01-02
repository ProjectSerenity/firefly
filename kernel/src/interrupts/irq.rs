//! irq contains code for handling interrupts via the programmable interrupt controller (PIC).

use crate::interrupts::{PICS, PIC_1_OFFSET};
use x86_64::instructions::interrupts;
use x86_64::structures::idt::{HandlerFunc, InterruptStackFrame};

/// Irq represents an IRQ number.
///
/// It can be used to acknowledge the current IRQ from
/// within the IRQ handler.
///
#[derive(Clone, Copy, Debug)]
pub struct Irq(u8);

impl Irq {
    /// new returns an IRQ if within the range [0, 15], or None otherwise.
    ///
    pub const fn new(irq: u8) -> Option<Irq> {
        if irq <= 15 {
            Some(Irq(irq))
        } else {
            None
        }
    }

    /// new_unsafe returns an IRQ if within the range [0, 15], or panics
    /// otherwise.
    ///
    pub const fn new_unsafe(irq: u8) -> Irq {
        if irq > 15 {
            panic!("invalid IRQ larger than 15");
        }

        Irq(irq)
    }

    /// as_u8 returns the IRQ in the range [0, 15].
    ///
    pub fn as_u8(&self) -> u8 {
        self.0
    }

    /// as_usize returns the IRQ in the range [0, 15].
    ///
    pub fn as_usize(&self) -> usize {
        self.0 as usize
    }

    /// interrupt_id returns the IRQ as its offset in the interrupt table,
    /// which will be offset from 0.
    ///
    pub fn interrupt_id(&self) -> u8 {
        self.0 + PIC_1_OFFSET as u8
    }

    /// acknowledge informs the programmable interrupt controller that
    /// this IRQ has been handled. Do not call acknowledge more than
    /// once.
    ///
    pub fn acknowledge(&self) {
        unsafe {
            PICS.lock().notify_end_of_interrupt(self.interrupt_id());
        }
    }
}

/// IrqHandler represents an IRQ handler function.
///
/// The irq argument is an integer between 0 and 15.
///
pub type IrqHandler = fn(frame: InterruptStackFrame, irq: Irq);

/// IRQS helps us to track which IRQs have been allocated.
///
static IRQS: spin::Mutex<[IrqHandler; 16]> = spin::Mutex::new([irq_handler_none; 16]);

/// register_irq sets the handler for the given IRQ.
///
/// The handler will almost certainly want to acknowledge
/// the interrupt using irq.acknowledge(), so that future
/// interrupts will follow.
///
/// The irq parameter must be an integer between 0 and 15.
///
/// If the given IRQ has already been assigned, register_irq
/// panics.
///
pub fn register_irq(irq: Irq, handler: IrqHandler) {
    interrupts::without_interrupts(|| {
        // Register the handler.
        let mut irqs = IRQS.lock();
        if irqs[irq.as_usize()] != irq_handler_none {
            panic!("IRQ {:?} has already been registered", irq);
        }

        irqs[irq.as_usize()] = handler;

        // Enable the PIC line.
        let mut pics = PICS.lock();
        let mut masks = unsafe { pics.read_masks() };
        let (pic, line) = if irq.as_u8() < 8 {
            (0, irq.as_u8())
        } else {
            (1, irq.as_u8() - 8)
        };

        masks[pic] &= !(1 << line);

        // We have to unmask the link between the
        // PICs to unmask the second PIC.
        if pic == 1 {
            masks[0] &= !(1 << 2);
        }

        unsafe { pics.write_masks(masks[0], masks[1]) };
    });
}

/// irq_handler_none is a dummy IRQ handler, which
/// does nothing.
///
fn irq_handler_none(_frame: InterruptStackFrame, _irq: Irq) {}

// IRQ handlers.

#[inline]
fn irq_handler_generic(frame: InterruptStackFrame, irq: Irq) {
    let handler = IRQS.lock()[irq.as_usize()];
    handler(frame, irq);
}

pub(super) const IRQ_HANDLERS: [HandlerFunc; 16] = [
    irq_handler_0,
    irq_handler_1,
    irq_handler_2,
    irq_handler_3,
    irq_handler_4,
    irq_handler_5,
    irq_handler_6,
    irq_handler_7,
    irq_handler_8,
    irq_handler_9,
    irq_handler_10,
    irq_handler_11,
    irq_handler_12,
    irq_handler_13,
    irq_handler_14,
    irq_handler_15,
];

extern "x86-interrupt" fn irq_handler_0(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(0u8));
}

extern "x86-interrupt" fn irq_handler_1(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(1u8));
}

extern "x86-interrupt" fn irq_handler_2(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(2u8));
}

extern "x86-interrupt" fn irq_handler_3(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(3u8));
}

extern "x86-interrupt" fn irq_handler_4(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(4u8));
}

extern "x86-interrupt" fn irq_handler_5(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(5u8));
}

extern "x86-interrupt" fn irq_handler_6(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(6u8));
}

extern "x86-interrupt" fn irq_handler_7(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(7u8));
}

extern "x86-interrupt" fn irq_handler_8(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(8u8));
}

extern "x86-interrupt" fn irq_handler_9(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(9u8));
}

extern "x86-interrupt" fn irq_handler_10(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(10u8));
}

extern "x86-interrupt" fn irq_handler_11(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(11u8));
}

extern "x86-interrupt" fn irq_handler_12(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(12u8));
}

extern "x86-interrupt" fn irq_handler_13(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(13u8));
}

extern "x86-interrupt" fn irq_handler_14(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(14u8));
}

extern "x86-interrupt" fn irq_handler_15(frame: InterruptStackFrame) {
    irq_handler_generic(frame, Irq(15u8));
}
