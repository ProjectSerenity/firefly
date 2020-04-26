#include "std.h"
#include "mmio.h"

uint8 mmio_ReadUint8(uintptr ptr) {
	return *((volatile uint8*)ptr);
}

uint16 mmio_ReadUint16(uintptr ptr) {
	return *((volatile uint16*)ptr);
}

uint32 mmio_ReadUint32(uintptr ptr) {
	return *((volatile uint32*)ptr);
}

uint64 mmio_ReadUint64(uintptr ptr) {
	return *((volatile uint64*)ptr);
}

void mmio_WriteUint8(uintptr ptr, uint8 v) {
	(*((volatile uint8*)ptr)) = v;
}

void mmio_WriteUint16(uintptr ptr, uint16 v) {
	(*((volatile uint16*)ptr)) = v;
}

void mmio_WriteUint32(uintptr ptr, uint32 v) {
	(*((volatile uint32*)ptr)) = v;
}

void mmio_WriteUint64(uintptr ptr, uint64 v) {
	(*((volatile uint64*)ptr)) = v;
}
