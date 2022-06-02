# bootloader

An experimental x86 bootloader written in Rust and inline assembly.

Forked from the `bootloader` Rust crate at version 0.9.22.

## Design

When you press the power button the computer loads the BIOS from some flash memory stored on the motherboard.
The BIOS initializes and self tests the hardware then loads the first 512 bytes into memory from the media device
(i.e. the cdrom or floppy disk). If the last two bytes equal 0xAA55 then the BIOS will jump to location 0x7C00 effectively
transferring control to the bootloader. At this point the CPU is running in 16 bit mode,
meaning only the 16 bit registers are available. Also since the BIOS only loads the first 512 bytes this means our bootloader
code has to stay below that limit, otherwise we’ll hit uninitialised memory!

Using [Bios interrupt calls](https://en.wikipedia.org/wiki/BIOS_interrupt_call) the bootloader prints debug information to the screen.
For more information on how to write a bootloader click [here](http://3zanders.co.uk/2017/10/13/writing-a-bootloader/).
The assembler files get imported through the [global_asm feature](https://doc.rust-lang.org/unstable-book/library-features/global-asm.html).
The assembler syntax definition used is the one llvm uses: [GNU Assembly](http://microelectronics.esa.int/erc32/doc/as.pdf).

* stage_1.s
This stage initializes the stack, enables the A20 line, loads the rest of
the bootloader from disk, and jumps to stage_2.

* stage_2.s
This stage sets the target operating mode, loads the kernel from disk,
creates an e820 memory map, enters protected mode, and jumps to the
third stage.

* stage_3.s
This stage performs some checks on the CPU (cpuid, long mode), sets up an
initial page table mapping (identity map the bootloader, map the P4
recursively, map the kernel blob to 4MB), enables paging, switches to long
mode, and jumps to stage_4.


## Build chain
The linker script tells the linker at which offsets the sections should be mapped to. In our case it tells the linker
that the bootloader asm files stage_0-3.s should be mapped to the very beginning of the executable. Read more about linker scripts
[here](https://www.sourceware.org/binutils/docs/ld/Scripts.html).

## Debugging
Set a breakpoint at address `0x7c00`. Disassemble instructions with gdb:
```
(gdb) target remote: 1234
(gdb) b *0x7c00
(gdb) x/i $rip
```

If you use the `-enable-kvm` flag you need to use hardware breakpoints `hb`.

## License

The original work in the bootloader crate is licensed under the MIT license ([LICENSE.orig](LICENSE.orig) or https://opensource.org/licenses/MIT).
Subsequent work is licensed under a BSD 3-clause license ([LICENSE](../LICENSE) or https://opensource.org/licenses/bsd-3-clause).

Unless you explicitly state otherwise, any contribution intentionally submitted for inclusion in the work by you, as defined in the Apache-2.0 license, shall be dual licensed as above, without any additional terms or conditions.
