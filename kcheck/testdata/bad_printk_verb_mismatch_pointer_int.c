#include "../../kernel/std.h"

void badPrintk(void) {
	std_Printk("%p", 1);
}
