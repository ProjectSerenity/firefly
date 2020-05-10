#pragma once
#ifndef CPU_H
#define CPU_H

extern bool cpu_IsIntel;
extern char cpu_Label[12];

typedef struct {
	uint64 Cores;
	uint64 Frequency;
	uint64 Memory;
} cpu_Info;

cpu_Info cpu_GetInfo(void);

typedef struct {
	uint32 eax;
	uint32 ebx;
	uint32 ecx;
	uint32 edx;
} cpu_ID;

void cpu_GetID(cpu_ID* info, uint32 leaf, uint32 subleaf);

void cpu_Init(void);

#endif // CPU_H
