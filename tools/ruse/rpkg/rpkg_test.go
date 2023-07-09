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
			0, 0, 0, 64, // ImportsOffset: 64.
			0, 0, 0, 64, // ExportsOffset: 64.
			0, 0, 0, 0, 0, 0, 0, 64, // TypesOffset: 64.
			0, 0, 0, 0, 0, 0, 0, 68, // SymbolsOffset: 68,
			0, 0, 0, 0, 0, 0, 0, 68, // StringsOffset: 68,
			0, 0, 0, 0, 0, 0, 0, 92, // LinkagesOffset: 92,
			0, 0, 0, 0, 0, 0, 0, 92, // CodeOffset: 92,
			0, 0, 0, 0, 0, 0, 0, 92, // ChecksumOffset: 92,
			// Imports.
			// Exports.
			// Types.
			// - The nil type.
			1,       // Kind: none.
			0, 0, 0, // Length: 0.
			// Symbols.
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
			0x70, 0x52, 0x8a, 0xae, 0x4d, 0x75, 0xe5, 0xd3,
			0x35, 0x9c, 0x66, 0x39, 0x3c, 0xc1, 0x62, 0xa5,
			0x93, 0x31, 0x6a, 0x8e, 0xa6, 0x00, 0x63, 0x93,
			0x41, 0x78, 0x0a, 0x2e, 0x48, 0xee, 0xa4, 0xc1,
		},
		Decoded: &decoded{
			header: header{
				Magic:          0x72706b67,
				Architecture:   ArchX86_64,
				Version:        1,
				PackageName:    4,
				ImportsOffset:  64,
				ImportsLength:  0,
				ExportsOffset:  64,
				ExportsLength:  0,
				TypesOffset:    64,
				TypesLength:    4,
				SymbolsOffset:  68,
				SymbolsLength:  0,
				StringsOffset:  68,
				StringsLength:  24,
				LinkagesOffset: 92,
				LinkagesLength: 0,
				CodeOffset:     92,
				CodeLength:     0,
				ChecksumOffset: 92,
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
			strings: map[uint64]string{
				0: "",
				4: "example.com/foo",
			},
			linkages: map[uint64]*linkage{},
			code:     map[uint64][]byte{},
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
			0, 0, 0, 64, // ImportsOffset: 64.
			0, 0, 0, 64, // ExportsOffset: 64.
			0, 0, 0, 0, 0, 0, 0, 80, // TypesOffset: 80.
			0, 0, 0, 0, 0, 0, 0, 108, // SymbolsOffset: 108,
			0, 0, 0, 0, 0, 0, 0, 252, // StringsOffset: 252,
			0, 0, 0, 0, 0, 0, 1, 100, // LinkagesOffset: 356,
			0, 0, 0, 0, 0, 0, 1, 100, // CodeOffset: 356,
			0, 0, 0, 0, 0, 0, 1, 100, // ChecksumOffset: 356,
			// Imports.
			// Exports (sorted).
			// - Big-negative.
			0, 0, 0, 0, 0, 0, 0, 108, // Symbol offset 108 (Big-negative).
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
			0, 0, 0, 0, 0, 0, 0, 4, // Type: 4 (untyped string).
			0, 0, 0, 0, 0, 0, 0, 32, // Value: 32 ("Hello, world!").
			// - num
			0, 0, 0, 2, // Kind: 2 (integer constant).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 52, // Name: 52 ("num").
			0, 0, 0, 0, 0, 0, 0, 12, // Type: 12 (uint16).
			0, 0, 0, 0, 0, 0, 0, 12, // Value: 12 (12).
			// - massive
			0, 0, 0, 3, // Kind: 3 (big integer constant).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 60, // Name: 60 ("massive").
			0, 0, 0, 0, 0, 0, 0, 20, // Type: 20 (untyped int).
			0, 0, 0, 0, 0, 0, 0, 72, // Value: 72 (0x112233445566778899).
			// - Big-negative
			0, 0, 0, 4, // Kind: 4 (big negative integer constant).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 88, // Name: 88 ("Big-negative").
			0, 0, 0, 0, 0, 0, 0, 20, // Type: 20 (untyped int).
			0, 0, 0, 0, 0, 0, 0, 72, // Value: 72 (0x112233445566778899).
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
			0xaa, 0x96, 0x60, 0xb5, 0x9f, 0x9b, 0x28, 0xf4,
			0x90, 0xf2, 0x41, 0xd2, 0x69, 0x1a, 0xa4, 0x0e,
			0x9d, 0xa3, 0xb4, 0xb8, 0xbb, 0xe5, 0x9e, 0x0c,
			0x1b, 0xd7, 0xe5, 0x8b, 0xe9, 0x34, 0x29, 0x83,
		},
		Decoded: &decoded{
			header: header{
				Magic:          0x72706b67,
				Architecture:   ArchX86_64,
				Version:        1,
				PackageName:    4,
				ImportsOffset:  64,
				ImportsLength:  0,
				ExportsOffset:  64,
				ExportsLength:  16,
				TypesOffset:    80,
				TypesLength:    28,
				SymbolsOffset:  108,
				SymbolsLength:  144,
				StringsOffset:  252,
				StringsLength:  104,
				LinkagesOffset: 356,
				LinkagesLength: 0,
				CodeOffset:     356,
				CodeLength:     0,
				ChecksumOffset: 356,
				ChecksumLength: 32,
			},
			imports: []uint32{},
			exports: []uint64{
				108,
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
				36: {
					Kind:        SymKindIntegerConstant,
					PackageName: 4,
					Name:        52,
					Type:        12,
					Value:       12,
				},
				72: {
					Kind:        SymKindBigIntegerConstant,
					PackageName: 4,
					Name:        60,
					Type:        20,
					Value:       72,
				},
				108: {
					Kind:        SymKindBigNegativeIntegerConstant,
					PackageName: 4,
					Name:        88,
					Type:        20,
					Value:       72,
				},
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
			code:     map[uint64][]byte{},
		},
	},
	{
		Name:    "code",
		Arch:    sys.X86_64,
		Package: "example.com/foo",
		Code: `
			(package foo)

			(let msg "Hello, world!")

			(asm-func triple-nop
				(nop)
				(nop)
				(nop))

			'(param (str string) rax)
			'(param (len uint64) rcx)
			'(result uint64 rax)
			(asm-func string-copy
				(repnz movsb))

			(asm-func looper
				(mov rcx (len msg))
				(jz 'done)

				'again
				(call (func triple-nop))
				(dec rcx)
				(jz 'done)
				(jmp 'again)

				'done
				(ret))
		`,
		Raw: []byte{
			// Header.
			0x72, 0x70, 0x6b, 0x67, // Magic.
			1,    // Arch: x86-64.
			1,    // Version: 1.
			0, 4, // PackageName: 4.
			0, 0, 0, 64, // ImportsOffset: 64.
			0, 0, 0, 64, // ExportsOffset: 64.
			0, 0, 0, 0, 0, 0, 0, 64, // TypesOffset: 64.
			0, 0, 0, 0, 0, 0, 0, 172, // SymbolsOffset: 172,
			0, 0, 0, 0, 0, 0, 1, 60, // StringsOffset: 316,
			0, 0, 0, 0, 0, 0, 1, 220, // LinkagesOffset: 476,
			0, 0, 0, 0, 0, 0, 2, 0, // CodeOffset: 512,
			0, 0, 0, 0, 0, 0, 2, 44, // ChecksumOffset: 556,
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
			0, 0, 0, 0, 0, 0, 0, 40, // Name: 40 ("(func)").
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
			0, 0, 0, 0, 0, 0, 0, 68, // Param 0 Name: 68 ("str").
			0, 0, 0, 0, 0, 0, 0, 28, // Param 0 Type: 28 (string).
			0, 0, 0, 0, 0, 0, 0, 76, // Param 1 Name: 76 ("len").
			0, 0, 0, 0, 0, 0, 0, 36, // Param 1 Type: 36 (uint64).
			0, 0, 0, 0, 0, 0, 0, 36, // Result: 36 (uint64).
			0, 0, 0, 0, 0, 0, 0, 84, // Name: 84 ("(func (string) (uint64) uint64)").
			// - Untyped string.
			2,       // Kind: 2 (basic).
			0, 0, 4, // Length: 4.
			0, 0, 0, 17, // BasicKind: 17 (untyped string).
			// Symbols.
			// - triple-nop.
			0, 0, 0, 6, // Kind: 6 (function).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 24, // Name: 24 ("triple-nop").
			0, 0, 0, 0, 0, 0, 0, 4, // Type: 4 (func).
			0, 0, 0, 0, 0, 0, 0, 0, // Value: 0 (function 0).
			// - string-copy
			0, 0, 0, 6, // Kind: 6 (function).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 52, // Name: 52 ("string-copy").
			0, 0, 0, 0, 0, 0, 0, 44, // Type: 44 (func (string) (uint64) uint64).
			0, 0, 0, 0, 0, 0, 0, 8, // Value: 8 (function 1).
			// - looper
			0, 0, 0, 6, // Kind: 6 (function).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 120, // Name: 120 ("looper").
			0, 0, 0, 0, 0, 0, 0, 4, // Type: 4 (func).
			0, 0, 0, 0, 0, 0, 0, 16, // Value: 16 (function 2).
			// - msg
			0, 0, 0, 5, // Kind: 5 (string constant).
			0, 0, 0, 0, 0, 0, 0, 4, // PackageName: 4 ("example.com/foo").
			0, 0, 0, 0, 0, 0, 0, 132, // Name: 132 ("msg").
			0, 0, 0, 0, 0, 0, 0, 100, // Type: 100 (untyped string).
			0, 0, 0, 0, 0, 0, 0, 140, // Value: 140 ("Hello, world!").
			// Strings.
			// - The empty string.
			0, 0, 0, 0, // Length: 0.
			// - "example.com/foo".
			0, 0, 0, 15, // Length: 15.
			'e', 'x', 'a', 'm', 'p', 'l', 'e', '.', 'c', 'o', 'm', '/', 'f', 'o', 'o', // Text.
			0, // Padding.
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
			// - "Hello, world!".
			0, 0, 0, 13, // Length: 13.
			'H', 'e', 'l', 'l', 'o', ',', ' ', 'w', 'o', 'r', 'l', 'd', '!', // Text.
			0, 0, 0, // Padding
			// Linkages.
			// - looper calling triple-nop.
			0, 0, 0, 0, 0, 0, 0, 72, // Source: 72 (looper).
			0, 0, 0, 0, 0, 0, 0, 4, // TargetPackage: 4 (example.com/foo).
			0, 0, 0, 0, 0, 0, 0, 24, // TargetSymbol: 24 (triple-nop).
			1,        // Type: 1 (relative address).
			0, 0, 32, // Size: 32 (32-bit address).
			0, 0, 0, 10, // Offset: 10.
			0, 0, 0, 14, // Address: 14.
			// Code.
			// - triple-nop.
			0, 0, 0, 3, // Length: 3.
			0x90, // (nop)
			0x90, // (nop)
			0x90, // (nop)
			0,    // Padding.
			// - string-copy.
			0, 0, 0, 2, // Length: 2.
			0xf2, 0xa4, // (repnz movsb)
			0, 0, // Padding.
			// - looper.
			0, 0, 0, 22, // Length: 22.
			0x48, 0xc7, 0xc1, 0x0d, 0x00, 0x00, 0x00, // (mov rcx (len msg))
			0x74, 0x0c, // (jz 'done)
			0xe8, 0x3f, 0x33, 0x22, 0x11, // (call (func triple-nop))
			0x48, 0xff, 0xc9, // (dec rcx)
			0x74, 0x02, // (jz 'done)
			0xeb, 0xf4, // (jmp 'again)
			0xc3, // (ret)
			0, 0, // Padding.
			// Checksum.
			0x88, 0xd4, 0x7d, 0xb8, 0x33, 0x08, 0x88, 0xce,
			0x7c, 0x56, 0x2a, 0x2e, 0xac, 0x4c, 0xf0, 0x18,
			0x11, 0x5d, 0xec, 0x02, 0x85, 0x44, 0x6a, 0x8c,
			0xbc, 0x55, 0x9b, 0x44, 0xc7, 0x65, 0x23, 0xee,
		},
		Decoded: &decoded{
			header: header{
				Magic:          0x72706b67,
				Architecture:   ArchX86_64,
				Version:        1,
				PackageName:    4,
				ImportsOffset:  64,
				ImportsLength:  0,
				ExportsOffset:  64,
				ExportsLength:  0,
				TypesOffset:    64,
				TypesLength:    108,
				SymbolsOffset:  172,
				SymbolsLength:  144,
				StringsOffset:  316,
				StringsLength:  160,
				LinkagesOffset: 476,
				LinkagesLength: 36,
				CodeOffset:     512,
				CodeLength:     44,
				ChecksumOffset: 556,
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
					Name:         40,
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
						{Name: 68, Type: 28},
						{Name: 76, Type: 36},
					},
					Result: 36,
					Name:   84,
				},
				100: {
					Kind:   TypeKindBasic,
					Length: 4,
					Basic:  BasicKindUntypedString,
				},
			},
			symbols: map[uint64]*symbol{
				0: {
					Kind:        SymKindFunction,
					PackageName: 4,
					Name:        24,
					Type:        4,
					Value:       0,
				},
				36: {
					Kind:        SymKindFunction,
					PackageName: 4,
					Name:        52,
					Type:        44,
					Value:       8,
				},
				72: {
					Kind:        SymKindFunction,
					PackageName: 4,
					Name:        120,
					Type:        4,
					Value:       16,
				},
				108: {
					Kind:        SymKindStringConstant,
					PackageName: 4,
					Name:        132,
					Type:        100,
					Value:       140,
				},
			},
			strings: map[uint64]string{
				0:   "",
				4:   "example.com/foo",
				24:  "triple-nop",
				40:  "(func)",
				52:  "string-copy",
				68:  "str",
				76:  "len",
				84:  "(func (string) (uint64) uint64)",
				120: "looper",
				132: "msg",
				140: "Hello, world!",
			},
			linkages: map[uint64]*linkage{
				0: {
					Source:        72,
					TargetPackage: 4,
					TargetSymbol:  24,
					Type:          ssafir.LinkRelativeAddress,
					Size:          32,
					Offset:        10,
					Address:       14,
				},
			},
			code: map[uint64][]byte{
				0: {
					0x90, // (nop)
					0x90, // (nop)
					0x90, // (nop)
				},
				8: {
					0xf2, 0xa4, // (repnz movsb)
				},
				16: {
					0x48, 0xc7, 0xc1, 0x0d, 0x00, 0x00, 0x00, // (mov rcx (len msg))
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
			darch, dpkg, err := Decode(dinfo, first.Bytes())
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