# Declare assembler, compiler, and linker flags.
CC = clang

_TARGET    = --target=i386-none-elf
_NOLINK    = -c
_NOHOST    = -ffreestanding
_NOSTD     = -nostdlib
_NOPIE     = -fno-PIE
_CSTD      = -std=c99
_WARN      = -Wall -Wextra -Wpedantic
_ERROR     = -Werror

ASFLAGS = ${_TARGET} ${_NOLINK}
CCFLAGS = ${_TARGET} ${_NOLINK} ${_NOPIE} ${_NOHOST} ${_CSTD} ${_WARN} ${_ERROR}
LDFLAGS = ${_TARGET} ${_NOHOST} ${_NOSTD}

OBJECTS = init.o kernel.o terminal.o std.o

# Main target.
.PHONY: all
all: firefly.bin

firefly.bin: ${OBJECTS}
	kcheck headers *.h
	${CC} ${LDFLAGS} -T linker.ld -o firefly.bin ${OBJECTS}
	grub-file --is-x86-multiboot firefly.bin

# Bootstrap.
init.o: init.s
	${CC} ${ASFLAGS} -o init.o init.s

# Generic rule for C files.
.SUFFIXES: .c .o

.c.o:
	kcheck object $<
	${CC} ${CCFLAGS} -o $@ $<

# Kernel objects.
kernel.o: terminal.h
terminal.o: terminal.h

# Cleanup.
.PHONY: clean
clean:
	rm firefly.bin *.o
