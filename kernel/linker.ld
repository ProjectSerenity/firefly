OUTPUT_FORMAT("binary")
OUTPUT_ARCH("i386:x86-64")

SECTIONS {
	. = 0x100000;
	.text : {
		*(.text)
	}
	.data : {
		*(.data)
	}
	.rodata : {
		*(.rodata)
	}
	.bss : {
		*(.bss)
	}
}
