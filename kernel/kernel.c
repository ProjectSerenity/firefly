#include "std.h"
#include "terminal.h"

void kmain(void) {
	terminal_Init();
	int x = 1;
	printk("Hello, %m12s! Stack address: %p\n", "kernel world", &x);

	if (!validNumericalTypes()) {
		return;
	}

	printk("Numerical types as expected\n");
}
