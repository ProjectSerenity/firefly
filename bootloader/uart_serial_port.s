# Forked from bootloader 0.9.22, copyright 2018 Philipp Oppermann.
#
# Use of the original source code is governed by the MIT
# license that can be found in the LICENSE.orig file.
#
# Subsequent work copyright 2022 The Firefly Authors.
#
# Use of new and modified source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

.section .boot, "awx"
.code32

# Print a string and a newline to
# the serial port.
# IN
#   esi: Points to a NUL-terminated string.
#
uart_println:
    push eax
    push edx

    call uart_print

    mov al, 13 # \r
    call uart_print_char
    mov al, 10 # \n
    call uart_print_char

    pop edx
    pop eax

    ret

# Print a string to the serial port.
# IN
#   esi: Points to a NUL-terminated string.
# CLOBBER
#   ax
#
uart_print:
    cld
uart_print_loop:
    # Note: if direction flag is set (via std)
    # this will DECREMENT the ptr, effectively
    # reading/printing in reverse.
    lodsb al, BYTE PTR [si]
    test al, al
    jz uart_print_done
    call uart_print_char
    jmp uart_print_loop
uart_print_done:
    ret

# Print a character to the serial port.
# IN
#   al: Character to print.
#
uart_print_char:
    push dx                  # Save DX.
    push ax                  # Save the character.
uart_print_char_loop:
    mov dx, 0x03fd           # Line status register.
    in al, dx
    test al, 0x20            # Use bit 5 to see if the transmitter holding register is clear.
    jz uart_print_char_loop  # Loop until it's clear and ready to transmit.

    pop ax                   # Restore the character.
    mov dx, 0x03f8           # Data register.
    out dx, al               # Write the character.
    pop dx                   # Restore DX.
    ret
