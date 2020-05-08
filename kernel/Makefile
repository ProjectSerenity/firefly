# Declare assembler, compiler, and linker flags.
CC = clang

_TARGET    = --target=x86_64-none-elf
_NOLINK    = -c
_NOHOST    = -ffreestanding
_NOPIE     = -fno-PIE
_NORED     = -mno-red-zone
_NOSTACK   = -fno-stack-protector -fomit-frame-pointer
_NOCOLOR   = -fno-color-diagnostics
_CSTD      = -std=c99
_WARN      = -Weverything -Wno-unused-const-variable -Wno-unused-function
_ERROR     = -Werror

CCFLAGS = ${_TARGET} ${_NOLINK} ${_NOPIE} ${_NORED} ${_NOSTACK} ${_NOHOST} ${_CSTD} ${_NOCOLOR} ${_WARN} ${_ERROR}

OBJECTS = init.o kernel.o cpu.o mem.o mmio.o ports.o std.o terminal.o

KCHECK = ../kcheck/kcheck
IMGBUILD = ../imgbuild/imgbuild
MBR = ../Pure64/bin/mbr.sys
LOADER = ../Pure64/bin/pure64.sys

# Main target.
.PHONY: all
all: firefly.img

firefly.img: firefly.bin ${LOADER} ${MBR}
	cat ${LOADER} firefly.bin > kernel.sys
	dd if=/dev/zero of=firefly.img count=128 bs=1048576
	dd if=${MBR} of=firefly.img conv=notrunc
	dd if=kernel.sys of=firefly.img bs=512 seek=16 conv=notrunc

firefly.bin: ${KCHECK} ${OBJECTS}
	${KCHECK} headers *.h
	ld -T linker.ld -o firefly.bin ${OBJECTS}

# Generic rule for C files.
.SUFFIXES: .c .o

.c.o:
	${KCHECK} object $<
	${CC} ${CCFLAGS} -o $@ $<

# Kernel objects.
cpu.o: ${KCHECK} cpu.c std.h cpu.h
init.o: ${KCHECK} init.c
kernel.o: ${KCHECK} kernel.c std.h cpu.h mem.h terminal.h
mem.o: ${KCHECK} mem.c std.h mem.h
mmio.o: ${KCHECK} mmio.c std.h mmio.h
pci.o: ${KCHECK} pci.c std.h pci.h ports.h
ports.o: ${KCHECK} ports.c std.h ports.h
std.o: ${KCHECK} std.c std.h
terminal.o: ${KCHECK} terminal.c font.h std.h terminal.h

# Cleanup.
.PHONY: clean
clean:
	rm firefly.img firefly.bin *.o
