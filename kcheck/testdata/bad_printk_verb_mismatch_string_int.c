#include "std.h"

void badPrintk(void) {
	printk("%m1s", 1);
}
