# Declare assembler, compiler, and linker flags.
CC = clang

_TARGET    = --target=x86_64-none-elf
_NOLINK    = -c
_NOHOST    = -ffreestanding
_NOPIE     = -fno-PIE
_NORED     = -mno-red-zone
_NOSTACK   = -fno-stack-protector -fomit-frame-pointer
_CSTD      = -std=c99
_WARN      = -Wall -Wextra -Wpedantic
_ERROR     = -Werror

CCFLAGS = ${_TARGET} ${_NOLINK} ${_NOPIE} ${_NORED} ${_NOSTACK} ${_NOHOST} ${_CSTD} ${_WARN} ${_ERROR}

OBJECTS = init.o kernel.o cpu.o mmio.o ports.o std.o terminal.o

# Main target.
.PHONY: all
all: firefly.bin
	cat ../Pure64/bin/pure64.sys firefly.bin > kernel.sys
	dd if=/dev/zero of=firefly.img count=128 bs=1048576
	dd if=../Pure64/bin/mbr.sys of=firefly.img conv=notrunc
	dd if=kernel.sys of=firefly.img bs=512 seek=16 conv=notrunc

firefly.bin: ${OBJECTS}
	kcheck headers *.h
	ld -T linker.ld -o firefly.bin ${OBJECTS}

# Generic rule for C files.
.SUFFIXES: .c .o

.c.o:
	# kcheck object $<
	${CC} ${CCFLAGS} -o $@ $<

# Kernel objects.
cpu.o: std.h cpu.h
init.o:
kernel.o: std.h terminal.h
mmio.o: std.h mmio.h
ports.o: std.h ports.h
std.o: std.h
terminal.o: font.h std.h terminal.h

# Cleanup.
.PHONY: clean
clean:
	rm firefly.bin *.o
