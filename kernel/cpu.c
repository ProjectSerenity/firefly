#include "std.h"
#include "cpu.h"

bool cpu_IsIntel;
char cpu_Label[12];

void cpu_Init() {
	cpu_ID info;
	cpu_IsIntel = true;
	cpu_GetID(&info, 0, 0);
	cpu_Label[0x0] = (char)((0x000000ff & info.ebx));
	cpu_Label[0x1] = (char)((0x0000ff00 & info.ebx)>>8);
	cpu_Label[0x2] = (char)((0x00ff0000 & info.ebx)>>16);
	cpu_Label[0x3] = (char)((0xff000000 & info.ebx)>>24);
	cpu_Label[0x4] = (char)((0x000000ff & info.edx));
	cpu_Label[0x5] = (char)((0x0000ff00 & info.edx)>>8);
	cpu_Label[0x6] = (char)((0x00ff0000 & info.edx)>>16);
	cpu_Label[0x7] = (char)((0xff000000 & info.edx)>>24);
	cpu_Label[0x8] = (char)((0x000000ff & info.ecx));
	cpu_Label[0x9] = (char)((0x0000ff00 & info.ecx)>>8);
	cpu_Label[0xa] = (char)((0x00ff0000 & info.ecx)>>16);
	cpu_Label[0xb] = (char)((0xff000000 & info.ecx)>>24);
	if (!std_Equal((uint8*)cpu_Label, (uint8*)"GenuineIntel", 12)) {
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
