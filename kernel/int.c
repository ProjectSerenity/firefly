#include "std.h"
#include "int.h"
#include "ports.h"

typedef struct {
	uintptr ip;
	uint64 cs;
	uint64 flags;
	uintptr sp;
	uint64 ss;
} interruptFrame;

static void handleUnsupportedInterrupt(interruptFrame* frame);
static void handleDivideException(interruptFrame* frame);
static void handleDebugException(interruptFrame* frame);
static void handleNMIInterrupt(interruptFrame* frame);
static void handleBreakpointInterrupt(interruptFrame* frame);
static void handleOverflowInterrupt(interruptFrame* frame);
static void handleBoundRangeInterrupt(interruptFrame* frame);
static void handleInvalidOpcodeInterrupt(interruptFrame* frame);
static void handleDeviceNotAvailableInterrupt(interruptFrame* frame);
static void handleDoubleFaultException(interruptFrame* frame, uint64 error);
static void handleCoprocessorSegmentOverrunInterrupt(interruptFrame* frame);
static void handleInvalidTSSException(interruptFrame* frame, uint64 error);
static void handleSegmentNotPresentException(interruptFrame* frame, uint64 error);
static void handleStackSegmentFaultException(interruptFrame* frame, uint64 error);
static void handleGeneralProtectionException(interruptFrame* frame, uint64 error);
static void handlePageFaultException(interruptFrame* frame, uint64 error);
static void handleMathFaultInterrupt(interruptFrame* frame);
static void handleAlignmentCheckException(interruptFrame* frame, uint64 error);
static void handleMachineCheckInterrupt(interruptFrame* frame);
static void handleSIMDFloatingPointExceptionInterrupt(interruptFrame* frame);
static void handleVirtualizationExceptionInterrupt(interruptFrame* frame);
static void handleControlProtectionException(interruptFrame* frame, uint64 error);
static void handleKeyboardInterrupt(interruptFrame* frame);
static void handleCascadeInterrupt(interruptFrame* frame);
static void handleClockInterrupt(interruptFrame* frame);
static void handleSpuriousInterrupt(interruptFrame* frame);

static void createInterruptGate(uint8 gate, uint8 privilege, void (*handler)(interruptFrame*));
static void createExceptionGate(uint8 gate, uint8 privilege, void (*handler)(interruptFrame*, uint64));
static void debugGate(uint8 gate);

void int_Init() {
	__asm__ ("cli");
	uint16 i;
	for (i = 0; i < 256; i++) {
		switch (i) {
		case 0:
			createInterruptGate((uint8)i, 0, handleDivideException);
			continue;
		case 1:
			createInterruptGate((uint8)i, 0, handleDebugException);
			continue;
		case 2:
			createInterruptGate((uint8)i, 0, handleNMIInterrupt);
			continue;
		case 3:
			createInterruptGate((uint8)i, 0, handleBreakpointInterrupt);
			continue;
		case 4:
			createInterruptGate((uint8)i, 0, handleOverflowInterrupt);
			continue;
		case 5:
			createInterruptGate((uint8)i, 0, handleBoundRangeInterrupt);
			continue;
		case 6:
			createInterruptGate((uint8)i, 0, handleInvalidOpcodeInterrupt);
			continue;
		case 7:
			createInterruptGate((uint8)i, 0, handleDeviceNotAvailableInterrupt);
			continue;
		case 8:
			createExceptionGate((uint8)i, 0, handleDoubleFaultException);
			continue;
		case 9:
			createInterruptGate((uint8)i, 0, handleCoprocessorSegmentOverrunInterrupt);
			continue;
		case 10:
			createExceptionGate((uint8)i, 0, handleInvalidTSSException);
			continue;
		case 11:
			createExceptionGate((uint8)i, 0, handleSegmentNotPresentException);
			continue;
		case 12:
			createExceptionGate((uint8)i, 0, handleStackSegmentFaultException);
			continue;
		case 13:
			createExceptionGate((uint8)i, 0, handleGeneralProtectionException);
			continue;
		case 14:
			createExceptionGate((uint8)i, 0, handlePageFaultException);
			continue;
		case 16:
			createInterruptGate((uint8)i, 0, handleMathFaultInterrupt);
			continue;
		case 17:
			createExceptionGate((uint8)i, 0, handleAlignmentCheckException);
			continue;
		case 18:
			createInterruptGate((uint8)i, 0, handleMachineCheckInterrupt);
			continue;
		case 19:
			createInterruptGate((uint8)i, 0, handleSIMDFloatingPointExceptionInterrupt);
			continue;
		case 20:
			createInterruptGate((uint8)i, 0, handleVirtualizationExceptionInterrupt);
			continue;
		case 21:
			createExceptionGate((uint8)i, 0, handleControlProtectionException);
			continue;
		case 33:
			createInterruptGate((uint8)i, 0, handleKeyboardInterrupt);
			continue;
		case 34:
			createInterruptGate((uint8)i, 0, handleCascadeInterrupt);
			continue;
		case 40:
			createInterruptGate((uint8)i, 0, handleClockInterrupt);
			continue;
		case 0xff:
			createInterruptGate((uint8)i, 0, handleSpuriousInterrupt);
			continue;
		default:
			//createInterruptGate((uint8)i, 0, handleUnsupportedInterrupt);
			continue;
		}
	}

	// Enable keyboard IRQs only.
	ports_WriteUint8(0x21, 0xfd);
	ports_WriteUint8(0xa1, 0xff);
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

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 0
__attribute__ ((interrupt))
void handleDivideException(interruptFrame* frame) {
	std_Printk("divide by zero exception\n");
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 1
__attribute__ ((interrupt))
void handleDebugException(interruptFrame* frame) {
	std_Printk("debug exception\n");
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 2
__attribute__ ((interrupt))
void handleNMIInterrupt(interruptFrame* frame) {
	std_Printk("NMI interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 3
__attribute__ ((interrupt))
void handleBreakpointInterrupt(interruptFrame* frame) {
	std_Printk("breakpoint interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 4
__attribute__ ((interrupt))
void handleOverflowInterrupt(interruptFrame* frame) {
	std_Printk("overflow interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 5
__attribute__ ((interrupt))
void handleBoundRangeInterrupt(interruptFrame* frame) {
	std_Printk("bound range interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 6
__attribute__ ((interrupt))
void handleInvalidOpcodeInterrupt(interruptFrame* frame) {
	std_Printk("invalid opcode interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 7
__attribute__ ((interrupt))
void handleDeviceNotAvailableInterrupt(interruptFrame* frame) {
	std_Printk("device not available interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 8
__attribute__ ((interrupt))
void handleDoubleFaultException(interruptFrame* frame, uint64 error) {
	std_Printk("double fault exception (error %u64x)\n", error);
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 9
__attribute__ ((interrupt))
void handleCoprocessorSegmentOverrunInterrupt(interruptFrame* frame) {
	std_Printk("coprocessor segment overrun interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 10
__attribute__ ((interrupt))
void handleInvalidTSSException(interruptFrame* frame, uint64 error) {
	std_Printk("invalid TSS exception (error %u64x)\n", error);
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 11
__attribute__ ((interrupt))
void handleSegmentNotPresentException(interruptFrame* frame, uint64 error) {
	std_Printk("segment not present exception (error %u64x)\n", error);
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 12
__attribute__ ((interrupt))
void handleStackSegmentFaultException(interruptFrame* frame, uint64 error) {
	std_Printk("stack segment fault exception (error %u64x)\n", error);
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 13
__attribute__ ((interrupt))
void handleGeneralProtectionException(interruptFrame* frame, uint64 error) {
	std_Printk("general protection exception (error %u64x)\n", error);
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 14
__attribute__ ((interrupt))
void handlePageFaultException(interruptFrame* frame, uint64 error) {
	std_Printk("page fault exception (error %u64x)\n", error);
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 16
__attribute__ ((interrupt))
void handleMathFaultInterrupt(interruptFrame* frame) {
	std_Printk("math fault interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 17
__attribute__ ((interrupt))
void handleAlignmentCheckException(interruptFrame* frame, uint64 error) {
	std_Printk("alignment check interrupt\n");
	(void)frame;
	(void)error;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 18
__attribute__ ((interrupt))
void handleMachineCheckInterrupt(interruptFrame* frame) {
	std_Printk("machine check interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 19
__attribute__ ((interrupt))
void handleSIMDFloatingPointExceptionInterrupt(interruptFrame* frame) {
	std_Printk("SIMD floating point exception interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 20
__attribute__ ((interrupt))
void handleVirtualizationExceptionInterrupt(interruptFrame* frame) {
	std_Printk("virtualization exception interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 21
__attribute__ ((interrupt))
void handleControlProtectionException(interruptFrame* frame, uint64 error) {
	std_Printk("control protection exception interrupt (error %u64x)\n", error);
	(void)frame;
	(void)error;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 33 (IRQ 1)
__attribute__ ((interrupt))
void handleKeyboardInterrupt(interruptFrame* frame) {
	uint8 scanCode = ports_ReadUint8(0x60);
	std_Printk("keyboard interrupt: %u8x\n", scanCode);
	(void)frame;

	// Fetch the code.
	(void)ports_ReadUint8(0x60);

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 34 (IRQ 2)
__attribute__ ((interrupt))
void handleCascadeInterrupt(interruptFrame* frame) {
	std_Printk("cascade interrupt\n");
	(void)frame;

	// Acknowledge the IRQ.
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 40 (IRQ 8)
__attribute__ ((interrupt))
void handleClockInterrupt(interruptFrame* frame) {
	std_Printk("clock interrupt at %p\n", frame->ip);
	(void)frame;

	// Read the value.
	ports_WriteUint8(0x70, 0x0c);
	(void)ports_ReadUint8(0x71);

	// Acknowledge the IRQ.
	ports_WriteUint8(0xa0, 0x20);
	ports_WriteUint8(0x20, 0x20);

	__asm__ ("cli");
}

// INT 0xff
__attribute__ ((interrupt))
void handleSpuriousInterrupt(interruptFrame* frame) {
	std_Printk("spurious interrupt\n");
	(void)frame;

	__asm__ ("cli");
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
