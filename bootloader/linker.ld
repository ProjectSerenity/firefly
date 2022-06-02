# Forked from bootloader 0.9.22, copyright 2018 Philipp Oppermann.
#
# Use of the original source code is governed by the MIT
# license that can be found in the LICENSE.orig file.
#
# Subsequent work copyright 2022 The Firefly Authors.
#
# Use of new and modified source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

ENTRY(_start)

SECTIONS {
    . = 0x500;
    /* buffer for loading the kernel */
    _kernel_buffer = .;
    . += 512;
    /* page tables */
    . = ALIGN(0x1000);
    __page_table_start = .;
    _p4 = .;
    . += 0x1000;
    _p3 = .;
    . += 0x1000;
    _p2 = .;
    . += 0x1000;
    _p1 = .;
    . += 0x1000;
    __page_table_end = .;
    __bootloader_start = .;
    _memory_map = .;
    . += 0x1000;

    _stack_start = .;
    . = 0x7c00;
    _stack_end = .;

    .bootloader :
    {
        /* first stage */
        *(.boot-first-stage)

        /* rest of bootloader */
        _rest_of_bootloader_start_addr = .;
        _kernel_size_addr = .;
        . += 4; /* Leave space for the kernel size. */
        *(.boot)
        *(.context_switch)
        *(.text .text.*)
        *(.rodata .rodata.*)
        *(.data .data.*)
        *(.got)
        . = ALIGN(512);
        _rest_of_bootloader_end_addr = .;
        __bootloader_end = .;
        _kernel_start_addr = .;
    }

    /DISCARD/ :
    {
        *(.eh_frame*)
    }
}
