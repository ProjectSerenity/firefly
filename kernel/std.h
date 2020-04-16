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

typedef unsigned int uint;
typedef unsigned long uintptr;
typedef float float32;
typedef double float64;

const void* nil;

void copy(char* dst, char* src, uint n);

typedef struct string {
	int len;
	const char* ptr;
} string;

#define str(s) ((string){.ptr=s, .len=sizeof(s)-1})

int printk(char format[], ...);

#endif // STD_H
