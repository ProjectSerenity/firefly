# Bootloader

An experimental x86 bootloader written in Ruse and Ruse assembly.

Inspired by the original Firefly booloader, which was forked from the `bootloader` Rust crate at version 0.9.22.

## Design

When you press the power button the computer loads the BIOS from some flash memory stored on the motherboard. The BIOS initializes and self tests the hardware then loads the first 512 bytes into memory from the media device (i.e. the cdrom or floppy disk). If the last two bytes equal 0xAA55 then the BIOS will jump to location 0x7C00 effectively transferring control to the bootloader. At this point the CPU is running in 16 bit mode, meaning only the 16 bit registers are available. Also since the BIOS only loads the first 512 bytes this means our bootloader code has to stay below that limit, otherwise we’ll hit uninitialised memory!

Using [Bios interrupt calls](https://en.wikipedia.org/wiki/BIOS_interrupt_call) the bootloader prints debug information to the screen. For more information on how to write a bootloader click [here](http://3zanders.co.uk/2017/10/13/writing-a-bootloader/).

* stage-1.ruse
This stage initializes the stack, enables the A20 line, loads the rest of the bootloader from disk, and jumps to stage-2.

* stage-2.ruse
This stage sets the target operating mode, loads the kernel from disk, creates an e820 memory map, enters protected mode, and jumps to the third stage.

* stage-3.ruse
This stage performs some checks on the CPU (cpuid, long mode), sets up an initial page table mapping (identity map the bootloader, map the P4 recursively, map the kernel blob to 4MB), enables paging, switches to long mode, and jumps to stage-4.

## License

The original work in the bootloader crate is licensed under the MIT license ([LICENSE.orig](LICENSE.orig) or https://opensource.org/licenses/MIT). Subsequent work is licensed under a BSD 3-clause license ([LICENSE](../LICENSE) or https://opensource.org/licenses/bsd-3-clause).

Unless you explicitly state otherwise, any contribution intentionally submitted for inclusion in the work by you, as defined in the Apache-2.0 license, shall be dual licensed as above, without any additional terms or conditions.
