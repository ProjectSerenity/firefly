#pragma once
#ifndef TERMINAL_H
#define TERMINAL_H

uint16 terminal_width;
uint16 terminal_height;

uint32 rgb(uint8 red, uint8 green, uint8 blue);

void terminal_Init(void);
void terminal_PixelAt(uint x, uint y, uint32 color);
void terminal_CharAt(char c, uint32 color, uint x, uint y);
void terminal_SetColor(uint32 color);
uint terminal_Write(const char* data, uint size);
uint terminal_WriteChar(char c);
uint terminal_WriteCharAt(char c, uint32 color, uint x, uint y);
uint terminal_WriteString(string s);
uint terminal_WriteError(string s);

#endif // TERMINAL_H
