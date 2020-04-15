#pragma once
#ifndef TERMINAL_H
#define TERMINAL_H

#include "std.h"

// Hardware text mode color constants.
enum vga_color {
	VGA_COLOR_BLACK = 0,
	VGA_COLOR_BLUE = 1,
	VGA_COLOR_GREEN = 2,
	VGA_COLOR_CYAN = 3,
	VGA_COLOR_RED = 4,
	VGA_COLOR_MAGENTA = 5,
	VGA_COLOR_BROWN = 6,
	VGA_COLOR_LIGHT_GREY = 7,
	VGA_COLOR_DARK_GREY = 8,
	VGA_COLOR_LIGHT_BLUE = 9,
	VGA_COLOR_LIGHT_GREEN = 10,
	VGA_COLOR_LIGHT_CYAN = 11,
	VGA_COLOR_LIGHT_RED = 12,
	VGA_COLOR_LIGHT_MAGENTA = 13,
	VGA_COLOR_LIGHT_BROWN = 14,
	VGA_COLOR_WHITE = 15,
};

void terminal_Init(void);
void terminal_SetColor(uint8 color);
void terminal_Write(const char* data, uint size);
void terminal_WriteChar(char c);
void terminal_WriteCharAt(char c, uint8 color, uint x, uint y);
void terminal_WriteString(string s);
void terminal_WriteError(string s);

#endif // TERMINAL_H
