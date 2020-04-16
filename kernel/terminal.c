#include "std.h"
#include "terminal.h"

static inline uint8 vga_entry_color(enum vga_color fg, enum vga_color bg) {
	return fg | bg << 4;
}

static inline uint16 vga_entry(unsigned char uc, uint8 color) {
	return (uint16) uc | (uint16) color << 8;
}

uint strlen(const char* str) {
	uint len = 0;
	while (str[len]) {
		len++;
	}

	return len;
}

static const uint VGA_WIDTH = 80;
static const uint VGA_HEIGHT = 25;
static uint16* const VGA_MEMORY = (uint16*) 0xB8000;

static uint terminal_row;
static uint terminal_column;
static uint8 terminal_color;
static uint16* terminal_buffer;

void terminal_Init(void) {
	terminal_row = 0;
	terminal_column = 0;
	terminal_color = vga_entry_color(VGA_COLOR_LIGHT_GREY, VGA_COLOR_BLACK);
	terminal_buffer = VGA_MEMORY;
	uint x, y;
	for (y = 0; y < VGA_HEIGHT; y++) {
		for (x = 0; x < VGA_WIDTH; x++) {
			const uint index = y * VGA_WIDTH + x;
			terminal_buffer[index] = vga_entry(' ', terminal_color);
		}
	}
}

void terminal_SetColor(uint8 color) {
	terminal_color = color;
}

uint terminal_WriteCharAt(char c, uint8 color, uint x, uint y) {
	const uint index = y * VGA_WIDTH + x;
	terminal_buffer[index] = vga_entry(c, color);
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
	if (++terminal_column < VGA_WIDTH) {
		return 1;
	}

	if (++terminal_row < VGA_HEIGHT) {
		return 1;
	}

	// We've reached the end of the terminal,
	// so we need to shift each line upwards
	// to make space for the next line.

	copy((char*)terminal_buffer, (char*)terminal_buffer+VGA_WIDTH, (VGA_HEIGHT-1)*VGA_WIDTH);

	// Go back to the penultimate line.
	terminal_row--;

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
	uint8 old = terminal_color;
	terminal_SetColor(vga_entry_color(VGA_COLOR_RED, VGA_COLOR_BLACK));
	terminal_Write(s.ptr, s.len);
	terminal_SetColor(old);
	return s.len;
}
