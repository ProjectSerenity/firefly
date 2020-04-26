#pragma once
#ifndef PORTS_H
#define PORTS_H

// Functionality for reading from and writing
// to hardware ports.

uint8 ports_ReadUint8(uint16 port);
uint16 ports_ReadUint16(uint16 port);
uint32 ports_ReadUint32(uint16 port);
void ports_WriteUint8(uint16 port, uint8 v);
void ports_WriteUint16(uint16 port, uint16 v);
void ports_WriteUint32(uint16 port, uint32 v);

#endif // PORTS_H
