#include "std.h"

void badPrintk(void) {
	printk("%m1x", 1);
}
