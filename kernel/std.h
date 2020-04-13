#pragma once

typedef unsigned char uint8;
typedef unsigned short uint16;
typedef unsigned int uint32;
typedef unsigned long uint64;
typedef unsigned int uint;

typedef signed char int8;
typedef signed short int16;
typedef signed int int32;
typedef signed long int64;

const int nil;

void copy(char* dst, char* src, uint n);

typedef struct string {
	int len;
	const char* ptr;
} string;

string S(const char* data);
