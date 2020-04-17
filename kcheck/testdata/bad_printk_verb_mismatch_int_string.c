#include "std.h"

void badPrintk(void) {
	char* x = "foo";
	printk("%u8d", x);
}
