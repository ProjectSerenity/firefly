// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package rpkg provides helpers to encode and decode compiled
// Ruse packages. The result is similar to a traditional object
// file.
//
// The rpkg format consists of a header, followed by a series
// of sections, each of which is length prefixed:
//
//   - The imports section contains the list of other packages
//     that this package imports.
//   - The exports section contains the list of symbols that
//     this package exports.
//   - The types section contains the public and private type
//     information for the package, including any aspects derived
//     from (or including) types from dependent packages.
//   - The symbols section contains the symbol table.
//   - The ABIs section contains any custom ABIs.
//   - The strings section contains length-prefixed string data
//     used by other sections.
//   - The linkages section contains liinkages and relocations
//     that must be performed at link time.
//   - The code section contains the machine code for each function
//     in the package for the architecture being encoded, referenced
//     from the symbol table.
//
// After the sections is a cryptographic checksum.
//
// All integers are stored in big-endian form. Each section must
// have a length that is an exact multiple of 32 bits.
//
// # Header
//
// The header structure is described with the following pseudocode
// (see [Arch] separately):
//
//	type Header struct {
//		// Details about the rpkg file.
//		Magic           uint32  // The magic value that identifies an rpkg file. (value: "rpkg")
//		Architecture    Arch    // The architecture this file targets (defined below).
//		Version         uint8   // The rpkg file format version. (value: rpkg.version)
//
//		// Details about the package.
//		PackageName     uint16  // The offset into the strings section where the package name begins.
//		BaseAddress     uint64  // The base address of the executable (0 for non-main packages).
//
//		// Location of the imports section.
//		ImportsOffset   uint32  // The offset into the file where the imports section begins.
//
//		// Location of the exports section.
//		ExportsOffset   uint32  // The offset into the file where the exports section begins.
//
//		// Location of the types section.
//		TypesOffset     uint64  // The offset into the file where the types section begins.
//
//		// Location of the symbols section.
//		SymbolsOffset   uint64  // The offset into the file where the symbols section begins.
//
//		// Location of the ABIs section.
//		ABIsOffset      uint64  // The offset into the file where the ABIs section begins.
//
//		// Location of the strings section.
//		StringsOffset   uint64  // The offset into the file where the strings section begins.
//
//		// Location of the linkages section.
//		LinkagesOffset  uint64  // The offset into the file where the linkages section begins.
//
//		// Location of the code section.
//		CodeOffset      uint64  // The offset into the file where the code section begins.
//
//		// Location of the checksum.
//		ChecksumOffset  uint64  // The offset into the file where the checksum begins.
//	}
//
// Note that the sections must begin immediately after the
// header and must be contiguous, in the given order. That
// is, the first byte of the symbols section must immediately
// follow the last byte of the types section.
//
// # Imports section
//
// The imports section consists of a sequence of string
// references, one for each imported package name. As the
// imports are expected to be added to the strings section
// before other data, every string reference should fit in
// a 32-bit integer. This means that the imports section
// will contain an integral number of 32-bit unsigned
// integers, which are offsets into the strings section.
// Each string reference is described with the following
// pseudocode:
//
//	type ImportReference uint32
//
// # Exports section
//
// The exports section consists of a sequence of symbol
// references, one for each exported symbol.The exports
// section contains an integral number of 64-bit unsigned
// integers, which are offsets into the symbols section.
// Each export reference is described with the following
// pseudocode:
//
//	type ExportReference uint64
//
// # Types section
//
// The types section contains type definitions. Each type
// definition is described with the following pseudocode
// (see [TypeKind] and [BasicKind] separately):
//
//	type Type struct {
//		Kind              TypeKind       // The type kind (defined below).
//		Length            uint24         // The length in bytes of the type definition.
//		switch Type.Kind {
//		case TypeKindNone:
//			// No further data.
//		case TypeKindBasic:
//			Basic         BasicKind      // The specific basic type.
//		case TypeKindFunction:
//			ParamsLength  uint32         // The length in bytes of the parameter types.
//			Params        [...]Variable  // Successive variables for each parameter.
//			Result        uint64         // The offset into the types section where the result type begins.
//			Name          uint64         // The offset into the strings section where the signature name begins.
//		case TypeKindABI:
//			ABIOffset     uint32         // The offset into the ABIs section where the ABI begins.
//		}
//	}
//
//	type Variable struct {
//		Name  uint64  // The offset into the strings section where the variable name begins.
//		Type  uint64  // The offset into the types section where the variable type begins.
//	}
//
// Note that the first type has length zero so that references
// to it can be used to represent the nil type.
//
// # Symbols section
//
// The symbols section consists of a sequence of contiguous
// symbols, where each symbol is described with the following
// pseudocode (see [SymKind] separately):
//
//	type Symbol struct {
//		Kind         SymKind  // The symbol kind (defined below).
//		PackageName  uint64   // The offset into the strings section where the package name begins. (e.g. "example.com/foo")
//		Name         uint64   // The offset into the strings section where the symbol name begins. (e.g. "Bar")
//		Type         uint64   // The offset into the types section where the symbol's type begins.
//		Value        uint64   // The symbol's value. The value format is explained below.
//	}
//
// # ABIs section
//
// The ABIs section consists of a sequence of contiguous
// ABIs, where each ABI is described with the following
// pseudocode:
//
//	type ABI struct {
//		Length         uint32   // The length of the remaining ABI data in bytes. Must be either 0 or greater than 4.
//		InvertedStack  bool     // Whether the stack is inverted (1 for true, 0 for false).
//		Params         []uint8  // A 1-byte total length field, followed by 1-byte indices into the architecture's ABI registers list.
//		Result         []uint8  // A 1-byte total length field, followed by 1-byte indices into the architecture's ABI registers list.
//		Scratch        []uint8  // A 1-byte total length field, followed by 1-byte indices into the architecture's ABI registers list.
//		Unused         []uint8  // A 1-byte total length field, followed by 1-byte indices into the architecture's ABI registers list (255 for the stack pointer).
//	}
//
// Note that the first ABI has lengths zero so that references
// to it can be used to represent the nil ABI. ABIs whose total
// length is not a multiple of four are followed by up to three
// bytes of padding to ensure that each `ABI` has 32-bit
// alignment.
//
// # Strings section
//
// The strings section consists of a sequence of contiguous
// strings, where each string is described with the following
// pseudocode:
//
//	type String struct {
//		Length  uint32     // The length in bytes of the string.
//		Data    [...]byte  // The string's contents. Strings are not null-terminated.
//	}
//
// Note that the first string has length zero so that references
// to it can be used to represent the empty string. Strings whose
// length is not a multiple of four are followed by up to three
// bytes of padding to ensure that each `String` has 32-bit
// alignment.
//
// # Linkages section
//
// The linkages section contains a sequence of link-time actions
// that must be conducted to connect sections of code by writing
// the address of the destination into the origin instruction.
// Each linkage is described with the following pseudocode:
//
//	type Linkage struct {
//		Source         uint64           // The offset into the symbols section where the source symbol begins.
//		TargetPackage  uint64           // The offset into the strings section where the target symbol's package name begins.
//		TargetSymbol   uint64           // The offset into the strings section where the target symbols' name begins.
//		Type           ssafir.LinkType  // The kind of linkage.
//		Size           uint24           // The address size in bits.
//		Offset         uint32           // The offset into the function code where the target address is inserted.
//		Address        uint32           // The offset into the function code used to calculate relative addresses.
//	}
//
// # Code section
//
// The code section consists of a sequence of contiguous
// functions, where each function is described with the
// following pseudocode:
//
//	type Function struct {
//		ABI     uint32     // The offset into the ABIs section where the function's ABI begins.
//		Length  uint32     // The length in bytes of the function's machine code.
//		Data    [...]byte  // The function's machine code.
//	}
//
// Each `Function` is followed by 0 to 3 bytes of padding so
// that the subsequent `Function` has 32-bit alignment.
//
// # Crypographic checksum
//
// Immediately after the code section is a 32-byte SHA-256
// checksum of the rest of the file. There must be no trailing
// data after the checksum.
package rpkg

import (
	"crypto/sha256"
	"fmt"

	"firefly-os.dev/tools/ruse/ssafir"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/types"
)

const (
	magic   uint32 = 0x72706b67 // "rpkg"
	version uint8  = 1

	// ChecksumLength is the length in bytes of the
	// checksum section.
	ChecksumLength = sha256.Size
)

type header struct {
	// Details about the rpkg file.
	Magic        uint32 // The magic value that identifies an rpkg file. (value: "rpkg")
	Architecture Arch   // The architecture this file targets (defined below).
	Version      uint8  // The rpkg file format version. (value: rpkg.version)

	// Details about the package.
	PackageName uint16 // The offset into the strings section where the package name begins.
	BaseAddress uint64 // The base address of the executable (0 for non-main packages).

	// Location of the imports section.
	ImportsOffset uint32 // The offset into the file where the imports section begins.
	ImportsLength uint32 // The length in bytes of the imports section.

	// Location of the exports section.
	ExportsOffset uint32 // The offset into the file where the exports section begins.
	ExportsLength uint32 // The length in bytes of the exports section.

	// Location of the types section.
	TypesOffset uint64 // The offset into the file where the types section begins.
	TypesLength uint64 // The length in bytes of the types section.

	// Location of the symbols section.
	SymbolsOffset uint64 // The offset into the file where the symbols section begins.
	SymbolsLength uint64 // The length in bytes of the symbols section.

	// Location of the ABIs section.
	ABIsOffset uint64 // The offset into the file where the ABIs section begins.
	ABIsLength uint32 // The length in bytes of the ABIs section.

	// Location of the strings section.
	StringsOffset uint64 // The offset into the file where the strings section begins.
	StringsLength uint64 // The lenght in bytes of the strings section.

	// Location of the linkages section.
	LinkagesOffset uint64 // The offset into the file where the linkages section begins.
	LinkagesLength uint64 // The lenght in bytes of the linkages section.

	// Location of the code section.
	CodeOffset uint64 // The offset into the file where the code section begins.
	CodeLength uint64 // The length in bytes of the code section.

	// Location of the checksum.
	ChecksumOffset uint64 // The offset into the file where the checksum begins.
	ChecksumLength uint64 // The length in bytes of the checksum.
}

const headerSize = 4 + // 32-bit magic.
	1 + // 8-bit architecture.
	1 + // 8-bit version.
	2 + // 16-bit package name string offset.
	8 + // 64-bit base address.
	4 + // 32-bit imports section offset.
	4 + // 32-bit exports section offset.
	8 + // 64-bit types section offset.
	8 + // 64-bit symbols section offset.
	8 + // 64-bit ABIs section offset.
	8 + // 64-bit strings section offset.
	8 + // 64-bit linkages section offset.
	8 + // 64-bit code section offset.
	8 // 64-bit checksum offset.

// Header contains the information from an rpkg header.
type Header struct {
	// Details about the rpkg file.
	Magic        uint32 // The magic value that identifies an rpkg file. (value: "rpkg")
	Architecture Arch   // The architecture this file targets (defined below).
	Version      uint8  // The rpkg file format version. (value: rpkg.version)
	Checksum     []byte // The rpkg checksum.

	// Details about the package.
	PackageName string // The offset into the strings section where the package name begins.
	BaseAddress uint64 // The base address of the executable (0 for non-main packages).

	// Location of the imports section.
	ImportsOffset uint32 // The offset into the file where the imports section begins.
	ImportsLength uint32 // The length in bytes of the imports section.

	// Location of the exports section.
	ExportsOffset uint32 // The offset into the file where the exports section begins.
	ExportsLength uint32 // The length in bytes of the exports section.

	// Location of the types section.
	TypesOffset uint64 // The offset into the file where the types section begins.
	TypesLength uint64 // The length in bytes of the types section.

	// Location of the symbols section.
	SymbolsOffset uint64 // The offset into the file where the symbols section begins.
	SymbolsLength uint64 // The length in bytes of the symbols section.

	// Location of the ABIs section.
	ABIsOffset uint64 // The offset into the file where the ABIs section begins.
	ABIsLength uint32 // The length in bytes of the ABIs section.

	// Location of the strings section.
	StringsOffset uint64 // The offset into the file where the strings section begins.
	StringsLength uint64 // The lenght in bytes of the strings section.

	// Location of the linkages section.
	LinkagesOffset uint64 // The offset into the file where the linkages section begins.
	LinkagesLength uint64 // The lenght in bytes of the linkages section.

	// Location of the code section.
	CodeOffset uint64 // The offset into the file where the code section begins.
	CodeLength uint64 // The length in bytes of the code section.

	// Location of the checksum.
	ChecksumOffset uint64 // The offset into the file where the checksum begins.
	ChecksumLength uint64 // The length in bytes of the checksum.
}

// Arch uniquely identifies the architecture that an rpkg
// was built for.
type Arch uint8

const (
	ArchInvalid Arch = 0x00
	ArchX86_64  Arch = 0x01 // x86-64.
)

func (a Arch) String() string {
	switch a {
	case ArchInvalid:
		return "invalid"
	case ArchX86_64:
		return "x86-64"
	default:
		return fmt.Sprintf("Arch(%d)", a)
	}
}

// TypeKind categorises types.
type TypeKind uint8

const (
	TypeKindInvalid  TypeKind = 0x00
	TypeKindNone     TypeKind = 0x01 // No type.
	TypeKindBasic    TypeKind = 0x02 // A basic type (bool, int, etc).
	TypeKindFunction TypeKind = 0x03 // A function signature.
	TypeKindABI      TypeKind = 0x04 // An ABI.
)

func (k TypeKind) String() string {
	switch k {
	case TypeKindInvalid:
		return "invalid"
	case TypeKindNone:
		return "none"
	case TypeKindBasic:
		return "basic"
	case TypeKindFunction:
		return "function"
	case TypeKindABI:
		return "ABI"
	default:
		return fmt.Sprintf("TypeKind(%d)", k)
	}
}

// BasicKind categorises basic types.
type BasicKind uint8

const (
	BasicKindInvalid       BasicKind = 0x00
	BasicKindBool          BasicKind = 0x01 // Basic bool.
	BasicKindInt           BasicKind = 0x02 // Basic int.
	BasicKindInt8          BasicKind = 0x03 // Basic int8.
	BasicKindInt16         BasicKind = 0x04 // Basic int16.
	BasicKindInt32         BasicKind = 0x05 // Basic int32.
	BasicKindInt64         BasicKind = 0x06 // Basic int64.
	BasicKindUint          BasicKind = 0x07 // Basic uint.
	BasicKindUint8         BasicKind = 0x08 // Basic uint8.
	BasicKindByte          BasicKind = 0x09 // Basic byte.
	BasicKindUint16        BasicKind = 0x0a // Basic uint16.
	BasicKindUint32        BasicKind = 0x0b // Basic uint32.
	BasicKindUint64        BasicKind = 0x0c // Basic uint64.
	BasicKindUintptr       BasicKind = 0x0d // Basic uintptr.
	BasicKindString        BasicKind = 0x0e // Basic string.
	BasicKindUntypedBool   BasicKind = 0x0f // Untyped bool.
	BasicKindUntypedInt    BasicKind = 0x10 // Untyped int.
	BasicKindUntypedString BasicKind = 0x11 // Untyped string.
)

func (k BasicKind) String() string {
	switch k {
	case BasicKindInvalid:
		return "invalid"
	case BasicKindBool:
		return "bool"
	case BasicKindInt:
		return "int"
	case BasicKindInt8:
		return "int8"
	case BasicKindInt16:
		return "int16"
	case BasicKindInt32:
		return "int32"
	case BasicKindInt64:
		return "int64"
	case BasicKindUint:
		return "uint"
	case BasicKindUint8:
		return "uint8"
	case BasicKindByte:
		return "byte"
	case BasicKindUint16:
		return "uint16"
	case BasicKindUint32:
		return "uint32"
	case BasicKindUint64:
		return "uint64"
	case BasicKindUintptr:
		return "uintptr"
	case BasicKindString:
		return "string"
	case BasicKindUntypedBool:
		return "untyped bool"
	case BasicKindUntypedInt:
		return "untyped int"
	case BasicKindUntypedString:
		return "untyped string"
	default:
		return fmt.Sprintf("BasicKind(%d)", k)
	}
}

// typeSplat is an expansion of the
// Type type, containing all fields.
//
// This is mainly used for testing.
type typeSplat struct {
	// Generic fields.
	Kind   TypeKind // The type kind.
	Length uint32   // The length in bytes of the type definition (uint24).

	// Basic fields.
	Basic BasicKind // The specific basic type.

	// Function fields.
	ParamsLength uint32     // The length in bytes of the parameter types.
	Params       []variable // Successive variables for each parameter.
	Result       uint64     // The offset into the types section where the result type begins.
	Name         uint64     // The offset into the strings section where the signature name begins.

	// ABI fields.
	ABI uint32 // The offset into the ABIs section.
}

// variable represents a type with
// an associated name, such as a
// parameter to a function.
type variable struct {
	Name uint64 // The offset into the strings section where the variable name begins.
	Type uint64 // The offset into the types section where the variable type begins.
}

type symbol struct {
	Kind        SymKind // The symbol kind (defined below).
	PackageName uint64  // The offset into the strings section where the package name begins. (e.g. "example.com/foo")
	Name        uint64  // The offset into the strings section where the symbol name begins. (e.g. "Bar")
	Type        uint64  // The offset into the types section where the symbol's type begins.
	Value       uint64  // The symbol's value. The value format is explained below.
}

const symbolSize = 4 + // 32-bit symbol kind.
	8 + // 64-bit package path string offset.
	8 + // 64-bit name string offset.
	8 + // 64-bit symbol type.
	8 // 64-bit symbol value.

// Symbol contains a parsed symbol from an rpkg
// file.
type Symbol struct {
	Kind        SymKind        // The symbol kind (defined below).
	PackageName string         // The symbol's package name.
	Name        string         // The symbol name.
	Type        types.Type     // The symbol type.
	Value       any            // The symbol value.
	Links       []*ssafir.Link // Any linkages the symbol has.
}

func (s *Symbol) AbsoluteName() string {
	if s.PackageName == "" {
		return s.Name
	}

	return s.PackageName + "." + s.Name
}

// SymKind defines a symbol kind for an entry in the
// symbol table.
type SymKind uint8

const (
	SymKindInvalid SymKind = 0x00

	// A boolean constant.
	// The Value field contains the
	// raw value (0 for false, 1 for
	// true).
	SymKindBooleanConstant SymKind = 0x01

	// An integer constant.
	// The Value field contains the
	// raw value.
	SymKindIntegerConstant SymKind = 0x02

	// A large untyped integer constant.
	// The Value field contains an
	// offset into the strings section,
	// containing the value as raw
	// bytes.
	SymKindBigIntegerConstant         SymKind = 0x03
	SymKindBigNegativeIntegerConstant SymKind = 0x04

	// A string constant.
	// The Value field contains an
	// offset into the strings section.
	SymKindStringConstant SymKind = 0x05

	// A function.
	// The Value field contains an
	// offset into the code section.
	SymKindFunction SymKind = 0x06

	// A named ABI.
	// The Value field is zero, as
	// the ABI is stored in the type.
	SymKindABI SymKind = 0x07
)

func (k SymKind) String() string {
	switch k {
	case SymKindInvalid:
		return "invalid"
	case SymKindBooleanConstant:
		return "boolean constant"
	case SymKindIntegerConstant:
		return "integer constant"
	case SymKindBigIntegerConstant:
		return "big integer constant"
	case SymKindBigNegativeIntegerConstant:
		return "big negative integer constant"
	case SymKindStringConstant:
		return "string constant"
	case SymKindFunction:
		return "function"
	case SymKindABI:
		return "abi"
	default:
		return fmt.Sprintf("SymKind(%d)", k)
	}
}

const (
	minABILength          = 5
	abiStackPointer uint8 = 255
)

type abi struct {
	Length        uint32
	InvertedStack bool
	Params        []uint8
	Result        []uint8
	Scratch       []uint8
	Unused        []uint8
}

type linkage struct {
	Source        uint64          // The offset into the symbols section where the source symbol begins.
	TargetPackage uint64          // The offset into the strings section where the target symbol's package name begins.
	TargetSymbol  uint64          // The offset into the strings section where the target symbols' name begins.
	Type          ssafir.LinkType // The kind of linkage.
	Size          uint32          // The address size in bits.
	Offset        uint32          // The offset into the function code where the target address is inserted.
	Address       uint32          // The offset into the function code used to calculate relative addresses.
}

const linkageSize = 8 + // 64-bit source symbol offset.
	8 + // 64-bit target symbol package offset.
	8 + // 64-bit target symbol name offset.
	1 + // 8-bit linkage type.
	3 + // 24-bit address size.
	4 + // 32-bit function offset.
	4 // 32-bit function address.

type Linkage struct {
	Source  string          // The absolute symbol name of the source function.
	Target  string          // The absolute symbol name of the target symbol.
	Type    ssafir.LinkType // The method for writing the address.
	Size    uint8           // The address size in bits.
	Offset  int             // The offset into the function code where the target address is inserted.
	Address uintptr         // The offset into the function code used to calculate relative addresses.
}

type function struct {
	ABI  uint32
	Code []byte
}

type Function struct {
	ABI  *sys.ABI
	Code []byte
}
