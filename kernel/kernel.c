// +build ignore

#include "std.h"
#include "terminal.h"

void kmain(void) {
	terminal_Init();
	printk("Hello, %m12s! (%%+8d=%+8d, %%u8d=%u8d)\n", "kernel world", 255, 255);
}
