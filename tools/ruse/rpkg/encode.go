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
	"strconv"
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
	arch   *sys.Arch

	imports []uint32

	exports []uint64

	// Used to build the types section
	// efficiently. This state is managed
	// by AddType.
	types        [][]byte
	typesOffset  uint64
	typesOffsets map[string]uint64

	symbols       []*symbol
	symbolOffsets map[types.Object]uint64

	// Used to build the ABIs section
	// efficiently. This state is managed
	// by AddABI.
	abis        [][]byte
	abisOffset  uint32
	abisOffsets map[string]uint32

	// Used to build the sections section
	// efficiently. This state is managed
	// by AddSection.
	sections        [][]byte
	sectionsOffset  uint32
	sectionsOffsets map[string]uint32

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
	code       []*function
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

	var baseAddr uint64
	if pkg.BaseAddr == nil {
		if pkg.Name == "main" {
			baseAddr = 0x20_0000 // 2 MiB in by default.
		}
	} else {
		var err error
		baseAddr, err = strconv.ParseUint(pkg.BaseAddr.Value, 0, 64)
		if err != nil {
			return fmt.Errorf("invalid base address: %v", err)
		}
	}

	// Build the header.
	e.header.Magic = magic
	e.header.Architecture = architecture
	e.header.Version = version
	e.header.PackageName = uint16(e.AddString(pkg.Path))
	e.header.BaseAddress = baseAddr
	e.header.ImportsOffset = headerSize
	e.header.ImportsLength = 4 * uint32(len(e.imports))
	e.header.ExportsOffset = e.header.ImportsOffset + e.header.ImportsLength
	e.header.ExportsLength = 8 * uint32(len(e.exports))
	e.header.TypesOffset = uint64(e.header.ExportsOffset) + uint64(e.header.ExportsLength)
	e.header.TypesLength = e.typesOffset
	e.header.SymbolsOffset = e.header.TypesOffset + e.header.TypesLength
	e.header.SymbolsLength = symbolSize * uint64(len(e.symbols))
	e.header.ABIsOffset = e.header.SymbolsOffset + e.header.SymbolsLength
	e.header.ABIsLength = e.abisOffset
	e.header.SectionsOffset = e.header.ABIsOffset + uint64(e.header.ABIsLength)
	e.header.SectionsLength = e.sectionsOffset
	e.header.StringsOffset = e.header.SectionsOffset + uint64(e.header.SectionsLength)
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
	case types.ABI:
		b.AddUint8(uint8(TypeKindABI))
		b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddUint32(e.AddABI(t.ABI()))
		})
	case types.Section:
		b.AddUint8(uint8(TypeKindSection))
		b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddUint32(e.AddSection(t))
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
		SectionName: e.AddString(fun.Section),
		Type:        e.AddType(fun.Type),
		Value:       e.AddCode(e.AddABI(fun.Func.ABI()), code.Bytes()),
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

	e.symbolOffsets[fun.Func] = symbolSize * uint64(len(e.symbols))
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
		if _, ok := conType.(types.ABI); ok {
			kind = SymKindABI
			value = 0
			break
		}

		if _, ok := conType.(types.Section); ok {
			kind = SymKindSection
			value = 0
			break
		}

		return fmt.Errorf("failed to record the type for constant %s.%s (%#v)", pkg.Path, con.Name(), conType)
	}

	sym := &symbol{
		Kind:        kind,
		PackageName: packageName,
		Name:        name,
		SectionName: e.AddString(con.Section()),
		Type:        e.AddType(conType),
		Value:       value,
	}

	e.symbolOffsets[con] = symbolSize * uint64(len(e.symbols))
	e.symbols = append(e.symbols, sym)

	return nil
}

// AddABI appends the ABI to the typeABIs
// section.
func (e *encoder) AddABI(abi *sys.ABI) uint32 {
	var b *cryptobyte.Builder
	if abi == nil {
		b = cryptobyte.NewFixedBuilder(make([]byte, 4))
	} else {
		length := 4 + // Overall length.
			minABILength + // InvertedStack and the length fields
			len(abi.ParamRegisters) +
			len(abi.ResultRegisters) +
			len(abi.ScratchRegisters) +
			len(abi.UnusedRegisters)
		if length%4 != 0 {
			length += 4 - (length % 4)
		}

		var invertedStack uint8
		if abi.InvertedStack {
			invertedStack = 1
		}

		regs := make(map[sys.Location]uint8, len(e.arch.ABIRegisters)+1)
		regs[e.arch.StackPointer] = abiStackPointer
		for i, reg := range e.arch.ABIRegisters {
			regs[reg] = uint8(i)
		}

		b = cryptobyte.NewFixedBuilder(make([]byte, 0, length))
		b.AddUint32LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddUint8(invertedStack)
			b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
				for _, reg := range abi.ParamRegisters {
					b.AddUint8(regs[reg])
				}
			})
			b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
				for _, reg := range abi.ResultRegisters {
					b.AddUint8(regs[reg])
				}
			})
			b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
				for _, reg := range abi.ScratchRegisters {
					b.AddUint8(regs[reg])
				}
			})
			b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
				for _, reg := range abi.UnusedRegisters {
					b.AddUint8(regs[reg])
				}
			})
		})
	}

	data := b.BytesOrPanic()

	offset, ok := e.abisOffsets[string(data)]
	if ok {
		return offset
	}

	offset = e.abisOffset
	e.abis = append(e.abis, data)
	e.abisOffset += uint32(len(data))
	if e.abisOffset%4 != 0 {
		e.abisOffset += 4 - (e.abisOffset % 4) // Padding.
	}

	e.abisOffsets[string(data)] = offset

	return offset
}

// AddSection appends the program section to
// the sections section.
func (e *encoder) AddSection(section types.Section) uint32 {
	var b *cryptobyte.Builder
	if section == (types.Section{}) {
		b = cryptobyte.NewFixedBuilder(make([]byte, 24))
	} else {
		base := section.Section()
		fixedAddr := section.FixedAddr()
		b = cryptobyte.NewFixedBuilder(make([]byte, 0, 24))
		b.AddUint64(e.AddString(base.Name))
		b.AddUint64(uint64(base.Address))
		b.AddUint8(uint8(base.Permissions))
		if fixedAddr {
			b.AddUint8(1)
		} else {
			b.AddUint8(0)
		}
		b.AddUint16(0) // Padding.
		b.AddUint32(0) // Padding.
	}

	data := b.BytesOrPanic()

	offset, ok := e.sectionsOffsets[string(data)]
	if ok {
		return offset
	}

	offset = e.sectionsOffset
	e.sections = append(e.sections, data)
	e.sectionsOffset += uint32(len(data))

	e.sectionsOffsets[string(data)] = offset

	return offset
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
func (e *encoder) AddCode(abi uint32, code []byte) uint64 {
	if len(code) > math.MaxUint32 {
		panic("code too large: length overflows uint32")
	}

	offset := e.codeOffset
	e.code = append(e.code, &function{ABI: abi, Code: code})
	e.codeOffset += 4 + 4 + uint64(len(code))
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
	b.AddUint64(h.BaseAddress)
	b.AddUint32(h.ImportsOffset)
	b.AddUint32(h.ExportsOffset)
	b.AddUint64(h.TypesOffset)
	b.AddUint64(h.SymbolsOffset)
	b.AddUint64(h.ABIsOffset)
	b.AddUint64(h.SectionsOffset)
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
	b.AddUint64(s.SectionName)
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

	for _, exp := range e.exports {
		b.AddUint64(exp)
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

	for _, abi := range e.abis {
		b.AddBytes(abi)
		switch len(abi) % 4 {
		case 1:
			b.AddUint24(0)
		case 2:
			b.AddUint16(0)
		case 3:
			b.AddUint8(0)
		}
	}

	for _, section := range e.sections {
		b.AddBytes(section)
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
		b.AddUint32(f.ABI)
		b.AddUint32(uint32(len(f.Code)))
		b.AddBytes(f.Code)
		switch len(f.Code) % 4 {
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
	e := &encoder{
		arch:            arch,
		typesOffsets:    make(map[string]uint64),
		symbolOffsets:   make(map[types.Object]uint64),
		abisOffsets:     make(map[string]uint32),
		sectionsOffsets: make(map[string]uint32),
		stringOffsets:   make(map[string]uint64),
	}

	// Build the rpkg file.
	e.AddType(nil)                // The nil type is always at offset 0.
	e.AddABI(nil)                 // The nil ABI is always at offset 0.
	e.AddSection(types.Section{}) // The nil section is always at offset 0.
	e.AddString("")               // The empty string is always at offset 0.
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

	noPkg := &compiler.Package{}
	for _, lit := range pkg.Literals {
		err := e.AddConstant(noPkg, lit)
		if err != nil {
			return err
		}
	}

	// Add any exports.
	scope := pkg.Types.Scope()
	names := scope.Names()
	for _, name := range names {
		obj := scope.Lookup(name)
		if obj == nil {
			return fmt.Errorf("failed to lookup symbol %q in package scope", name)
		}

		if !obj.Exported() {
			continue
		}

		offset, ok := e.symbolOffsets[obj]
		if !ok {
			for obj, offset := range e.symbolOffsets {
				fmt.Printf("1: %#v (%p): %d\n", obj, obj, offset)
			}
			fmt.Printf("2: %#v (%p)\n", types.Object(obj), obj)
			return fmt.Errorf("failed to lookup symbol offset for %q", name)
		}

		e.exports = append(e.exports, offset)
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
