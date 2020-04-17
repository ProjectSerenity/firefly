#include "std.h"

void badPrintk(void) {
	char* x = "foo";
	printk("%c", x);
}
