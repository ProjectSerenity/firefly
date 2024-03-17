// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package rpkg

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/compiler"
	"firefly-os.dev/tools/ruse/parser"
	"firefly-os.dev/tools/ruse/ssafir"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

var tests = []struct {
	Name    string
	Arch    *sys.Arch
	Package string
	Code    string
	Raw     []byte
	Decoded *decoded
}{
	{
		Name:    "empty",
		Arch:    sys.X86_64,
		Package: "example.com/foo",
		Code: `
			(package foo)
		`,
		Raw: []byte{
			// Header.
			0x72, 0x70, 0x6b, 0x67, // Magic.
			1,    // Arch: x86-64.
			1,    // Version: 1.
			0, 4, // PackageName: 4.
			0, 0, 0, 0, 0, 0, 0, 0, // BaseAddress: 0.
			0, 0, 0, 0, // NumSections: 0.
			0, 0, 0, 92, // ImportsOffset: 92.
			0, 0, 0, 92, // ExportsOffset: 92.
			0, 0, 0, 0, 0, 0, 0, 92, // TypesOffset: 92.
			0, 0, 0, 0, 0, 0, 0, 96, // SymbolsOffset: 96.
			0, 0, 0, 0, 0, 0, 0, 96, // ABIsOffset: 96.
			0, 0, 0, 0, 0, 0, 0, 100, // SectionsOffset: 100.
			0, 0, 0, 0, 0, 0, 0, 124, // StringsOffset: 124.
			0, 0, 0, 0, 0, 0, 0, 148, // LinkagesOffset: 148.
			0, 0, 0, 0, 0, 0, 0, 148, // CodeOffset: 148.
			0, 0, 0, 0, 0, 0, 0, 148, // ChecksumOffset: 148.
			// Imports.
			// Exports.
			// Types.
			// - The nil type.
			1,       // Kind: none.
			0, 0, 0, // Length: 0.
			// Symbols.
			// ABIs.
			// - The nil ABI.
			0, 0, 0, 0, // Length: 0.
			// Sections.
			// - The nil section.
			0, 0, 0, 0, 0, 0, 0, 0, // Name: 0.
			0, 0, 0, 0, 0, 0, 0, 0, // Address: 0x0,
			0,                // Permissions: 0 (---).
			0,                // FixedAddr: 0 (false).
			0, 0, 0, 0, 0, 0, // Padding.
			// Strings.
			// - The empty string.
			0, 0, 0, 0, // Length: 0.
			// - The package name.
			0, 0, 0, 15, // Length: 15.
			'e', 'x', 'a', 'm', 'p', 'l', 'e', '.', 'c', 'o', 'm', '/', 'f', 'o', 'o', // Text.
			0, // Padding.
			// Linkages.
			// Code.
			// Checksum.
			0x40, 0x16, 0xc4, 0xa6, 0x72, 0xb1, 0x46, 0xa0,
			0x14, 0xa6, 0xe5, 0xa5, 0x6a, 0xb6, 0x70, 0x18,
			0xe2, 0x48, 0x5a, 0xcf, 0xe7, 0x38, 0xdf, 0xbe,
			0x24, 0x5f, 0xb8, 0xca, 0x69, 0x7e, 0x8a, 0x8d,
		},
		Decoded: &decoded{
			header: header{
				Magic:          0x72706b67,
				Architecture:   ArchX86_64,
				Version:        1,
				PackageName:    4,
				BaseAddress:    0,
				NumSections:    0,
				ImportsOffset:  92,
				ImportsLength:  0,
				ExportsOffset:  92,
				ExportsLength:  0,
				TypesOffset:    92,
				TypesLength:    4,
				SymbolsOffset:  96,
				SymbolsLength:  0,
				ABIsOffset:     96,
				ABIsLength:     4,
				SectionsOffset: 100,
				SectionsLength: 24,
				StringsOffset:  124,
				StringsLength:  24,
				LinkagesOffset: 148,
				LinkagesLength: 0,
				CodeOffset:     148,
				CodeLength:     0,
				ChecksumOffset: 148,
				ChecksumLength: 32,
			},
			imports: []uint32{},
			exports: []uint64{},
			types: map[uint64]typeSplat{
				0: {
					Kind:   TypeKindNone,
					Length: 0,
				},
			},
			symbols: map[uint64]*symbol{},
			abis: map[uint32]*abi{
				0: nil,
			},
			sections: map[uint32]*programSection{
				0: nil,
			},
			strings: map[uint64]string{
				0: "",
				4: "example.com/foo",
			},
			linkages: map[uint64]*linkage{},
			code:     map[uint64]*function{},
		},
	},
	{
		Name:    "constants",
		Arch:    sys.X86_64,
		Package: "example.com/foo",
		Code: `
			(package foo)
			(let Text (+ "Hello, " "world!"))
			(let (num uint16) (* 4 3))
			(let massive 0x112233445566778899)
			(let Big-negative -0x112233445566778899)
		`,
		Raw: []byte{
			// Header.
			0x72, 0x70, 0x6b, 0x67, // Magic.
			1,    // Arch: x86-64.
			1,    // Version: 1.
			0, 4, // PackageName: 4.
			0, 0, 0, 0, 0, 0, 0, 0, // BaseAddress: 0.
			0, 0, 0, 0, // NumSections: 0.
			0, 0, 0, 92, // ImportsOffset: 92.
			0, 0, 0, 92, // ExportsOffset: 92.
			0, 0, 0, 0, 0, 0, 0, 108, // TypesOffset: 108.
			0, 0, 0, 0, 0, 0, 0, 136, // SymbolsOffset: 136.
			0, 0, 0, 0, 0, 0, 1, 56, // ABIsOffset: 312.
			0, 0, 0, 0, 0, 0, 1, 60, // SectionsOffset: 316.
			0, 0, 0, 0, 0, 0, 1, 84, // StringsOffset: 340.
			0, 0, 0, 0, 0, 0, 1, 188, // LinkagesOffset: 444.
			0, 0, 0, 0, 0, 0, 1, 188, // CodeOffset: 444.
			0, 0, 0, 0, 0, 0, 1, 188, // ChecksumOffset: 444.
			// Imports.
			// Exports (sorted).
			// - Big-negative.
			0, 0, 0, 0, 0, 0, 0, 132, // Symbol offset 132 (Big-negative).
			// - Text.
			0, 0, 0, 0, 0, 0, 0, 0, // Symbol offset: 0 (Text).
			// Types.
			// - The nil type.
			1,       // Kind: 1 (none).
			0, 0, 0, // Length: 0.
			// - Untyped string.
			2,       // Kind: 2 (basic).
			0, 0, 4, // Length: 4.
			0, 0, 0, 17, // BasicKind: 17 (untyped string).
			// - Uint16.
			2,       // Kind: 2 (basic).
			0, 0, 4, // Length: 4.
			0, 0, 0, 10, // BasicKind: 10 (uint16).
			// - Untyped int.
			2,       // Kind: 2 (basic).
			0, 0, 4, // Length: 4.
			0, 0, 0, 16, // BasicKind: 16 (untyped int).
			// Symbols.
			// - Text
			0, 0, 0, 5, // Kind: 5 (string constant).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 24, // Name: 24 ("Text").
			0, 0, 0, 0, 0, 0, 0, 0, // SectionName: 0 (default).
			0, 0, 0, 0, 0, 0, 0, 4, // Type: 4 (untyped string).
			0, 0, 0, 0, 0, 0, 0, 32, // Value: 32 ("Hello, world!").
			// - num
			0, 0, 0, 2, // Kind: 2 (integer constant).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 52, // Name: 52 ("num").
			0, 0, 0, 0, 0, 0, 0, 0, // SectionName: 0 (default).
			0, 0, 0, 0, 0, 0, 0, 12, // Type: 12 (uint16).
			0, 0, 0, 0, 0, 0, 0, 12, // Value: 12 (12).
			// - massive
			0, 0, 0, 3, // Kind: 3 (big integer constant).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 60, // Name: 60 ("massive").
			0, 0, 0, 0, 0, 0, 0, 0, // SectionName: 0 (default).
			0, 0, 0, 0, 0, 0, 0, 20, // Type: 20 (untyped int).
			0, 0, 0, 0, 0, 0, 0, 72, // Value: 72 (0x112233445566778899).
			// - Big-negative
			0, 0, 0, 4, // Kind: 4 (big negative integer constant).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 88, // Name: 88 ("Big-negative").
			0, 0, 0, 0, 0, 0, 0, 0, // SectionName: 0 (default).
			0, 0, 0, 0, 0, 0, 0, 20, // Type: 20 (untyped int).
			0, 0, 0, 0, 0, 0, 0, 72, // Value: 72 (0x112233445566778899).
			// ABIs.
			// - The nil ABI.
			0, 0, 0, 0, // Length: 0.
			// Sections.
			// - The nil section.
			0, 0, 0, 0, 0, 0, 0, 0, // Name: 0.
			0, 0, 0, 0, 0, 0, 0, 0, // Address: 0x0,
			0,                // Permissions: 0 (---).
			0,                // FixedAddr: 0 (false).
			0, 0, 0, 0, 0, 0, // Padding.
			// Strings.
			// - The empty string.
			0, 0, 0, 0, // Length: 0.
			// - "example.com/foo".
			0, 0, 0, 15, // Length: 15.
			'e', 'x', 'a', 'm', 'p', 'l', 'e', '.', 'c', 'o', 'm', '/', 'f', 'o', 'o', // Text.
			0, // Padding.
			// - "Text".
			0, 0, 0, 4, // Length: 4.
			'T', 'e', 'x', 't', // Text.
			// - "Hello, world!".
			0, 0, 0, 13, // Length: 13.
			'H', 'e', 'l', 'l', 'o', ',', ' ', 'w', 'o', 'r', 'l', 'd', '!', // Text.
			0, 0, 0, // Padding
			// - "num".
			0, 0, 0, 3, // Length: 3,
			'n', 'u', 'm', // Text.
			0, // Padding.
			// - "massive".
			0, 0, 0, 7, // Length: 7.
			'm', 'a', 's', 's', 'i', 'v', 'e', // Text.
			0, // Padding.
			// - "\x11\x22\x33\x44\x55\x66\x77\x88\x99".
			0, 0, 0, 9, // Length: 9.
			0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, // Text.
			0, 0, 0, // Padding
			// - "Big-negative".
			0, 0, 0, 12, // Length: 12.
			'B', 'i', 'g', '-', 'n', 'e', 'g', 'a', 't', 'i', 'v', 'e', // Text.
			// Linkages.
			// Code.
			// Checksum.
			0x0a, 0x99, 0x64, 0x43, 0x5e, 0xa0, 0x76, 0x12,
			0x0d, 0xfa, 0x35, 0x7c, 0xe7, 0x58, 0xaa, 0x83,
			0x3b, 0x1d, 0x32, 0x62, 0xba, 0x54, 0x63, 0x45,
			0x37, 0xd5, 0x62, 0xd1, 0x95, 0x82, 0xe6, 0x7b,
		},
		Decoded: &decoded{
			header: header{
				Magic:          0x72706b67,
				Architecture:   ArchX86_64,
				Version:        1,
				PackageName:    4,
				BaseAddress:    0,
				NumSections:    0,
				ImportsOffset:  92,
				ImportsLength:  0,
				ExportsOffset:  92,
				ExportsLength:  16,
				TypesOffset:    108,
				TypesLength:    28,
				SymbolsOffset:  136,
				SymbolsLength:  176,
				ABIsOffset:     312,
				ABIsLength:     4,
				SectionsOffset: 316,
				SectionsLength: 24,
				StringsOffset:  340,
				StringsLength:  104,
				LinkagesOffset: 444,
				LinkagesLength: 0,
				CodeOffset:     444,
				CodeLength:     0,
				ChecksumOffset: 444,
				ChecksumLength: 32,
			},
			imports: []uint32{},
			exports: []uint64{
				132,
				0,
			},
			types: map[uint64]typeSplat{
				0: {
					Kind:   TypeKindNone,
					Length: 0,
				},
				4: {
					Kind:   TypeKindBasic,
					Length: 4,
					Basic:  BasicKindUntypedString,
				},
				12: {
					Kind:   TypeKindBasic,
					Length: 4,
					Basic:  BasicKindUint16,
				},
				20: {
					Kind:   TypeKindBasic,
					Length: 4,
					Basic:  BasicKindUntypedInt,
				},
			},
			symbols: map[uint64]*symbol{
				0: {
					Kind:        SymKindStringConstant,
					PackageName: 4,
					Name:        24,
					Type:        4,
					Value:       32,
				},
				44: {
					Kind:        SymKindIntegerConstant,
					PackageName: 4,
					Name:        52,
					Type:        12,
					Value:       12,
				},
				88: {
					Kind:        SymKindBigIntegerConstant,
					PackageName: 4,
					Name:        60,
					Type:        20,
					Value:       72,
				},
				132: {
					Kind:        SymKindBigNegativeIntegerConstant,
					PackageName: 4,
					Name:        88,
					Type:        20,
					Value:       72,
				},
			},
			abis: map[uint32]*abi{
				0: nil,
			},
			sections: map[uint32]*programSection{
				0: nil,
			},
			strings: map[uint64]string{
				0:  "",
				4:  "example.com/foo",
				24: "Text",
				32: "Hello, world!",
				52: "num",
				60: "massive",
				72: "\x11\x22\x33\x44\x55\x66\x77\x88\x99",
				88: "Big-negative",
			},
			linkages: map[uint64]*linkage{},
			code:     map[uint64]*function{},
		},
	},
	{
		Name:    "code",
		Arch:    sys.X86_64,
		Package: "example.com/foo",
		Code: `
			'(base-address 0x10_0000)
			'(sections extra-section extra-section)
			(package main)

			(let msg "Hello, world!")

			(asm-func (triple-nop)
				(nop)
				(nop)
				(nop))

			'(abi (abi
				(params rsi rcx)))
			(asm-func (string-copy (str string) (len uint64) uint64)
				(repnz movsb))

			(let custom-abi (abi
				(params rsi rcx rdx)
				(result rax)))

			(let extra-section (section
				(name "extra")
				(fixed-address 0x1122334455667788)
				(permissions r_x)))

			'(abi custom-abi)
			(asm-func (looper (msg string))
				(test rcx rcx)
				(jz 'done)

				'again
				(call (@ triple-nop))
				(dec rcx)
				(jz 'done)
				(jmp 'again)

				'done
				(ret))

			(let simple-array (array/2/uint16 0x0033 0x0044))

			(let multi-dimensional-array
				(array/3/array/2/uint16
					(array/2/uint16 0x0011 0x0022)
					simple-array
					(array/uint16 0x5566 0x789a)))
		`,
		Raw: []byte{
			// Header.
			0x72, 0x70, 0x6b, 0x67, // Magic.
			1,    // Arch: x86-64.
			1,    // Version: 1.
			0, 4, // PackageName: 4.
			0, 0, 0, 0, 0, 16, 0, 0, // BaseAddress: 0x10_0000.
			0, 0, 0, 2, // NumSections: 2.
			0, 0, 0, 24, // Section: 24 ("example.com/foo.extra-section").
			0, 0, 0, 24, // Section: 24 ("example.com/foo.extra-section").
			0, 0, 0, 100, // ImportsOffset: 100.
			0, 0, 0, 100, // ExportsOffset: 100.
			0, 0, 0, 0, 0, 0, 0, 100, // TypesOffset: 100.
			0, 0, 0, 0, 0, 0, 1, 56, // SymbolsOffset: 312.
			0, 0, 0, 0, 0, 0, 2, 152, // ABIsOffset: 664.
			0, 0, 0, 0, 0, 0, 2, 204, // SectionsOffset: 716.
			0, 0, 0, 0, 0, 0, 2, 252, // StringsOffset: 764.
			0, 0, 0, 0, 0, 0, 4, 72, // LinkagesOffset: 1096.
			0, 0, 0, 0, 0, 0, 4, 108, // CodeOffset: 1132.
			0, 0, 0, 0, 0, 0, 4, 160, // ChecksumOffset: 1184.
			// Imports.
			// Exports.
			// Types.
			// - The nil type.
			1,       // Kind: 1 (none).
			0, 0, 0, // Length: 0.
			// - Function.
			3,        // Kind: 3 (function signature).
			0, 0, 20, // Length: 20.
			0, 0, 0, 0, // ParamsLength: 0.
			0, 0, 0, 0, 0, 0, 0, 0, // Result: 0 (nil type).
			0, 0, 0, 0, 0, 0, 0, 76, // Name: 76 ("(func)").
			// - String.
			2,       // Kind: 2 (basic).
			0, 0, 4, // Length: 4.
			0, 0, 0, 14, // BasicKind: 14 (string).
			// - Uint64.
			2,       // Kind: 2 (basic).
			0, 0, 4, // Length: 4.
			0, 0, 0, 12, // BasicKind: 12 (uint64).
			// - Function.
			3,        // Kind: 3 (function signature).
			0, 0, 52, // Length: 52.
			0, 0, 0, 32, // ParamsLength: 32.
			0, 0, 0, 0, 0, 0, 0, 104, // Param 0 Name: 104 ("str").
			0, 0, 0, 0, 0, 0, 0, 28, // Param 0 Type: 28 (string).
			0, 0, 0, 0, 0, 0, 0, 112, // Param 1 Name: 112 ("len").
			0, 0, 0, 0, 0, 0, 0, 36, // Param 1 Type: 36 (uint64).
			0, 0, 0, 0, 0, 0, 0, 36, // Result: 36 (uint64).
			0, 0, 0, 0, 0, 0, 0, 120, // Name: 120 ("(func (string) (uint64) uint64)").
			// - Function.
			3,        // Kind: 3 (function signature).
			0, 0, 36, // Length: 36.
			0, 0, 0, 16, // ParamsLength: 16.
			0, 0, 0, 0, 0, 0, 0, 168, // Param 0 Name: 168 ("msg").
			0, 0, 0, 0, 0, 0, 0, 28, // Param 0 Type: 28 (string).
			0, 0, 0, 0, 0, 0, 0, 0, // Result: 0 (nil).
			0, 0, 0, 0, 0, 0, 0, 176, // Name: 176 ("(func (string)").
			// - Untyped string.
			2,       // Kind: 2 (basic).
			0, 0, 4, // Length: 4.
			0, 0, 0, 17, // BasicKind: 17 (untyped string).
			// - ABI.
			4,       // Kind: 4 (ABI).
			0, 0, 4, // Length: 4.
			0, 0, 0, 28, // ABI: custom-abi.
			// - Section.
			5,       // Kind: 5 (section).
			0, 0, 4, // Length: 4,
			0, 0, 0, 24, // Section: extra-section.
			// - Uint16.
			2,       // Kind: 2 (basic).
			0, 0, 4, // Length 4.
			0, 0, 0, 10, // BasicKind: 10 (uint16).
			// - Array of 2 uint16s.
			6,        // Kind: 6 (array).
			0, 0, 16, // Length: 16.
			0, 0, 0, 0, 0, 0, 0, 2, // Array length: 2.
			0, 0, 0, 0, 0, 0, 0, 164, // Element type: 164 (uint16).
			// - Array of 3 arrays of 2 uint16s.
			6,        // Kind: 6 (array).
			0, 0, 16, // Length: 16.
			0, 0, 0, 0, 0, 0, 0, 3, // Array length: 3.
			0, 0, 0, 0, 0, 0, 0, 172, // Element type: 172 (array/2/uint16).
			// Symbols.
			// - triple-nop.
			0, 0, 0, 6, // Kind: 6 (function).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 60, // Name: 60 ("triple-nop").
			0, 0, 0, 0, 0, 0, 0, 0, // SectionName: 0 (default).
			0, 0, 0, 0, 0, 0, 0, 4, // Type: 4 (func).
			0, 0, 0, 0, 0, 0, 0, 0, // Value: 0 (function 0).
			// - string-copy
			0, 0, 0, 6, // Kind: 6 (function).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 88, // Name: 88 ("string-copy").
			0, 0, 0, 0, 0, 0, 0, 0, // SectionName: 0 (default).
			0, 0, 0, 0, 0, 0, 0, 44, // Type: 44 (func (string) (uint64) uint64).
			0, 0, 0, 0, 0, 0, 0, 12, // Value: 12 (function 1).
			// - looper
			0, 0, 0, 6, // Kind: 6 (function).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 156, // Name: 156 ("looper").
			0, 0, 0, 0, 0, 0, 0, 0, // SectionName: 0 (default).
			0, 0, 0, 0, 0, 0, 0, 100, // Type: 100 (func (string)).
			0, 0, 0, 0, 0, 0, 0, 24, // Value: 24 (function 2).
			// - msg
			0, 0, 0, 5, // Kind: 5 (string constant).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 168, // Name: 168 ("msg").
			0, 0, 0, 0, 0, 0, 0, 0, // SectionName: 0 (default).
			0, 0, 0, 0, 0, 0, 0, 140, // Type: 140 (untyped string).
			0, 0, 0, 0, 0, 0, 0, 196, // Value: 196 ("Hello, world!").
			// - custom-abi
			0, 0, 0, 7, // Kind: 7 (ABI).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 216, // Name: 216 ("custom-abi").
			0, 0, 0, 0, 0, 0, 0, 0, // SectionName: 0 (default).
			0, 0, 0, 0, 0, 0, 0, 148, // Type: 148 (ABI custom-abi).
			0, 0, 0, 0, 0, 0, 0, 0, // Value: 0 (ABI).
			// - extra-section
			0, 0, 0, 8, // Kind: 8 (section).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 232, // Name: 232 ("extra-section").
			0, 0, 0, 0, 0, 0, 0, 0, // SectionName: 0 (default).
			0, 0, 0, 0, 0, 0, 0, 156, // Type: 156 (section extra-section).
			0, 0, 0, 0, 0, 0, 0, 0, // Value: 0 (Section).
			// - simple-array
			0, 0, 0, 9, // Kind: 9 (array constant).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 1, 8, // Name: 264 ("simple-array").
			0, 0, 0, 0, 0, 0, 0, 0, // SectionName: 0 (default).
			0, 0, 0, 0, 0, 0, 0, 172, // Type: 172 (array/2/uint16).
			0, 0, 0, 0, 0, 0, 1, 24, // Value: 280 (array data).
			// - multi-dimensional-array
			0, 0, 0, 9, // Kind: 9 (array constant).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 1, 32, // Name: 288 ("multi-dimensional-array").
			0, 0, 0, 0, 0, 0, 0, 0, // SectionName: 0 (default).
			0, 0, 0, 0, 0, 0, 0, 192, // Type: 192 (array/3/array/2/uint16).
			0, 0, 0, 0, 0, 0, 1, 60, // Value: 316 (array data).
			// ABIs.
			// - The nil ABI.
			0, 0, 0, 0, // Length: 0.
			// - string-copy's ABI.
			0, 0, 0, 20, // Length: 20.
			0,         // InvertedStack: 0 (false).
			2, 59, 53, // ParamRegisters length: 2, ParamRegisters: RSI, RCX.
			0,                              // ResultRegisters length: 0.
			0,                              // ScratchRegisters length: 0.
			13, 52, 54, 55, 56, 58, 60, 61, // UnusedRegisters length: 13, UnusedRegisters: RAX, RDX, RBX, RBP, RDI, R8.
			62, 63, 64, 65, 66, 67, // UnusedRegisters: R9, R10, R11, R12, R13, R14.
			// - custom-abi.
			0, 0, 0, 20, // Length: 20.
			0,             // InvertedStack: 0 (false).
			3, 59, 53, 54, // ParamRegisters length: 3, ParamRegisters: RSI, RCX, RDX.
			1, 52, // ResultRegisters length: 1, ResultRegisters: RAX.
			0,                              // ScratchRegisters length: 0.
			11, 55, 56, 58, 60, 61, 62, 63, // UnusedRegisters length: 11, UnusedRegisters: RBX, RBP, RDI, R8, R9, R10.
			64, 65, 66, 67, // UnusedRegisters: R11, R12, R13, R14.
			// Sections.
			// - The nil section.
			0, 0, 0, 0, 0, 0, 0, 0, // Name: 0.
			0, 0, 0, 0, 0, 0, 0, 0, // Address: 0x0,
			0,                // Permissions: 0 (---).
			0,                // FixedAddr: 0 (false).
			0, 0, 0, 0, 0, 0, // Padding.
			// - extra-section.
			0, 0, 0, 0, 0, 0, 0, 252, // Name: 252 ("extra").
			17, 34, 51, 68, 85, 102, 119, 136, // Address: 0x1122334455667788,
			5,                // Permissions: 5 (r-x).
			1,                // FixedAddr: 1 (true).
			0, 0, 0, 0, 0, 0, // Padding.
			// Strings.
			// - The empty string.
			0, 0, 0, 0, // Length: 0.
			// - "example.com/foo".
			0, 0, 0, 15, // Length: 15.
			'e', 'x', 'a', 'm', 'p', 'l', 'e', '.', 'c', 'o', 'm', '/', 'f', 'o', 'o', // Text.
			0, // Padding.
			// - "example.com/foo.extra-section"
			0, 0, 0, 29, // Length: 29.
			'e', 'x', 'a', 'm', 'p', 'l', 'e', '.', 'c', 'o', 'm', '/', 'f', 'o', 'o', '.', 'e', 'x', 't', 'r', 'a', '-', 's', 'e', 'c', 't', 'i', 'o', 'n', // Text.
			0, 0, 0, // Padding.
			// - "triple-nop".
			0, 0, 0, 10, // Length: 10.
			't', 'r', 'i', 'p', 'l', 'e', '-', 'n', 'o', 'p', // Text.
			0, 0, // Padding.
			// - "(func)".
			0, 0, 0, 6, // Length: 6.
			'(', 'f', 'u', 'n', 'c', ')', // Text.
			0, 0, // Padding
			// - "string-copy".
			0, 0, 0, 11, // Length: 1,
			's', 't', 'r', 'i', 'n', 'g', '-', 'c', 'o', 'p', 'y', // Text.
			0, // Padding.
			// - "str".
			0, 0, 0, 3, // Length: 3.
			's', 't', 'r', // Text.
			0, // Padding.
			// - "len".
			0, 0, 0, 3, // Length: 3.
			'l', 'e', 'n', // Text.
			0, // Padding
			// - "(func (string) (uint64) uint64)".
			0, 0, 0, 31, // Length: 31.
			'(', 'f', 'u', 'n', 'c', ' ', '(', 's', 't', 'r', 'i', 'n', 'g', ')', ' ', '(', 'u', 'i', 'n', 't', '6', '4', ')', ' ', 'u', 'i', 'n', 't', '6', '4', ')', // Text.
			0, // Padding.
			// - "looper".
			0, 0, 0, 6, // Length: 6.
			'l', 'o', 'o', 'p', 'e', 'r', // Text.
			0, 0, // Padding
			// - "msg".
			0, 0, 0, 3, // Length: 3.
			'm', 's', 'g', // Text.
			0, // Padding
			// - "(func (string))"
			0, 0, 0, 15, // Length: 15.
			'(', 'f', 'u', 'n', 'c', ' ', '(', 's', 't', 'r', 'i', 'n', 'g', ')', ')', // Text.
			0, // Padding
			// - "Hello, world!".
			0, 0, 0, 13, // Length: 13.
			'H', 'e', 'l', 'l', 'o', ',', ' ', 'w', 'o', 'r', 'l', 'd', '!', // Text.
			0, 0, 0, // Padding
			// - "custom-abi".
			0, 0, 0, 10, // Length: 10.
			'c', 'u', 's', 't', 'o', 'm', '-', 'a', 'b', 'i', // Text.
			0, 0, // Padding.
			// - "extra-section".
			0, 0, 0, 13, // Length: 13.
			'e', 'x', 't', 'r', 'a', '-', 's', 'e', 'c', 't', 'i', 'o', 'n', // Text.
			0, 0, 0, // Padding.
			// - "extra".
			0, 0, 0, 5, // Length: 5.
			'e', 'x', 't', 'r', 'a', // Text.
			0, 0, 0, // Padding.
			// - "simple-array".
			0, 0, 0, 12, // Length: 11.
			's', 'i', 'm', 'p', 'l', 'e', '-', 'a', 'r', 'r', 'a', 'y', // Text.
			// Padding.
			// encoded simple-array data.
			0, 0, 0, 4, // Length: 4.
			0x00, 0x33, 0x00, 0x44, // Data.
			// Padding.
			// - "multi-dimensional-array".
			0, 0, 0, 23, // Length: 23.
			'm', 'u', 'l', 't', 'i', '-', 'd', 'i', 'm', 'e', 'n', 's', 'i', 'o', 'n', 'a', 'l', '-', 'a', 'r', 'r', 'a', 'y', // text
			0, // Padding.
			// - encoded multi-dimensional-array data.
			0, 0, 0, 12, // Length: 12.
			0x00, 0x11, 0x00, 0x22, 0x00, 0x33, 0x00, 0x44, 0x55, 0x66, 0x78, 0x9a, // Data.
			// Padding.
			// Linkages.
			// - looper calling triple-nop.
			0, 0, 0, 0, 0, 0, 0, 88, // Source: 88 (looper).
			0, 0, 0, 0, 0, 0, 0, 4, // TargetPackage: 4 (example.com/foo).
			0, 0, 0, 0, 0, 0, 0, 60, // TargetSymbol: 60 ("triple-nop").
			1,        // Type: 1 (relative address).
			0, 0, 32, // Size: 32 (32-bit address).
			0, 0, 0, 6, // Offset: 6.
			0, 0, 0, 10, // Address: 10.
			// Code.
			// - triple-nop.
			0, 0, 0, 0, // ABI: nil.
			0, 0, 0, 3, // Length: 3.
			0x90, // (nop)
			0x90, // (nop)
			0x90, // (nop)
			0,    // Padding.
			// - string-copy.
			0, 0, 0, 4, // ABI: string-copy ABI.
			0, 0, 0, 2, // Length: 2.
			0xf2, 0xa4, // (repnz movsb)
			0, 0, // Padding.
			// - looper.
			0, 0, 0, 28, // ABI: custom-abi.
			0, 0, 0, 18, // Length: 18.
			0x48, 0x85, 0xc9, // (test rcx rcx)
			0x74, 0x0c, // (jz 'done)
			0xe8, 0x3f, 0x33, 0x22, 0x11, // (call (func triple-nop))
			0x48, 0xff, 0xc9, // (dec rcx)
			0x74, 0x02, // (jz 'done)
			0xeb, 0xf4, // (jmp 'again)
			0xc3, // (ret)
			0, 0, // Padding.
			// Checksum.
			0x10, 0x3f, 0xe7, 0x26, 0x0a, 0x4f, 0x5f, 0x0e, 0x90, 0xe9, 0x36, 0xc1, 0x19, 0xf7, 0x08, 0x1e,
			0xc1, 0xff, 0x0c, 0x7c, 0x4e, 0x00, 0x3c, 0x74, 0x95, 0x40, 0xd9, 0x8e, 0x79, 0x5b, 0x08, 0x51,
		},
		Decoded: &decoded{
			header: header{
				Magic:          0x72706b67,
				Architecture:   ArchX86_64,
				Version:        1,
				PackageName:    4,
				BaseAddress:    0x10_0000,
				NumSections:    2,
				Sections:       []uint32{24, 24},
				ImportsOffset:  100,
				ImportsLength:  0,
				ExportsOffset:  100,
				ExportsLength:  0,
				TypesOffset:    100,
				TypesLength:    212,
				SymbolsOffset:  312,
				SymbolsLength:  352,
				ABIsOffset:     664,
				ABIsLength:     52,
				SectionsOffset: 716,
				SectionsLength: 48,
				StringsOffset:  764,
				StringsLength:  332,
				LinkagesOffset: 1096,
				LinkagesLength: 36,
				CodeOffset:     1132,
				CodeLength:     52,
				ChecksumOffset: 1184,
				ChecksumLength: 32,
			},
			imports: []uint32{},
			exports: []uint64{},
			types: map[uint64]typeSplat{
				0: {
					Kind:   TypeKindNone,
					Length: 0,
				},
				4: {
					Kind:         TypeKindFunction,
					Length:       20,
					ParamsLength: 0,
					Params:       []variable{},
					Result:       0,
					Name:         76,
				},
				28: {
					Kind:   TypeKindBasic,
					Length: 4,
					Basic:  BasicKindString,
				},
				36: {
					Kind:   TypeKindBasic,
					Length: 4,
					Basic:  BasicKindUint64,
				},
				44: {
					Kind:         TypeKindFunction,
					Length:       52,
					ParamsLength: 32,
					Params: []variable{
						{Name: 104, Type: 28},
						{Name: 112, Type: 36},
					},
					Result: 36,
					Name:   120,
				},
				100: {
					Kind:         TypeKindFunction,
					Length:       36,
					ParamsLength: 16,
					Params: []variable{
						{Name: 168, Type: 28},
					},
					Result: 0,
					Name:   176,
				},
				140: {
					Kind:   TypeKindBasic,
					Length: 4,
					Basic:  BasicKindUntypedString,
				},
				148: {
					Kind:   TypeKindABI,
					Length: 4,
					ABI:    28,
				},
				156: {
					Kind:    TypeKindSection,
					Length:  4,
					Section: 24,
				},
				164: {
					Kind:   TypeKindBasic,
					Length: 4,
					Basic:  BasicKindUint16,
				},
				172: {
					Kind:        TypeKindArray,
					Length:      16,
					ArrayLength: 2,
					Element:     164,
				},
				192: {
					Kind:        TypeKindArray,
					Length:      16,
					ArrayLength: 3,
					Element:     172,
				},
			},
			symbols: map[uint64]*symbol{
				0: {
					Kind:        SymKindFunction,
					PackageName: 4,
					Name:        60,
					Type:        4,
					Value:       0,
				},
				44: {
					Kind:        SymKindFunction,
					PackageName: 4,
					Name:        88,
					Type:        44,
					Value:       12,
				},
				88: {
					Kind:        SymKindFunction,
					PackageName: 4,
					Name:        156,
					Type:        100,
					Value:       24,
				},
				132: {
					Kind:        SymKindStringConstant,
					PackageName: 4,
					Name:        168,
					Type:        140,
					Value:       196,
				},
				176: {
					Kind:        SymKindABI,
					PackageName: 4,
					Name:        216,
					Type:        148,
					Value:       0,
				},
				220: {
					Kind:        SymKindSection,
					PackageName: 4,
					Name:        232,
					Type:        156,
					Value:       0,
				},
				264: {
					Kind:        SymKindArrayConstant,
					PackageName: 4,
					Name:        264,
					Type:        172,
					Value:       280,
				},
				308: {
					Kind:        SymKindArrayConstant,
					PackageName: 4,
					Name:        288,
					Type:        192,
					Value:       316,
				},
			},
			abis: map[uint32]*abi{
				0: nil,
				4: {
					Length:  20,
					Params:  []uint8{59, 53},
					Result:  []uint8{},
					Scratch: []uint8{},
					Unused:  []uint8{52, 54, 55, 56, 58, 60, 61, 62, 63, 64, 65, 66, 67},
				},
				28: {
					Length:  20,
					Params:  []uint8{59, 53, 54},
					Result:  []uint8{52},
					Scratch: []uint8{},
					Unused:  []uint8{55, 56, 58, 60, 61, 62, 63, 64, 65, 66, 67},
				},
			},
			sections: map[uint32]*programSection{
				0: nil,
				24: {
					Name:        252,
					Address:     0x1122334455667788,
					Permissions: 0b101,
					FixedAddr:   true,
				},
			},
			strings: map[uint64]string{
				0:   "",
				4:   "example.com/foo",
				24:  "example.com/foo.extra-section",
				60:  "triple-nop",
				76:  "(func)",
				88:  "string-copy",
				104: "str",
				112: "len",
				120: "(func (string) (uint64) uint64)",
				156: "looper",
				168: "msg",
				176: "(func (string))",
				196: "Hello, world!",
				216: "custom-abi",
				232: "extra-section",
				252: "extra",
				264: "simple-array",
				280: "\x00\x33\x00\x44",
				288: "multi-dimensional-array",
				316: "\x00\x11\x00\x22\x00\x33\x00\x44\x55\x66\x78\x9a",
			},
			linkages: map[uint64]*linkage{
				0: {
					Source:        88,
					TargetPackage: 4,
					TargetSymbol:  60,
					Type:          ssafir.LinkRelativeAddress,
					Size:          32,
					Offset:        6,
					Address:       10,
				},
			},
			code: map[uint64]*function{
				0: {
					ABI: 0,
					Code: []byte{
						0x90, // (nop)
						0x90, // (nop)
						0x90, // (nop)
					},
				},
				8: {
					ABI: 4,
					Code: []byte{
						0xf2, 0xa4, // (repnz movsb)
					},
				},
				16: {
					ABI: 28,
					Code: []byte{
						0x48, 0x85, 0xc9, // (test rcx rcx)
						0x74, 0x0c, // (jz 'done)
						0xe8, 0x3f, 0x33, 0x22, 0x11, // (call (func triple-nop))
						0x48, 0xff, 0xc9, // (dec rcx)
						0x74, 0x02, // (jz 'done)
						0xeb, 0xf4, // (jmp 'again)
						0xc3, // (ret)
					},
				},
			},
		},
	},
}

func TestEncode(t *testing.T) {
	opts := []cmp.Option{
		cmp.AllowUnexported(
			decoded{},
			header{},
			symbol{},
		),
	}

	var buf bytes.Buffer
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.ruse", test.Code, 0)
			if err != nil {
				t.Fatalf("failed to parse code: %v", err)
			}

			files := []*ast.File{file}
			info := &types.Info{
				List:        make([]types.Type, 0, 10),
				Indices:     make(map[types.Type]int),
				Types:       make(map[ast.Expression]types.TypeAndValue),
				Definitions: make(map[*ast.Identifier]types.Object),
				Uses:        make(map[*ast.Identifier]types.Object),
			}

			tpkg, err := types.Check(test.Package, fset, files, test.Arch, info)
			if err != nil {
				t.Fatalf("failed to type-check code: %v", err)
			}

			cpkg, err := compiler.Compile(fset, test.Arch, tpkg, files, info, types.SizesFor(test.Arch))
			if err != nil {
				t.Fatalf("failed to compile code: %v", err)
			}

			buf.Reset()
			err = Encode(&buf, fset, test.Arch, cpkg, info)
			if err != nil {
				t.Fatalf("failed to encode package: %v", err)
			}

			if !bytes.Equal(buf.Bytes(), test.Raw) {
				diff := cmp.Diff(test.Raw, buf.Bytes())
				t.Fatalf("encoding mismatch: (-want, +got)\n%s", diff)
			}

			got, err := decodeSimple(buf.Bytes())
			if err != nil {
				t.Fatalf("failed to decode package: %v", err)
			}

			if diff := cmp.Diff(test.Decoded, got, opts...); diff != "" {
				t.Fatalf("Encode(): (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestRoundTripping(t *testing.T) {
	// We want to ensure that if we
	// compile a package into an rpkg
	// and then parse the rpkg, we
	// get back all of the important
	// state.
	//
	// We could test this by comparing
	// the input and output, but we
	// lose a lot of unimportant info,
	// such as position information,
	// so the comparison would be very
	// noisy.
	//
	// Instead, we check that if we
	// compile the 'decompiled' rpkg,
	// we get the same byte sequence
	// in both rpkg files.

	opts := []cmp.Option{
		cmp.AllowUnexported(
			decoded{},
			header{},
			symbol{},
		),
	}

	var first, second bytes.Buffer
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.ruse", test.Code, 0)
			if err != nil {
				t.Fatalf("failed to parse code: %v", err)
			}

			files := []*ast.File{file}
			info := &types.Info{
				List:        make([]types.Type, 0, 10),
				Indices:     make(map[types.Type]int),
				Types:       make(map[ast.Expression]types.TypeAndValue),
				Definitions: make(map[*ast.Identifier]types.Object),
				Uses:        make(map[*ast.Identifier]types.Object),
			}

			tpkg, err := types.Check(test.Package, fset, files, test.Arch, info)
			if err != nil {
				t.Fatalf("failed to type-check code: %v", err)
			}

			cpkg, err := compiler.Compile(fset, test.Arch, tpkg, files, info, types.SizesFor(test.Arch))
			if err != nil {
				t.Fatalf("failed to compile code: %v", err)
			}

			first.Reset()
			err = Encode(&first, fset, test.Arch, cpkg, info)
			if err != nil {
				t.Fatalf("failed to encode package: %v", err)
			}

			dinfo := new(types.Info)
			darch, dpkg, _, err := Decode(dinfo, first.Bytes())
			if err != nil {
				t.Fatalf("failed to decode package: %v", err)
			}

			second.Reset()
			err = Encode(&second, fset, darch, dpkg, dinfo)
			if err != nil {
				t.Fatalf("failed to encode decoded package: %v", err)
			}

			if !bytes.Equal(first.Bytes(), second.Bytes()) {
				firstDecoded, err := decodeSimple(first.Bytes())
				if err != nil {
					t.Fatalf("failed to decode package after output mismatch: %v", err)
				}

				secondDecoded, err := decodeSimple(second.Bytes())
				if err != nil {
					t.Fatalf("failed to decode re-encoded package after output mismatch: %v", err)
				}

				if diff := cmp.Diff(firstDecoded, secondDecoded, opts...); diff != "" {
					t.Fatalf("Re-encode mismatch: (-want, +got)\n%s", diff)
				}

				diff := cmp.Diff(first.Bytes(), second.Bytes())
				t.Fatalf("Re-encode mismatch fallback: (-first, +second)\n%s", diff)
			}
		})
	}
}
