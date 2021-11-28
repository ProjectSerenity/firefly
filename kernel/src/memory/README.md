# Memory layout

Firefly uses the following layout of virtual memory:

| Region               |           Start address |            Last address |                 Pages |      Size |
| -------------------- | ----------------------: | ----------------------: | --------------------: | --------: |
| NULL page            |                   `0x0` |             `0x1f_ffff` |            not mapped |     2 MiB |
| Userspace            |             `0x20_0000` |      `0x7fff_ffff_ffff` |        rest of memory | < 128 TiB |
| Kernel binary        | `0xffff_8000_0000_0000` | `0xffff_8000_3fff_ffff` | up to 512x 2 MiB page |     1 GiB |
| Bootloader info      | `0xffff_8000_4000_0000` | `0xffff_8000_4000_0fff` |         1x 4 KiB page |     4 KiB |
| Kernel heap          | `0xffff_8000_4444_0000` | `0xffff_8000_444b_ffff` |       128x 4 KiB page |   512 KiB |
| Kernel stack 0 guard | `0xffff_8000_5554_f000` | `0xffff_8000_5554_ffff` |            not mapped |     4 KiB |
| Kernel stack 0       | `0xffff_8000_5555_0000` | `0xffff_8000_555c_ffff` |       128x 4 KiB page |   512 KiB |
| Kernel stacks 1+     | `0xffff_8000_555d_0000` | `0xffff_8000_5d5c_ffff` |    32,768x 4 KiB page |   128 MiB |
| MMIO address space   | `0xffff_8000_6666_0000` | `0xffff_8000_6675_ffff` |       256x 4 KiB page |     1 MiB |
| Physical memory map  | `0xffff_8000_8000_0000` | `0xffff_ffff_ffff_ffff` |        rest of memory | < 128 TiB |
