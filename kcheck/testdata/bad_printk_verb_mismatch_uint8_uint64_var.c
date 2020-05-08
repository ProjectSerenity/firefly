#include "../../kernel/std.h"

void badPrintk(void) {
	uint64 x = 1;
	std_Printk("%u8d", x);
}
