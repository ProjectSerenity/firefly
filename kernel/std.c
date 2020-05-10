#include "std.h"
#include "term.h"
#include <stdarg.h>

const bool true = 1;
const bool false = 0;

// std_ValidNumericalTypes confirms that the numerical
// types have the size we expect. If any types do
// not have the expected size, error messages will
// be printed and false returned. If all numerical
// types are as expected, std_ValidNumericalTypes will
// return true and otherwise do nothing.
//
bool std_ValidNumericalTypes() {
	bool ok = true;
	if (8*sizeof(int8) != 8) {
		ok = false;
		std_Printk("int8 has unexpected size %+64d bits\n", 8*sizeof(int8));
	}
	if (8*sizeof(int16) != 16) {
		ok = false;
		std_Printk("int16 has unexpected size %+64d bits\n", 8*sizeof(int16));
	}
	if (8*sizeof(int32) != 32) {
		ok = false;
		std_Printk("int32 has unexpected size %+64d bits\n", 8*sizeof(int32));
	}
	if (8*sizeof(int64) != 64) {
		ok = false;
		std_Printk("int64 has unexpected size %+64d bits\n", 8*sizeof(int64));
	}
	if (8*sizeof(uint8) != 8) {
		ok = false;
		std_Printk("uint8 has unexpected size %+64d bits\n", 8*sizeof(uint8));
	}
	if (8*sizeof(uint16) != 16) {
		ok = false;
		std_Printk("uint16 has unexpected size %+64d bits\n", 8*sizeof(uint16));
	}
	if (8*sizeof(uint32) != 32) {
		ok = false;
		std_Printk("uint32 has unexpected size %+64d bits\n", 8*sizeof(uint32));
	}
	if (8*sizeof(uint64) != 64) {
		ok = false;
		std_Printk("uint64 has unexpected size %+64d bits\n", 8*sizeof(uint64));
	}
	if (8*sizeof(uintptr) != 64) {
		ok = false;
		std_Printk("uintptr has unexpected size %+64d bits\n", 8*sizeof(uintptr));
	}
	if (8*sizeof(float32) != 32) {
		ok = false;
		std_Printk("float32 has unexpected size %+64d bits\n", 8*sizeof(float32));
	}
	if (8*sizeof(float64) != 64) {
		ok = false;
		std_Printk("float64 has unexpected size %+64d bits\n", 8*sizeof(float64));
	}

	return ok;
}

const void* nil = 0;

void std_Copy(uint8* dst, uint8* src, uint64 n) {
	uint64 i;
	for (i = 0; i < n; i++) {
		dst[i] = src[i];
	}
}

static int64 printBits(uint64 v, uint8 base, bool upper, int32 minWidth, char padChar);

// std_Printk is somewhere between C's and Go's
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
// 	%p         Pointer address
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
// 	std_Printk("%u8d", 255);    // "255"
// 	std_Printk("%+16o", -0777); // "-777"
// 	std_Printk("%08u8b", 7);    // "00000111"
// 	std_Printk("%w3u8d", 37);   // " 37"
// 	std_Printk("%u8x", 255);    // "ff"
// 	std_Printk("%u8X", 255);    // "FF"
//
// Characters
//
// Characters are printed using %c. This verb
// takes no modifiers.
//
// Common examples:
//
// 	std_Printk("%c", 'a');   // "a"
// 	std_Printk("%c", 97);    // "a"
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
// 	std_Printk("%m5s", "Hello, World");    // "Hello"
// 	std_Printk("%w7m5s", "Hello, World");  // "  Hello"
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
// 	std_Printk("%m4x", "asdk");    // "6173646b"
// 	std_Printk("%m4 X", "asdk");   // "61 73 64 6B"
// 	std_Printk("%m41h", "C is an open source programming language.");
// 		"00000000  43 20 69 73 20 61 6e 20  6f 70 65 6e 20 73 6f 75  |C is an open sou|\n"
// 		"00000010  72 63 65 20 70 72 6f 67  72 61 6d 6d 69 6e 67 20  |rce programming |\n"
// 		"00000020  6c 61 6e 67 75 61 67 65  2e                       |language.|\n"
//
// Pointers
//
// Pointers are printed in hexadecimal format with
// an '0x' prefix with %p. This verb takes no
// modifiers.
//
// Common examples:
//
// 	std_Printk("%p", std_Printk);   // 0x1234567890abcdef
//
int64 std_Printk(const char format[], ...) {
	va_list parameters;
	va_start(parameters, format);

	int64 written = 0;

	bool inVerb = false;
	bool isUnsigned = false;
	bool isSigned = false;
	bool isMemory = false;
	bool isWidth = false;
	bool isZero = false;
	bool addSpace = false;
	int8 size = 0;
	int32 minWidth = 0;
	int64 i;
	for (i = 0; format[i] != 0; i++) {
		char c = format[i];

		// Non-verb content.

		if (!inVerb && c != '%') {
			written += term_WriteChar(c);
			continue;
		}

		// Escaped percents.

		if (!inVerb && format[i+1] == '%') {
			written += term_WriteChar('%');
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
				written += term_WriteError(str("%u!(BAD_MODIFIER)"));
				continue;
			}

			isUnsigned = true;
			continue;
		case '+':
			if (isUnsigned || isSigned || isMemory) {
				written += term_WriteError(str("%+!(BAD_MODIFIER)"));
				continue;
			}

			isSigned = true;
			continue;
		case 'm':
			if (isUnsigned || isSigned || isMemory) {
				written += term_WriteError(str("%m!(BAD_MODIFIER)"));
				continue;
			}

			isMemory = true;
			continue;
		case ' ':
			if (addSpace) {
				written += term_WriteError(str("% !(BAD_MODIFIER)"));
				continue;
			}

			addSpace = true;
			continue;
		case 'w':
			if (isUnsigned || isSigned || isZero) {
				written += term_WriteError(str("%u!(BAD_MODIFIER)"));
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
				written += term_WriteError(str("%u!(BAD_MODIFIER)"));
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
				written += term_WriteError(str("%!n(MEMORY)"));
				goto exit_verb;
			} else if (addSpace) {
				written += term_WriteError(str("%!n(SPACE)"));
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
				written += term_WriteError(str("%!n(MISSING_SIZE)"));
				goto exit_verb;
			default:
				written += term_WriteError(str("%!n(BAD_SIZE)"));
				goto exit_verb;
			}

			if (isNeg) {
				written += term_WriteChar('-');
			} else if (isSigned) {
				written += term_WriteChar('+');
			}

			if (v == 0) {
				written += term_WriteChar('0');
				goto exit_verb;
			}

			uint8 base = 10;
			switch (c) {
			case 'b':
				base = 2;
				break;
			case 'o':
				base = 8;
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
				written += term_WriteError(str("%!c(UNSIGNED)"));
				goto exit_verb;
			} else if (isSigned) {
				written += term_WriteError(str("%!c(SIGNED)"));
				goto exit_verb;
			} else if (isMemory) {
				written += term_WriteError(str("%!c(MEMORY)"));
				goto exit_verb;
			} else if (addSpace) {
				written += term_WriteError(str("%!c(SPACE)"));
				goto exit_verb;
			} else if (isWidth) {
				written += term_WriteError(str("%!c(WIDTH)"));
				goto exit_verb;
			} else if (isZero) {
				written += term_WriteError(str("%!c(ZERO)"));
				goto exit_verb;
			}

			char v = (char)va_arg(parameters, int); // char is promoted to int.
			written += term_WriteChar(v);
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
				written += term_WriteError(str("%!s(UNSIGNED)"));
				goto exit_verb;
			} else if (isSigned) {
				written += term_WriteError(str("%!s(SIGNED)"));
				goto exit_verb;
			} else if (addSpace) {
				written += term_WriteError(str("%!s(SPACE)"));
				goto exit_verb;
			} else if (isZero) {
				written += term_WriteError(str("%!s(ZERO)"));
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
				written += term_WriteError(str("%!s(STRING)"));
				goto exit_verb;
			}

			while (minWidth > s.len) {
				written += term_WriteChar(' ');
				minWidth--;
			}

			written += term_WriteString(s);
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
				written += term_WriteError(str("%!x(UNSIGNED)"));
				goto exit_verb;
			} else if (isSigned) {
				written += term_WriteError(str("%!x(SIGNED)"));
				goto exit_verb;
			} else if (isWidth) {
				written += term_WriteError(str("%!x(WIDTH)"));
				goto exit_verb;
			} else if (isZero) {
				written += term_WriteError(str("%!x(ZERO)"));
				goto exit_verb;
			}

			char* buffer = (char*) va_arg(parameters, char*);
			int64 j;
			for (j = 0; j < size; j++) {
				written += printBits(0xff & buffer[j], 16, c == 'X', 0, '0');
				if (addSpace && j+1 < size) {
					written += term_WriteChar(' ');
				}
			}

			goto exit_verb;
		}

		// Hexdump

		if (c == 'h') {
			if (isUnsigned) {
				written += term_WriteError(str("%!h(UNSIGNED)"));
				goto exit_verb;
			} else if (isSigned) {
				written += term_WriteError(str("%!h(SIGNED)"));
				goto exit_verb;
			} else if (addSpace) {
				written += term_WriteError(str("%!h(SPACE)"));
				goto exit_verb;
			} else if (isWidth) {
				written += term_WriteError(str("%!h(WIDTH)"));
				goto exit_verb;
			} else if (isZero) {
				written += term_WriteError(str("%!h(ZERO)"));
				goto exit_verb;
			}

			// Output lines look like:
			// 00000010  2e 2f 30 31 32 33 34 35  36 37 38 39 3a 3b 3c 3d  |./0123456789:;<=|
			// ^ offset                          ^ extra space              ^ ASCII of line.

			int8 used = 0;       // Bytes in the current line.
			uint64 n = 0;        // Bytes written in total.
			char rightChars[18]; // ASCII chars to the right.
			string right = {.ptr=rightChars, .len=18};

			char* buffer = (char*) va_arg(parameters, char*);
			int32 k;
			for (k = 0; k < size; k++) {
				if (used == 0) {
					// At the beginning of a line we print the current
					// offset in hex.
					written += printBits(n, 16, false, 8, '0');
					written += term_WriteString(str("  "));
				}

				written += printBits(0xff & buffer[k], 16, false, 2, '0');
				written += term_WriteChar(' ');
				if (used == 7) {
					// There's an additional space after the 8th byte.
					written += term_WriteChar(' ');
				} else if (used == 15) {
					// At the end of the line there's an extra space and
					// the bar for the right column.
					written += term_WriteString(str(" |"));
				}

				n++;
				rightChars[used] = buffer[k];
				if (buffer[k] < 32 || 126 < buffer[k]) {
					rightChars[used] = '.';
				}

				used++;
				n++;
				if (used == 16) {
					rightChars[16] = '|';
					rightChars[17] = '\n';
					written += term_WriteString(right);
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

					written += term_WriteString(spaces);
					rightChars[used] = ' ';
					used++;
				}

				rightChars[16] = '|';
				rightChars[17] = '\n';
				written += term_WriteString(right);
			}

			goto exit_verb;
		}

		// Pointer

		if (c == 'p') {
			if (isUnsigned) {
				written += term_WriteError(str("%!p(UNSIGNED)"));
				goto exit_verb;
			} else if (isSigned) {
				written += term_WriteError(str("%!p(SIGNED)"));
				goto exit_verb;
			} else if (isMemory) {
				written += term_WriteError(str("%!p(MEMORY)"));
				goto exit_verb;
			} else if (addSpace) {
				written += term_WriteError(str("%!p(SPACE)"));
				goto exit_verb;
			} else if (isWidth) {
				written += term_WriteError(str("%!p(WIDTH)"));
				goto exit_verb;
			} else if (isZero) {
				written += term_WriteError(str("%!p(ZERO)"));
				goto exit_verb;
			}

			uintptr v = (uintptr)va_arg(parameters, void*);
			written += term_WriteString(str("0x"));
			written += printBits(v, 16, false, 16, '0');

			goto exit_verb;
		}

		// Unrecognised verb.

		written += term_WriteError(str("%!(BAD_MODIFIER)"));
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
		written += term_WriteError(str("%!(MISSING)"));
	}

	va_end(parameters);
	return written;
}

static const char* smallsString= "00010203040506070809"
	"10111213141516171819"
	"20212223242526272829"
	"30313233343536373839"
	"40414243444546474849"
	"50515253545556575859"
	"60616263646566676869"
	"70717273747576777879"
	"80818283848586878889"
	"90919293949596979899";

static const char* digits_lower = "0123456789abcdefghijklmnopqrstuvwxyz";
static const char* digits_upper = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ";

// printBits is for internal use within std_Printk
// only. The base must be 2, 8, 10, or 16.
//
// printBits returns the number of bytes printed.
//
int64 printBits(uint64 v, uint8 base, bool upper, int32 minWidth, char padChar) {
	char buffer[64+1]; // +1 for sign of 64bit value in base 2.
	int8 i = 65;

	int8 written = 0;
	if (base == 10) {
		while (v >= 100) {
			int64 is = v % 100 * 2;
			v /= 100;
			i -= 2;
			buffer[i+0] = smallsString[is+0];
			buffer[i+1] = smallsString[is+1];
			written += 2;
		}

		// v < 100
		uint64 is = v * 2;
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

		term_WriteString((string){.len=written, .ptr=buffer+i});

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
	uint16 shift;
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
		term_WriteError(err);
		return err.len;
	}

	const char* digits = digits_lower;
	if (upper) {
		digits = digits_upper;
	}

	uint64 b = base;
	uint64 m = (uint64)base - 1; // == 1<<shift - 1
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

	term_WriteString((string){.len=written, .ptr=buffer+i});

	return written;
}
