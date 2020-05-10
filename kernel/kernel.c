#include "std.h"
#include "cpu.h"
#include "int.h"
#include "mem.h"
#include "rand.h"
#include "term.h"
#include "time.h"

void kmain(void);

void kmain() {
	term_Init();
	uint64 x = 1;
	std_Printk("Hello, %m12s! Stack address: %p\n", "kernel world", &x);

	if (!std_ValidNumericalTypes()) {
		return;
	}

	std_Printk("Resolution: %u64d x %u64d\n", term_Width, term_Height);

	cpu_Init();
	cpu_Info info = cpu_GetInfo();
	std_Printk("CPU cores: %u64d, frequency: %u64dGHz\n", info.Cores, info.Frequency/(uint64)1000000000);
	std_Printk("RAM: %u64dMB\n", info.Memory/(((uint64)1)<<20));

	mem_Init();
	time_Init();
	rand_Init();
	int_Init();
}
