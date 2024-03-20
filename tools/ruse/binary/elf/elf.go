// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package elf encodes Ruse binaries according to the ELF
// format.
package elf

import (
	"bytes"
	gobinary "encoding/binary"
	"fmt"
	"io"

	"firefly-os.dev/tools/ruse/binary"
	"firefly-os.dev/tools/ruse/sys"
)

// Encode writes the binary to w as an ELF binary.
//
// Encode also updates the section offsets, taking
// into account any necessary padding.
func Encode(w io.Writer, bin *binary.Binary) error {
	write := false
	b, ok := w.(*bytes.Buffer)
	if !ok {
		write = true
		b = new(bytes.Buffer)
	}

	var err error
	switch bin.Arch.PointerSize {
	case 8:
		err = encode64(b, bin)
	default:
		return fmt.Errorf("elf: %d-bit binaries are not supported", 8*bin.Arch.PointerSize)
	}

	if write && err == nil {
		_, err = w.Write(b.Bytes())
	}

	return err
}

func endianness(bo gobinary.ByteOrder) uint8 {
	switch bo.String() {
	case "LittleEndian":
		return 1
	case "BigEndian":
		return 2
	}

	return 0
}

func arch2machine(arch *sys.Arch) uint16 {
	switch arch {
	case sys.X86:
		return 0x03
	case sys.X86_64:
		return 0x3e
	}

	return 0
}

func progPermissions(perm binary.Permissions) uint32 {
	var out uint32
	if perm.Read() {
		out |= 0x04
	}
	if perm.Write() {
		out |= 0x02
	}
	if perm.Execute() {
		out |= 0x01
	}

	return out
}

func sectPermissions(perm binary.Permissions) uint64 {
	var out uint64
	if perm.Write() {
		out |= 0x01
	}
	if perm.Execute() {
		out |= 0x04
	}

	return out
}

func symbolData(sym *binary.Symbol) (typ uint8, value, size uint64) {
	switch sym.Kind {
	case binary.SymbolFunction:
		return 0x02, uint64(sym.Address), uint64(sym.Length)
	case binary.SymbolArray, binary.SymbolBool, binary.SymbolInteger, binary.SymbolString:
		return 0x01, uint64(sym.Address), uint64(sym.Length)
	default:
		panic(fmt.Errorf("symbol %q: invalid symbol kind %d", sym.Name, sym.Kind))
	}
}

func encode64(b *bytes.Buffer, bin *binary.Binary) error {
	// See https://en.wikipedia.org/wiki/Executable_and_Linkable_Format
	bo := bin.Arch.ByteOrder
	write := func(data any) {
		gobinary.Write(b, bo, data)
	}

	// Check the entry point is an
	// existing function.
	if bin.Entry == nil {
		return fmt.Errorf("no entry point symbol")
	}
	if bin.Entry.Kind != binary.SymbolFunction {
		return fmt.Errorf("entry point is %s symbol, want function", bin.Entry.Kind)
	}

	const (
		// Size constants.
		pageSize       = 0x1000 // 4kB page size in bytes.
		elfHeaderSize  = 0x40   // ELF header size in bytes.
		progHeaderSize = 0x38   // Program header size in bytes.
		sectHeaderSize = 0x40   // Section header size in bytes.
		symtabSize     = 24     // Symbol table entry size in bytes.

		// Section names table.
		shstrtabName  = "section names"
		symtabName    = "symbol table"
		symstrtabName = "symbol names"

		// Value constants.
		ET_EXEC      = 0x02
		PT_LOAD      = 0x01
		SHT_PROGBITS = 0x01
		SHT_SYMTAB   = 0x02
		SHT_STRTAB   = 0x03
		SHF_ALLOC    = 0x02
		SHF_STRINGS  = 0x20
		STB_GLOBAL   = 0x10
		STV_DEFAULT  = 0x00
	)

	nextPage := func(offset uint64) uint64 {
		// Round the offset up to the start
		// of the next 4kB page.
		remainder := offset % pageSize
		topUp := pageSize - remainder
		return offset + topUp
	}

	// Copy the section table in case
	// we need to add to it.
	sections := bin.Sections

	// Build the symbol table.
	var symtab, symstrtab *binary.Section
	symbolAddrs := make([]int, len(bin.Symbols))
	if bin.SymbolTable {
		anonymousStrings := 0
		var symtabData, symstrtabData bytes.Buffer
		symtabData.Write(make([]byte, symtabSize)) // Add the empty symbol.
		symstrtabData.WriteByte(0)                 // Add the null terminator for the empty string.
		for i, sym := range bin.Symbols {
			// Start with the index into symstrtab
			// where the name begins as a null-terminated
			// string.
			if sym.Name == "" {
				gobinary.Write(&symtabData, bo, uint32(0))
			} else if sym.Name[0] == '.' {
				anonymousStrings++
				gobinary.Write(&symtabData, bo, uint32(symstrtabData.Len()))
				symstrtabData.WriteString(fmt.Sprintf("<anonymous %s %d>", sym.Kind, anonymousStrings))
				symstrtabData.WriteByte(0)
			} else {
				gobinary.Write(&symtabData, bo, uint32(symstrtabData.Len()))
				symstrtabData.WriteString(sym.Name)
				symstrtabData.WriteByte(0)
			}

			typ, value, size := symbolData(sym)
			symtabData.WriteByte(STB_GLOBAL | typ)                 // Symbol info.
			symtabData.WriteByte(STV_DEFAULT)                      // Symbol visibility.
			gobinary.Write(&symtabData, bo, uint16(sym.Section+2)) // Symbol section (add 2 for the null section and section names).
			symbolAddrs[i] = symtabData.Len()
			gobinary.Write(&symtabData, bo, value) // Symbol value.
			gobinary.Write(&symtabData, bo, size)  // Symbol size.
		}

		// Find the address of the final
		// byte in the existing sections,
		// so we know where to place the
		// symbol table.
		var lastAddr uint64
		for _, section := range sections {
			this := uint64(section.Address) + uint64(len(section.Data)-1)
			if lastAddr < this {
				lastAddr = this
			}
		}

		nextAddr := uintptr(nextPage(lastAddr))
		symtab = &binary.Section{
			Name:        symtabName,
			Address:     nextAddr,
			Permissions: binary.Read,
			Data:        symtabData.Bytes(),
		}

		nextAddr = uintptr(nextPage(uint64(nextAddr) + uint64(symtabData.Len())))
		symstrtab = &binary.Section{
			Name:        symstrtabName,
			Address:     nextAddr,
			Permissions: binary.Read,
			Data:        symstrtabData.Bytes(),
		}

		sections = append(sections[:len(sections):len(sections)], symtab, symstrtab)
	}

	sectionType := func(section *binary.Section) uint32 {
		switch section {
		case symtab:
			return SHT_SYMTAB
		case symstrtab:
			return SHT_STRTAB
		default:
			return SHT_PROGBITS
		}
	}

	sectionLink := func(section *binary.Section) uint32 {
		switch section {
		case symtab:
			return uint32(len(sections)) - 1 + 2 // The final entry (add 2 for the null section and section names).
		default:
			return 0
		}
	}

	sectionInfo := func(section *binary.Section) uint32 {
		switch section {
		case symtab:
			return 1 // One more than the index of the last local symbol (which is the empty symbol; all others are global).
		default:
			return 0
		}
	}

	sectionEntrySize := func(section *binary.Section) uint64 {
		switch section {
		case symtab:
			return symtabSize
		default:
			return 0
		}
	}

	// Build the section names table.
	var shstrtab bytes.Buffer
	sectionNames := make(map[string]uint32)
	addSectionName := func(s string) {
		offset, ok := sectionNames[s]
		if ok {
			return
		}

		offset = uint32(shstrtab.Len())
		shstrtab.WriteString(s)
		shstrtab.WriteByte(0)
		sectionNames[s] = offset
	}

	addSectionName("")
	addSectionName(shstrtabName)
	for _, section := range sections {
		addSectionName(section.Name)
	}

	// Start with the offsets that we don't align.
	progHeadOff := uint64(elfHeaderSize)                    // Offset of the program headers (ELF header length).
	progHeadLen := progHeaderSize * uint64(len(sections))   // Length of the program headers.
	progHeadEnd := progHeadOff + progHeadLen                // Offset where the program headers end.
	sectHeadOff := progHeadEnd                              // Offset of the section headers.
	sectHeadLen := sectHeaderSize * uint64(2+len(sections)) // Length of the section headers (including the NULL section and an extra for the section names table).
	sectHeadEnd := sectHeadOff + sectHeadLen                // Offset where the section headers end.
	sectDataOff := sectHeadEnd                              // Offset of the section names table.
	sectDataLen := uint64(shstrtab.Len())                   // Length of the section names table.
	sectDataEnd := sectDataOff + sectDataLen                // Offset where the section names table ends.
	progDataOff := nextPage(sectDataEnd)                    // Offset of the program data.

	// SectionOffsets is used to simplify the
	// calculation of section offsets.
	type SectionOffsets struct {
		MemStart   uint64 // Address in memory.
		MemEnd     uint64 // End address in memory.
		MemSize    uint64 // Size in memory.
		FileStart  uint64 // Offset into the file.
		FileEnd    uint64 // Section data end in the file.
		FileSize   uint64 // Size in the file.
		FilePadded uint64 // Size in the file with padding.
	}

	// Determine the section offsets.
	offset := progDataOff
	offsets := make([]SectionOffsets, len(sections))
	for i := range offsets {
		// Memory offsets.
		offsets[i].MemStart = uint64(sections[i].Address)
		offsets[i].MemSize = uint64(len(sections[i].Data))
		offsets[i].MemEnd = offsets[i].MemStart + offsets[i].MemSize

		// File offsets.
		offsets[i].FileStart = offset
		offsets[i].FileSize = uint64(len(sections[i].Data))
		offsets[i].FileEnd = offsets[i].FileStart + offsets[i].FileSize
		offsets[i].FilePadded = nextPage(offsets[i].FileEnd)

		// No data when it's zeroed.
		if sections[i].IsZeroed {
			offsets[i].FileSize = 0
			offsets[i].FileEnd = offsets[i].FileStart
			offsets[i].FilePadded = offsets[i].FileStart
		}

		// We don't pad the final section, as
		// nothing comes after it.
		if i+1 == len(offsets) {
			offsets[i].FilePadded = offsets[i].FileEnd
		}

		offset = offsets[i].FilePadded
		sections[i].Offset = uintptr(offsets[i].FileStart)
	}

	// Finish the symbol table.
	for i, sym := range bin.Symbols {
		sectionOffset := sym.Offset
		sym.Offset = bin.Sections[sym.Section].Offset + sectionOffset
		sym.Address = bin.Sections[sym.Section].Address + sectionOffset
		if bin.SymbolTable {
			offset := symbolAddrs[i]
			_, value, _ := symbolData(sym)
			bo.PutUint64(symtab.Data[offset:], value)
		}
	}

	// The entry point is now absolute.
	entry := uint64(bin.Entry.Address)

	b.Write([]byte{0x7f, 'E', 'L', 'F'}) // Magic number.
	b.WriteByte(2)                       // 64-bit format.
	b.WriteByte(endianness(bo))          // Endianness.
	b.WriteByte(1)                       // ELF version 1.
	b.WriteByte(0x1e)                    // Firefly.
	b.WriteByte(0)                       // ABI version.
	b.Write(make([]byte, 7))             // Padding.
	write(uint16(ET_EXEC))               // Executable file.
	write(arch2machine(bin.Arch))        // Architecture.
	write(uint32(1))                     // ELF version 1.
	write(entry)                         // Entry point address.
	write(progHeadOff)                   // Program header table offset.
	write(sectHeadOff)                   // Section header table offset.
	write(uint32(0))                     // Flags (which we don't use).
	write(uint16(elfHeaderSize))         // File header size.
	write(uint16(progHeaderSize))        // Program header size.
	write(uint16(len(sections)))         // Number of program headers.
	write(uint16(sectHeaderSize))        // Section header size.
	write(uint16(2 + len(sections)))     // Number of section headers.
	write(uint16(1))                     // Section header table index for section names (always second).

	// Add the program headers.
	for i, section := range sections {
		sectionOffsets := offsets[i]
		write(uint32(PT_LOAD))                      // Loadable segment.
		write(progPermissions(section.Permissions)) // Section flags.
		write(sectionOffsets.FileStart)             // File offset where segment begins.
		write(sectionOffsets.MemStart)              // Section virtual address in memory.
		write(sectionOffsets.MemStart)              // Section physical address in memory.
		write(sectionOffsets.FileSize)              // Size in the binary file.
		write(sectionOffsets.MemSize)               // Size in memory.
		write(uint64(pageSize))                     // Alignment in memory.
	}

	// Add the section headers.
	b.Write(make([]byte, sectHeaderSize)) // The NULL section.
	write(sectionNames[shstrtabName])     // Section name offset in section names table.
	write(uint32(SHT_STRTAB))             // Section names table.
	write(uint64(SHF_STRINGS))            // Section flags.
	write(uint64(0))                      // Section virtual address in memory.
	write(sectDataOff)                    // File offset where segment begins.
	write(sectDataLen)                    // Size in the binary file.
	write(uint32(0))                      // sh_link
	write(uint32(0))                      // sh_info
	write(uint64(1))                      // Alignment in memory.
	write(uint64(0))                      // sh_entsize
	for i, section := range sections {
		sectionOffsets := offsets[i]
		write(sectionNames[section.Name])                       // Section name offset in section names table.
		write(sectionType(section))                             // Section type (loadable by default, or symbol/string table).
		write(SHF_ALLOC | sectPermissions(section.Permissions)) // Section flags.
		write(sectionOffsets.MemStart)                          // Section virtual address in memory.
		write(sectionOffsets.FileStart)                         // File offset where segment begins.
		write(sectionOffsets.FileSize)                          // Size in the binary file.
		write(sectionLink(section))                             // sh_link
		write(sectionInfo(section))                             // sh_info
		write(uint64(pageSize))                                 // Alignment in memory.
		write(sectionEntrySize(section))                        // sh_entsize
	}

	// Add the section names table.
	b.Write(shstrtab.Bytes())

	// Add the padding between the end of the
	// program headers and the start of the
	// section data (which is page-aligned).
	b.Write(make([]byte, progDataOff-sectDataEnd))

	// Add the section data.
	for i, section := range sections {
		sectionOffsets := offsets[i]
		if !section.IsZeroed {
			b.Write(section.Data)
		}

		if !section.IsZeroed && sectionOffsets.FilePadded > sectionOffsets.FileEnd {
			padding := sectionOffsets.FilePadded - sectionOffsets.FileEnd
			b.Write(make([]byte, padding))
		}
	}

	return nil
}
