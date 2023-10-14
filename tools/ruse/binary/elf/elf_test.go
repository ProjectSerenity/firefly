// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package elf

import (
	"bytes"
	"debug/elf"
	gobinary "encoding/binary"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"firefly-os.dev/tools/ruse/binary"
	"firefly-os.dev/tools/ruse/sys"
)

func TestEncode(t *testing.T) {
	tests := []struct {
		Name   string
		Binary *binary.Binary
		Want   *elf.File
	}{
		{
			Name: "simple",
			Binary: &binary.Binary{
				Arch:     sys.X86_64,
				BaseAddr: 0x0123456789abcdef,
				Sections: []*binary.Section{
					{
						Name:        "code",
						Address:     0x0123456789abcdef,
						Permissions: binary.Read | binary.Execute,
						Data:        []byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
					},
					{
						Name:        "data",
						Address:     0xf1e2d3c4b5a60798,
						IsZeroed:    true,
						Permissions: binary.Read | binary.Write,
						Data:        []byte{0xff},
					},
					{
						Name:        "strings",
						Address:     0xfedcba9876543210,
						Permissions: binary.Read,
						Data:        []byte("Hello, world!"),
					},
				},
				Symbols: []*binary.Symbol{
					{
						Name:    "example.com/first.main",
						Kind:    binary.SymbolFunction,
						Section: 0,
						Address: 0x0123456789abcdef,
					},
					{
						Name:    "example.com/second.Message",
						Kind:    binary.SymbolString,
						Section: 1,
						Address: 0xfedcba9876543210,
					},
				},
				SymbolTable: false,
			},
			Want: &elf.File{
				FileHeader: elf.FileHeader{
					Class:      elf.ELFCLASS64,
					Data:       elf.ELFDATA2LSB,
					Version:    elf.EV_CURRENT,
					OSABI:      0x1e,
					ABIVersion: 0,
					ByteOrder:  gobinary.LittleEndian,
					Type:       elf.ET_EXEC,
					Machine:    elf.EM_X86_64,
					Entry:      0x0123456789abcdef,
				},
				Sections: []*elf.Section{
					{
						SectionHeader: elf.SectionHeader{
							Name:      "",
							Type:      elf.SHT_NULL,
							Flags:     0,
							Addr:      0,
							Offset:    0,
							Size:      0,
							Link:      0,
							Info:      0,
							Addralign: 0,
							Entsize:   0,
							FileSize:  0,
						},
					},
					{
						SectionHeader: elf.SectionHeader{
							Name:      "section names",
							Type:      elf.SHT_STRTAB,
							Flags:     elf.SHF_STRINGS,
							Addr:      0,
							Offset:    552,
							Size:      33,
							Link:      0,
							Info:      0,
							Addralign: 1,
							Entsize:   0,
							FileSize:  33,
						},
					},
					{
						SectionHeader: elf.SectionHeader{
							Name:      "code",
							Type:      elf.SHT_PROGBITS,
							Flags:     elf.SHF_ALLOC | elf.SHF_EXECINSTR,
							Addr:      0x0123456789abcdef,
							Offset:    0x1000,
							Size:      10,
							Link:      0,
							Info:      0,
							Addralign: 0x1000,
							Entsize:   0,
							FileSize:  10,
						},
					},
					{
						SectionHeader: elf.SectionHeader{
							Name:      "data",
							Type:      elf.SHT_PROGBITS,
							Flags:     elf.SHF_ALLOC | elf.SHF_WRITE,
							Addr:      0xf1e2d3c4b5a60798,
							Offset:    0x2000,
							Size:      0,
							Link:      0,
							Info:      0,
							Addralign: 0x1000,
							Entsize:   0,
							FileSize:  0,
						},
					},
					{
						SectionHeader: elf.SectionHeader{
							Name:      "strings",
							Type:      elf.SHT_PROGBITS,
							Flags:     elf.SHF_ALLOC,
							Addr:      0xfedcba9876543210,
							Offset:    0x2000,
							Size:      13,
							Link:      0,
							Info:      0,
							Addralign: 0x1000,
							Entsize:   0,
							FileSize:  13,
						},
					},
				},
				Progs: []*elf.Prog{
					{
						ProgHeader: elf.ProgHeader{
							Type:   elf.PT_LOAD,
							Flags:  elf.PF_R | elf.PF_X,
							Off:    0x1000,
							Vaddr:  0x0123456789abcdef,
							Paddr:  0x0123456789abcdef,
							Filesz: 10,
							Memsz:  10,
							Align:  0x1000,
						},
					},
					{
						ProgHeader: elf.ProgHeader{
							Type:   elf.PT_LOAD,
							Flags:  elf.PF_R | elf.PF_W,
							Off:    0x2000,
							Vaddr:  0xf1e2d3c4b5a60798,
							Paddr:  0xf1e2d3c4b5a60798,
							Filesz: 0,
							Memsz:  1,
							Align:  0x1000,
						},
					},
					{
						ProgHeader: elf.ProgHeader{
							Type:   elf.PT_LOAD,
							Flags:  elf.PF_R,
							Off:    0x2000,
							Vaddr:  0xfedcba9876543210,
							Paddr:  0xfedcba9876543210,
							Filesz: 13,
							Memsz:  13,
							Align:  0x1000,
						},
					},
				},
			},
		},
		{
			Name: "simple-with-symbols",
			Binary: &binary.Binary{
				Arch:     sys.X86_64,
				BaseAddr: 0x0123456789abcdef,
				Sections: []*binary.Section{
					{
						Name:        "code",
						Address:     0x0123456789abcdef,
						Permissions: binary.Read | binary.Execute,
						Data:        []byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
					},
					{
						Name:        "data",
						Address:     0xf1e2d3c4b5a60798,
						IsZeroed:    true,
						Permissions: binary.Read | binary.Write,
						Data:        []byte{0xff},
					},
					{
						Name:        "strings",
						Address:     0xfedcba9876543210,
						Permissions: binary.Read,
						Data:        []byte("Hello, world!"),
					},
				},
				Symbols: []*binary.Symbol{
					{
						Name:    "example.com/first.main",
						Kind:    binary.SymbolFunction,
						Section: 0,
						Address: 0x0123456789abcdef,
					},
					{
						Name:    "example.com/second.Message",
						Kind:    binary.SymbolString,
						Section: 1,
						Address: 0xfedcba9876543210,
					},
				},
				SymbolTable: true,
			},
			Want: &elf.File{
				FileHeader: elf.FileHeader{
					Class:      elf.ELFCLASS64,
					Data:       elf.ELFDATA2LSB,
					Version:    elf.EV_CURRENT,
					OSABI:      0x1e,
					ABIVersion: 0,
					ByteOrder:  gobinary.LittleEndian,
					Type:       elf.ET_EXEC,
					Machine:    elf.EM_X86_64,
					Entry:      0x0123456789abcdef,
				},
				Sections: []*elf.Section{
					{
						SectionHeader: elf.SectionHeader{
							Name:      "",
							Type:      elf.SHT_NULL,
							Flags:     0,
							Addr:      0,
							Offset:    0,
							Size:      0,
							Link:      0,
							Info:      0,
							Addralign: 0,
							Entsize:   0,
							FileSize:  0,
						},
					},
					{
						SectionHeader: elf.SectionHeader{
							Name:      "section names",
							Type:      elf.SHT_STRTAB,
							Flags:     elf.SHF_STRINGS,
							Addr:      0,
							Offset:    792,
							Size:      59,
							Link:      0,
							Info:      0,
							Addralign: 1,
							Entsize:   0,
							FileSize:  59,
						},
					},
					{
						SectionHeader: elf.SectionHeader{
							Name:      "code",
							Type:      elf.SHT_PROGBITS,
							Flags:     elf.SHF_ALLOC | elf.SHF_EXECINSTR,
							Addr:      0x0123456789abcdef,
							Offset:    0x1000,
							Size:      10,
							Link:      0,
							Info:      0,
							Addralign: 0x1000,
							Entsize:   0,
							FileSize:  10,
						},
					},
					{
						SectionHeader: elf.SectionHeader{
							Name:      "data",
							Type:      elf.SHT_PROGBITS,
							Flags:     elf.SHF_ALLOC | elf.SHF_WRITE,
							Addr:      0xf1e2d3c4b5a60798,
							Offset:    0x2000,
							Size:      0,
							Link:      0,
							Info:      0,
							Addralign: 0x1000,
							Entsize:   0,
							FileSize:  0,
						},
					},
					{
						SectionHeader: elf.SectionHeader{
							Name:      "strings",
							Type:      elf.SHT_PROGBITS,
							Flags:     elf.SHF_ALLOC,
							Addr:      0xfedcba9876543210,
							Offset:    0x2000,
							Size:      13,
							Link:      0,
							Info:      0,
							Addralign: 0x1000,
							Entsize:   0,
							FileSize:  13,
						},
					},
					{
						SectionHeader: elf.SectionHeader{
							Name:      "symbol table",
							Type:      elf.SHT_SYMTAB,
							Flags:     elf.SHF_ALLOC,
							Addr:      0xfedcba9876544000,
							Offset:    0x3000,
							Size:      72,
							Link:      6,
							Info:      1,
							Addralign: 0x1000,
							Entsize:   24,
							FileSize:  72,
						},
					},
					{
						SectionHeader: elf.SectionHeader{
							Name:      "symbol names",
							Type:      elf.SHT_STRTAB,
							Flags:     elf.SHF_ALLOC,
							Addr:      0xfedcba9876545000,
							Offset:    0x4000,
							Size:      51,
							Link:      0,
							Info:      0,
							Addralign: 0x1000,
							Entsize:   0,
							FileSize:  51,
						},
					},
				},
				Progs: []*elf.Prog{
					{
						ProgHeader: elf.ProgHeader{
							Type:   elf.PT_LOAD,
							Flags:  elf.PF_R | elf.PF_X,
							Off:    0x1000,
							Vaddr:  0x0123456789abcdef,
							Paddr:  0x0123456789abcdef,
							Filesz: 10,
							Memsz:  10,
							Align:  0x1000,
						},
					},
					{
						ProgHeader: elf.ProgHeader{
							Type:   elf.PT_LOAD,
							Flags:  elf.PF_R | elf.PF_W,
							Off:    0x2000,
							Vaddr:  0xf1e2d3c4b5a60798,
							Paddr:  0xf1e2d3c4b5a60798,
							Filesz: 0,
							Memsz:  1,
							Align:  0x1000,
						},
					},
					{
						ProgHeader: elf.ProgHeader{
							Type:   elf.PT_LOAD,
							Flags:  elf.PF_R,
							Off:    0x2000,
							Vaddr:  0xfedcba9876543210,
							Paddr:  0xfedcba9876543210,
							Filesz: 13,
							Memsz:  13,
							Align:  0x1000,
						},
					},
					{
						ProgHeader: elf.ProgHeader{
							Type:   elf.PT_LOAD,
							Flags:  elf.PF_R,
							Off:    0x3000,
							Vaddr:  0xfedcba9876544000,
							Paddr:  0xfedcba9876544000,
							Filesz: 72,
							Memsz:  72,
							Align:  0x1000,
						},
					},
					{
						ProgHeader: elf.ProgHeader{
							Type:   elf.PT_LOAD,
							Flags:  elf.PF_R,
							Off:    0x4000,
							Vaddr:  0xfedcba9876545000,
							Paddr:  0xfedcba9876545000,
							Filesz: 51,
							Memsz:  51,
							Align:  0x1000,
						},
					},
				},
			},
		},
	}

	opts := []cmp.Option{
		cmp.AllowUnexported(io.SectionReader{}, bytes.Reader{}),
		cmpopts.IgnoreUnexported(elf.File{}, elf.Section{}, elf.Prog{}),
	}

	var b bytes.Buffer
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			b.Reset()
			err := Encode(&b, test.Binary)
			if err != nil {
				t.Fatalf("failed to encode: %v", err)
			}

			raw := b.Bytes()
			br := bytes.NewReader(raw)
			// Sort out the expected ReadersAt.
			for _, sect := range test.Want.Sections {
				sect.ReaderAt = io.NewSectionReader(br, int64(sect.Offset), int64(sect.FileSize))
			}
			for _, prog := range test.Want.Progs {
				prog.ReaderAt = io.NewSectionReader(br, int64(prog.Off), int64(prog.Filesz))
			}

			got, err := elf.NewFile(br)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			if diff := cmp.Diff(test.Want, got, opts...); diff != "" {
				t.Fatalf("Encode(): (-want, +got)\n%s", diff)
			}
		})
	}
}
