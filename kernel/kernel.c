#include "terminal.h"

void kmain(void) {
	terminal_Init();
	terminal_WriteString(newString("Hello, kernel World!\n"));
}
