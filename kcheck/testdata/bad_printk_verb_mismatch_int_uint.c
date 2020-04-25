#include "std.h"

void badPrintk(void) {
	int8 x = 1;
	std_Printk("%u8d", x);
}
