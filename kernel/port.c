#include "std.h"
#include "port.h"

uint8 port_In8(uint16 port) {
	uint8 v;
	__asm__ volatile("inb %1, %0" : "=a" (v) : "dN" (port));
	return v;
}

uint16 port_In16(uint16 port) {
	uint16 v;
	__asm__ volatile("inw %1, %0" : "=a" (v) : "dN" (port));
	return v;
}

uint32 port_In32(uint16 port) {
	uint32 v;
	__asm__ volatile("inl %1, %0" : "=a" (v) : "dN" (port));
	return v;
}

void port_Out8(uint16 port, uint8 v) {
	__asm__ volatile ("outb %1, %0" : : "dN" (port), "a" (v));
}

void port_Out16(uint16 port, uint16 v) {
	__asm__ volatile ("outw %1, %0" : : "dN" (port), "a" (v));
}

void port_Out32(uint16 port, uint32 v) {
	__asm__ volatile ("outl %1, %0" : : "dN" (port), "a" (v));
}
