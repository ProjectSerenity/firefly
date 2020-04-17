#include "std.h"

void badPrintk(void) {
	printk("%c", (int64)0xffffffffffffff);
}
