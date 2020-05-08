#include "../../kernel/std.h"

void badPrintk(void) {
	std_Printk("%u8d", (uint64)0xfffffffffffffff);
}
