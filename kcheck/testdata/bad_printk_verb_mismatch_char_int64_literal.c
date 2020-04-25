#include "std.h"

void badPrintk(void) {
	std_Printk("%c", (int64)0xffffffffffffff);
}
