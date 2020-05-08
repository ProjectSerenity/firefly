#include "../../kernel/std.h"

void badPrintk(void) {
	char* format = "";
	std_Printk(format);
}