#include "std.h"
#include "term.h"
#include "font.h"

uint64 term_Width;
uint64 term_Height;

static uint8 term_pixelWidth;
static uint16 term_pitch;
static uint8* term_addr;

static uint64 term_row;
static uint64 term_column;
static uint32 term_color;

uint32 term_RGB(uint8 red, uint8 green, uint8 blue) {
	// Little-endian.
	return ((uint32)blue)<<16 | ((uint32)green)<<8 | (uint32)red;
}

void term_Init(void) {
	term_Width = (uint64)*(uint16*)0x5084;
	term_Height = (uint64)*(uint16*)0x5086;
	term_pixelWidth = (*(uint8*)0x5088)>>3; // Bits to bytes (/8).
	term_pitch = *(uint16*)0x508A;
	term_addr = (uint8*)(uintptr)*(uint32*)0x5080;
	term_row = 0;
	term_column = 0;
	term_color = term_RGB(255, 255, 255);
}

void term_PixelAt(uint64 x, uint64 y, uint32 color) {
	uint64 offset = y*term_pitch + x*term_pixelWidth;
	term_addr[offset+0] = (uint8)(color>>16);
	term_addr[offset+1] = (uint8)(color>>8);
	term_addr[offset+2] = (uint8)color;
}

uint32 term_GetColor() {
	return term_color;
}

void term_SetColor(uint32 color) {
	term_color = color;
}

uint64 term_WriteCharAt(char c, uint32 color, uint64 x, uint64 y) {
	x *= 8;
	y *= 8;
	uint64 data = font_Data[(int)c];
	int8 i, j;
	for (i = 7; i >= 0; i--) {
		uint64 offset = y*term_pitch + x*term_pixelWidth;
		for (j = 7; j >= 0; j--) {
			if (data & (((uint64)1)<<((i*8)+j))) {
				term_addr[offset+0] = (uint8)(color>>16);
				term_addr[offset+1] = (uint8)(color>>8);
				term_addr[offset+2] = (uint8)color;
			} else {
				term_addr[offset+0] = (uint8)0;
				term_addr[offset+1] = (uint8)0;
				term_addr[offset+2] = (uint8)0;
			}

			offset += term_pixelWidth;
		}

		y++;
	}

	return 1;
}

uint64 term_WriteChar(char c) {
	switch (c) {
	case '\n':
		goto wrap_bottom;
	case '\r':
		term_column = 0;
		return 1;
	}

	term_WriteCharAt(c, term_color, term_column, term_row);
	if (8*(++term_column) < term_Width) {
		return 1;
	}

wrap_bottom:
	term_column = 0;
	if (8*(++term_row) < term_Height) {
		return 1;
	}

	term_row = 0;
	// TODO: handle when we reach the bottom row.

	return 1;
}

uint64 term_Write(const char* data, uint64 size) {
	uint64 i;
	for (i = 0; i < size; i++) {
		term_WriteChar(data[i]);
	}

	return size;
}

uint64 term_WriteString(string s) {
	term_Write(s.ptr, (uint64)s.len);
	return (uint64)s.len;
}

uint64 term_WriteError(string s) {
	uint32 old = term_color;
	term_SetColor(term_RGB(255, 0, 0));
	term_Write(s.ptr, (uint64)s.len);
	term_SetColor(old);
	return (uint64)s.len;
}
