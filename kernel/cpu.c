#include "std.h"
#include "cpu.h"

cpu_Info cpu_GetInfo() {
	cpu_Info info = {
		.Cores = *(uint16*)0x5012,
		.Frequency = ((uint64)1000000) * (uint64)(*(uint16*)0x5010), // MHz to Hz.
		.Memory = (((uint64)1)<<20) * (uint64)(*(uint32*)0x5020), // MiB to B.
	};

	return info;
}
