/* The entry point. */
ENTRY(kinit)

/* Declare where each section of the object files will */
/* be put in the final kernel image. */
SECTIONS {
	/* It's conventional to start sections at 1 MiB. */
	. = 1M;

	/* Start with the multiboot header and the text */
	/* section. */
	.text BLOCK(4K) : ALIGN(4K) {
		*(.multiboot)
		*(.text)
	}

	/* Then read-only data. */
	.rodata BLOCK(4K) : ALIGN(4K) {
		*(.rodata)
	}

	/* Then initialized read-write data. */
	.data BLOCK(4K) : ALIGN(4K) {
		*(.data)
	}

	/* Then uninitialized read-write data, including the stack. */
	.bss BLOCK(4K) : ALIGN(4K) {
		*(COMMON)
		*(.bss)
	}
}
