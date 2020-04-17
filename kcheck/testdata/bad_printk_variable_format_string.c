#include "std.h"

void badPrintk(void) {
	char* format = "";
	printk(format);
}