# Memory layout

Firefly uses the following layout of virtual memory:

| Region              |           Start address |            Last address |                 Pages |      Size |
| ------------------- | ----------------------: | ----------------------: | --------------------: | --------: |
| NULL page           |                   `0x0` |             `0x1f_ffff` |            not mapped |     2 MiB |
| Userspace           |             `0x20_0000` |      `0x7fff_ffff_ffff` |        rest of memory | < 128 TiB |
| Kernel binary       | `0xffff_8000_0000_0000` | `0xffff_8000_3fff_ffff` | up to 512x 2 MiB page |     1 GiB |
| Bootloader info     | `0xffff_8000_4000_0000` | `0xffff_8000_4000_0fff` |         1x 4 KiB page |     4 KiB |
| Kernel heap         | `0xffff_8000_4444_0000` | `0xffff_8000_444b_ffff` |       128x 4 KiB page |   512 KiB |
| Kernel stack guard  | `0xffff_8000_5554_f000` | `0xffff_8000_5554_ffff` |            not mapped |     4 KiB |
| Kernel stack        | `0xffff_8000_5555_0000` | `0xffff_8000_555c_ffff` |       128x 4 KiB page |   512 KiB |
| Physical memory map | `0xffff_8000_6000_0000` | `0xffff_ffff_ffff_ffff` |        rest of memory | < 128 TiB |
