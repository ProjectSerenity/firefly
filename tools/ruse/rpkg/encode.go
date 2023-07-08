// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package rpkg

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/constant"
	"io"
	"math"
	"math/big"
	"strings"

	"golang.org/x/crypto/cryptobyte"

	"firefly-os.dev/tools/ruse/compiler"
	"firefly-os.dev/tools/ruse/ssafir"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

// encoder is used to encode a compiled Ruse
// package into an rpkg file.
type encoder struct {
	header header

	imports []uint32

	// Used to build the types section
	// efficiently. This state is managed
	// by AddType.
	types        [][]byte
	typesOffset  uint64
	typesOffsets map[string]uint64

	symbols []*symbol

	// Used to build the strings section
	// efficiently. This state is managed
	// by AddString.
	strings       []string
	stringOffset  uint64
	stringOffsets map[string]uint64

	linkages []*linkage

	// Used to build the code section
	// efficiently. This state is managed
	// by AddCode.
	code       [][]byte
	codeOffset uint64
}

// AddHeader builds the rpkg header.
func (e *encoder) AddHeader(arch *sys.Arch, pkg *compiler.Package) error {
	var architecture Arch
	switch arch {
	case sys.X86_64:
		architecture = ArchX86_64
	default:
		return fmt.Errorf("unsupported architecture: %v", arch)
	}

	// Build the header.
	e.header.Magic = magic
	e.header.Architecture = architecture
	e.header.Version = version
	e.header.PackageName = uint16(e.AddString(pkg.Path))
	e.header.ImportsOffset = headerSize
	e.header.ImportsLength = 4 * uint32(len(e.imports))
	e.header.TypesOffset = uint64(e.header.ImportsOffset) + uint64(e.header.ImportsLength)
	e.header.TypesLength = e.typesOffset
	e.header.SymbolsOffset = e.header.TypesOffset + e.header.TypesLength
	e.header.SymbolsLength = symbolSize * uint64(len(e.symbols))
	e.header.StringsOffset = e.header.SymbolsOffset + e.header.SymbolsLength
	e.header.StringsLength = e.stringOffset
	e.header.LinkagesOffset = e.header.StringsOffset + e.header.StringsLength
	e.header.LinkagesLength = linkageSize * uint64(len(e.linkages))
	e.header.CodeOffset = e.header.LinkagesOffset + e.header.LinkagesLength
	e.header.CodeLength = e.codeOffset
	e.header.ChecksumOffset = e.header.CodeOffset + e.header.CodeLength
	e.header.ChecksumLength = ChecksumLength

	return nil
}

// AddType appends the type to the type
// section.
func (e *encoder) AddType(t types.Type) uint64 {
	var b *cryptobyte.Builder
	switch t.(type) {
	case nil:
		b = cryptobyte.NewFixedBuilder(make([]byte, 0, 4))
	case *types.Basic:
		b = cryptobyte.NewFixedBuilder(make([]byte, 0, 8))
	default:
		b = cryptobyte.NewBuilder(nil)
	}

	e.appendType(b, t)
	data := b.BytesOrPanic()

	offset, ok := e.typesOffsets[string(data)]
	if ok {
		return offset
	}

	offset = e.typesOffset
	e.types = append(e.types, data)
	e.typesOffset += uint64(len(data))
	e.typesOffsets[string(data)] = offset

	return offset
}

func (e *encoder) appendType(b *cryptobyte.Builder, t types.Type) {
	switch t := t.(type) {
	case nil:
		// Used to add the nil type.
		b.AddUint8(uint8(TypeKindNone))
		b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {})
	case *types.Basic:
		var kind BasicKind
		switch t.Kind() {
		case types.KindBool:
			kind = BasicKindBool
		case types.KindInt:
			kind = BasicKindInt
		case types.KindInt8:
			kind = BasicKindInt8
		case types.KindInt16:
			kind = BasicKindInt16
		case types.KindInt32:
			kind = BasicKindInt32
		case types.KindInt64:
			kind = BasicKindInt64
		case types.KindUint:
			kind = BasicKindUint
		case types.KindUint8:
			if t == types.Byte {
				kind = BasicKindByte
			} else {
				kind = BasicKindUint8
			}
		case types.KindUint16:
			kind = BasicKindUint16
		case types.KindUint32:
			kind = BasicKindUint32
		case types.KindUint64:
			kind = BasicKindUint64
		case types.KindUintptr:
			kind = BasicKindUintptr
		case types.KindString:
			kind = BasicKindString
		case types.KindUntypedBool:
			kind = BasicKindUntypedBool
		case types.KindUntypedInt:
			kind = BasicKindUntypedInt
		case types.KindUntypedString:
			kind = BasicKindUntypedString
		default:
			panic(fmt.Sprintf("unrecognised basic type kind %v", t.Kind()))
		}

		b.AddUint8(uint8(TypeKindBasic))
		b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddUint32(uint32(kind))
		})
	case *types.Signature:
		b.AddUint8(uint8(TypeKindFunction))
		b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddUint32LengthPrefixed(func(b *cryptobyte.Builder) {
				for _, param := range t.Params() {
					b.AddUint64(e.AddString(param.Name()))
					b.AddUint64(e.AddType(param.Type()))
				}
			})
			b.AddUint64(e.AddType(t.Result()))
			b.AddUint64(e.AddString(t.String()))
		})
	default:
		panic(fmt.Sprintf("AddType(%T): type not supported", t))
	}
}

// AddFunction adds the function to the
// symbol and code sections.
func (e *encoder) AddFunction(fset *token.FileSet, arch *sys.Arch, pkg *compiler.Package, fun *ssafir.Function) error {
	var code bytes.Buffer
	err := compiler.EncodeTo(&code, fset, arch, fun)
	if err != nil {
		return fmt.Errorf("failed to compile %s.%s: %v", pkg.Path, fun.Name, err)
	}

	sym := &symbol{
		Kind:        SymKindFunction,
		PackageName: e.AddString(pkg.Path),
		Name:        e.AddString(fun.Name),
		Type:        e.AddType(fun.Type),
		Value:       e.AddCode(code.Bytes()),
	}

	source := symbolSize * uint64(len(e.symbols))
	for _, link := range fun.Links {
		// Builtin functions have no package.
		name := link.Name
		var pkg string
		if i := strings.LastIndexByte(link.Name, '.'); i > 0 {
			pkg, name = link.Name[:i], link.Name[i+1:]
		}

		e.linkages = append(e.linkages, &linkage{
			Source:        source,
			TargetPackage: e.AddString(pkg),
			TargetSymbol:  e.AddString(name),
			Type:          link.Type,
			Size:          uint32(link.Size),
			Offset:        uint32(link.Offset),
			Address:       uint32(link.Address),
		})
	}

	e.symbols = append(e.symbols, sym)

	return nil
}

// AddConstant adds the constant to the
// symbol (and possibly strings) section.
func (e *encoder) AddConstant(pkg *compiler.Package, con *types.Constant) error {
	var (
		kind        SymKind
		packageName = e.AddString(pkg.Path)
		name        = e.AddString(con.Name())
		value       uint64
	)

	val := con.Value()
	conType := con.Type()
	switch conType {
	case types.Bool, types.UntypedBool:
		kind = SymKindBooleanConstant
		if constant.BoolVal(val) {
			value = 1
		}
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64:
		kind = SymKindIntegerConstant
		v, _ := constant.Int64Val(val)
		value = uint64(v)
	case types.Uint, types.Uint8, types.Byte, types.Uint16, types.Uint32, types.Uint64, types.Uintptr:
		kind = SymKindIntegerConstant
		value, _ = constant.Uint64Val(val)
	case types.UntypedInt:
		switch v := constant.Val(val).(type) {
		case int64:
			kind = SymKindIntegerConstant
			value = uint64(v)
		case *big.Int:
			if v.Sign() >= 0 {
				kind = SymKindBigIntegerConstant
			} else {
				kind = SymKindBigNegativeIntegerConstant
			}
			raw := v.Bytes()
			value = e.AddString(string(raw))
		default:
			return fmt.Errorf("rpkg: internal error: found constant %s.%s with type %#v: neither int64 nor *big.Int", pkg.Path, con.Name(), conType)
		}
	case types.String, types.UntypedString:
		kind = SymKindStringConstant
		value = e.AddString(constant.StringVal(val))
	default:
		return fmt.Errorf("failed to record the type for constant %s.%s (%#v)", pkg.Path, con.Name(), conType)
	}

	sym := &symbol{
		Kind:        kind,
		PackageName: packageName,
		Name:        name,
		Type:        e.AddType(conType),
		Value:       value,
	}

	e.symbols = append(e.symbols, sym)

	return nil
}

// AddString ensures that `s` is included
// exactly once in the rpkg file. The string's
// offset into the string section is returned.
//
// The string must have a length that fits in
// a uint32.
func (e *encoder) AddString(s string) uint64 {
	offset, ok := e.stringOffsets[s]
	if ok {
		return offset
	}

	if len(s) > math.MaxUint32 {
		panic("string too large: length overflows uint32")
	}

	offset = e.stringOffset
	e.strings = append(e.strings, s)
	e.stringOffset += 4 + uint64(len(s))
	if e.stringOffset%4 != 0 {
		e.stringOffset += 4 - (e.stringOffset % 4)
	}
	e.stringOffsets[s] = offset

	return offset
}

// AddCode includes `code` in the rpkg file.
// The code's offset into the code section is
// returned.
//
// The code must have a length that fits in
// a uint32.
func (e *encoder) AddCode(code []byte) uint64 {
	if len(code) > math.MaxUint32 {
		panic("code too large: length overflows uint32")
	}

	offset := e.codeOffset
	e.code = append(e.code, code)
	e.codeOffset += 4 + uint64(len(code))
	if e.codeOffset%4 != 0 {
		e.codeOffset += 4 - (e.codeOffset % 4)
	}

	return offset
}

func (h *header) Marshal(b *cryptobyte.Builder) error {
	b.AddUint32(h.Magic)
	b.AddUint8(uint8(h.Architecture))
	b.AddUint8(h.Version)
	b.AddUint16(h.PackageName)
	b.AddUint32(h.ImportsOffset)
	b.AddUint64(h.TypesOffset)
	b.AddUint64(h.SymbolsOffset)
	b.AddUint64(h.StringsOffset)
	b.AddUint64(h.LinkagesOffset)
	b.AddUint64(h.CodeOffset)
	b.AddUint64(h.ChecksumOffset)

	return nil
}

func (s *symbol) Marshal(b *cryptobyte.Builder) error {
	b.AddUint32(uint32(s.Kind))
	b.AddUint64(s.PackageName)
	b.AddUint64(s.Name)
	b.AddUint64(s.Type)
	b.AddUint64(s.Value)

	return nil
}

func (l *linkage) Marshal(b *cryptobyte.Builder) error {
	b.AddUint64(l.Source)
	b.AddUint64(l.TargetPackage)
	b.AddUint64(l.TargetSymbol)
	b.AddUint8(uint8(l.Type))
	b.AddUint24(l.Size)
	b.AddUint32(l.Offset)
	b.AddUint32(l.Address)

	return nil
}

// WriteTo encodes the rpkg file to w.
func (e *encoder) WriteTo(w io.Writer) (n int64, err error) {
	b := cryptobyte.NewFixedBuilder(make([]byte, 0, e.header.ChecksumOffset+e.header.ChecksumLength))
	b.AddValue(&e.header)

	for _, imp := range e.imports {
		b.AddUint32(imp)
	}

	for _, typ := range e.types {
		b.AddBytes(typ)
		switch len(typ) % 4 {
		case 1:
			b.AddUint24(0)
		case 2:
			b.AddUint16(0)
		case 3:
			b.AddUint8(0)
		}
	}

	for _, sym := range e.symbols {
		b.AddValue(sym)
	}

	for _, s := range e.strings {
		b.AddUint32(uint32(len(s)))
		b.AddBytes([]byte(s))
		switch len(s) % 4 {
		case 1:
			b.AddUint24(0)
		case 2:
			b.AddUint16(0)
		case 3:
			b.AddUint8(0)
		}
	}

	for _, link := range e.linkages {
		b.AddValue(link)
	}

	for _, f := range e.code {
		b.AddUint32(uint32(len(f)))
		b.AddBytes(f)
		switch len(f) % 4 {
		case 1:
			b.AddUint24(0)
		case 2:
			b.AddUint16(0)
		case 3:
			b.AddUint8(0)
		}
	}

	// Add the checksum.
	buf := b.BytesOrPanic()
	if uint64(len(buf)) != e.header.ChecksumOffset {
		return 0, fmt.Errorf("rpkg: internal error: encoded rpkg has length %d before the checksum, expected %d", len(buf), e.header.ChecksumOffset)
	}

	sum := sha256.Sum256(buf)
	buf = append(buf, sum[:]...)

	m, err := w.Write(buf)
	return int64(m), err
}

// Encode is used to create an rpkg file.
//
// The type `info` must have been populated
// with both `List` and `Indices` fields.
func Encode(w io.Writer, fset *token.FileSet, arch *sys.Arch, pkg *compiler.Package, info *types.Info) error {
	// We build the sections individually, using
	// the cryptobyte package to ensure a correct
	// encoding.
	var _ = (*cryptobyte.Builder)(nil)
	e := &encoder{
		typesOffsets:  make(map[string]uint64),
		stringOffsets: make(map[string]uint64),
	}

	// Build the rpkg file.
	e.AddType(nil)  // The nil type is always at offset 0.
	e.AddString("") // The empty string is always at offset 0.
	e.AddString(pkg.Path)

	// Add the imports early, so that
	// they're at the beginning of the
	// strings section, ensuring that
	// all of the strings offsets fit
	// within uint32s.
	e.imports = make([]uint32, len(pkg.Imports))
	for i, imp := range pkg.Imports {
		offset := e.AddString(imp)
		if offset >= math.MaxUint32 {
			return fmt.Errorf("cannot encode import %q: offset %d overflows uint32", imp, offset)
		}

		e.imports[i] = uint32(offset)
	}

	for _, fun := range pkg.Functions {
		err := e.AddFunction(fset, arch, pkg, fun)
		if err != nil {
			return err
		}
	}

	for _, con := range pkg.Constants {
		err := e.AddConstant(pkg, con)
		if err != nil {
			return err
		}
	}

	err := e.AddHeader(arch, pkg)
	if err != nil {
		return err
	}

	_, err = e.WriteTo(w)
	if err != nil {
		return err
	}

	return nil
}
