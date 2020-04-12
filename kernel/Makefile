
# Declare assembler, compiler, and linker flags.
CC = clang

# Expect no OS, 64-bit, build an ELF binary.
_TARGET = --target=x86_64-none-elf
# Expect no OS, 32-bit, build an ELF binary.
_TARGET = --target=i386-none-elf
# Just compile, don't link.
_NOLINK = -c
# Don't expect an existing host.
_NOHOST = -ffreestanding
# Don't include libc.
_NOSTD  = -nostdlib
# Target C99.
_CSTD   = -std=c99
# Enable all warnings.
_WARN   = -Wall -Wextra -Wpedantic
# Make all warnings errors.
_ERROR  = -Werror

ASFLAGS = ${_TARGET} ${_NOLINK}
CCFLAGS = ${_TARGET} ${_NOLINK} ${_NOHOST} ${_CSTD} ${_WARN} ${_ERROR}
LDFLAGS = ${_TARGET} ${_NOHOST} ${_NOSTD}

OBJECTS = init.o kernel.o terminal.o types.o

# Main target.
all: firefly.bin

firefly.bin: ${OBJECTS}
	${CC} ${LDFLAGS} -o firefly.bin -T linker.ld ${OBJECTS}
	grub-file --is-x86-multiboot firefly.bin

# Bootstrap.
init.o: init.s
	${CC} ${ASFLAGS} -o init.o init.s

# Generic rule for C files.
.SUFFIXES: .c .o

.c.o:
	${CC} ${CCFLAGS} -o $@ $<

# Kernel objects.
kernel.o: terminal.h
terminal.o: terminal.h

# Cleanup.
clean:
	rm firefly.bin *.o
