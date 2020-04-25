#include "std.h"

void badPrintk(void) {
	char* x = "foo";
	std_Printk("%c", x);
}
