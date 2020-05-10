#include "std.h"
#include "cpu.h"
#include "rand.h"

uint64 rand_Retries;

static bool rand_read8(uint8* buf);
static bool rand_read16(uint16* buf);
static bool rand_read32(uint32* buf);
static bool rand_read64(uint64* buf);

void rand_Init() {
	rand_Retries = 16;
	if (!cpu_IsIntel) {
		std_Printk("WARNING: not intel CPU\n");
		return;
	}

	cpu_ID info;
	cpu_GetID(&info, 1, 0);
	if ((info.ecx & 0x40000000) == 0) {
		std_Printk("WARNING: no RDRAND support\n");
		return;
	}
}

// rand_Read copies len bytes of random
// data into buf. The number of bytes
// copied is returned.
//
uint64 rand_Read(uint8* buf, uint64 len) {
	uint64 i, tempRand;
	uint8* headStart;
	uint8* tailStart = 0;
	uint64* blockStart;
	uint64 count, ltail, lhead, lblock;

	// See https://software.intel.com/content/www/us/en/develop/articles/intel-digital-random-number-generator-drng-software-implementation-guide.html
	// section 5.2.4.

	// Get the address of the first 64-bit
	// aligned block in the destination
	// buffer.

	headStart = buf;
	if (((uintptr)headStart % (uintptr)8) == 0) {
		blockStart = (uint64*)(void*)headStart;
		lblock = len;
		lhead = 0;
	} else {
		blockStart = (uint64*)(((uintptr)headStart & ~(uintptr)7) + (uintptr)8);
		lblock = len - (8 - (uint64)((uintptr)headStart & (uintptr)8));
		lhead = (uint64)((uintptr)blockStart - (uintptr)headStart);
	}

	// Compute the number of 64-bit blocks
	// and the remaining number of bytes
	// (the tail).

	ltail = len - lblock - lhead;
	count = lblock / 8; // The number of 64-bit reads needed.

	if (ltail) {
		tailStart = (uint8*)((uintptr)blockStart + (uintptr)lblock);
	}

	// Populate the starting, mis-aligned
	// section (the head).

	if (lhead) {
		if (!rand_read64(&tempRand)) {
			return 0;
		}

		std_Copy(headStart, (uint8*)&tempRand, lhead);
	}

	// Populate the central, aligned block.

	for (i = 0; i < count; i++, blockStart++) {
		if (!rand_read64(blockStart)) {
			return i*8 + lhead;
		}
	}

	// Populate the tail.

	if (ltail) {
		if (!rand_read64(&tempRand)) {
			return count*8 + lhead;
		}

		std_Copy(tailStart, (uint8*)&tempRand, ltail);
	}

	return len;
}

bool rand_read8(uint8* buf) {
	uint16 v = 0;
	bool ok = rand_read16(&v);
	if (ok) {
		*buf = (uint8)(0xff & v);
	}

	return ok;
}

bool rand_read16(uint16* buf) {
	uint64 tries;
	for (tries = 0; tries < rand_Retries; tries++) {
		uint8 ok;
		__asm__ volatile ("rdrand %0; setc %1" : "=r" (*buf), "=qm" (ok));
		if (ok) {
			return true;
		}
	}

	return false;
}

bool rand_read32(uint32* buf) {
	uint64 tries;
	for (tries = 0; tries < rand_Retries; tries++) {
		uint8 ok;
		__asm__ volatile ("rdrand %0; setc %1" : "=r" (*buf), "=qm" (ok));
		if (ok) {
			return true;
		}
	}

	return false;
}

bool rand_read64(uint64* buf) {
	uint64 tries;
	for (tries = 0; tries < rand_Retries; tries++) {
		uint8 ok;
		__asm__ volatile ("rdrand %0; setc %1" : "=r" (*buf), "=qm" (ok));
		if (ok) {
			return true;
		}
	}

	return false;
}
