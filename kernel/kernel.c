#include "std.h"
#include "cpu.h"
#include "mem.h"
#include "terminal.h"

void kmain(void);

void kmain() {
	terminal_Init();
	int x = 1;
	std_Printk("Hello, %m12s! Stack address: %p\n", "kernel world", &x);

	if (!std_ValidNumericalTypes()) {
		return;
	}

	std_Printk("Numerical types as expected\n");

	std_Printk("Resolution: %u64d x %u64d\n", terminal_Width, terminal_Height);

	cpu_Info info = cpu_GetInfo();
	std_Printk("CPU cores: %u16d, frequency: %u64dGHz\n", info.Cores, info.Frequency/(uint64)1000000000);
	std_Printk("RAM: %u64dMB\n", info.Memory/(((uint64)1)<<20));

	mem_Init();
	mem_DebugPaging(10);
}
