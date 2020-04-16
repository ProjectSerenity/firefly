// +build ignore

#include "std.h"
#include "terminal.h"
#include <stdarg.h>

const bool true = 1;
const bool false = 0;

const void* nil = 0;

void copy(char* dst, char* src, uint n) {
	uint i;
	for (i = 0; i < n; i++) {
		dst[i] = src[i];
	}
}

int printBits(uint64 v, int base, bool upper, int minWidth, char padChar);

// printk is somewhere between C's and Go's
// printf functions. The format verbs are
// close to Go's, but use more verbose size
// indicators.
//
// Currently supported verbs:
//
// 	%b         Binary integer
// 	%o         Octal integer
// 	%d         Decimal integer
// 	%x         Hexadecimal integer or buffer (lowercase)
// 	%X         Hexadecimal integer or buffer (uppercase)
// 	%c         Character
// 	%s         String
// 	%h         Hexdump buffer
// 	%%         Percent literal
//
// Verb modifiers (which appear between the
// percent and the verb):
//
// 	u{n}       Unsigned integer (of n bits)
// 	+{n}       Signed integer (of n bits)
// 	m{n}       Memory buffer (of n bytes)
// 	{ }        Separate each byte with a space
// 	w{n}       Minimum width (of n chars), prefixing with spaces
// 	0{n}       Minimum width (of n chars), prefixing with zeros
//
// Integers
//
// Integers are printed with a base indicated
// by the verb used (base 2 for %b, base 8 for
// %o, base 10 for %d, and base 16 for %x/%X).
// Integers must not be accompanied by the
// memory (%m) or space (% ) modifiers, although
// %mh is used to print buffers. Integers may
// use the unsigned (%u), signed (%+), width (%w),
// and zero-prefix (%0) modifiers. The unsigned
// and signed modifiers cannot be used together,
// nor can the width and zero-prefix modifiers.
// To avoid ambiguity, the width or zero-prefix
// modifier must come before the unsigned or
// signed modifier.
//
// Common examples:
//
// 	printk("%u8d", 255);    // "255"
// 	printk("%+16o", -0777); // "-777"
// 	printk("%08u8b", 7);    // "00000111"
// 	printk("%w3u8d", 37);   // " 37"
// 	printk("%u8x", 255);    // "ff"
// 	printk("%u8X", 255);    // "FF"
//
// Characters
//
// Characters are printed using %c. This verb
// takes no modifiers.
//
// Common examples:
//
// 	printk("%c", 'a');   // "a"
// 	printk("%c", 97);    // "a"
//
// Strings
//
// Strings are printed using %s. If the memory
// modifier (%m) is used, it specifies the number
// of bytes to print and indicates that the string
// is passed as a `char*`. If the memory modifier
// is absent, a `string` is expected (not yet
// implemented). The width modifier (%w) can be
// used to set the minimum width of the string.
// No other modifiers can be used with %s.
//
// Common examples:
//
// 	printk("%m5s", "Hello, World");    // "Hello"
// 	printk("%w7m5s", "Hello, World");  // "  Hello"
//
// Buffers
//
// Buffers are printed with in hexadecimal format
// with %x/%X or in hexdump format with %h. The
// memory modifier (%m) is used to indicate a
// buffer and specifies the number of bytes from
// the buffer to print. The unsigned (%u), signed
// (%+), width (%w), and zero-prefix (%0) modifiers
// cannot be used with buffers. The space modifier
// (% ) can be used with %x/%X to add a space
// between each byte.
//
// Common examples:
//
// 	printk("%m4x", "asdk");    // "6173646b"
// 	printk("%m4 X", "asdk");   // "61 73 64 6B"
// 	printk("%m41h", "C is an open source programming language.");
// 		"00000000  43 20 69 73 20 61 6e 20  6f 70 65 6e 20 73 6f 75  |C is an open sou|\n"
// 		"00000010  72 63 65 20 70 72 6f 67  72 61 6d 6d 69 6e 67 20  |rce programming |\n"
// 		"00000020  6c 61 6e 67 75 61 67 65  2e                       |language.|\n"
//
int printk(char format[], ...) {
	va_list parameters;
	va_start(parameters, format);

	int written = 0;

	bool inVerb = false;
	bool isUnsigned = false;
	bool isSigned = false;
	bool isMemory = false;
	bool isWidth = false;
	bool isZero = false;
	bool addSpace = false;
	int size = 0;
	int minWidth = 0;
	for (int i = 0; format[i] != 0; i++) {
		char c = format[i];

		// Non-verb content.

		if (!inVerb && c != '%') {
			written += terminal_WriteChar(c);
			continue;
		}

		// Escaped percents.

		if (!inVerb && format[i+1] == '%') {
			written += terminal_WriteChar('%');
			i++;
			continue;
		}

		// Verb-initiating percents.

		if (!inVerb) {
			inVerb = true;
			continue;
		}

		// Modifiers.

		switch (c) {
		case 'u':
			if (isUnsigned || isSigned || isMemory) {
				written += terminal_WriteError(str("%u!(BAD_MODIFIER)"));
				continue;
			}

			isUnsigned = true;
			continue;
		case '+':
			if (isUnsigned || isSigned || isMemory) {
				written += terminal_WriteError(str("%+!(BAD_MODIFIER)"));
				continue;
			}

			isSigned = true;
			continue;
		case 'm':
			if (isUnsigned || isSigned || isMemory) {
				written += terminal_WriteError(str("%m!(BAD_MODIFIER)"));
				continue;
			}

			isMemory = true;
			continue;
		case ' ':
			if (addSpace) {
				written += terminal_WriteError(str("% !(BAD_MODIFIER)"));
				continue;
			}

			addSpace = true;
			continue;
		case 'w':
			if (isUnsigned || isSigned || isZero) {
				written += terminal_WriteError(str("%u!(BAD_MODIFIER)"));
				continue;
			}

			isWidth = true;
			continue;
		case '0':
			if (!isZero && !isWidth && size == 0 && minWidth == 0) {
				isZero = true;
				continue;
			}
			// fallthrough
		case '1':
		case '2':
		case '3':
		case '4':
		case '5':
		case '6':
		case '7':
		case '8':
		case '9':
			if (isUnsigned || isSigned || isMemory) {
				size = 10*size + (c-'0');
			} else if (isWidth || isZero) {
				minWidth = 10*minWidth + (c-'0');
			} else {
				written += terminal_WriteError(str("%u!(BAD_MODIFIER)"));
				goto exit_verb;
			}

			continue;
		}

		// Integer verbs.

		// Integers are printed with a base indicated
		// by the verb used (base 2 for %b, base 8 for
		// %o, base 10 for %d, and base 16 for %x/%X).
		// Integers must not be accompanied by the
		// memory (%m) or space (% ) modifiers, although
		// %mh is used to print buffers. Integers may
		// use the unsigned (%u), signed (%+), width (%w),
		// and zero-prefix (%0) modifiers. The unsigned
		// and signed modifiers cannot be used together,
		// nor can the width and zero-prefix modifiers.
		if (c == 'b' || c == 'o' || c == 'd' || (c == 'x' && !isMemory) || (c == 'X' && !isMemory)) {
			if (isMemory) {
				written += terminal_WriteError(str("%!n(MEMORY)"));
				goto exit_verb;
			} else if (addSpace) {
				written += terminal_WriteError(str("%!n(SPACE)"));
				goto exit_verb;
			}

			uint64 v;
			bool isNeg = false;
			switch (size) {
			case 8:
				v = (uint64)va_arg(parameters, int); // uint8 is promoted to int.
				if (isUnsigned) {
					v = (uint64)(uint8)v;
				} else {
					v = (uint64)(int8)v;
					if ((int8)v < 0) {
						isNeg = true;
						v = (uint64)(-(int8)v);
					}
				}
				break;
			case 16:
				v = (uint64)va_arg(parameters, int); // uint16 is promoted to int.
				if (isUnsigned) {
					v = (uint64)(uint16)v;
				} else {
					v = (uint64)(int16)v;
					if ((int16)v < 0) {
						isNeg = true;
						v = (uint64)(-(int16)v);
					}
				}
				break;
			case 32:
				v = (uint64)va_arg(parameters, uint32);
				if (isUnsigned) {
					v = (uint64)(uint32)v;
				} else {
					v = (uint64)(int32)v;
					if ((int32)v < 0) {
						isNeg = true;
						v = (uint64)(-(int32)v);
					}
				}
				break;
			case 64:
				v = (uint64)va_arg(parameters, uint64);
				if (!isUnsigned) {
					if ((int64)v < 0) {
						isNeg = true;
						v = (uint64)(-(int64)v);
					}
				}
				break;
			case 0:
				written += terminal_WriteError(str("%!n(MISSING_SIZE)"));
				goto exit_verb;
			default:
				written += terminal_WriteError(str("%!n(BAD_SIZE)"));
				goto exit_verb;
			}

			if (isNeg) {
				written += terminal_WriteChar('-');
			} else if (isSigned) {
				written += terminal_WriteChar('+');
			}

			if (v == 0) {
				written += terminal_WriteChar('0');
				goto exit_verb;
			}

			int base;
			switch (c) {
			case 'b':
				base = 2;
				break;
			case 'o':
				base = 8;
				break;
			case 'd':
				base = 10;
				break;
			case 'x':
			case 'X':
				base = 16;
				break;
			}

			char padChar = ' ';
			if (isZero) {
				padChar = '0';
			}

			written += printBits(v, base, c == 'X', minWidth, padChar);
			goto exit_verb;
		}

		// Characters

		// Characters are printed using %c. This verb
		// takes no modifiers.
		if (c == 'c') {
			if (isUnsigned) {
				written += terminal_WriteError(str("%!c(UNSIGNED)"));
				goto exit_verb;
			} else if (isSigned) {
				written += terminal_WriteError(str("%!c(SIGNED)"));
				goto exit_verb;
			} else if (isMemory) {
				written += terminal_WriteError(str("%!c(MEMORY)"));
				goto exit_verb;
			} else if (addSpace) {
				written += terminal_WriteError(str("%!c(SPACE)"));
				goto exit_verb;
			}

			char v = (char)va_arg(parameters, int); // char is promoted to int.
			written += terminal_WriteChar(v);
			goto exit_verb;
		}

		// Strings

		// Strings are printed using %s. If the memory
		// modifier (%m) is used, it specifies the number
		// of bytes to print and indicates that the string
		// is passed as a `char*`. If the memory modifier
		// is absent, a `string` is expected (not yet
		// implemented). The width modifier (%w) can be
		// used to set the minimum width of the string.
		// No other modifiers can be used with %s.
		if (c == 's') {
			if (isUnsigned) {
				written += terminal_WriteError(str("%!s(UNSIGNED)"));
				goto exit_verb;
			} else if (isSigned) {
				written += terminal_WriteError(str("%!s(SIGNED)"));
				goto exit_verb;
			} else if (addSpace) {
				written += terminal_WriteError(str("%!s(SPACE)"));
				goto exit_verb;
			} else if (isZero) {
				written += terminal_WriteError(str("%!s(ZERO)"));
				goto exit_verb;
			}

			string s;
			if (isMemory) {
				char* b = (char*) va_arg(parameters, char*);
				s.ptr = b;
				s.len = size;
			} else {
				// TODO
				//s = (string) va_arg(parameters, string);
				written += terminal_WriteError(str("%!s(STRING)"));
				goto exit_verb;
			}

			while (minWidth > s.len) {
				written += terminal_WriteChar(' ');
				minWidth--;
			}

			written += terminal_WriteString(s);
			goto exit_verb;
		}

		// Buffers

		// Buffers are printed with in hexadecimal format
		// with %x/%X or in hexdump format with %h. The
		// memory modifier (%m) is used to indicate a
		// buffer and specifies the number of bytes from
		// the buffer to print. The unsigned (%u), signed
		// (%+), width (%w), and zero-prefix (%0) modifiers
		// cannot be used with buffers. The space modifier
		// (% ) can be used with %x/%X to add a space
		// between each byte.
		if (c == 'x' || c == 'X') {
			if (isUnsigned) {
				written += terminal_WriteError(str("%!x(UNSIGNED)"));
				goto exit_verb;
			} else if (isSigned) {
				written += terminal_WriteError(str("%!x(SIGNED)"));
				goto exit_verb;
			}

			char* buffer = (char*) va_arg(parameters, char*);
			for (int i = 0; i < size; i++) {
				written += printBits(buffer[i], 16, c == 'X', 0, '0');
				if (addSpace && i+1 < size) {
					written += terminal_WriteChar(' ');
				}
			}

			goto exit_verb;
		}

		// Hexdump

		if (c == 'h') {
			// Output lines look like:
			// 00000010  2e 2f 30 31 32 33 34 35  36 37 38 39 3a 3b 3c 3d  |./0123456789:;<=|
			// ^ offset                          ^ extra space              ^ ASCII of line.

			int used = 0;        // Bytes in the current line.
			uint64 n = 0;        // Bytes written in total.
			char rightChars[18]; // ASCII chars to the right.
			string right = {.ptr=rightChars, .len=18};

			char* buffer = (char*) va_arg(parameters, char*);
			for (int i = 0; i < size; i++) {
				if (used == 0) {
					// At the beginning of a line we print the current
					// offset in hex.
					written += printBits(n, 16, false, 8, '0');
					written += terminal_WriteString(str("  "));
				}

				written += printBits(buffer[i], 16, false, 0, '0');
				written += terminal_WriteChar(' ');
				if (used == 7) {
					// There's an additional space after the 8th byte.
					written += terminal_WriteChar(' ');
				} else if (used == 15) {
					// At the end of the line there's an extra space and
					// the bar for the right column.
					written += terminal_WriteString(str(" |"));
				}

				n++;
				rightChars[used] = buffer[i];
				if (buffer[i] < 32 || 126 < buffer[i]) {
					rightChars[used] = '.';
				}

				used++;
				n++;
				if (used == 16) {
					rightChars[16] = '|';
					rightChars[17] = '\n';
					written += terminal_WriteString(right);
					used = 0;
				}
			}

			if (size > 0 && used > 0) {
				string spaces = {.ptr="    |", .len=3};
				while (used < 16) {
					spaces.len = 3;
					if (used == 7) {
						spaces.len = 4;
					} else if (used == 15) {
						spaces.len = 5;
					}

					written += terminal_WriteString(spaces);
					rightChars[used] = ' ';
					used++;
				}

				rightChars[16] = '|';
				rightChars[17] = '\n';
				written += terminal_WriteString(right);
			}

			goto exit_verb;
		}

		// Unrecognised verb.

		written += terminal_WriteError(str("%!(BAD_MODIFIER)"));
		continue;

	exit_verb:
		inVerb = false;
		isUnsigned = false;
		isSigned = false;
		isMemory = false;
		isWidth = false;
		isZero = false;
		size = 0;
		minWidth = 0;
	}

	if (inVerb) {
		written += terminal_WriteError(str("%!(MISSING)"));
	}

	va_end(parameters);
	return written;
}

const char* smallsString= "00010203040506070809"
	"10111213141516171819"
	"20212223242526272829"
	"30313233343536373839"
	"40414243444546474849"
	"50515253545556575859"
	"60616263646566676869"
	"70717273747576777879"
	"80818283848586878889"
	"90919293949596979899";

const char* digits_lower = "0123456789abcdefghijklmnopqrstuvwxyz";
const char* digits_upper = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ";

// printBits is for internal use within printk
// only. The base must be 2, 8, 10, or 16.
//
// printBits returns the number of bytes printed.
//
int printBits(uint64 v, int base, bool upper, int minWidth, char padChar) {
	char buffer[64+1]; // +1 for sign of 64bit value in base 2.
	int i = 65;

	int written = 0;
	if (base == 10) {
		while (v >= 100) {
			int is = v % 100 * 2;
			v /= 100;
			i -= 2;
			buffer[i+0] = smallsString[is+0];
			buffer[i+1] = smallsString[is+1];
			written += 2;
		}

		// v < 100
		int is = v * 2;
		i--;
		buffer[i] = smallsString[is+1];
		written++;
		if (v >= 10) {
			i--;
			buffer[i] = smallsString[is+0];
			written++;
		}

		while (written < minWidth) {
			i--;
			buffer[i] = padChar;
			written++;
		}

		terminal_WriteString((string){.len=written, .ptr=buffer+i});

		return written;
	}

	// Use shifts and masks instead of / and %.
	// Base is a power of 2 and 2 <= base <= len(digits) where len(digits) is 36.
	// The largest power of 2 below or equal to 36 is 32, which is 1 << 5;
	// i.e., the largest possible shift count is 5. By &-ind that value with
	// the constant 7 we tell the compiler that the shift count is always
	// less than 8 which is smaller than any register width. This allows
	// the compiler to generate better code for the shift operation.

	// The Go code does:
	//
	// 	shift := uint(bits.TrailingZeros(uint(base))) & 7
	//
	// Rather than worry about getting the instruction for bits.TrailingZeros,
	// we just hard-code the supported values.
	//
	uint shift;
	string err;
	switch (base) {
	case 2:
		shift = 1;
		break;
	case 8:
		shift = 3;
		break;
	case 16:
		shift = 4;
		break;
	default:
		err = str("%!(BAD_BASE)");
		terminal_WriteError(err);
		return err.len;
	}

	const char* digits = digits_lower;
	if (upper) {
		digits = digits_upper;
	}

	uint64 b = base;
	uint m = (uint)base - 1; // == 1<<shift - 1
	while (v >= b) {
		i--;
		buffer[i] = digits[v&m];
		written++;
		v >>= shift;
	}

	// v < base
	i--;
	buffer[i] = digits[v];
	written++;

	while (written < minWidth) {
		i--;
		buffer[i] = padChar;
		written++;
	}

	terminal_WriteString((string){.len=written, .ptr=buffer+i});

	return written;
}
