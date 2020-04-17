#include "std.h"

void badPrintk(void) {
	printk("%u8d", (uint64)0xfffffffffffffff);
}
