#include "terminal.h"

void kmain(void) {
	terminal_Init();
	terminal_WriteCString("Hello, kernel World!\n");
	terminal_WriteCError("This is an error!\n");
	terminal_WriteCString("Back to normal again.\n");
}
