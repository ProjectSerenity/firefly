#include "../../kernel/std.h"

void badPrintk(void) {
	char* x = "foo";
	std_Printk("%u8d", x);
}
