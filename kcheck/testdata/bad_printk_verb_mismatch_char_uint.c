#include "../../kernel/std.h"

void badPrintk(void) {
	uint8 x = 1;
	std_Printk("%c", x);
}
