#include "std.h"

const bool true = 1;
const bool false = 0;

const void* nil = 0;

void copy(char* dst, char* src, uint n) {
	for (uint i = 0; i < n; i++) {
		dst[i] = src[i];
	}
}
