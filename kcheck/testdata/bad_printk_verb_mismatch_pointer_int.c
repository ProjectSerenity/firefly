#include "std.h"

void badPrintk(void) {
	printk("%p", 1);
}
