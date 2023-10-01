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
	"path"

	"golang.org/x/crypto/cryptobyte"

	"firefly-os.dev/tools/ruse/compiler"
	"firefly-os.dev/tools/ruse/ssafir"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

// decodeHeader performs the first phase of
// decoding an rpkg; reading the header and
// verifying the checksum.
func decodeHeader(h *header, b []byte) error {
	if len(b) < headerSize {
		return fmt.Errorf("invalid rpkg header: %w", io.ErrUnexpectedEOF)
	}

	s := cryptobyte.String(b[:headerSize])

	// Start with the header.
	var arch uint8
	if !s.ReadUint32(&h.Magic) ||
		!s.ReadUint8(&arch) ||
		!s.ReadUint8(&h.Version) ||
		!s.ReadUint16(&h.PackageName) ||
		!s.ReadUint32(&h.ImportsOffset) ||
		!s.ReadUint32(&h.ExportsOffset) ||
		!s.ReadUint64(&h.TypesOffset) ||
		!s.ReadUint64(&h.SymbolsOffset) ||
		!s.ReadUint64(&h.ABIsOffset) ||
		!s.ReadUint64(&h.StringsOffset) ||
		!s.ReadUint64(&h.LinkagesOffset) ||
		!s.ReadUint64(&h.CodeOffset) ||
		!s.ReadUint64(&h.ChecksumOffset) {
		return fmt.Errorf("rpkg: internal error: failed to read rpkg header: %w", io.ErrUnexpectedEOF)
	}

	// Sanity-check the header.
	if h.Magic != magic {
		return fmt.Errorf("invalid rpkg header: got magic %x, want %x", h.Magic, magic)
	}

	h.Architecture = Arch(arch)
	h.ImportsLength = h.ExportsOffset - h.ImportsOffset
	h.ExportsLength = uint32(h.TypesOffset) - h.ExportsOffset
	h.TypesLength = h.SymbolsOffset - h.TypesOffset
	h.SymbolsLength = h.ABIsOffset - h.SymbolsOffset
	h.ABIsLength = uint32(h.StringsOffset - h.ABIsOffset)
	h.StringsLength = h.LinkagesOffset - h.StringsOffset
	h.LinkagesLength = h.CodeOffset - h.LinkagesOffset
	h.CodeLength = h.ChecksumOffset - h.CodeOffset
	h.ChecksumLength = ChecksumLength

	switch h.Architecture {
	case ArchX86_64:
	default:
		return fmt.Errorf("invalid rpkg header: unrecognised architecture %d", h.Architecture)
	}
	if h.Version != version {
		return fmt.Errorf("unsupported rpkg header: got version %d, but only %d is supported", h.Version, version)
	}
	if uint64(h.PackageName) >= h.StringsLength {
		return fmt.Errorf("invalid rpkg header: package name offset %d is beyond strings section", h.PackageName)
	}
	if h.ImportsOffset != headerSize {
		return fmt.Errorf("invalid rpkg header: got imports offset %d, want %d", h.ImportsOffset, headerSize)
	}
	if h.ImportsLength%4 != 0 {
		return fmt.Errorf("invalid rpkg header: got invalid imports length %d", h.ImportsLength)
	}
	if h.ExportsOffset < h.ImportsOffset || uint64(h.ExportsOffset) > h.TypesOffset || h.ExportsOffset%4 != 0 {
		return fmt.Errorf("invalid rpkg header: got invalid exports offset %d", h.ExportsOffset)
	}
	if h.ExportsLength%8 != 0 {
		return fmt.Errorf("invalid rpkg header: got invalid exports length %d", h.ExportsLength)
	}
	if h.TypesOffset < uint64(h.ExportsOffset) || h.TypesOffset > h.SymbolsOffset || h.TypesOffset%4 != 0 {
		return fmt.Errorf("invalid rpkg header: got invalid types offset %d", h.TypesOffset)
	}
	if h.SymbolsOffset > h.ABIsOffset || h.SymbolsOffset%4 != 0 {
		return fmt.Errorf("invalid rpkg header: got invalid symbols offset %d", h.SymbolsOffset)
	}
	if h.SymbolsLength%symbolSize != 0 {
		return fmt.Errorf("invalid rpkg header: got invalid symbols length %d", h.SymbolsLength)
	}
	if h.ABIsOffset > h.StringsOffset || h.ABIsOffset%4 != 0 {
		return fmt.Errorf("invalid rpkg header: got invalid ABIs offset %d", h.ABIsOffset)
	}
	if (h.StringsOffset - h.ABIsOffset) > math.MaxUint32 {
		return fmt.Errorf("invalid rpkg header: ABIs length %d overflows uint32", h.ABIsLength)
	}
	if h.StringsOffset > h.LinkagesOffset || h.StringsOffset%4 != 0 {
		return fmt.Errorf("invalid rpkg header: got strings offset %d", h.StringsOffset)
	}
	if h.StringsLength%4 != 0 {
		return fmt.Errorf("invalid rpkg header: got invalid strings length %d", h.StringsLength)
	}
	if h.LinkagesOffset > h.CodeOffset || h.LinkagesOffset%4 != 0 {
		return fmt.Errorf("invalid rpkg header: got linkages offset %d", h.LinkagesOffset)
	}
	if h.LinkagesLength%4 != 0 {
		return fmt.Errorf("invalid rpkg header: got invalid linkages length %d", h.LinkagesLength)
	}
	if h.CodeOffset > h.ChecksumOffset || h.CodeOffset%4 != 0 {
		return fmt.Errorf("invalid rpkg header: got code offset %d", h.CodeOffset)
	}
	if h.CodeLength%4 != 0 {
		return fmt.Errorf("invalid rpkg header: got invalid code length %d", h.CodeLength)
	}
	if h.ChecksumOffset%4 != 0 {
		return fmt.Errorf("invalid rpkg header: got checksum offset %d", h.ChecksumOffset)
	}

	return nil
}

// This set of functionality is only used for testing
// the encoding process and for debugging. It just
// decodes into a structured representation of the
// encoded form.
//
// By contrast, the proper decoding code transforms
// the result to richer, more complex data types.

// decoded contains structured contents of an
// rpkg file.
type decoded struct {
	header   header
	imports  []uint32
	exports  []uint64
	types    map[uint64]typeSplat
	symbols  map[uint64]*symbol
	abis     map[uint32]*abi
	strings  map[uint64]string
	linkages map[uint64]*linkage
	code     map[uint64]*function
}

// decodeSimple performs the first phase of decoding
// an rpkg; pulling out the different sections
// and verifying the checksum.
func decodeSimple(b []byte) (*decoded, error) {
	var d decoded
	err := decodeHeader(&d.header, b)
	if err != nil {
		return nil, err
	}

	// Verify the checksum.
	if d.header.ChecksumOffset+d.header.ChecksumLength != uint64(len(b)) {
		return nil, fmt.Errorf("invalid rpkg header: got file ending %d, found %d bytes", d.header.ChecksumOffset+d.header.ChecksumLength, len(b))
	}

	checksum := b[len(b)-ChecksumLength:]
	want := ([ChecksumLength]byte)(checksum)
	got := sha256.Sum256(b[:len(b)-ChecksumLength])
	if got != want {
		return nil, fmt.Errorf("invalid rpkg file: checksum mismatch")
	}

	// Read the types section.
	s := cryptobyte.String(b[d.header.TypesOffset:d.header.SymbolsOffset])
	d.types, err = d.decodeTypes(s)
	if err != nil {
		return nil, err
	}

	// Read the symbols section.
	s = cryptobyte.String(b[d.header.SymbolsOffset:d.header.ABIsOffset])
	d.symbols, err = d.decodeSymbols(s)
	if err != nil {
		return nil, err
	}

	// Read the ABIs section.
	s = cryptobyte.String(b[d.header.ABIsOffset:d.header.StringsOffset])
	d.abis, err = d.decodeABIs(s)
	if err != nil {
		return nil, err
	}

	// Read the strings section.
	s = cryptobyte.String(b[d.header.StringsOffset:d.header.LinkagesOffset])
	d.strings, err = d.decodeStrings(s)
	if err != nil {
		return nil, err
	}

	// Read the linkages section.
	s = cryptobyte.String(b[d.header.LinkagesOffset:d.header.CodeOffset])
	d.linkages, err = d.decodeLinkages(s)
	if err != nil {
		return nil, err
	}

	// Read the code section.
	s = cryptobyte.String(b[d.header.CodeOffset:d.header.ChecksumOffset])
	d.code, err = d.decodeCode(s)
	if err != nil {
		return nil, err
	}

	// Read the imports section.
	s = cryptobyte.String(b[d.header.ImportsOffset:d.header.ExportsOffset])
	d.imports, err = d.decodeImports(s)
	if err != nil {
		return nil, err
	}

	// Read the exports section.
	s = cryptobyte.String(b[d.header.ExportsOffset:d.header.TypesOffset])
	d.exports, err = d.decodeExports(s)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

// decodeImports reads the imports from `s`,
// checking that each import is valid.
func (d *decoded) decodeImports(s cryptobyte.String) (imports []uint32, err error) {
	var imp uint32
	imports = make([]uint32, 0, len(s)/4)
	for !s.Empty() {
		if !s.ReadUint32(&imp) {
			return nil, fmt.Errorf("invalid imports section: %w", io.ErrUnexpectedEOF)
		}

		if _, ok := d.strings[uint64(imp)]; !ok {
			return nil, fmt.Errorf("invalid imports section: import %d is not a valid string offset", imp)
		}

		imports = append(imports, imp)
	}

	return imports, nil
}

// decodeExports reads the exports from `s`,
// checking that each export is valid.
func (d *decoded) decodeExports(s cryptobyte.String) (exports []uint64, err error) {
	var exp uint64
	exports = make([]uint64, 0, len(s)/8)
	for !s.Empty() {
		if !s.ReadUint64(&exp) {
			return nil, fmt.Errorf("invalid exports section: %w", io.ErrUnexpectedEOF)
		}

		if _, ok := d.symbols[exp]; !ok {
			return nil, fmt.Errorf("invalid exports section: export %d is not a valid symbol offset", exp)
		}

		exports = append(exports, exp)
	}

	return exports, nil
}

// decodeTypes reads the types from `s`,
// checking that each is valid.
func (d *decoded) decodeTypes(s cryptobyte.String) (types map[uint64]typeSplat, err error) {
	var offset uint64
	types = make(map[uint64]typeSplat)
	for !s.Empty() {
		here := offset
		var kind uint8
		var rest cryptobyte.String
		if !s.ReadUint8(&kind) ||
			!s.ReadUint24LengthPrefixed(&rest) {
			return nil, fmt.Errorf("failed to read type: %w", io.ErrUnexpectedEOF)
		}

		length := len(rest)
		offset += 1 + 3 + uint64(length)

		switch TypeKind(kind) {
		case TypeKindNone:
			if !rest.Empty() {
				return nil, fmt.Errorf("invalid type: got type kind %s with %d bytes of further data", TypeKind(kind), len(rest))
			}

			types[here] = typeSplat{
				Kind:   TypeKind(kind),
				Length: uint32(length),
			}
		case TypeKindBasic:
			var basic uint32
			if !rest.ReadUint32(&basic) {
				return nil, fmt.Errorf("invalid type: failed to read %s type kind: %w", TypeKind(kind), io.ErrUnexpectedEOF)
			}

			if !rest.Empty() {
				return nil, fmt.Errorf("invalid type: got type kind %s with %d bytes of further data", TypeKind(kind), len(rest))
			}

			types[here] = typeSplat{
				Kind:   TypeKind(kind),
				Length: uint32(length),
				Basic:  BasicKind(basic),
			}
		case TypeKindFunction:
			var paramsData []byte
			var paramsLength uint32
			var result, name uint64
			if !rest.ReadUint32(&paramsLength) ||
				!rest.ReadBytes(&paramsData, int(paramsLength)) ||
				!rest.ReadUint64(&result) ||
				!rest.ReadUint64(&name) {
				return nil, fmt.Errorf("invalid type: failed to read %s type kind: %w", TypeKind(kind), io.ErrUnexpectedEOF)
			}

			if !rest.Empty() {
				return nil, fmt.Errorf("invalid type: got type kind %s with %d bytes of further data", TypeKind(kind), len(rest))
			}

			params := make([]variable, 0, paramsLength/16)
			paramsString := cryptobyte.String(paramsData)
			for !paramsString.Empty() {
				var name, typ uint64
				if !paramsString.ReadUint64(&name) ||
					!paramsString.ReadUint64(&typ) {
					return nil, fmt.Errorf("invalid type: failed to read %s type kind parameter: %w", TypeKind(kind), io.ErrUnexpectedEOF)
				}

				params = append(params, variable{Name: name, Type: typ})
			}

			if result >= d.header.TypesLength {
				return nil, fmt.Errorf("invalid type: %s result %d is beyond types section", TypeKind(kind), result)
			}
			if name >= d.header.StringsLength {
				return nil, fmt.Errorf("invalid type: %s name %d is beyond strings section", TypeKind(kind), name)
			}

			types[here] = typeSplat{
				Kind:         TypeKind(kind),
				Length:       uint32(length),
				ParamsLength: paramsLength,
				Params:       params,
				Result:       result,
				Name:         name,
			}
		default:
			return nil, fmt.Errorf("invalid type: got unrecognised type kind %d", kind)
		}
	}

	return types, nil
}

// decodeSymbols reads the symbols from `s`,
// checking that each symbol is valid.
func (d *decoded) decodeSymbols(s cryptobyte.String) (symbols map[uint64]*symbol, err error) {
	var offset uint64
	symbols = make(map[uint64]*symbol)
	for !s.Empty() {
		var sym symbol
		var kind uint32
		if !s.ReadUint32(&kind) ||
			!s.ReadUint64(&sym.PackageName) ||
			!s.ReadUint64(&sym.Name) ||
			!s.ReadUint64(&sym.Type) ||
			!s.ReadUint64(&sym.Value) {
			return nil, fmt.Errorf("failed to read symbol: %w", io.ErrUnexpectedEOF)
		}

		here := offset
		offset += symbolSize

		sym.Kind = SymKind(kind)
		switch sym.Kind {
		case SymKindBooleanConstant:
			if sym.Value != 0 && sym.Value != 1 {
				return nil, fmt.Errorf("invalid symbol: got value %d, want 0 or 1 for kind %q", sym.Value, sym.Kind)
			}
		case SymKindIntegerConstant:
		case SymKindBigIntegerConstant, SymKindBigNegativeIntegerConstant:
			if sym.Value >= d.header.StringsLength {
				return nil, fmt.Errorf("invalid symbol: %s value %d is beyond strings section", sym.Kind, sym.Value)
			}
		case SymKindStringConstant:
			if sym.Value >= d.header.StringsLength {
				return nil, fmt.Errorf("invalid symbol: %s value %d is beyond strings section", sym.Kind, sym.Value)
			}
		case SymKindFunction:
			if sym.Value >= d.header.CodeLength {
				return nil, fmt.Errorf("invalid symbol: %s value %d is beyond code section", sym.Kind, sym.Value)
			}
		default:
			return nil, fmt.Errorf("invalid symbol: unrecognised kind %d", sym.Kind)
		}

		if sym.PackageName >= d.header.StringsLength {
			return nil, fmt.Errorf("invalid symbol: package path offset %d is beyond strings section", sym.PackageName)
		}

		if sym.Name >= d.header.StringsLength {
			return nil, fmt.Errorf("invalid symbol: name offset %d is beyond strings section", sym.Name)
		}

		if sym.Type == 0 {
			return nil, fmt.Errorf("invalid symbol: got type %d for kind %q", sym.Type, sym.Kind)
		}

		symbols[here] = &sym
	}

	return symbols, nil
}

// decodeABIs reads the ABIs from `s`,
// checking that each ABI is valid.
func (d *decoded) decodeABIs(s cryptobyte.String) (ABIs map[uint32]*abi, err error) {
	var offset uint32
	ABIs = make(map[uint32]*abi)
	for !s.Empty() {
		var length uint32
		var rest []byte
		if !s.ReadUint32(&length) ||
			!s.ReadBytes(&rest, int(length)) {
			return nil, fmt.Errorf("failed to read ABI: %w", io.ErrUnexpectedEOF)
		}

		here := offset
		offset += 4 + length
		if length == 0 {
			ABIs[here] = nil
			continue
		}
		if length < minABILength {
			return nil, fmt.Errorf("invalid ABI: length %d is less than %d", length, minABILength)
		}

		abiString := cryptobyte.String(rest)

		var invertedStack uint8
		var params, result, scratch, unused cryptobyte.String
		if !abiString.ReadUint8(&invertedStack) ||
			!abiString.ReadUint8LengthPrefixed(&params) ||
			!abiString.ReadUint8LengthPrefixed(&result) ||
			!abiString.ReadUint8LengthPrefixed(&scratch) ||
			!abiString.ReadUint8LengthPrefixed(&unused) ||
			!abiString.Empty() {
			return nil, fmt.Errorf("failed to read ABI: %w", io.ErrUnexpectedEOF)
		}

		if invertedStack != 1 && invertedStack != 0 {
			return nil, fmt.Errorf("invalid ABI: got inverted stack %d, want 0 or 1", invertedStack)
		}

		switch length % 4 {
		case 1:
			offset += 3
			var padding uint32
			if !s.ReadUint24(&padding) {
				return nil, fmt.Errorf("invalid ABIs section: %w", io.ErrUnexpectedEOF)
			}
			if padding != 0 {
				return nil, fmt.Errorf("invalid ABIs section: invalid padding %06x", padding)
			}
		case 2:
			offset += 2
			var padding uint16
			if !s.ReadUint16(&padding) {
				return nil, fmt.Errorf("invalid ABIs section: %w", io.ErrUnexpectedEOF)
			}
			if padding != 0 {
				return nil, fmt.Errorf("invalid ABIs section: invalid padding %04x", padding)
			}
		case 3:
			offset += 1
			var padding uint8
			if !s.ReadUint8(&padding) {
				return nil, fmt.Errorf("invalid ABIs section: %w", io.ErrUnexpectedEOF)
			}
			if padding != 0 {
				return nil, fmt.Errorf("invalid ABIs section: invalid padding %02x", padding)
			}
		}

		ABIs[here] = &abi{
			Length:        length,
			InvertedStack: invertedStack == 1,
			Params:        []uint8(params),
			Result:        []uint8(result),
			Scratch:       []uint8(scratch),
			Unused:        []uint8(unused),
		}
	}

	return ABIs, nil
}

// decodeStrings reads the strings from `s`,
// checking that each string is valid.
func (d *decoded) decodeStrings(s cryptobyte.String) (strings map[uint64]string, err error) {
	var offset uint64
	var length uint32
	strings = make(map[uint64]string)
	for !s.Empty() {
		var data []byte
		here := offset
		if !s.ReadUint32(&length) ||
			!s.ReadBytes(&data, int(length)) {
			return nil, fmt.Errorf("invalid strings section: %w", io.ErrUnexpectedEOF)
		}

		offset += 4 + uint64(length)
		switch length % 4 {
		case 1:
			offset += 3
			var padding uint32
			if !s.ReadUint24(&padding) {
				return nil, fmt.Errorf("invalid strings section: %w", io.ErrUnexpectedEOF)
			}
			if padding != 0 {
				return nil, fmt.Errorf("invalid strings section: invalid padding %06x", padding)
			}
		case 2:
			offset += 2
			var padding uint16
			if !s.ReadUint16(&padding) {
				return nil, fmt.Errorf("invalid strings section: %w", io.ErrUnexpectedEOF)
			}
			if padding != 0 {
				return nil, fmt.Errorf("invalid strings section: invalid padding %04x", padding)
			}
		case 3:
			offset += 1
			var padding uint8
			if !s.ReadUint8(&padding) {
				return nil, fmt.Errorf("invalid strings section: %w", io.ErrUnexpectedEOF)
			}
			if padding != 0 {
				return nil, fmt.Errorf("invalid strings section: invalid padding %02x", padding)
			}
		}

		strings[here] = string(data)
	}

	return strings, nil
}

// decodeLinkages reads the linkages from `s`,
// checking that each link is valid.
func (d *decoded) decodeLinkages(s cryptobyte.String) (linkages map[uint64]*linkage, err error) {
	var offset uint64
	linkages = make(map[uint64]*linkage)
	for !s.Empty() {
		var link linkage
		var typ uint8
		if !s.ReadUint64(&link.Source) ||
			!s.ReadUint64(&link.TargetPackage) ||
			!s.ReadUint64(&link.TargetSymbol) ||
			!s.ReadUint8(&typ) ||
			!s.ReadUint24(&link.Size) ||
			!s.ReadUint32(&link.Offset) ||
			!s.ReadUint32(&link.Address) {
			return nil, fmt.Errorf("invalid linkages section: %w", io.ErrUnexpectedEOF)
		}

		here := offset
		offset += linkageSize

		if link.Source > d.header.SymbolsLength {
			return nil, fmt.Errorf("invalid linkage: source symbol offset %d is beyond symbols section", link.Source)
		}
		if link.Source%symbolSize != 0 {
			return nil, fmt.Errorf("invalid linkage: source symbol offset %d is not on a symbol boundary", link.Source)
		}
		if link.TargetPackage > d.header.StringsLength {
			return nil, fmt.Errorf("invalid linkage: target symbol package offset %d is beyond strings section", link.TargetPackage)
		}
		if link.TargetSymbol > d.header.StringsLength {
			return nil, fmt.Errorf("invalid linkage: target symbol name offset %d is beyond strings section", link.TargetSymbol)
		}
		link.Type = ssafir.LinkType(typ)
		switch link.Type {
		case ssafir.LinkFullAddress,
			ssafir.LinkRelativeAddress:
		default:
			return nil, fmt.Errorf("invalid linkage: link type %v not recognised", link.Type)
		}
		if link.Size > 64 {
			return nil, fmt.Errorf("invalid linkage: implausible address size %d", link.Size)
		}
		if uint64(link.Offset) > d.header.CodeLength {
			return nil, fmt.Errorf("invalid linkage: function code offset %d is beyond code section", link.Offset)
		}
		if uint64(link.Address) > d.header.CodeLength {
			return nil, fmt.Errorf("invalid linkage: function code address offset %d is beyond code section", link.Address)
		}

		linkages[here] = &link
	}

	return linkages, nil
}

// decodeCode reads the functions from `s`,
// checking that each is valid.
func (d *decoded) decodeCode(s cryptobyte.String) (code map[uint64]*function, err error) {
	var abi uint32
	var offset uint64
	var length uint32
	code = make(map[uint64]*function)
	for !s.Empty() {
		var data []byte
		here := offset
		if !s.ReadUint32(&abi) ||
			!s.ReadUint32(&length) ||
			!s.ReadBytes(&data, int(length)) {
			return nil, fmt.Errorf("invalid code section: %w", io.ErrUnexpectedEOF)
		}

		offset += 4 + uint64(length)
		switch length % 4 {
		case 1:
			offset += 3
			var padding uint32
			if !s.ReadUint24(&padding) {
				return nil, fmt.Errorf("invalid code section: %w", io.ErrUnexpectedEOF)
			}
			if padding != 0 {
				return nil, fmt.Errorf("invalid code section: invalid padding %06x", padding)
			}
		case 2:
			offset += 2
			var padding uint16
			if !s.ReadUint16(&padding) {
				return nil, fmt.Errorf("invalid code section: %w", io.ErrUnexpectedEOF)
			}
			if padding != 0 {
				return nil, fmt.Errorf("invalid code section: invalid padding %04x", padding)
			}
		case 3:
			offset += 1
			var padding uint8
			if !s.ReadUint8(&padding) {
				return nil, fmt.Errorf("invalid code section: %w", io.ErrUnexpectedEOF)
			}
			if padding != 0 {
				return nil, fmt.Errorf("invalid code section: invalid padding %02x", padding)
			}
		}

		code[here] = &function{
			ABI:  abi,
			Code: data,
		}
	}

	return code, nil
}

// This is the proper decoding code, which returns
// richer, more complex data representations. For
// example, rather than returning a string offset,
// we fetch the string at that offset and return
// the string.

// Decoder is a helper type for decoding an rpkg into
// a compiled package. This loses some information,
// particularly source code position information, but
// should still have enough to be effective.
type Decoder struct {
	b []byte

	header header
	arch   *sys.Arch

	pkg *types.Package

	packageName string

	allImports  []string                // Cached result from Imports.
	allExports  []types.Object          // Cached result from Exports.
	allTypes    []types.Type            // Cached result from Types.
	types       map[uint64]types.Type   // Cached lookup of each type.
	allSymbols  []*Symbol               // Cached result from Symbols.
	allObjects  []types.Object          // Cached result from Symbols.
	symbols     map[uint64]*Symbol      // Cached lookup of each symbol.
	objects     map[uint64]types.Object // Cached lookup of each object.
	allABIs     []*sys.ABI              // Cached result from ABIs.
	abis        map[uint32]*sys.ABI     // Cached lookup of each ABI.
	allStrings  []string                // Cached result from Strings.
	strings     map[uint64]string       // Cached lookup of each string.
	allLinkages []*Linkage              // Cached result from Linkages.
	code        map[uint64]*Function    // Cached lookup of each function.
}

// NewDecoder helps parse an rpkg into a compiled package.
func NewDecoder(b []byte) (*Decoder, error) {
	d := &Decoder{
		b:     b,
		types: make(map[uint64]types.Type),
		abis:  make(map[uint32]*sys.ABI),
		code:  make(map[uint64]*Function),
	}

	err := decodeHeader(&d.header, b)
	if err != nil {
		return nil, err
	}

	// Verify the checksum.
	if d.header.ChecksumOffset+d.header.ChecksumLength != uint64(len(b)) {
		return nil, fmt.Errorf("invalid rpkg header: got file ending %d, found %d bytes", d.header.ChecksumOffset+d.header.ChecksumLength, len(b))
	}

	checksum := b[len(b)-ChecksumLength:]
	want := ([ChecksumLength]byte)(checksum)
	got := sha256.Sum256(b[:len(b)-ChecksumLength])
	if got != want {
		return nil, fmt.Errorf("invalid rpkg file: checksum mismatch")
	}

	d.packageName, err = d.getString(uint64(d.header.PackageName))
	if err != nil {
		return nil, fmt.Errorf("invalid rpkg header: invalid package name: %v", err)
	}

	switch d.header.Architecture {
	case ArchX86_64:
		d.arch = sys.X86_64
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", d.header.Architecture)
	}

	d.pkg = &types.Package{
		Path: d.packageName,
		Name: path.Base(d.packageName),
	}

	return d, nil
}

// Header returns the decoded rpkg header.
func (d *Decoder) Header() *Header {
	h := &Header{
		Magic:        d.header.Magic,
		Architecture: d.header.Architecture,
		Version:      d.header.Version,
		Checksum:     bytes.Clone(d.b[d.header.ChecksumOffset : d.header.ChecksumOffset+d.header.ChecksumLength]),

		PackageName: d.packageName,

		ImportsOffset: d.header.ImportsOffset,
		ImportsLength: d.header.ImportsLength,

		ExportsOffset: d.header.ExportsOffset,
		ExportsLength: d.header.ExportsLength,

		TypesOffset: d.header.TypesOffset,
		TypesLength: d.header.TypesLength,

		SymbolsOffset: d.header.SymbolsOffset,
		SymbolsLength: d.header.SymbolsLength,

		ABIsOffset: d.header.ABIsOffset,
		ABIsLength: d.header.ABIsLength,

		StringsOffset: d.header.StringsOffset,
		StringsLength: d.header.StringsLength,

		LinkagesOffset: d.header.LinkagesOffset,
		LinkagesLength: d.header.LinkagesLength,

		CodeOffset: d.header.CodeOffset,
		CodeLength: d.header.CodeLength,

		ChecksumOffset: d.header.ChecksumOffset,
		ChecksumLength: d.header.ChecksumLength,
	}

	return h
}

// Imports reads all imports in the rpkg, caching
// them in the decoder.
func (d *Decoder) Imports() ([]string, error) {
	if d.allImports != nil {
		return d.allImports, nil
	}

	var offset uint32
	var result []string
	s := cryptobyte.String(d.b[d.header.ImportsOffset:d.header.ExportsOffset])
	for !s.Empty() {
		if !s.ReadUint32(&offset) {
			return nil, fmt.Errorf("invalid imports section: %w", io.ErrUnexpectedEOF)
		}

		s, err := d.getString(uint64(offset))
		if err != nil {
			return nil, err
		}

		result = append(result, s)
	}

	d.allImports = result

	return result, nil
}

// Exports reads all exports in the rpkg, caching
// them in the decoder.
func (d *Decoder) Exports() ([]types.Object, error) {
	if d.allExports != nil {
		return d.allExports, nil
	}

	var offset uint64
	var result []types.Object
	s := cryptobyte.String(d.b[d.header.ExportsOffset:d.header.TypesOffset])
	for !s.Empty() {
		if !s.ReadUint64(&offset) {
			return nil, fmt.Errorf("invalid exports section: %w", io.ErrUnexpectedEOF)
		}

		obj, ok := d.objects[offset]
		if !ok {
			return nil, fmt.Errorf("invalid exports section: no symbol found at offset %d", offset)
		}

		result = append(result, obj)
	}

	d.allExports = result

	return result, nil
}

// Types reads all types in the rpkg, caching
// them in the decoder.
func (d *Decoder) Types() ([]types.Type, error) {
	if d.allTypes != nil {
		return d.allTypes, nil
	}

	var offset uint64
	s := cryptobyte.String(d.b[d.header.TypesOffset:d.header.SymbolsOffset])
	remaining := len(s)
	var result []types.Type
	for !s.Empty() {
		typ, err := d.getTypeFrom(&s)
		if err != nil {
			return nil, err
		}

		d.types[offset] = typ
		offset += uint64(remaining - len(s))
		remaining = len(s)
		result = append(result, typ)
	}

	d.allTypes = result

	return result, nil
}

// getType reads the type at the given offset,
// caching the result.
func (d *Decoder) getType(offset uint64) (types.Type, error) {
	typ, ok := d.types[offset]
	if ok {
		return typ, nil
	}

	s := cryptobyte.String(d.b[d.header.TypesOffset+offset : d.header.SymbolsOffset])
	typ, err := d.getTypeFrom(&s)
	if err != nil {
		return nil, err
	}

	d.types[offset] = typ

	return typ, nil
}

// getTypeFrom reads the type from the given string.
func (d *Decoder) getTypeFrom(s *cryptobyte.String) (types.Type, error) {
	var kind uint8
	var rest cryptobyte.String
	if !s.ReadUint8(&kind) ||
		!s.ReadUint24LengthPrefixed(&rest) {
		return nil, fmt.Errorf("failed to read type: %w", io.ErrUnexpectedEOF)
	}

	switch TypeKind(kind) {
	case TypeKindNone:
		if !rest.Empty() {
			return nil, fmt.Errorf("invalid type: got type kind none with %d bytes of further data", len(rest))
		}

		return nil, nil
	case TypeKindBasic:
		var basic uint32
		if !rest.ReadUint32(&basic) {
			return nil, fmt.Errorf("invalid type: failed to read %s type kind: %w", TypeKind(kind), io.ErrUnexpectedEOF)
		}

		if !rest.Empty() {
			return nil, fmt.Errorf("invalid type: got type kind %s with %d bytes of further data", TypeKind(kind), len(rest))
		}

		switch BasicKind(basic) {
		case BasicKindBool:
			return types.Bool, nil
		case BasicKindInt:
			return types.Int, nil
		case BasicKindInt8:
			return types.Int8, nil
		case BasicKindInt16:
			return types.Int16, nil
		case BasicKindInt32:
			return types.Int32, nil
		case BasicKindInt64:
			return types.Int64, nil
		case BasicKindUint:
			return types.Uint, nil
		case BasicKindUint8:
			return types.Uint8, nil
		case BasicKindByte:
			return types.Byte, nil
		case BasicKindUint16:
			return types.Uint16, nil
		case BasicKindUint32:
			return types.Uint32, nil
		case BasicKindUint64:
			return types.Uint64, nil
		case BasicKindUintptr:
			return types.Uintptr, nil
		case BasicKindString:
			return types.String, nil
		case BasicKindUntypedBool:
			return types.UntypedBool, nil
		case BasicKindUntypedInt:
			return types.UntypedInt, nil
		case BasicKindUntypedString:
			return types.UntypedString, nil
		default:
			return nil, fmt.Errorf("invalid type: got type kind %s with unrecognised basic kind %d", TypeKind(kind), basic)
		}
	case TypeKindFunction:
		var paramsData []byte
		var paramsLength uint32
		var resultOffset, nameOffset uint64
		if !rest.ReadUint32(&paramsLength) ||
			!rest.ReadBytes(&paramsData, int(paramsLength)) ||
			!rest.ReadUint64(&resultOffset) ||
			!rest.ReadUint64(&nameOffset) {
			return nil, fmt.Errorf("invalid type: failed to read %s type kind: %w", TypeKind(kind), io.ErrUnexpectedEOF)
		}

		if !rest.Empty() {
			return nil, fmt.Errorf("invalid type: got type kind %s with %d bytes of further data", TypeKind(kind), len(rest))
		}

		params := make([]*types.Variable, 0, paramsLength/16)
		paramsString := cryptobyte.String(paramsData)
		for !paramsString.Empty() {
			var nameOffset, typeOffset uint64
			if !paramsString.ReadUint64(&nameOffset) ||
				!paramsString.ReadUint64(&typeOffset) {
				return nil, fmt.Errorf("invalid type: failed to read %s type kind parameter: %w", TypeKind(kind), io.ErrUnexpectedEOF)
			}

			// At this point, we assume we've
			// already parsed and cached any
			// parameter types.
			name, err := d.getString(nameOffset)
			if err != nil {
				return nil, fmt.Errorf("invalid type: failed to read %s type kind parameter name: %v", TypeKind(kind), err)
			}

			typ, ok := d.types[typeOffset]
			if !ok {
				return nil, fmt.Errorf("invalid type: failed to read %s type kind parameter type: no type information at offset %d", TypeKind(kind), typeOffset)
			}

			params = append(params, types.NewParameter(nil, token.NoPos, token.NoPos, nil, name, typ))
		}

		if resultOffset >= d.header.TypesLength {
			return nil, fmt.Errorf("invalid type: %s result %d is beyond types section", TypeKind(kind), resultOffset)
		}
		if nameOffset >= d.header.StringsLength {
			return nil, fmt.Errorf("invalid type: %s name %d is beyond strings section", TypeKind(kind), nameOffset)
		}

		result, ok := d.types[resultOffset]
		if !ok {
			return nil, fmt.Errorf("invalid type: failed to read %s type kind result: no type information at offset %d", TypeKind(kind), resultOffset)
		}

		name, err := d.getString(nameOffset)
		if err != nil {
			return nil, fmt.Errorf("invalid type: failed to read %s type kind name: %v", TypeKind(kind), err)
		}

		return types.NewSignature(name, params, result), nil
	case TypeKindABI:
		var offset uint32
		if !rest.ReadUint32(&offset) {
			return nil, fmt.Errorf("invalid type: failed to read %s type kind: %w", TypeKind(kind), io.ErrUnexpectedEOF)
		}

		if !rest.Empty() {
			return nil, fmt.Errorf("invalid type: got type kind %s with %d bytes of further data", TypeKind(kind), len(rest))
		}

		abi, ok := d.abis[offset]
		if !ok {
			return nil, fmt.Errorf("invalid type: invalid ABI offset %d", offset)
		}

		return types.NewABI(abi), nil
	default:
		return nil, fmt.Errorf("invalid type: got unrecognised type kind %d", kind)
	}
}

// Symbols reads all symbols in the rpkg, caching
// them in the decoder.
func (d *Decoder) Symbols() ([]*Symbol, []types.Object, error) {
	if d.allSymbols != nil && d.allObjects != nil {
		return d.allSymbols, d.allObjects, nil
	}

	var abi *sys.ABI
	var offset uint64
	var result []*Symbol
	var objects []types.Object
	d.symbols = make(map[uint64]*Symbol)
	d.objects = make(map[uint64]types.Object)
	s := cryptobyte.String(d.b[d.header.SymbolsOffset:d.header.ABIsOffset])
	for !s.Empty() {
		here := offset
		var kind uint32
		var packageOffset, nameOffset, typeOffset, rawValue uint64
		if !s.ReadUint32(&kind) ||
			!s.ReadUint64(&packageOffset) ||
			!s.ReadUint64(&nameOffset) ||
			!s.ReadUint64(&typeOffset) ||
			!s.ReadUint64(&rawValue) {
			return nil, nil, fmt.Errorf("failed to read symbol: %w", io.ErrUnexpectedEOF)
		}

		offset += symbolSize

		var value any
		switch SymKind(kind) {
		case SymKindBooleanConstant:
			if rawValue != 0 && rawValue != 1 {
				return nil, nil, fmt.Errorf("invalid symbol: got value %d, want 0 or 1 for kind %q", rawValue, SymKind(kind))
			}

			value = rawValue == 1
		case SymKindIntegerConstant:
			value = constant.MakeInt64(int64(rawValue))
		case SymKindBigIntegerConstant:
			if rawValue >= d.header.StringsLength {
				return nil, nil, fmt.Errorf("invalid symbol: %s value %d is beyond strings section", SymKind(kind), rawValue)
			}

			data, err := d.getString(rawValue)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid symbol: invalid %s: %v", SymKind(kind), err)
			}

			x := new(big.Int).SetBytes([]byte(data))
			value = constant.Make(x)
		case SymKindBigNegativeIntegerConstant:
			if rawValue >= d.header.StringsLength {
				return nil, nil, fmt.Errorf("invalid symbol: %s value %d is beyond strings section", SymKind(kind), rawValue)
			}

			data, err := d.getString(rawValue)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid symbol: invalid %s: %v", SymKind(kind), err)
			}

			x := new(big.Int).SetBytes([]byte(data))
			value = constant.Make(x.Neg(x))
		case SymKindStringConstant:
			if rawValue >= d.header.StringsLength {
				return nil, nil, fmt.Errorf("invalid symbol: %s value %d is beyond strings section", SymKind(kind), rawValue)
			}

			data, err := d.getString(rawValue)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid symbol: invalid %s: %v", SymKind(kind), err)
			}

			value = constant.MakeString(data)
		case SymKindFunction:
			if rawValue >= d.header.CodeLength {
				return nil, nil, fmt.Errorf("invalid symbol: %s value %d is beyond code section", SymKind(kind), rawValue)
			}

			fun, err := d.getCode(rawValue)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid symbol: invalid %s: %v", SymKind(kind), err)
			}

			abi = fun.ABI
			value = compiler.MachineCode(fun.Code)
		default:
			return nil, nil, fmt.Errorf("invalid symbol: unrecognised kind %d", SymKind(kind))
		}

		if packageOffset >= d.header.StringsLength {
			return nil, nil, fmt.Errorf("invalid symbol: package path offset %d is beyond strings section", packageOffset)
		}
		if nameOffset >= d.header.StringsLength {
			return nil, nil, fmt.Errorf("invalid symbol: name offset %d is beyond strings section", nameOffset)
		}
		if typeOffset == 0 {
			return nil, nil, fmt.Errorf("invalid symbol: got type %d for kind %q", typeOffset, SymKind(kind))
		}

		pkgName, err := d.getString(packageOffset)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid symbol: invalid package name: %v", err)
		}

		name, err := d.getString(nameOffset)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid symbol: invalid symbol name: %v", err)
		}

		typ, err := d.getType(typeOffset)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid symbol: invalid type: %v", err)
		}

		symbol := &Symbol{
			Kind:        SymKind(kind),
			PackageName: pkgName,
			Name:        name,
			Type:        typ,
			Value:       value,
		}

		var object types.Object
		switch symbol.Kind {
		case SymKindBooleanConstant,
			SymKindIntegerConstant,
			SymKindBigIntegerConstant,
			SymKindBigNegativeIntegerConstant,
			SymKindStringConstant:
			value, ok := symbol.Value.(constant.Value)
			if !ok {
				return nil, nil, fmt.Errorf("rpkg: internal error: found symbol %q with kind %v and unexpected value type %#v", symbol.Name, symbol.Kind, symbol.Value)
			}

			object = types.NewConstant(nil, token.NoPos, token.NoPos, d.pkg, symbol.Name, symbol.Type, value)
		case SymKindFunction:
			sig, ok := symbol.Type.(*types.Signature)
			if !ok {
				return nil, nil, fmt.Errorf("rpkg: internal error: found symbol %q with kind %v and unexpected type %#v", symbol.Name, symbol.Kind, symbol.Type)
			}

			fun := types.NewFunction(nil, token.NoPos, token.NoPos, d.pkg, symbol.Name, sig)
			fun.SetABI(abi)
			object = fun
		default:
			return nil, nil, fmt.Errorf("rpkg: internal error: found symbol %q with unsupported kind: %v", symbol.Name, symbol.Kind)
		}

		d.symbols[here] = symbol
		d.objects[here] = object
		result = append(result, symbol)
		objects = append(objects, object)
	}

	d.allSymbols = result
	d.allObjects = objects

	return result, objects, nil
}

// ABIs reads all ABIs in the rpkg, caching
// them in the decoder.
func (d *Decoder) ABIs() ([]*sys.ABI, error) {
	if d.allABIs != nil {
		return d.allABIs, nil
	}

	var offset uint32
	var out []*sys.ABI
	d.abis = make(map[uint32]*sys.ABI)
	s := cryptobyte.String(d.b[d.header.ABIsOffset:d.header.StringsOffset])
	for !s.Empty() {
		var length uint32
		var rest []byte
		if !s.ReadUint32(&length) ||
			!s.ReadBytes(&rest, int(length)) {
			return nil, fmt.Errorf("failed to read ABI: %w", io.ErrUnexpectedEOF)
		}

		here := offset
		offset += 4 + length

		if length == 0 {
			d.abis[here] = nil
			continue
		}
		if length < minABILength {
			return nil, fmt.Errorf("invalid ABI: length %d is less than %d", length, minABILength)
		}

		abiString := cryptobyte.String(rest)

		var invertedStack uint8
		var params, result, scratch, unused cryptobyte.String
		if !abiString.ReadUint8(&invertedStack) ||
			!abiString.ReadUint8LengthPrefixed(&params) ||
			!abiString.ReadUint8LengthPrefixed(&result) ||
			!abiString.ReadUint8LengthPrefixed(&scratch) ||
			!abiString.ReadUint8LengthPrefixed(&unused) {
			return nil, fmt.Errorf("failed to read ABI: %w", io.ErrUnexpectedEOF)
		}

		if invertedStack != 1 && invertedStack != 0 {
			return nil, fmt.Errorf("invalid ABI: got inverted stack %d, want 0 or 1", invertedStack)
		}

		switch length % 4 {
		case 1:
			offset += 3
			var padding uint32
			if !s.ReadUint24(&padding) {
				return nil, fmt.Errorf("invalid ABIs section: %w", io.ErrUnexpectedEOF)
			}
			if padding != 0 {
				return nil, fmt.Errorf("invalid ABIs section: invalid padding %06x", padding)
			}
		case 2:
			offset += 2
			var padding uint16
			if !s.ReadUint16(&padding) {
				return nil, fmt.Errorf("invalid ABIs section: %w", io.ErrUnexpectedEOF)
			}
			if padding != 0 {
				return nil, fmt.Errorf("invalid ABIs section: invalid padding %04x", padding)
			}
		case 3:
			offset += 1
			var padding uint8
			if !s.ReadUint8(&padding) {
				return nil, fmt.Errorf("invalid ABIs section: %w", io.ErrUnexpectedEOF)
			}
			if padding != 0 {
				return nil, fmt.Errorf("invalid ABIs section: invalid padding %02x", padding)
			}
		}

		abi := &sys.ABI{
			InvertedStack:    invertedStack == 1,
			ParamRegisters:   make([]sys.Location, len(params)),
			ResultRegisters:  make([]sys.Location, len(result)),
			ScratchRegisters: make([]sys.Location, len(scratch)),
			UnusedRegisters:  make([]sys.Location, len(unused)),
		}

		for i, param := range params {
			if int(param) >= len(d.arch.ABIRegisters) {
				return nil, fmt.Errorf("invalid ABI: invalid parameter register index %d: overflows %s.ABIRegisters (length %d)", param, d.arch.Name, len(d.arch.ABIRegisters))
			}

			abi.ParamRegisters[i] = d.arch.ABIRegisters[param]
		}

		for i, result := range result {
			if int(result) >= len(d.arch.ABIRegisters) {
				return nil, fmt.Errorf("invalid ABI: invalid result register index %d: overflows %s.ABIRegisters (length %d)", result, d.arch.Name, len(d.arch.ABIRegisters))
			}

			abi.ResultRegisters[i] = d.arch.ABIRegisters[result]
		}

		for i, scratch := range scratch {
			if int(scratch) >= len(d.arch.ABIRegisters) {
				return nil, fmt.Errorf("invalid ABI: invalid scratch register index %d: overflows %s.ABIRegisters (length %d)", scratch, d.arch.Name, len(d.arch.ABIRegisters))
			}

			abi.ScratchRegisters[i] = d.arch.ABIRegisters[scratch]
		}

		for i, unused := range unused {
			if unused == abiStackPointer {
				abi.UnusedRegisters[i] = d.arch.StackPointer
				continue
			}

			if int(unused) >= len(d.arch.ABIRegisters) {
				return nil, fmt.Errorf("invalid ABI: invalid unused register index %d: overflows %s.ABIRegisters (length %d)", unused, d.arch.Name, len(d.arch.ABIRegisters))
			}

			abi.UnusedRegisters[i] = d.arch.ABIRegisters[unused]
		}

		if err := d.arch.Validate(abi); err != nil {
			return nil, fmt.Errorf("invalid ABI: %v", err)
		}

		d.abis[here] = abi
		out = append(out, abi)
	}

	d.allABIs = out

	return out, nil
}

// Strings reads all strings in the rpkg, caching
// them in the decoder.
func (d *Decoder) Strings() ([]string, error) {
	if d.allStrings != nil {
		return d.allStrings, nil
	}

	var offset uint64
	var length uint32
	d.strings = make(map[uint64]string)
	var result []string
	s := cryptobyte.String(d.b[d.header.StringsOffset:d.header.LinkagesOffset])
	for !s.Empty() {
		var data []byte
		here := offset
		if !s.ReadUint32(&length) ||
			!s.ReadBytes(&data, int(length)) {
			return nil, fmt.Errorf("invalid strings section: %w", io.ErrUnexpectedEOF)
		}

		offset += 4 + uint64(length)
		switch length % 4 {
		case 1:
			offset += 3
			var padding uint32
			if !s.ReadUint24(&padding) {
				return nil, fmt.Errorf("invalid strings section: %w", io.ErrUnexpectedEOF)
			}
			if padding != 0 {
				return nil, fmt.Errorf("invalid strings section: invalid padding %06x", padding)
			}
		case 2:
			offset += 2
			var padding uint16
			if !s.ReadUint16(&padding) {
				return nil, fmt.Errorf("invalid strings section: %w", io.ErrUnexpectedEOF)
			}
			if padding != 0 {
				return nil, fmt.Errorf("invalid strings section: invalid padding %04x", padding)
			}
		case 3:
			offset += 1
			var padding uint8
			if !s.ReadUint8(&padding) {
				return nil, fmt.Errorf("invalid strings section: %w", io.ErrUnexpectedEOF)
			}
			if padding != 0 {
				return nil, fmt.Errorf("invalid strings section: invalid padding %02x", padding)
			}
		}

		str := string(data)
		d.strings[here] = str
		result = append(result, str)
	}

	d.allStrings = result

	return result, nil
}

// getString reads the string at the given offset,
// caching the result.
func (d *Decoder) getString(offset uint64) (string, error) {
	if offset >= d.header.StringsLength {
		return "", fmt.Errorf("invalid string offset: %d is beyond strings section", offset)
	}
	if offset%4 != 0 {
		return "", fmt.Errorf("invalid string offset: %d is not 32-bit aligned", offset)
	}

	str, ok := d.strings[offset]
	if ok {
		return str, nil
	}

	var data []byte
	var length uint32
	s := cryptobyte.String(d.b[d.header.StringsOffset+offset : d.header.LinkagesOffset])
	if !s.ReadUint32(&length) ||
		!s.ReadBytes(&data, int(length)) {
		return "", fmt.Errorf("invalid string offset: %w", io.ErrUnexpectedEOF)
	}

	switch length % 4 {
	case 1:
		var padding uint32
		if !s.ReadUint24(&padding) {
			return "", fmt.Errorf("invalid strings section: %w", io.ErrUnexpectedEOF)
		}
		if padding != 0 {
			return "", fmt.Errorf("invalid strings section: invalid padding %06x", padding)
		}
	case 2:
		var padding uint16
		if !s.ReadUint16(&padding) {
			return "", fmt.Errorf("invalid strings section: %w", io.ErrUnexpectedEOF)
		}
		if padding != 0 {
			return "", fmt.Errorf("invalid strings section: invalid padding %04x", padding)
		}
	case 3:
		var padding uint8
		if !s.ReadUint8(&padding) {
			return "", fmt.Errorf("invalid strings section: %w", io.ErrUnexpectedEOF)
		}
		if padding != 0 {
			return "", fmt.Errorf("invalid strings section: invalid padding %02x", padding)
		}
	}

	if d.strings == nil {
		d.strings = make(map[uint64]string)
	}

	str = string(data)
	d.strings[offset] = str

	return str, nil
}

// Linkages reads all linkages in the rpkg,
// caching them in the decoder.
func (d *Decoder) Linkages() ([]*Linkage, error) {
	if d.allLinkages != nil {
		return d.allLinkages, nil
	}

	var result []*Linkage
	s := cryptobyte.String(d.b[d.header.LinkagesOffset:d.header.CodeOffset])
	for !s.Empty() {
		var link linkage
		var typ uint8
		if !s.ReadUint64(&link.Source) ||
			!s.ReadUint64(&link.TargetPackage) ||
			!s.ReadUint64(&link.TargetSymbol) ||
			!s.ReadUint8(&typ) ||
			!s.ReadUint24(&link.Size) ||
			!s.ReadUint32(&link.Offset) ||
			!s.ReadUint32(&link.Address) {
			return nil, fmt.Errorf("invalid linkages section: %w", io.ErrUnexpectedEOF)
		}

		if link.Source > d.header.SymbolsLength {
			return nil, fmt.Errorf("invalid linkage: source symbol offset %d is beyond symbols section", link.Source)
		}
		if link.Source%symbolSize != 0 {
			return nil, fmt.Errorf("invalid linkage: source symbol offset %d is not on a symbol boundary", link.Source)
		}
		if link.TargetPackage > d.header.StringsLength {
			return nil, fmt.Errorf("invalid linkage: target symbol package offset %d is beyond strings section", link.TargetPackage)
		}
		if link.TargetSymbol > d.header.StringsLength {
			return nil, fmt.Errorf("invalid linkage: target symbol name offset %d is beyond strings section", link.TargetSymbol)
		}
		link.Type = ssafir.LinkType(typ)
		switch link.Type {
		case ssafir.LinkFullAddress,
			ssafir.LinkRelativeAddress:
		default:
			return nil, fmt.Errorf("invalid linkage: link type %v not recognised", link.Type)
		}
		if link.Size > 64 {
			return nil, fmt.Errorf("invalid linkage: implausible address size %d", link.Size)
		}
		if uint64(link.Offset) > d.header.CodeLength {
			return nil, fmt.Errorf("invalid linkage: function code offset %d is beyond code section", link.Offset)
		}
		if uint64(link.Address) > d.header.CodeLength {
			return nil, fmt.Errorf("invalid linkage: function code address offset %d is beyond code section", link.Address)
		}

		source, ok := d.symbols[link.Source]
		if !ok {
			return nil, fmt.Errorf("invalid linkage: invalid source symbol: %v", link.Source)
		}

		if source.Kind != SymKindFunction {
			return nil, fmt.Errorf("invalid linkage: source symbol has kind %s, want %s", source.Kind, SymKindFunction)
		}

		targetPackage, err := d.getString(link.TargetPackage)
		if err != nil {
			return nil, fmt.Errorf("invalid linkage: invalid target package: %v", err)
		}

		targetSymbol, err := d.getString(link.TargetSymbol)
		if err != nil {
			return nil, fmt.Errorf("invalid linkage: invalid target symbol: %v", err)
		}

		targetName := targetSymbol
		if targetPackage != "" {
			targetName = targetPackage + "." + targetSymbol
		}

		source.Links = append(source.Links, &ssafir.Link{
			Name:    targetName,
			Type:    link.Type,
			Size:    uint8(link.Size),
			Offset:  int(link.Offset),
			Address: uintptr(link.Address),
		})

		result = append(result, &Linkage{
			Source:  source.AbsoluteName(),
			Target:  targetName,
			Type:    link.Type,
			Size:    uint8(link.Size),
			Offset:  int(link.Offset),
			Address: uintptr(link.Address),
		})
	}

	return result, nil
}

// getCode reads the function at the given offset,
// caching the result.
func (d *Decoder) getCode(offset uint64) (*Function, error) {
	if offset >= d.header.CodeLength {
		return nil, fmt.Errorf("invalid code offset: %d is beyond code section", offset)
	}
	if offset%4 != 0 {
		return nil, fmt.Errorf("invalid code offset: %d is not 32-bit aligned", offset)
	}

	fun, ok := d.code[offset]
	if ok {
		return fun, nil
	}

	var abiOffset uint32
	var length uint32
	var code []byte
	s := cryptobyte.String(d.b[d.header.CodeOffset+offset : d.header.ChecksumOffset])
	if !s.ReadUint32(&abiOffset) ||
		!s.ReadUint32(&length) ||
		!s.ReadBytes(&code, int(length)) {
		return nil, fmt.Errorf("invalid code offset: %w", io.ErrUnexpectedEOF)
	}

	switch length % 4 {
	case 1:
		var padding uint32
		if !s.ReadUint24(&padding) {
			return nil, fmt.Errorf("invalid code section: %w", io.ErrUnexpectedEOF)
		}
		if padding != 0 {
			return nil, fmt.Errorf("invalid code section: invalid padding %06x", padding)
		}
	case 2:
		var padding uint16
		if !s.ReadUint16(&padding) {
			return nil, fmt.Errorf("invalid code section: %w", io.ErrUnexpectedEOF)
		}
		if padding != 0 {
			return nil, fmt.Errorf("invalid code section: invalid padding %04x", padding)
		}
	case 3:
		var padding uint8
		if !s.ReadUint8(&padding) {
			return nil, fmt.Errorf("invalid code section: %w", io.ErrUnexpectedEOF)
		}
		if padding != 0 {
			return nil, fmt.Errorf("invalid code section: invalid padding %02x", padding)
		}
	}

	abi, ok := d.abis[abiOffset]
	if !ok {
		return nil, fmt.Errorf("invalid code section: invalid ABI offset %d", abiOffset)
	}

	fun = &Function{
		ABI:  abi,
		Code: code,
	}

	d.code[offset] = fun

	return fun, nil
}

// Decode parses an rpkg file, returning the compiled
// package. The types in the package are populated in
// the type information. Specifically, the List and
// Indices fields.
func Decode(info *types.Info, b []byte) (arch *sys.Arch, pkg *compiler.Package, checksum []byte, err error) {
	d, err := NewDecoder(b)
	if err != nil {
		return nil, nil, nil, err
	}

	// Prepare our outputs.

	arch = d.arch
	pkg = &compiler.Package{
		Name:  d.pkg.Name,
		Path:  d.pkg.Path,
		Types: d.pkg,
	}

	// Pull all the data from the package.
	// The order of these steps is important.

	_, err = d.ABIs()
	if err != nil {
		return nil, nil, nil, err
	}

	_, err = d.Strings()
	if err != nil {
		return nil, nil, nil, err
	}

	typs, err := d.Types()
	if err != nil {
		return nil, nil, nil, err
	}

	// Populate the types information.
	// Note that we're careful to
	// avoid overwriting or duplicating
	// any existing types.

	if info.List == nil {
		info.List = make([]types.Type, 0, len(typs))
	}

	if info.Indices == nil {
		info.Indices = make(map[types.Type]int)
	}

	for i, typ := range typs {
		_, ok := info.Indices[typ]
		if ok {
			continue
		}

		info.Indices[typ] = i
		info.List = append(info.List, typ)
	}

	symbols, objects, err := d.Symbols()
	if err != nil {
		return nil, nil, nil, err
	}

	_, err = d.Linkages()
	if err != nil {
		return nil, nil, nil, err
	}

	_, err = d.Imports()
	if err != nil {
		return nil, nil, nil, err
	}

	exports, err := d.Exports()
	if err != nil {
		return nil, nil, nil, err
	}

	// Add the exported symbols to
	// the package scope.
	scope := pkg.Types.Scope()
	for _, exp := range exports {
		scope.Insert(exp)
	}

	// Spread the symbols out into
	// the package's constants and
	// functions.

	for i, obj := range objects {
		symbol := symbols[i]
		switch obj := obj.(type) {
		case *types.Constant:
			if symbol.PackageName == "" {
				pkg.Literals = append(pkg.Literals, obj)
			} else {
				pkg.Constants = append(pkg.Constants, obj)
			}
		case *types.Function:
			code, ok := symbol.Value.(compiler.MachineCode)
			if !ok {
				return nil, nil, nil, fmt.Errorf("found symbol %q with kind %v and unexpected value type %#v", symbol.Name, symbol.Kind, symbol.Value)
			}

			fun := &ssafir.Function{
				Name:  symbol.Name,
				Func:  obj,
				Type:  obj.Type().(*types.Signature),
				Extra: code,
				Links: symbol.Links,
			}

			pkg.Functions = append(pkg.Functions, fun)
		default:
			return nil, nil, nil, fmt.Errorf("rpkg: internal error: symbol %q (%#v) has unexpected object type: %#v", symbol.Name, symbol, obj)
		}
	}

	checksum = bytes.Clone(d.b[d.header.ChecksumOffset : d.header.ChecksumOffset+d.header.ChecksumLength])

	return arch, pkg, checksum, nil
}
