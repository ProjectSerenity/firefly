#include "std.h"

void badPrintk(void) {
	uint64 x = 1;
	printk("%u8d", x);
}
