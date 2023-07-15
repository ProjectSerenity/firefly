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
				Sections: []*elf.Section{},
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
