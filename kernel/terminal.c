#include "std.h"
#include "terminal.h"
#include "font.h"

static uint8 terminal_pixelwidth;
static uint16 terminal_pitch;
static uint8* terminal_addr;

static uint terminal_row;
static uint terminal_column;
static uint32 terminal_color;

uint32 rgb(uint8 red, uint8 green, uint8 blue) {
	// Little-endian.
	return ((uint32)blue)<<16 | ((uint32)green)<<8 | (uint32)red;
}

void terminal_Init(void) {
	terminal_width = *(uint16*)0x5084;
	terminal_height = *(uint16*)0x5086;
	terminal_pixelwidth = (*(uint8*)0x5088)>>3; // Bits to bytes (/8).
	terminal_pitch = *(uint16*)0x508A;
	terminal_addr = (uint8*)(uintptr)*(uint32*)0x5080;
	terminal_row = 0;
	terminal_column = 0;
	terminal_color = rgb(255, 255, 255);
}

void terminal_PixelAt(uint x, uint y, uint32 color) {
	uint offset = y*terminal_pitch + x*terminal_pixelwidth;
	terminal_addr[offset+0] = (uint8)(color>>16);
	terminal_addr[offset+1] = (uint8)(color>>8);
	terminal_addr[offset+2] = (uint8)color;
}

void terminal_SetColor(uint32 color) {
	terminal_color = color;
}

uint terminal_WriteCharAt(char c, uint32 color, uint x, uint y) {
	x *= 8;
	y *= 8;
	uint64 data = font_data[(int)c];
	int i, j;
	for (i = 7; i >= 0; i--) {
		uint offset = y*terminal_pitch + x*terminal_pixelwidth;
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

			offset += terminal_pixelwidth;
		}

		y++;
	}

	return 1;
}

uint terminal_WriteChar(char c) {
	switch (c) {
	case '\n':
		terminal_row++;
		terminal_column = 0;
		return 1;
	}

	terminal_WriteCharAt(c, terminal_color, terminal_column, terminal_row);
	if (++terminal_column < terminal_width) {
		return 1;
	}

	if (++terminal_row < terminal_height) {
		return 1;
	}

	// TODO: handle when we reach the bottom row.

	return 1;
}

uint terminal_Write(const char* data, uint size) {
	uint i;
	for (i = 0; i < size; i++) {
		terminal_WriteChar(data[i]);
	}

	return size;
}

uint terminal_WriteString(string s) {
	terminal_Write(s.ptr, s.len);
	return s.len;
}

uint terminal_WriteError(string s) {
	uint32 old = terminal_color;
	terminal_SetColor(rgb(255, 0, 0));
	terminal_Write(s.ptr, s.len);
	terminal_SetColor(old);
	return s.len;
}
