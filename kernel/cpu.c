#include "std.h"
#include "cpu.h"

bool cpu_IsIntel;

void cpu_Init() {
	cpu_ID info;
	cpu_IsIntel = true;
	cpu_GetID(&info, 0, 0);
	if (!std_Equal((uint8*)&info.ebx, (uint8*)"Genu", 4) ||
		!std_Equal((uint8*)&info.edx, (uint8*)"ineI", 4) ||
		!std_Equal((uint8*)&info.ecx, (uint8*)"ntel", 4)) {
		cpu_IsIntel = false;
	}
}

cpu_Info cpu_GetInfo() {
	cpu_Info info = {
		.Cores = (uint64)*(uint16*)0x5012,
		.Frequency = ((uint64)1000000) * (uint64)(*(uint16*)0x5010), // MHz to Hz.
		.Memory = (((uint64)1)<<20) * (uint64)(*(uint32*)0x5020), // MiB to B.
	};

	return info;
}

void cpu_GetID(cpu_ID* info, uint32 leaf, uint32 subleaf) {
	__asm__ volatile ("cpuid" : "=a" (info->eax), "=b" (info->ebx), "=c" (info->ecx), "=d" (info->edx) : "a" (leaf), "c" (subleaf));
}
