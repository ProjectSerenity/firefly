#include "std.h"

void badStr(void) {
	char* x = "asdf";
	string s = str(x);
	(void)s;
}