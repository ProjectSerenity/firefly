#pragma once
#ifndef TERMINAL_H
#define TERMINAL_H

uint64 terminal_Width;
uint64 terminal_Height;

uint32 terminal_RGB(uint8 red, uint8 green, uint8 blue);

void terminal_Init(void);
void terminal_PixelAt(uint64 x, uint64 y, uint32 color);
void terminal_CharAt(char c, uint32 color, uint64 x, uint64 y);
uint32 terminal_GetColor();
void terminal_SetColor(uint32 color);
uint64 terminal_Write(const char* data, uint64 size);
uint64 terminal_WriteChar(char c);
uint64 terminal_WriteCharAt(char c, uint32 color, uint64 x, uint64 y);
uint64 terminal_WriteString(string s);
uint64 terminal_WriteError(string s);

#endif // TERMINAL_H
