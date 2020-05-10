#include "std.h"
#include "int.h"
#include "port.h"
#include "time.h"

typedef struct {
	uintptr ip;
	uint64 cs;
	uint64 flags;
	uintptr sp;
	uint64 ss;
} interruptFrame;

static void handleUnsupportedInterrupt(interruptFrame* frame);

static void handleDivideException(interruptFrame* frame);         // INT 0
static void handleDebugException(interruptFrame* frame);          // INT 1
static void handleNMIInterrupt(interruptFrame* frame);            // INT 2
static void handleBreakpointInterrupt(interruptFrame* frame);     // INT 3

static void handleOverflowInterrupt(interruptFrame* frame);           // INT 4
static void handleBoundRangeInterrupt(interruptFrame* frame);         // INT 5
static void handleInvalidOpcodeInterrupt(interruptFrame* frame);      // INT 6
static void handleDeviceNotAvailableInterrupt(interruptFrame* frame); // INT 7

static void handleDoubleFaultException(interruptFrame* frame, uint64 error);       // INT 8
static void handleCoprocessorSegmentOverrunInterrupt(interruptFrame* frame);       // INT 9
static void handleInvalidTSSException(interruptFrame* frame, uint64 error);        // INT 10
static void handleSegmentNotPresentException(interruptFrame* frame, uint64 error); // INT 11

static void handleStackSegmentFaultException(interruptFrame* frame, uint64 error); // INT 12
static void handleGeneralProtectionException(interruptFrame* frame, uint64 error); // INT 13
static void handlePageFaultException(interruptFrame* frame, uint64 error);         // INT 14

static void handleMathFaultInterrupt(interruptFrame* frame);                    // INT 16
static void handleAlignmentCheckException(interruptFrame* frame, uint64 error); // INT 17
static void handleMachineCheckInterrupt(interruptFrame* frame);                 // INT 18
static void handleSIMDFloatingPointExceptionInterrupt(interruptFrame* frame);   // INT 19

static void handleVirtualizationExceptionInterrupt(interruptFrame* frame);         // INT 20
static void handleControlProtectionException(interruptFrame* frame, uint64 error); // INT 21

static void handleTimerInterrupt(interruptFrame* frame);    // INT 32 (IRQ 0)
static void handleKeyboardInterrupt(interruptFrame* frame); // INT 33 (IRQ 1)
static void handleCascadeInterrupt(interruptFrame* frame);  // INT 34 (IRQ 2)
static void handleCOM2Interrupt(interruptFrame* frame);     // INT 35 (IRQ 3)

static void handleCOM1Interrupt(interruptFrame* frame);       // INT 36 (IRQ 4)
static void handleLPT2Interrupt(interruptFrame* frame);       // INT 37 (IRQ 5)
static void handleFloppyDiskInterrupt(interruptFrame* frame); // INT 38 (IRQ 6)
static void handleLPT1Interrupt(interruptFrame* frame);       // INT 39 (IRQ 7)

static void handleClockInterrupt(interruptFrame* frame); // INT 40 (IRQ 8)
static void handleIRQ9Interrupt(interruptFrame* frame);  // INT 41 (IRQ 9)
static void handleIRQ10Interrupt(interruptFrame* frame); // INT 42 (IRQ 10)
static void handleIRQ11Interrupt(interruptFrame* frame); // INT 43 (IRQ 11)

static void handlePS2MouseInterrupt(interruptFrame* frame);     // INT 44 (IRQ 12)
static void handleCoprocessorInterrupt(interruptFrame* frame);  // INT 45 (IRQ 13)
static void handlePrimaryATAInterrupt(interruptFrame* frame);   // INT 46 (IRQ 14)
static void handleSecondaryATAInterrupt(interruptFrame* frame); // INT 47 (IRQ 15)

static void handleSpuriousInterrupt(interruptFrame* frame); // INT 255

static void createInterruptGate(uint8 gate, uint8 privilege, void (*handler)(interruptFrame*));
static void createExceptionGate(uint8 gate, uint8 privilege, void (*handler)(interruptFrame*, uint64));
static void debugGate(uint8 gate);

void int_Init() {
	__asm__ ("cli");
	uint16 i;
	// List of interrupts is in table 6-1 in section 6.14.1
	// of Intel 64 and IA-32 Architectures Software Developer's
	// Manual, Volume 3A.
	for (i = 0; i < 256; i++) {
		createInterruptGate((uint8)i, 0, handleUnsupportedInterrupt);
	}

	createInterruptGate((uint8)0, 0, handleDivideException);     // INT 0
	createInterruptGate((uint8)1, 0, handleDebugException);      // INT 1
	createInterruptGate((uint8)2, 0, handleNMIInterrupt);        // INT 2
	createInterruptGate((uint8)3, 0, handleBreakpointInterrupt); // INT 3

	createInterruptGate((uint8)4, 0, handleOverflowInterrupt);           // INT 4
	createInterruptGate((uint8)5, 0, handleBoundRangeInterrupt);         // INT 5
	createInterruptGate((uint8)6, 0, handleInvalidOpcodeInterrupt);      // INT 6
	createInterruptGate((uint8)7, 0, handleDeviceNotAvailableInterrupt); // INT 7

	createExceptionGate((uint8)8, 0, handleDoubleFaultException);                // INT 8
	createInterruptGate((uint8)9, 0, handleCoprocessorSegmentOverrunInterrupt);  // INT 9
	createExceptionGate((uint8)10, 0, handleInvalidTSSException);                // INT 10
	createExceptionGate((uint8)11, 0, handleSegmentNotPresentException);         // INT 11

	createExceptionGate((uint8)12, 0, handleStackSegmentFaultException);         // INT 12
	createExceptionGate((uint8)13, 0, handleGeneralProtectionException);         // INT 13
	createExceptionGate((uint8)14, 0, handlePageFaultException);                 // INT 14

	createInterruptGate((uint8)16, 0, handleMathFaultInterrupt);                  // INT 16
	createExceptionGate((uint8)17, 0, handleAlignmentCheckException);             // INT 17
	createInterruptGate((uint8)18, 0, handleMachineCheckInterrupt);               // INT 18
	createInterruptGate((uint8)19, 0, handleSIMDFloatingPointExceptionInterrupt); // INT 19

	createInterruptGate((uint8)20, 0, handleVirtualizationExceptionInterrupt); // INT 20
	createExceptionGate((uint8)21, 0, handleControlProtectionException);       // INT 21

	createInterruptGate((uint8)32, 0, handleTimerInterrupt);    // INT 32 (IRQ 0)
	createInterruptGate((uint8)33, 0, handleKeyboardInterrupt); // INT 33 (IRQ 1)
	createInterruptGate((uint8)34, 0, handleCascadeInterrupt);  // INT 34 (IRQ 2)
	createInterruptGate((uint8)35, 0, handleCOM2Interrupt);     // INT 35 (IRQ 3)

	createInterruptGate((uint8)36, 0, handleCOM1Interrupt);       // INT 36 (IRQ 4)
	createInterruptGate((uint8)37, 0, handleLPT2Interrupt);       // INT 37 (IRQ 5)
	createInterruptGate((uint8)38, 0, handleFloppyDiskInterrupt); // INT 38 (IRQ 6)
	createInterruptGate((uint8)39, 0, handleLPT1Interrupt);       // INT 39 (IRQ 7)

	createInterruptGate((uint8)40, 0, handleClockInterrupt); // INT 40 (IRQ 8)
	createInterruptGate((uint8)41, 0, handleIRQ9Interrupt);  // INT 41 (IRQ 9)
	createInterruptGate((uint8)42, 0, handleIRQ10Interrupt); // INT 42 (IRQ 10)
	createInterruptGate((uint8)43, 0, handleIRQ11Interrupt); // INT 43 (IRQ 11)

	createInterruptGate((uint8)44, 0, handlePS2MouseInterrupt);     // INT 44 (IRQ 12)
	createInterruptGate((uint8)45, 0, handleCoprocessorInterrupt);  // INT 45 (IRQ 13)
	createInterruptGate((uint8)46, 0, handlePrimaryATAInterrupt);   // INT 46 (IRQ 14)
	createInterruptGate((uint8)47, 0, handleSecondaryATAInterrupt); // INT 47 (IRQ 15)

	createInterruptGate((uint8)255, 0, handleSpuriousInterrupt); // INT 255

	// Enable keyboard and clock IRQs only.
	port_Out8(0x21, 1);
	port_Out8(0xa1, 0);
	__asm__ ("sti");
}

__attribute__ ((interrupt))
void handleUnsupportedInterrupt(interruptFrame* frame) {
	std_Printk("unsupported interrupt:\n");
	std_Printk("  ip:    %u64x\n", frame->ip);
	std_Printk("  cs:    %u64x\n", frame->cs);
	std_Printk("  flags: %u64x\n", frame->flags);
	std_Printk("  sp:    %u64x\n", frame->sp);
	std_Printk("  ss:    %u64x\n", frame->ss);
}

// INT 0
__attribute__ ((interrupt))
void handleDivideException(interruptFrame* frame) {
	std_Printk("divide by zero exception\n");
	(void)frame;
}

// INT 1
__attribute__ ((interrupt))
void handleDebugException(interruptFrame* frame) {
	std_Printk("debug exception\n");
	(void)frame;
}

// INT 2
__attribute__ ((interrupt))
void handleNMIInterrupt(interruptFrame* frame) {
	std_Printk("NMI interrupt\n");
	(void)frame;
}

// INT 3
__attribute__ ((interrupt))
void handleBreakpointInterrupt(interruptFrame* frame) {
	std_Printk("breakpoint interrupt\n");
	(void)frame;
}

// INT 4
__attribute__ ((interrupt))
void handleOverflowInterrupt(interruptFrame* frame) {
	std_Printk("overflow interrupt\n");
	(void)frame;
}

// INT 5
__attribute__ ((interrupt))
void handleBoundRangeInterrupt(interruptFrame* frame) {
	std_Printk("bound range interrupt\n");
	(void)frame;
}

// INT 6
__attribute__ ((interrupt))
void handleInvalidOpcodeInterrupt(interruptFrame* frame) {
	std_Printk("invalid opcode interrupt\n");
	(void)frame;
}

// INT 7
__attribute__ ((interrupt))
void handleDeviceNotAvailableInterrupt(interruptFrame* frame) {
	std_Printk("device not available interrupt\n");
	(void)frame;
}

// INT 8
__attribute__ ((interrupt))
void handleDoubleFaultException(interruptFrame* frame, uint64 error) {
	std_Printk("double fault exception (error %u64x)\n", error);
	(void)frame;
}

// INT 9
__attribute__ ((interrupt))
void handleCoprocessorSegmentOverrunInterrupt(interruptFrame* frame) {
	std_Printk("coprocessor segment overrun interrupt\n");
	(void)frame;
}

// INT 10
__attribute__ ((interrupt))
void handleInvalidTSSException(interruptFrame* frame, uint64 error) {
	std_Printk("invalid TSS exception (error %u64x)\n", error);
	(void)frame;
}

// INT 11
__attribute__ ((interrupt))
void handleSegmentNotPresentException(interruptFrame* frame, uint64 error) {
	std_Printk("segment not present exception (error %u64x)\n", error);
	(void)frame;
}

// INT 12
__attribute__ ((interrupt))
void handleStackSegmentFaultException(interruptFrame* frame, uint64 error) {
	std_Printk("stack segment fault exception (error %u64x)\n", error);
	(void)frame;
}

// INT 13
__attribute__ ((interrupt))
void handleGeneralProtectionException(interruptFrame* frame, uint64 error) {
	std_Printk("general protection exception (error %u64x)\n", error);
	(void)frame;
}

// INT 14
__attribute__ ((interrupt))
void handlePageFaultException(interruptFrame* frame, uint64 error) {
	std_Printk("page fault exception (error %u64x)\n", error);
	(void)frame;
}

// INT 16
__attribute__ ((interrupt))
void handleMathFaultInterrupt(interruptFrame* frame) {
	std_Printk("math fault interrupt\n");
	(void)frame;
}

// INT 17
__attribute__ ((interrupt))
void handleAlignmentCheckException(interruptFrame* frame, uint64 error) {
	std_Printk("alignment check interrupt (error %u64x)\n", error);
	(void)frame;
}

// INT 18
__attribute__ ((interrupt))
void handleMachineCheckInterrupt(interruptFrame* frame) {
	std_Printk("machine check interrupt\n");
	(void)frame;
}

// INT 19
__attribute__ ((interrupt))
void handleSIMDFloatingPointExceptionInterrupt(interruptFrame* frame) {
	std_Printk("SIMD floating point exception interrupt\n");
	(void)frame;
}

// INT 20
__attribute__ ((interrupt))
void handleVirtualizationExceptionInterrupt(interruptFrame* frame) {
	std_Printk("virtualization exception interrupt\n");
	(void)frame;
}

// INT 21
__attribute__ ((interrupt))
void handleControlProtectionException(interruptFrame* frame, uint64 error) {
	std_Printk("control protection exception interrupt (error %u64x)\n", error);
	(void)frame;
}

// INT 32 (IRQ 0)
__attribute__ ((interrupt))
void handleTimerInterrupt(interruptFrame* frame) {
	std_Printk("timer interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	port_Out8(0x20, 0x20);
}

// INT 33 (IRQ 1)
__attribute__ ((interrupt))
void handleKeyboardInterrupt(interruptFrame* frame) {
	uint8 scanCode = port_In8(0x60);
	std_Printk("keyboard interrupt: %u8x\n", scanCode);
	(void)frame;

	// Fetch the code.
	(void)port_In8(0x60);

	// Acknowledge the IRQ.
	port_Out8(0x20, 0x20);
}

// INT 34 (IRQ 2)
__attribute__ ((interrupt))
void handleCascadeInterrupt(interruptFrame* frame) {
	std_Printk("cascade interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	port_Out8(0x20, 0x20);
}

// INT 35 (IRQ 3)
__attribute__ ((interrupt))
void handleCOM2Interrupt(interruptFrame* frame) {
	std_Printk("COM2 interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	port_Out8(0x20, 0x20);
}

// INT 36 (IRQ 4)
__attribute__ ((interrupt))
void handleCOM1Interrupt(interruptFrame* frame) {
	std_Printk("COM1 interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	port_Out8(0x20, 0x20);
}

// INT 37 (IRQ 5)
__attribute__ ((interrupt))
void handleLPT2Interrupt(interruptFrame* frame) {
	std_Printk("LPT2 interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	port_Out8(0x20, 0x20);
}

// INT 38 (IRQ 6)
__attribute__ ((interrupt))
void handleFloppyDiskInterrupt(interruptFrame* frame) {
	std_Printk("floppy disk interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	port_Out8(0x20, 0x20);
}

// INT 39 (IRQ 7)
__attribute__ ((interrupt))
void handleLPT1Interrupt(interruptFrame* frame) {
	std_Printk("LPT1 interrupt\n");
	(void)frame;

	// Ignore spurious interrupts.
	// https://wiki.osdev.org/IDT_problems#I_keep_getting_an_IRQ7_for_no_apparent_reason
	port_Out8(0x20, 0x0b);
	uint8 irr = port_In8(0x20);
	if (irr & 0x80) {
		// Acknowledge the IRQ.
		port_Out8(0x20, 0x20);
	}
}

// INT 40 (IRQ 8)
__attribute__ ((interrupt))
void handleClockInterrupt(interruptFrame* frame) {
	(void)frame;

	// Read the value.
	port_Out8(0x70, 0x0c);
	(void)port_In8(0x71);

	time_Ticker++;
	if (time_Ticker%1024 == 0) {
		std_Printk("\ruptime: %u64ds", time_Ticker/1024);
	}

	// Acknowledge the IRQ.
	port_Out8(0xa0, 0x20);
	port_Out8(0x20, 0x20);
}

// INT 41 (IRQ 9)
__attribute__ ((interrupt))
void handleIRQ9Interrupt(interruptFrame* frame) {
	std_Printk("IRQ 9 interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	port_Out8(0xa0, 0x20);
	port_Out8(0x20, 0x20);
}

// INT 42 (IRQ 10)
__attribute__ ((interrupt))
void handleIRQ10Interrupt(interruptFrame* frame) {
	std_Printk("IRQ 10 interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	port_Out8(0xa0, 0x20);
	port_Out8(0x20, 0x20);
}

// INT 43 (IRQ 11)
__attribute__ ((interrupt))
void handleIRQ11Interrupt(interruptFrame* frame) {
	std_Printk("IRQ 11 interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	port_Out8(0xa0, 0x20);
	port_Out8(0x20, 0x20);
}

// INT 44 (IRQ 12)
__attribute__ ((interrupt))
void handlePS2MouseInterrupt(interruptFrame* frame) {
	std_Printk("PS/2 mouse interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	port_Out8(0xa0, 0x20);
	port_Out8(0x20, 0x20);
}

// INT 45 (IRQ 13)
__attribute__ ((interrupt))
void handleCoprocessorInterrupt(interruptFrame* frame) {
	std_Printk("Coprocessor interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	port_Out8(0xa0, 0x20);
	port_Out8(0x20, 0x20);
}

// INT 46 (IRQ 14)
__attribute__ ((interrupt))
void handlePrimaryATAInterrupt(interruptFrame* frame) {
	std_Printk("primary ATA interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	port_Out8(0xa0, 0x20);
	port_Out8(0x20, 0x20);
}

// INT 47 (IRQ 15)
__attribute__ ((interrupt))
void handleSecondaryATAInterrupt(interruptFrame* frame) {
	std_Printk("secondary ATA interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	port_Out8(0xa0, 0x20);
	port_Out8(0x20, 0x20);
}

// INT 0xff
__attribute__ ((interrupt))
void handleSpuriousInterrupt(interruptFrame* frame) {
	std_Printk("spurious interrupt\n");
	(void)frame;
}

typedef struct {
	uint16 offset1;   // offset bits 0..15
	uint16 selector;  // a code segment selector in GDT or LDT
	uint8  ist;       // bits 0..2 holds Interrupt Stack Table offset, rest of bits zero.
	uint8  type_attr; // type and attributes
	uint16 offset2;   // offset bits 16..31
	uint32 offset3;   // offset bits 32..63
	uint32 zero;      // reserved
} idtDescriptor;

static const uint16 IDT_SELECTOR = 0x8;

static const uint8 IDT_FLAG_PRESENT = 1 << 7;
static const uint8 IDT_FLAG_ABSENT  = 0 << 7;

// See table 3-2 of Intel 64 and IA-32 Architectures Software
// Developer’s Manual, Volume 3A.
static const uint8 IDT_TYPE_LDT            = 2;  // bits 0010
static const uint8 IDT_TYPE_TSS_AVAILABLE  = 9;  // bits 1001
static const uint8 IDT_TYPE_TSS_BUSY       = 11; // bits 1011
static const uint8 IDT_TYPE_CALL_GATE      = 12; // bits 1100
static const uint8 IDT_TYPE_INTERRUPT_GATE = 14; // bits 1110
static const uint8 IDT_TYPE_TRAP_GATE      = 15; // bits 1111

static const uintptr MASK_BITS_31_TO_16 = 0xFFFF0000;
static const uintptr MASK_BITS_15_TO_0  = 0x0000FFFF;

// createInterruptGate stores an entry into the IDT, as described in
// section 6.14.1 of Intel 64 and IA-32 Architectures Software
// Developer’s Manual, Volume 3A:
//
// 	   3                   2                   1
// 	 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	|                            Reserved                           |
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	|                         Offset 63..32                         |
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	|         Offset 31..16         |P|DPL|0|  Type |0|0|0|0|0| IST |
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	|        Segment Selector       |          Offset 15..0         |
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// 	DPL:       Descriptor Privilege Level
// 	Offset:    Offset to procedure entry point
// 	P:         Segment Present flag
// 	Selector:  Segment Selector for destination code segment
// 	IST:       Interrupt Stack Table
//
void createInterruptGate(uint8 gate, uint8 privilege, void (*handler)(interruptFrame*)) {
	uintptr offset = (uintptr)handler;

	// Multiply the gate number by 16 to make it an
	// index into the IDT, which starts at 0x00.
	idtDescriptor* idte = (idtDescriptor*)(uintptr)((uint64)gate << 4);

	idte->zero = 0;
	idte->offset3 = (uint32)(offset >> 32);
	idte->offset2 = (uint16)((MASK_BITS_31_TO_16 & offset)>>16);
	idte->offset1 = (uint16)(MASK_BITS_15_TO_0 & offset);
	idte->selector = IDT_SELECTOR;
	idte->ist = 0;
	idte->type_attr = IDT_FLAG_PRESENT | (uint8)((3&privilege)<<5) | IDT_TYPE_INTERRUPT_GATE;
}

// createExceptionGate is the same as createInterruptGate, except
// it also receives an error value.
//
void createExceptionGate(uint8 gate, uint8 privilege, void (*handler)(interruptFrame*, uint64)) {
	uintptr offset = (uintptr)handler;

	// Multiply the gate number by 16 to make it an
	// index into the IDT, which starts at 0x00.
	idtDescriptor* idte = (idtDescriptor*)(uintptr)((uint64)gate << 4);

	idte->zero = 0;
	idte->offset3 = (uint32)(offset >> 32);
	idte->offset2 = (uint16)((MASK_BITS_31_TO_16 & offset)>>16);
	idte->offset1 = (uint16)(MASK_BITS_15_TO_0 & offset);
	idte->selector = IDT_SELECTOR;
	idte->ist = 0;
	idte->type_attr = IDT_FLAG_PRESENT | (uint8)((3&privilege)<<5) | IDT_TYPE_INTERRUPT_GATE;
}

void debugGate(uint8 gate) {
	// Multiply the gate number by 16 to make it an
	// index into the IDT, which starts at 0x00.
	idtDescriptor idte = *(idtDescriptor*)(uintptr)((uint64)gate << 4);
	uintptr ptr = ((uintptr)idte.offset1) | (((uintptr)idte.offset2)<<16) | (((uintptr)idte.offset3)<<32);

	std_Printk("interrupt handler %u8d:\n", gate);
	std_Printk("  offset:   %p\n", ptr);
	std_Printk("  selector: %u16b\n", idte.selector);
	std_Printk("  ist:      %u8b\n", idte.ist);
	std_Printk("  type:     %u8b\n", idte.type_attr);
	std_Printk("  reserved: %u32b\n", idte.zero);
}
