#include "std.h"

void badPrintk(void) {
	std_Printk("%m1s", 1);
}
