#pragma once
#ifndef STD_H
#define STD_H

typedef unsigned char bool;
const bool true;
const bool false;

typedef unsigned char uint8;
typedef unsigned short uint16;
typedef unsigned int uint32;
typedef unsigned long uint64;

typedef signed char int8;
typedef signed short int16;
typedef signed int int32;
typedef signed long int64;

typedef unsigned long uintptr;
typedef float float32;
typedef double float64;

bool validNumericalTypes();

const void* nil;

void copy(char* dst, char* src, uint64 n);

typedef struct string {
	int64 len;
	const char* ptr;
} string;

// This macro is what is actually used to convert
// string literals to the string type.
//
#define str(s) ((string){.ptr=s, .len=sizeof(s)-1})

// The cc parser ignores macros, so we use this
// function declaration to allow cc to type-check
// the macro. The function is not implemented, as
// the macro is used instead.
//
#if 0
string str(char s[]);
#endif

int64 std_Printk(char format[], ...);

#endif // STD_H
