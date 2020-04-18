#include "std.h"
#include "terminal.h"

void kmain(void) {
	terminal_Init();
	int x = 1;
	printk("Hello, %m12s! Stack address: %p\n", "kernel world", &x);

	printk("   int8: %w2u8d\n", (uint8)(8*sizeof(int8)));
	printk("  int16: %w2u8d\n", (uint8)(8*sizeof(int16)));
	printk("  int32: %w2u8d\n", (uint8)(8*sizeof(int32)));
	printk("  int64: %w2u8d\n", (uint8)(8*sizeof(int64)));
	printk("\n");
	printk("  uint8: %w2u8d\n", (uint8)(8*sizeof(uint8)));
	printk(" uint16: %w2u8d\n", (uint8)(8*sizeof(uint16)));
	printk(" uint32: %w2u8d\n", (uint8)(8*sizeof(uint32)));
	printk(" uint64: %w2u8d\n", (uint8)(8*sizeof(uint64)));
	printk("\n");
	printk("    int: %w2u8d\n", (uint8)(8*sizeof(int)));
	printk("   uint: %w2u8d\n", (uint8)(8*sizeof(uint)));
	printk("uintptr: %w2u8d\n", (uint8)(8*sizeof(uintptr)));
}
