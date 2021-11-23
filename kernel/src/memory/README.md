# Memory layout

Firefly uses the following layout of virtual memory:

| Region              |        Start address |         Last address |           Pages |    Size |
| ------------------- | -------------------: | -------------------: | --------------: | ------: |
| Kernel constants    |           `0x200000` |           `0x205fff` |   6x 4 kiB page |  24 kiB |
| Kernel code (.text) |           `0x206000` |           `0x23efff` |  57x 4 kiB page | 228 kiB |
| Kernel strings      |           `0x23f000` |           `0x241fff` |   3x 4 kiB page |  12 kiB |
| Kernel static data  |           `0x242000` |           `0x249fff` |   8x 4 kiB page |  32 kiB |
| Bootloader info     |     `0x444444433000` |     `0x444444433fff` |   1x 4 kiB page |   4 kiB |
| Kernel heap         |     `0x444444440000` |     `0x444444458fff` |  25x 4 kiB page | 100 kiB |
| Kernel stack        |     `0x777777771000` |     `0x777777970fff` | 512x 4 kiB page |   2 MiB |
| Physical memory map | `0xffff800000000000` | `0xffffffffffffffff` |  rest of memory | 128 TiB |
