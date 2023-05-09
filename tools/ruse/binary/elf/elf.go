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

	progHeadOffs := 0x40                                  // Offset of the program headers.
	progDataOffs := uint64(0x40 + 0x38*len(bin.Sections)) // Offset of the program data.
	entry := uint64(bin.BaseAddr) + progDataOffs          // Entry point.

	b.Write([]byte{0x7f, 'E', 'L', 'F'}) // Magic number.
	b.WriteByte(2)                       // 64-bit format.
	b.WriteByte(endianness(bo))          // Endianness.
	b.WriteByte(1)                       // ELF version 1.
	b.WriteByte(0x1e)                    // Firefly.
	b.WriteByte(0)                       // ABI version.
	b.Write(make([]byte, 7))             // Padding.
	write(uint16(2))                     // Executable file.
	write(arch2machine(bin.Arch))        // Architecture.
	write(uint32(1))                     // ELF version 1.
	write(entry)                         // Entry point address.
	write(uint64(progHeadOffs))          // Program header table offset.
	write(uint64(0))                     // Section header table offset (which we don't use).
	write(uint32(0))                     // Flags (which we don't use).
	write(uint16(0x40))                  // File header size.
	write(uint16(0x38))                  // Program header size.
	write(uint16(len(bin.Sections)))     // Number of program headers.
	write(uint16(0x40))                  // Section header size.
	write(uint16(0))                     // Number of section headers (which we don't use).
	write(uint16(0))                     // Section header table index for section names (which we don't use).

	// Add the program headers.
	for _, section := range bin.Sections {
		var fileSize uint64 // Size in the section.
		if !section.IsZeroed {
			fileSize = uint64(len(section.Data))
		}

		base := uint64(section.Address) + progDataOffs
		section.Offset = uintptr(progDataOffs)  // Update the section data.
		write(uint32(1))                        // Loadable segment.
		write(permissions(section.Permissions)) // Section flags.
		write(progDataOffs)                     // File offset where segment begins.
		write(base)                             // Section virtual address in memory.
		write(base)                             // Section physical address in memory.
		write(fileSize)                         // Size in the binary file.
		write(uint64(len(section.Data)))        // Size in memory.
		write(uint64(0x1000))                   // Alignment in memory.
		progDataOffs += fileSize                // Skip over the section data in the file.
	}

	// Add the section data.
	for _, section := range bin.Sections {
		if !section.IsZeroed {
			b.Write(section.Data)
		}
	}

	return nil
}
