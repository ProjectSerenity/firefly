#pragma once
#ifndef MMIO_H
#define MMIO_H

uint8 mmio_ReadUint8(uintptr ptr);
uint16 mmio_ReadUint16(uintptr ptr);
uint32 mmio_ReadUint32(uintptr ptr);
uint64 mmio_ReadUint64(uintptr ptr);
void mmio_WriteUint8(uintptr ptr, uint8 v);
void mmio_WriteUint16(uintptr ptr, uint16 v);
void mmio_WriteUint32(uintptr ptr, uint32 v);
void mmio_WriteUint64(uintptr ptr, uint64 v);

#endif // MMIO_H
