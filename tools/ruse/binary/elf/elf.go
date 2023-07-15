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

func permissions(perm binary.Permissions) uint32 {
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

func encode64(b *bytes.Buffer, bin *binary.Binary) error {
	// See https://en.wikipedia.org/wiki/Executable_and_Linkable_Format
	bo := bin.Arch.ByteOrder
	write := func(data any) {
		gobinary.Write(b, bo, data)
	}

	const (
		// Size constants.
		pageSize       = 0x1000 // 4kB page size in bytes.
		elfHeaderSize  = 0x40   // ELF header size in bytes.
		progHeaderSize = 0x38   // Program header size in bytes.
		sectHeaderSize = 0x40   // Section header size in bytes.

		// Value constants.
		ET_EXEC = 0x02
		PT_LOAD = 0x01
	)

	nextPage := func(offset uint64) uint64 {
		// Round the offset up to the start
		// of the next 4kB page.
		remainder := offset % pageSize
		topUp := pageSize - remainder
		return offset + topUp
	}

	// Start with the offsets that we don't align.
	progHeadOff := uint64(elfHeaderSize)                      // Offset of the program headers (ELF header length).
	progHeadLen := progHeaderSize * uint64(len(bin.Sections)) // Length of the program headers.
	progHeadEnd := progHeadOff + progHeadLen                  // Offset where the program headers end.
	progDataOff := nextPage(progHeadEnd)                      // Offset of the program data.

	// The entry point needs to be the start
	// of one of the sections so we can find
	// the right address.
	entry := uint64(bin.BaseAddr) // Entry point.

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
	offsets := make([]SectionOffsets, len(bin.Sections))
	for i := range offsets {
		// Memory offsets.
		offsets[i].MemStart = uint64(bin.Sections[i].Address)
		offsets[i].MemSize = uint64(len(bin.Sections[i].Data))
		offsets[i].MemEnd = offsets[i].MemStart + offsets[i].MemSize

		// File offsets.
		offsets[i].FileStart = offset
		offsets[i].FileSize = uint64(len(bin.Sections[i].Data))
		offsets[i].FileEnd = offsets[i].FileStart + offsets[i].FileSize
		offsets[i].FilePadded = nextPage(offsets[i].FileEnd)

		// No data when it's zeroed.
		if bin.Sections[i].IsZeroed {
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
	}

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
	write(uint64(0))                     // Section header table offset (which we don't use).
	write(uint32(0))                     // Flags (which we don't use).
	write(uint16(elfHeaderSize))         // File header size.
	write(uint16(progHeaderSize))        // Program header size.
	write(uint16(len(bin.Sections)))     // Number of program headers.
	write(uint16(sectHeaderSize))        // Section header size.
	write(uint16(0))                     // Number of section headers (which we don't use).
	write(uint16(0))                     // Section header table index for section names (which we don't use).

	// Add the program headers.
	for i, section := range bin.Sections {
		sectionOffsets := offsets[i]
		section.Offset = uintptr(sectionOffsets.FileStart)
		write(uint32(PT_LOAD))                  // Loadable segment.
		write(permissions(section.Permissions)) // Section flags.
		write(sectionOffsets.FileStart)         // File offset where segment begins.
		write(sectionOffsets.MemStart)          // Section virtual address in memory.
		write(sectionOffsets.MemStart)          // Section physical address in memory.
		write(sectionOffsets.FileSize)          // Size in the binary file.
		write(sectionOffsets.MemSize)           // Size in memory.
		write(uint64(pageSize))                 // Alignment in memory.
	}

	// Add the padding between the end of the
	// program headers and the start of the
	// section data (which is page-aligned).
	b.Write(make([]byte, progDataOff-progHeadEnd))

	// Add the section data.
	for i, section := range bin.Sections {
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
