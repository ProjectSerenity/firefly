#include "std.h"
#include "terminal.h"

void kmain(void) {
	terminal_Init();
	int x = 1;
	std_Printk("Hello, %m12s! Stack address: %p\n", "kernel world", &x);

	if (!std_ValidNumericalTypes()) {
		return;
	}

	std_Printk("Numerical types as expected\n");
	std_Printk("Resolution: %u64d x %u64d\n", terminal_Width, terminal_Height);
}
