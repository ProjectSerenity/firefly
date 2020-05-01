OBJECTS = bin/pure64.sys bin/mbr.sys bin/pxestart.sys bin/multiboot.sys bin/multiboot2.sys

.PHONY: all
all: ${OBJECTS}

# Generic rule for assembly files.
.SUFFIXES: .asm .sys

.asm.sys:
	mkdir -p bin
	nasm $< -o $@

bin/pure64.sys: src/pure64.asm
bin/mbr.sys: src/bootsectors/mbr.asm
bin/pxestart.sys: src/bootsectors/pxestart.asm
bin/multiboot.sys: src/bootsectors/multiboot.asm
bin/multiboot2.sys: src/bootsectors/multiboot2.asm
