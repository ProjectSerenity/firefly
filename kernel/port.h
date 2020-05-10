#pragma once
#ifndef PORT_H
#define PORT_H

// Functionality for reading from and writing
// to hardware ports.

uint8 port_In8(uint16 port);
uint16 port_In16(uint16 port);
uint32 port_In32(uint16 port);
void port_Out8(uint16 port, uint8 v);
void port_Out16(uint16 port, uint16 v);
void port_Out32(uint16 port, uint32 v);

#endif // PORT_H
