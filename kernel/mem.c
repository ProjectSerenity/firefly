#include "std.h"
#include "mem.h"

// Paging.
//
// Details here copied from Intel® 64 and IA-32 Architectures Software Developer’s Manual,
// Volume 3A, section 4.5 (4-Level Paging), in particular, figure 4-9 and tables 4-14 to 4-17.
// In the combined volume this starts on page 2907.
//
// Paging maps a virtual address (referred to in the Intel manuals as a 'linear address')
// to a physical address, through a series of page tables. Different parts of the virtual
// address reference different tables, as shown below:
//
// 	       6                   5                   4
// 	 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	|            Ignored            |       PML4      |    PDPT     ~
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// 	   3                   2                   1
// 	 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	~   |       PDT       |                 Offset                  |
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Ignored:     Not used during address translation.
// PML4:        Used as an index into the Page Map Level 4 table (9 bits, 0-511).
// PDPT:        Used as an index into the Page Directory Pointer table (9 bits, 0-511).
// PDT:         Used as an index into the Page Directory table (9 bits, 0-511).
// Offset:      Used as an index into the page (21 bits, 2MiB).
//
// A PML4 table comprises 512 64-bit entries (PML4Es)
//
// PML4 entry referencing a PDP entry:
//
// 	       6                   5                   4
// 	 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	|X|          -          |              PDPT Address             ~
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// 	   3                   2                   1
// 	 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	~             PDPT Address              |   -   |S|-|A|C|W|U|R|P|
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// X (Execute disable): Whether the memory is executable (0) or not (1).
// - (Ignored)
// PDPT Address:        The address of the entry in the Page Directory Pointer Table.
// - (Ignored)
// S (Page size):       Must be 0.
// - (Ignored)
// A (Accessed):        Whether the memory has been accessed (1) or not (0).
// C (Cache disable):   Whether the memory has caching enabled (0) or disabled (1).
// W (Write-through):   Whether the memory has write-through caching (1) or write-back (0).
// U (User):            Whether the memory is accessible to userspace.
// R (Read-only):       Whether the memory is read/write (1) or read-only (0).
// P (Present):         Whether this entry is active (1) or absent (0).
//
// A 4-KByte naturally aligned page-directory-pointer table is located at the
// physical address specified in bits 51:12 of the PML4E. A page-directory-pointer
// table comprises 512 64-bit entries (PDPTEs).
//
// PDPT entry referencing a PD entry:
//
// 	       6                   5                   4
// 	 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	|X|          -          |               PD Address              ~
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// 	   3                   2                   1
// 	 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	~               PD Address              |   -   |S|-|A|C|W|U|R|P|
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// X (Execute disable): Whether the memory is executable (0) or not (1).
// - (Ignored)
// PD Address:          The address of the entry in the Page Directory table.
// - (Ignored)
// S (Page size):       Whether the address is for a PD entry (0) or a physical address (1).
// - (Ignored)
// A (Accessed):        Whether the memory has been accessed (1) or not (0).
// C (Cache disable):   Whether the memory has caching enabled (0) or disabled (1).
// W (Write-through):   Whether the memory has write-through caching (1) or write-back (0).
// U (User):            Whether the memory is accessible to userspace.
// R (Read-only):       Whether the memory is read/write (1) or read-only (0).
// P (Present):         Whether this entry is active (1) or absent (0).
//
// Because a PDPTE is identified using bits 47:30 of the linear address, it controls
// access to a 1-GByte region of the linear-address space. Use of the PDPTE depends
// on its PS flag:
//
// - If the PDPTE’s PS flag is 1, the PDPTE maps a 1-GByte page.
// - If the PDPTE’s PS flag is 0, a 4-KByte naturally aligned page directory is
//   located at the physical address specified in bits 51:12 of the PDPTE. A page
//   directory comprises 512 64-bit entries.
//
// PD entry referencing a 2MB page:
//
// 	       6                   5                   4
// 	 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	|X|          -          |              Page Address             ~
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// 	   3                   2                   1
// 	 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// 	~     Page Address    |        -        |  -  |G|S|D|A|C|W|U|R|P|
// 	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// X (Execute disable): Whether the memory is executable (0) or not (1).
// - (Ignored)
// Page Address:        The address of the page.
// - (Ignored)
// G (Global):          Whether the translation is global (1) or not (0).
// S (Page size):       Whether the address is for a PT entry (0) or a physical address (1).
// D (Dirty):           Whether the memory has been written (1) or not (0).
// A (Accessed):        Whether the memory has been accessed (1) or not (0).
// C (Cache disable):   Whether the memory has caching enabled (0) or disabled (1).
// W (Write-through):   Whether the memory has write-through caching (1) or write-back (0).
// U (User):            Whether the memory is accessible to userspace.
// R (Read-only):       Whether the memory is read/write (1) or read-only (0).
// P (Present):         Whether this entry is active (1) or absent (0).
//
// Because a PDE is identified using bits 47:21 of the linear address, it
// controls access to a 2-MByte region of the linear-address space. Use of
// the PDE depends on its PS flag:
//
// - If the PDE's PS flag is 1, the PDE maps a 2-MByte page.
// - If the PDE’s PS flag is 0, a 4-KByte naturally aligned page table is
//   located at the physical address specified in bits 51:12 of the PDE.
//   A page table comprises 512 64-bit entries.

// The text above describes the behaviour of the CPU, according to the
// Intel manual. In addition to this, Firefly uses bits 62-53 of each
// PML4E and PDPTE to store the number of entries in the table referenced
// that are currently present. For example, PML4[0] will contain the
// address of a PDPT. Bits 62-53 of PML4[0] will show how many of the 512
// entries in the PDPT are currently present. This enables the efficent
// optimisation of marking the entire PDPT absent in the PML4E if no PDPT
// entries are present.
//
// These values are initialised in mem_Init and maintained as new pages are
// added.

// Paging starting point.
uintptr* PML4 = (uintptr*)(uintptr)0x2000;

// Page entries per table.
const uintptr PAGE_ENTRIES_PER_TABLE = 512;

// Paging flags.
const uintptr PAGE_FLAG_PRESENT         = 1 << 0;
const uintptr PAGE_FLAG_ABSENT          = 0 << 0;
const uintptr PAGE_FLAG_READ_WRITE      = 1 << 1;
const uintptr PAGE_FLAG_READ_ONLY       = 0 << 1;
const uintptr PAGE_FLAG_USERSPACE       = 1 << 2;
const uintptr PAGE_FLAG_KERNEL_ONLY     = 0 << 2;
const uintptr PAGE_FLAG_WRITE_THROUGH   = 1 << 3;
const uintptr PAGE_FLAG_WRITE_BACK      = 0 << 3;
const uintptr PAGE_FLAG_CACHE_DISABLED  = 1 << 4;
const uintptr PAGE_FLAG_CACHE_ENABLED   = 0 << 4;
const uintptr PAGE_FLAG_ACCESSED        = 1 << 5;
const uintptr PAGE_FLAG_UNACCESSED      = 0 << 5;
const uintptr PAGE_FLAG_MODIFIED        = 1 << 6;
const uintptr PAGE_FLAG_UNMODIFIED      = 0 << 6;
const uintptr PAGE_FLAG_LARGE_PAGE_SIZE = 1 << 7;
const uintptr PAGE_FLAG_SMALL_PAGE_SIZE = 0 << 7;
const uintptr PAGE_FLAG_GLOBAL          = 1 << 8;
const uintptr PAGE_FLAG_LOCAL           = 0 << 8;
const uintptr PAGE_FLAG_NOT_EXECUTABLE  = (uintptr)1 << 63;
const uintptr PAGE_FLAG_EXECUTABLE      = (uintptr)0 << 63;

// Common offsets.
const uintptr MASK_BITS_51_TO_12 = 0x000FFFFFFFFFF000;
const uintptr MASK_BITS_47_TO_21 = 0x0000FFFFFFE00000;
const uintptr MASK_BITS_20_TO_0  = 0x00000000001FFFFF;

void mem_Init() {
	// Initialise the page table entries count.
	uintptr i, j, k;
	uint64 PDPTEs, PDTEs;
	for (i = 0; i < PAGE_ENTRIES_PER_TABLE; i++) {
		uintptr pml4e = PML4[i];
		if ((pml4e & PAGE_FLAG_PRESENT) == 0) {
			// Not present.
			continue;
		}

		PDPTEs = 0;
		uintptr* pdpt = (uintptr*)(MASK_BITS_51_TO_12 & pml4e);
		for (j = 0; j < PAGE_ENTRIES_PER_TABLE; j++) {
			uintptr pdpte = pdpt[j];
			if (pdpte & PAGE_FLAG_PRESENT) {
				PDPTEs++;
			}

			PDTEs = 0;
			uintptr* pdt = (uintptr*)(MASK_BITS_51_TO_12 & pdpte);
			for (k = 0; k < PAGE_ENTRIES_PER_TABLE; k++) {
				uintptr pde = pdt[k];
				if (pde & PAGE_FLAG_PRESENT) {
					PDTEs++;
				}
			}

			if (PDTEs > 0) {
				pdpt[j] = (PDTEs << 53) | pdpte;
			}
		}

		if (PDPTEs > 0) {
			PML4[i] = (PDPTEs << 53) | pml4e;
		}
	}
}

void mem_DebugPaging(uint64 maxPagesPrinted) {
	std_Printk("mem_DebugPaging start\n");
	uintptr i, j, k;
	uintptr virtualStart, virtualEnd;
	uintptr prevVirtualStart = 0;
	uintptr prevVirtualEnd = 0;
	uintptr prevPageStart = 0;
	uintptr prevPageEnd = 0;
	uint64 pages = 0;
	for (i = 0; i < PAGE_ENTRIES_PER_TABLE; i++) {
		uintptr pml4e = PML4[i];
		if ((pml4e & PAGE_FLAG_PRESENT) == 0) {
			if (i == 0) {
				std_Printk("PML4E 0 not present\n");
			}

			// Not present.
			continue;
		}

		uintptr* pdpt = (uintptr*)(MASK_BITS_51_TO_12 & pml4e);
		for (j = 0; j < PAGE_ENTRIES_PER_TABLE; j++) {
			uintptr pdpte = pdpt[j];
			if ((pdpte & PAGE_FLAG_PRESENT) == 0) {
				// Not present.
				continue;
			}

			uintptr* pdt = (uintptr*)(MASK_BITS_51_TO_12 & pdpte);
			for (k = 0; k < PAGE_ENTRIES_PER_TABLE; k++) {
				uintptr pde = pdt[k];
				if ((pde & PAGE_FLAG_PRESENT) == 0) {
					// Not present.
					continue;
				}

				if ((pde & PAGE_FLAG_LARGE_PAGE_SIZE) == 0) {
					// Page size 5kB.
					std_Printk("Page %u64d/%u64d/%u64d has S bit unset\n", i, j, k);
					continue;
				}

				// Bits 47-21.
				uintptr pageStart = (uintptr)(MASK_BITS_47_TO_21 & pde);
				uintptr pageEnd = pageStart | MASK_BITS_20_TO_0; // Max offset.

				virtualStart =
					((0x1ff & i) << 39) |    // PML4
					((0x1ff & j) << 30) |    // PDPT
					((0x1ff & k) << 21);     // PDT
				virtualEnd = virtualStart | MASK_BITS_20_TO_0; // Max offset.

				if (prevPageEnd > 0) {
					if ((prevPageEnd+1) == pageStart &&
						(prevVirtualEnd+1) == virtualStart) {
						prevPageEnd = pageEnd;
						prevVirtualEnd = virtualEnd;
						continue;
					}

					pages++;
					if (pages < maxPagesPrinted) {
						std_Printk("Page map virtual addresses %p-%p to physical addresses %p-%p\n", prevVirtualStart, prevVirtualEnd, prevPageStart, prevPageEnd);
					} else if (pages == maxPagesPrinted) {
						std_Printk("Stopping after %u64d pages printed.\n", maxPagesPrinted);
					}
				}

				prevPageStart = pageStart;
				prevPageEnd = pageEnd;
				prevVirtualStart = virtualStart;
				prevVirtualEnd = virtualEnd;
			}
		}
	}

	if (prevPageStart > 0) {
		pages++;
		std_Printk("Page map virtual addresses %p-%p to physical addresses %p-%p\n", prevVirtualStart, prevVirtualEnd, prevPageStart, prevPageEnd);
	}

	std_Printk("%u64d contiguous page mappings.\n", pages);

	std_Printk("mem_DebugPaging end\n");
}
