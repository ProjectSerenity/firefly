#include "terminal.h"

void kmain(void) {
	terminal_Init();
	terminal_WriteString(S("Hello, kernel World!\n"));
}
