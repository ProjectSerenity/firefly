#include "../../kernel/std.h"

void validPrintk(void) {
	char* foo = "foo";
	std_Printk("happy text %+64d (%m3s)\n", 1, foo);
}