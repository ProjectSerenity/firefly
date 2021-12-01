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
| CPU-local storage    | `0xffff_8000_7777_0000` | `0xffff_8000_7f76_ffff` |    32,768x 4 KiB page |   128 MiB |
| Physical memory map  | `0xffff_8000_8000_0000` | `0xffff_ffff_ffff_ffff` |        rest of memory | < 128 TiB |

## Paging

The following high-level discussion of 4-level paging may be helpful:

Paging maps a virtual address (referred to in the Intel manuals as a 'linear address')
to a physical address, through a series of page tables. Different parts of the virtual
address reference different tables, as shown below:

```
	       6                   5                   4
	 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|            Ignored            |       PML4      |    PDPT     ~
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

	   3                   2                   1
	 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	~   |       PDT       |      Table      |         Offset        |
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

Ignored:     Not used during address translation.
PML4:        Used as an index into the Page Map Level 4 table (9 bits, 0-511).
PDPT:        Used as an index into the Page Directory Pointer table (9 bits, 0-511).
PDT:         Used as an index into the Page Directory table (9 bits, 0-511).
PT:          Used as an index into the Page table (9 bits, 0-511).
Offset:      Used as an index into the page (12 bits, 4kB).
```

A PML4 table comprises 512 64-bit entries (PML4Es)

PML4 entry referencing a PDP entry:

```
	       6                   5                   4
	 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|X|          -          |              PDPT Address             ~
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

	   3                   2                   1
	 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	~             PDPT Address              |   -   |S|-|A|C|W|U|R|P|
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

X (Execute disable): Whether the memory is executable (0) or not (1).
- (Ignored)
PDPT Address:        The address of the entry in the Page Directory Pointer Table.
- (Ignored)
S (Page size):       Must be 0.
- (Ignored)
A (Accessed):        Whether the memory has been accessed (1) or not (0).
C (Cache disable):   Whether the memory has caching enabled (0) or disabled (1).
W (Write-through):   Whether the memory has write-through caching (1) or write-back (0).
U (User):            Whether the memory is accessible to userspace.
R (Read-only):       Whether the memory is read/write (1) or read-only (0).
P (Present):         Whether this entry is active (1) or absent (0).
```

A 4-KByte naturally aligned page-directory-pointer table is located at the
physical address specified in bits 51:12 of the PML4E. A page-directory-pointer
table comprises 512 64-bit entries (PDPTEs).

PDPT entry referencing a PD entry:

```
	       6                   5                   4
	 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|X|          -          |               PD Address              ~
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

	   3                   2                   1
	 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	~               PD Address              |   -   |S|-|A|C|W|U|R|P|
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

X (Execute disable): Whether the memory is executable (0) or not (1).
- (Ignored)
PD Address:          The address of the entry in the Page Directory table.
- (Ignored)
S (Page size):       Whether the address is for a PD entry (0) or a physical address (1).
- (Ignored)
A (Accessed):        Whether the memory has been accessed (1) or not (0).
C (Cache disable):   Whether the memory has caching enabled (0) or disabled (1).
W (Write-through):   Whether the memory has write-through caching (1) or write-back (0).
U (User):            Whether the memory is accessible to userspace.
R (Read-only):       Whether the memory is read/write (1) or read-only (0).
P (Present):         Whether this entry is active (1) or absent (0).
```

Because a PDPTE is identified using bits 47:30 of the linear address, it controls
access to a 1-GByte region of the linear-address space. Use of the PDPTE depends
on its PS flag:

- If the PDPTE’s PS flag is 1, the PDPTE maps a 1-GByte page.
- If the PDPTE’s PS flag is 0, a 4-KByte naturally aligned page directory is
  located at the physical address specified in bits 51:12 of the PDPTE. A page
  directory comprises 512 64-bit entries.

PD entry referencing a 2MB page:

```
	       6                   5                   4
	 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|X|          -          |               PT Address              ~
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

	   3                   2                   1
	 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	~               PT Address              |   -   |S|-|A|C|W|U|R|P|
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

X (Execute disable): Whether the memory is executable (0) or not (1).
- (Ignored)
PT Address:          The address of the page table.
- (Ignored)
S (Page size):       Whether the address is for a PT entry (0) or a physical address (1).
- (Ignored)
A (Accessed):        Whether the memory has been accessed (1) or not (0).
C (Cache disable):   Whether the memory has caching enabled (0) or disabled (1).
W (Write-through):   Whether the memory has write-through caching (1) or write-back (0).
U (User):            Whether the memory is accessible to userspace.
R (Read-only):       Whether the memory is read/write (1) or read-only (0).
P (Present):         Whether this entry is active (1) or absent (0).
```

Because a PDE is identified using bits 47:21 of the linear address, it
controls access to a 2-MByte region of the linear-address space. Use of
the PDE depends on its PS flag:

- If the PDE's PS flag is 1, the PDE maps a 2-MByte page.
- If the PDE’s PS flag is 0, a 4-KByte naturally aligned page table is
  located at the physical address specified in bits 51:12 of the PDE.
  A page table comprises 512 64-bit entries.

PT entry referencing a 4kB page:

```
	       6                   5                   4
	 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|X|          -          |              Page Address             ~
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

	   3                   2                   1
	 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	~              Page Address             |  -  |G|S|-|A|C|W|U|R|P|
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

X (Execute disable): Whether the memory is executable (0) or not (1).
- (Ignored)
PT Address:          The address of the page table.
- (Ignored)
G (Global):          Whether to flush the TLB cache when changing mappings.
S (Page size):       Whether the address is for a PT entry (0) or a physical address (1).
D (Dirty):           Whether the memory has been written (1) or not (0).
A (Accessed):        Whether the memory has been accessed (1) or not (0).
C (Cache disable):   Whether the memory has caching enabled (0) or disabled (1).
W (Write-through):   Whether the memory has write-through caching (1) or write-back (0).
U (User):            Whether the memory is accessible to userspace.
R (Read-only):       Whether the memory is read/write (1) or read-only (0).
P (Present):         Whether this entry is active (1) or absent (0).
```

Because a PTE is identified using bits 47:21 of the linear address, it
controls access to a 4-kByte region of the linear-address space.
