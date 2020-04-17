#include "std.h"

void badPrintk(void) {
	uint8 x = 1;
	printk("%+8d", x);
}
