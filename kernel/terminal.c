#include "std.h"
#include "terminal.h"
#include "font.h"

uint64 terminal_Width;
uint64 terminal_Height;

static uint8 terminal_pixelWidth;
static uint16 terminal_pitch;
static uint8* terminal_addr;

static uint64 terminal_row;
static uint64 terminal_column;
static uint32 terminal_color;

uint32 terminal_RGB(uint8 red, uint8 green, uint8 blue) {
	// Little-endian.
	return ((uint32)blue)<<16 | ((uint32)green)<<8 | (uint32)red;
}

void terminal_Init(void) {
	terminal_Width = (uint64)*(uint16*)0x5084;
	terminal_Height = (uint64)*(uint16*)0x5086;
	terminal_pixelWidth = (*(uint8*)0x5088)>>3; // Bits to bytes (/8).
	terminal_pitch = *(uint16*)0x508A;
	terminal_addr = (uint8*)(uintptr)*(uint32*)0x5080;
	terminal_row = 0;
	terminal_column = 0;
	terminal_color = terminal_RGB(255, 255, 255);
}

void terminal_PixelAt(uint64 x, uint64 y, uint32 color) {
	uint64 offset = y*terminal_pitch + x*terminal_pixelWidth;
	terminal_addr[offset+0] = (uint8)(color>>16);
	terminal_addr[offset+1] = (uint8)(color>>8);
	terminal_addr[offset+2] = (uint8)color;
}

uint32 terminal_GetColor() {
	return terminal_color;
}

void terminal_SetColor(uint32 color) {
	terminal_color = color;
}

uint64 terminal_WriteCharAt(char c, uint32 color, uint64 x, uint64 y) {
	x *= 8;
	y *= 8;
	uint64 data = font_Data[(int)c];
	int8 i, j;
	for (i = 7; i >= 0; i--) {
		uint64 offset = y*terminal_pitch + x*terminal_pixelWidth;
		for (j = 7; j >= 0; j--) {
			if (data & (((uint64)1)<<((i*8)+j))) {
				terminal_addr[offset+0] = (uint8)(color>>16);
				terminal_addr[offset+1] = (uint8)(color>>8);
				terminal_addr[offset+2] = (uint8)color;
			} else {
				terminal_addr[offset+0] = (uint8)0;
				terminal_addr[offset+1] = (uint8)0;
				terminal_addr[offset+2] = (uint8)0;
			}

			offset += terminal_pixelWidth;
		}

		y++;
	}

	return 1;
}

uint64 terminal_WriteChar(char c) {
	switch (c) {
	case '\n':
		goto wrap_bottom;
	}

	terminal_WriteCharAt(c, terminal_color, terminal_column, terminal_row);
	if (8*(++terminal_column) < terminal_Width) {
		return 1;
	}

wrap_bottom:
	terminal_column = 0;
	if (8*(++terminal_row) < terminal_Height) {
		return 1;
	}

	terminal_row = 0;
	// TODO: handle when we reach the bottom row.

	return 1;
}

uint64 terminal_Write(const char* data, uint64 size) {
	uint64 i;
	for (i = 0; i < size; i++) {
		terminal_WriteChar(data[i]);
	}

	return size;
}

uint64 terminal_WriteString(string s) {
	terminal_Write(s.ptr, (uint64)s.len);
	return (uint64)s.len;
}

uint64 terminal_WriteError(string s) {
	uint32 old = terminal_color;
	terminal_SetColor(terminal_RGB(255, 0, 0));
	terminal_Write(s.ptr, (uint64)s.len);
	terminal_SetColor(old);
	return (uint64)s.len;
}
