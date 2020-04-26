#include "std.h"
#include "ports.h"

uint8 ports_ReadUint8(uint16 port) {
	uint8 v;
	__asm__ volatile("inb %1, %0" : "=a" (v) : "dN" (port));
	return v;
}

uint16 ports_ReadUint16(uint16 port) {
	uint16 v;
	__asm__ volatile("inw %1, %0" : "=a" (v) : "dN" (port));
	return v;
}

uint32 ports_ReadUint32(uint16 port) {
	uint32 v;
	__asm__ volatile("inl %1, %0" : "=a" (v) : "dN" (port));
	return v;
}

void ports_WriteUint8(uint16 port, uint8 v) {
	__asm__ volatile ("outb %1, %0" : : "dN" (port), "a" (v));
}

void ports_WriteUint16(uint16 port, uint16 v) {
	__asm__ volatile ("outw %1, %0" : : "dN" (port), "a" (v));
}

void ports_WriteUint32(uint16 port, uint32 v) {
	__asm__ volatile ("outl %1, %0" : : "dN" (port), "a" (v));
}
