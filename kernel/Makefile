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
_WARN      = -Weverything -Wno-unused-const-variable -Wno-unused-function -Wno-missing-noreturn
_ERROR     = -Werror

CCFLAGS = ${_TARGET} ${_NOLINK} ${_NOPIE} ${_NORED} ${_NOSTACK} ${_NOHOST} ${_CSTD} ${_NOCOLOR} ${_WARN} ${_ERROR}

OBJECTS = init.o kernel.o cpu.o int.o mem.o mmio.o ports.o std.o terminal.o

# Main target.
.PHONY: all
all: firefly.img

firefly.img: firefly.bin linker.ld pure64.sys mbr.sys imgbuild
	./imgbuild -mbr mbr.sys -loader pure64.sys -kernel firefly.bin -o firefly.img

firefly.bin: kcheck ${OBJECTS}
	./kcheck headers *.h
	ld -T linker.ld -o firefly.bin ${OBJECTS}

# Generic rule for C and assembly files.
.SUFFIXES: .c .o

.c.o:
	./kcheck object $<
	${CC} ${CCFLAGS} -o $@ $<

# Kernel objects.
cpu.o: kcheck cpu.c std.h cpu.h
init.o: kcheck init.c
kernel.o: kcheck kernel.c std.h cpu.h mem.h terminal.h
int.o: kcheck int.c std.h int.h ports.h
mem.o: kcheck mem.c std.h mem.h
mmio.o: kcheck mmio.c std.h mmio.h
pci.o: kcheck pci.c std.h pci.h ports.h
ports.o: kcheck ports.c std.h ports.h
std.o: kcheck std.c std.h
terminal.o: kcheck terminal.c font.h std.h terminal.h

# Dependencies.
kcheck: $(wildcard ../kcheck/*.go) ../kcheck/go.mod ../kcheck/go.sum $(wildcard ../kcheck/cc/*.go)
	cd ../kcheck && go test
	cd ../kcheck && go build -o ../kernel/kcheck

imgbuild: $(wildcard ../imgbuild/*.go) ../imgbuild/go.mod ../imgbuild/go.sum
	cd ../imgbuild && go test
	cd ../imgbuild && go build -o ../kernel/imgbuild

pure64.sys: ../Pure64/src/pure64.asm
	cd ../Pure64/src && nasm pure64.asm -o ../../kernel/pure64.sys

mbr.sys: ../Pure64/src/bootsectors/mbr.asm
	cd ../Pure64/src/bootsectors && nasm mbr.asm -o ../../../kernel/mbr.sys

# Cleanup.
.PHONY: clean
clean:
	rm firefly.img firefly.bin *.o imgbuild kcheck pure64.sys mbr.sys
