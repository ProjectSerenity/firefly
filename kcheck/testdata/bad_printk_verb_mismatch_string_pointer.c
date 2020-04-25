#include "std.h"

void badPrintk(void) {
	int x = 1;
	std_Printk("%m1s", &x);
}
