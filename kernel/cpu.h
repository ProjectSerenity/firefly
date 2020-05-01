#pragma once
#ifndef CPU_H
#define CPU_H

typedef struct {
	uint16 Cores;
	uint64 Frequency;
	uint64 Memory;
} cpu_Info;

cpu_Info cpu_GetInfo(void);

#endif // CPU_H
