#include "std.h"

void badPrintk(void) {
	int x = 1;
	printk("%m1s", &x);
}
