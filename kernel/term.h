#pragma once
#ifndef TERM_H
#define TERM_H

extern uint64 term_Width;
extern uint64 term_Height;

uint32 term_RGB(uint8 red, uint8 green, uint8 blue);

void term_Init(void);
void term_PixelAt(uint64 x, uint64 y, uint32 color);
void term_CharAt(char c, uint32 color, uint64 x, uint64 y);
uint32 term_GetColor(void);
void term_SetColor(uint32 color);
uint64 term_Write(const char* data, uint64 size);
uint64 term_WriteChar(char c);
uint64 term_WriteCharAt(char c, uint32 color, uint64 x, uint64 y);
uint64 term_WriteString(string s);
uint64 term_WriteError(string s);

#endif // TERM_H
