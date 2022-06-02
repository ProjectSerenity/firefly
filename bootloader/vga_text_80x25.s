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
.code16

config_video_mode:
    mov ah, 0
    mov al, 0x03 # 80x25 16 color text
    int 0x10
    ret

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

vga_map_frame_buffer:
    mov eax, 0xa0000
    or eax, (1 | 2)
vga_map_frame_buffer_loop:
    mov ecx, eax
    shr ecx, 12
    mov [_p1 + ecx * 8], eax

    add eax, 4096
    cmp eax, 0xc0000
    jl vga_map_frame_buffer_loop

    ret

# print a string and a newline
# IN
#   esi: points at zero-terminated String
vga_println:
    push eax
    push ebx
    push ecx
    push edx

    call vga_print

    # newline
    mov edx, 0
    mov eax, vga_position
    mov ecx, 80 * 2
    div ecx
    add eax, 1
    mul ecx
    mov vga_position, eax

    pop edx
    pop ecx
    pop ebx
    pop eax

    ret

# print a string
# IN
#   esi: points at zero-terminated String
# CLOBBER
#   ah, ebx
vga_print:
    cld
vga_print_loop:
    # note: if direction flag is set (via std)
    # this will DECREMENT the ptr, effectively
    # reading/printing in reverse.
    lodsb al, BYTE PTR [esi]
    test al, al
    jz vga_print_done
    call vga_print_char
    jmp vga_print_loop
vga_print_done:
    ret


# print a character
# IN
#   al: character to print
# CLOBBER
#   ah, ebx
vga_print_char:
    mov ebx, vga_position
    mov ah, 0x0f
    mov [ebx + 0xb8000], ax

    add ebx, 2
    mov [vga_position], ebx

    ret

vga_position:
    .double 0
