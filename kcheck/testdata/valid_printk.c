#include "std.h"

void validPrintk(void) {
	char* foo = "foo";
	printk("happy text %+64d (%m3s)\n", 1, foo);
}