#include "std.h"

void badPrintk(void) {
	int8 x = 1;
	printk("%u8d", x);
}
