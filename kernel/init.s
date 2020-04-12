// Declare constants for the multiboot header.
.set ALIGN,    1<<0             // Align loaded modules to page boundaries.
.set MEMINFO,  1<<1             // Provide a memory map.
.set FLAGS,    ALIGN | MEMINFO  // Declare the multiboot flag field.
.set MAGIC,    0x1BADB002       // Declare the multiboot magic number.
.set CHECKSUM, -(MAGIC + FLAGS) // Declare the multiboot checksum.

// Start by recording the mulitboot header.
.section .multiboot
.align 4
.long MAGIC
.long FLAGS
.long CHECKSUM

// Prepare the stack.
.section .bss
.align 16
stack_bottom:
.skip 16384    // 16 KiB
stack_top:

// Start the kernel.
.section .text
.global start
.type start, @function
start:
	// The bootloader has loaded us in 32-bit
	// protected mode. Interrupts and paging
	// are disabled.

	mov $stack_top, %esp // Set the stack pointer.
	call kmain           // Start the kernel.

	// If the kernel returns, we halt the
	// CPU forever.
	cli                  // Disable interrupts again.
1:	hlt                  // Halt the CPU.
	jmp 1b               // Re-halt if interrupted.

// Set the size of start.
.size start, . - start
