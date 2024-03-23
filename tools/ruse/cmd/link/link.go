// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package link links a set of Ruse rpkg files into an executable binary.
package link

import (
	"bytes"
	"context"
	gobinary "encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"golang.org/x/crypto/cryptobyte"

	"firefly-os.dev/tools/ruse/binary"
	"firefly-os.dev/tools/ruse/binary/elf"
	"firefly-os.dev/tools/ruse/compiler"
	"firefly-os.dev/tools/ruse/constant"
	"firefly-os.dev/tools/ruse/rpkg"
	"firefly-os.dev/tools/ruse/types"
)

var program = filepath.Base(os.Args[0])

type binaryEncoder func(w io.Writer, bin *binary.Binary) error

var zeros [512]uint8

// Main links together a set of Ruse rpkg files into an executable
// binary.
func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("link", flag.ExitOnError)

	var help, symbolTable, provenance bool
	var out, stdlib string
	var rpkgs []string
	var encode binaryEncoder
	flags.BoolVar(&help, "h", false, "Show this message and exit.")
	flags.BoolVar(&symbolTable, "symbol-table", true, "Include a symbol table in the compiled binary.")
	flags.BoolVar(&provenance, "provenance", true, "Include the set of input rpkg files in the compiled binary.")
	flags.Func("binary", "The binary encoding (elf).", func(s string) error {
		if encode != nil {
			return fmt.Errorf("-binary can only be specified once")
		}

		switch s {
		case "elf":
			encode = elf.Encode
		default:
			return fmt.Errorf("unrecognised -binary format: %q", s)
		}

		return nil
	})
	flags.Func("rpkg", "One or more dependency rpkg files.", func(s string) error {
		rpkgs = append(rpkgs, s)
		return nil
	})
	flags.StringVar(&stdlib, "stdlib", "", "The standard library rpkg file.")
	flags.StringVar(&out, "o", "", "The name of the compiled binary.")

	flags.Usage = func() {
		log.Printf("Usage:\n  %s %s OPTIONS RPKG\n\n", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(2)
	}

	err := flags.Parse(args)
	if err != nil || help {
		flags.Usage()
	}

	if encode == nil || out == "" {
		flags.Usage()
		os.Exit(2)
	}

	filenames := flags.Args()
	if len(filenames) != 1 {
		flags.Usage()
		os.Exit(2)
	}

	data, err := os.ReadFile(filenames[0])
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", filenames[0], err)
	}

	info := &types.Info{}
	arch, p, checksum, err := rpkg.Decode(info, data)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", filenames[0], err)
	}

	// Put the main function first.
	sort.Slice(p.Functions, func(i, j int) bool {
		switch {
		case p.Functions[i].Name == "main":
			return true
		case p.Functions[j].Name == "main":
			return false
		default:
			return p.Functions[i].Name < p.Functions[j].Name
		}
	})

	// Check that we have a suitable main function.
	if len(p.Functions) == 0 ||
		p.Functions[0].Name != "main" {
		return fmt.Errorf("function main is undeclared")
	}

	if len(p.Functions[0].Type.Params()) != 0 ||
		p.Functions[0].Type.Result() != nil {
		return fmt.Errorf("main function: must have no parameters or result, found function signature %s", p.Functions[0].Type)
	}

	var rpkgsData cryptobyte.Builder
	if provenance {
		rpkgsData.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes([]byte(p.Path))
		})
		rpkgsData.AddBytes(checksum)
	}

	// Add the dependencies, checking
	// that we have all the imports we
	// need.
	seenPackages := make(map[string]bool)
	needPackages := make(map[string]bool)
	seenPackages[p.Path] = true
	packages := []*compiler.Package{p}
	for _, imp := range p.Imports {
		needPackages[imp] = true
	}
	for _, name := range rpkgs {
		data, err := os.ReadFile(name)
		if err != nil {
			return fmt.Errorf("failed to read rpkg %q: %v", name, err)
		}

		depArch, p, checksum, err := rpkg.Decode(info, data)
		if err != nil {
			return fmt.Errorf("failed to parse rpkg %q: %v", name, err)
		}

		if depArch != arch {
			return fmt.Errorf("cannot import rpkg %q: compiled for %s: need %s", name, depArch.Name, arch.Name)
		}

		seenPackages[p.Path] = true
		for _, imp := range p.Imports {
			needPackages[imp] = true
		}

		packages = append(packages, p)
		if provenance {
			rpkgsData.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
				b.AddBytes([]byte(p.Path))
			})
			rpkgsData.AddBytes(checksum)
		}
	}

	isStdlib := make(map[string]bool)
	if stdlib != "" {
		data, err := os.ReadFile(stdlib)
		if err != nil {
			return fmt.Errorf("failed to read stdlib rpkg %q: %v", stdlib, err)
		}

		rstd, err := rpkg.NewStdlibDecoder(data)
		if err != nil {
			return fmt.Errorf("failed to parse stdlib rstd %q: %v", stdlib, err)
		}

		pkgs := rstd.Packages()
		for _, hdr := range pkgs {
			info := new(types.Info)
			depArch, p, checksum, err := rstd.Decode(info, hdr)
			if err != nil {
				return fmt.Errorf("failed to parse stdlib rpkg %q from %q: %v", hdr.PackageName, stdlib, err)
			}

			if depArch != arch {
				return fmt.Errorf("cannot import stdlib rpkg %q: compiled for %s: need %s", hdr.PackageName, depArch.Name, arch.Name)
			}

			isStdlib[hdr.PackageName] = true
			seenPackages[hdr.PackageName] = true

			packages = append(packages, p)
			if provenance {
				rpkgsData.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
					b.AddBytes([]byte(p.Path))
				})
				rpkgsData.AddBytes(checksum)
			}
		}
	}

	// Check that we have seen every package
	// that we need.
	for pkg := range needPackages {
		if !seenPackages[pkg] {
			return fmt.Errorf("no rpkg provided for package %q", pkg)
		}
	}

	const (
		defaultCodeSectionSymbol    = "sections.Code"
		defaultStringsSectionSymbol = "sections.Strings"
		defaultRPKGsSectionSymbol   = "sections.RPKGs"
	)

	// Prepare the list of sections.
	var sections []types.Section
	var sectionsData []*bytes.Buffer
	symbolToSection := make(map[string]*binary.Section)
	symbolToSectionIndex := make(map[string]int)
	for _, p := range packages {
		// Look for any named sections.
		for _, con := range p.Constants {
			section, ok := con.Type().(types.Section)
			if !ok {
				continue
			}

			symbolName := con.Package().Path + "." + con.Name()
			if _, ok := symbolToSection[symbolName]; !ok {
				symbolToSection[symbolName] = section.Section()
				symbolToSectionIndex[symbolName] = len(sections)
				sections = append(sections, section)
				sectionsData = append(sectionsData, new(bytes.Buffer))
			}
		}
	}

	// Track the default sections, which should be
	// declared in the sections package in the
	// standard library and we error if not.
	if symbolToSection[defaultCodeSectionSymbol] == nil {
		return fmt.Errorf("internal error: could not find default code section")
	}
	if symbolToSection[defaultStringsSectionSymbol] == nil {
		return fmt.Errorf("internal error: could not find default strings section")
	}
	if symbolToSection[defaultRPKGsSectionSymbol] == nil {
		return fmt.Errorf("internal error: could not find default rpkgs section")
	}

	// Add the rpkgs data.
	rpkgsDataRaw := rpkgsData.BytesOrPanic()
	rpkgsDataSized := cryptobyte.NewFixedBuilder(make([]byte, 0, 4+len(rpkgsDataRaw)))
	rpkgsDataSized.AddUint32LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddBytes(rpkgsDataRaw)
	})
	sectionsData[symbolToSectionIndex[defaultRPKGsSectionSymbol]].Write(rpkgsDataSized.BytesOrPanic())

	pickSection := func(fallback, symbol string) (index int, data *bytes.Buffer, err error) {
		index, ok := symbolToSectionIndex[symbol]
		if !ok {
			if symbol != "" || fallback == "" {
				return 0, nil, fmt.Errorf("internal error: no section was found at symbol %q", symbol)
			}

			index = symbolToSectionIndex[fallback]
		}

		return index, sectionsData[index], nil
	}

	// If the package declares the set of sections,
	// we may need to change the order and drop any
	// sections that have been omitted.
	if len(p.Sections) != 0 {
		indices := make([]int, len(p.Sections))
		mapping := make(map[string]int)
		for i, section := range p.Sections {
			index, _, err := pickSection("", section)
			if err != nil {
				return err
			}

			indices[i] = index
			if _, ok := mapping[section]; !ok {
				mapping[section] = i
			}
		}

		list := make([]types.Section, len(p.Sections))
		data := make([]*bytes.Buffer, len(p.Sections))
		for i, index := range indices {
			list[i] = sections[index]
			data[i] = sectionsData[index]
		}

		sections = list
		sectionsData = data
		symbolToSectionIndex = mapping
	}

	// Build the symbol table.
	var main *binary.Symbol
	var table []*binary.Symbol
	var arrayLiterals int
	symbols := make(map[string]*binary.Symbol)
	for i, p := range packages {
		for _, fun := range p.Functions {
			index, data, err := pickSection(defaultCodeSectionSymbol, fun.Section)
			if err != nil {
				return err
			}

			// If necessary, add padding to
			// meet alignment constraints.
			prev := data.Len()
			align := fun.Func.Alignment()
			padding := align - (prev % align)
			if padding != align {
				data.Write(zeros[:padding])
				prev += padding
			}

			sym := &binary.Symbol{
				Name:    p.Path + "." + fun.Name,
				Kind:    binary.SymbolFunction,
				Section: index,
				Offset:  uintptr(prev), // Just the offset within the section for now.
			}

			err = compiler.EncodeTo(data, nil, arch, fun)
			if err != nil {
				return err
			}

			sym.Length = data.Len() - prev
			table = append(table, sym)
			symbols[sym.Name] = sym
			if i == 0 && fun.Name == "main" {
				if main != nil {
					return fmt.Errorf("main function: unexpectedly found a second main function")
				}

				main = sym
			}
		}

		for _, con := range p.Constants {
			index, data, err := pickSection(defaultStringsSectionSymbol, con.Section())
			if err != nil {
				return err
			}

			val := con.Value()
			if val == nil {
				continue
			}

			switch val.Kind() {
			case constant.String:
				s := constant.StringVal(val)
				offset := uintptr(data.Len()) // Just the offset within the section for now.

				// If necessary, add padding to
				// meet alignment constraints.
				align := uintptr(con.Alignment())
				padding := align - (offset % align)
				if padding != align {
					data.Write(zeros[:padding])
					offset += padding
				}

				data.WriteString(s)

				sym := &binary.Symbol{
					Name:    p.Path + "." + con.Name(),
					Kind:    binary.SymbolString,
					Section: index,
					Offset:  offset,
					Length:  len(s),
				}

				table = append(table, sym)
				symbols[sym.Name] = sym
			case constant.Array, constant.Bool, constant.Integer:
				offset := uintptr(data.Len()) // Just the offset within the section for now.

				// If necessary, add padding to
				// meet alignment constraints.
				align := uintptr(con.Alignment())
				padding := align - (offset % align)
				if padding != align {
					data.Write(zeros[:padding])
					offset += padding
				}

				name := p.Path + "." + con.Name()
				err := encodeConstant(data, arch.ByteOrder, val, con.Type())
				if err != nil {
					return fmt.Errorf("failed to encode symbol %s: %v", name, err)
				}

				var kind binary.SymbolKind
				switch val.Kind() {
				case constant.Array:
					kind = binary.SymbolArray
				case constant.Bool:
					kind = binary.SymbolBool
				case constant.Integer:
					kind = binary.SymbolInteger
				}

				sym := &binary.Symbol{
					Name:    name,
					Kind:    kind,
					Section: index,
					Offset:  offset,
					Length:  data.Len() - int(offset),
				}

				table = append(table, sym)
				symbols[sym.Name] = sym
			}
		}

		for _, lit := range p.Literals {
			index, data, err := pickSection(defaultStringsSectionSymbol, lit.Section())
			if err != nil {
				return err
			}

			val := lit.Value()

			switch val.Kind() {
			case constant.String:
				s := constant.StringVal(val)
				offset := uintptr(data.Len()) // Just the offset within the section for now.

				// If necessary, add padding to
				// meet alignment constraints.
				align := uintptr(lit.Alignment())
				padding := align - (offset % align)
				if padding != align {
					data.Write(zeros[:padding])
					offset += padding
				}

				data.WriteString(s)

				sym := &binary.Symbol{
					Name:    "." + s,
					Kind:    binary.SymbolString,
					Section: index,
					Offset:  offset,
					Length:  len(s),
				}

				table = append(table, sym)
				symbols[sym.Name] = sym
			case constant.Array:
				offset := uintptr(data.Len()) // Just the offset within the section for now.

				// If necessary, add padding to
				// meet alignment constraints.
				align := uintptr(lit.Alignment())
				padding := align - (offset % align)
				if padding != align {
					data.Write(zeros[:padding])
					offset += padding
				}

				name := fmt.Sprintf(".<array-literal-%d>", arrayLiterals)
				err := encodeConstant(data, arch.ByteOrder, val, lit.Type())
				if err != nil {
					return fmt.Errorf("failed to encode symbol %s: %v", name, err)
				}

				arrayLiterals++
				sym := &binary.Symbol{
					Name:    name,
					Kind:    binary.SymbolArray,
					Section: index,
					Offset:  offset,
					Length:  data.Len() - int(offset),
				}

				table = append(table, sym)
				symbols[sym.Name] = sym
			default:
				// Non-string, non-array constants are inlined.
				continue
			}
		}
	}

	const page4k = 0x1000 // One 4 KiB page.
	nextPage := func(n uintptr) uintptr {
		// Return the start of the next page.
		remainder := n % page4k
		gap := page4k - remainder
		return n + gap
	}

	bin := &binary.Binary{
		Arch:        arch,
		Entry:       main,
		Sections:    make([]*binary.Section, 0, 3),
		Symbols:     table,
		SymbolTable: symbolTable,
	}

	baseAddr := uintptr(0x20_0000) // 2 MiB in.
	lastAddr := uintptr(0)
	if p.BaseAddr != "" {
		addr, err := strconv.ParseUint(p.BaseAddr, 0, 64)
		if err != nil {
			return fmt.Errorf("internal error: failed to parse package base address %q: %v", p.BaseAddr, err)
		}

		baseAddr = uintptr(addr)
	}

	// Store the section data to each section.
	for i := range sections {
		sections[i].Section().Data = sectionsData[i].Bytes()
	}

	addSection := func(section types.Section) {
		if len(section.Section().Data) == 0 {
			return
		}

		nextAddr := baseAddr
		if lastAddr > baseAddr {
			nextAddr = nextPage(lastAddr)
		}

		if !section.FixedAddr() {
			section.Section().Address = nextAddr
		}

		bin.Sections = append(bin.Sections, section.Section())
		lastAddr = nextAddr + uintptr(len(section.Section().Data)) - 1
	}

	for _, section := range sections {
		addSection(section)
	}

	var b bytes.Buffer
	err = encode(&b, bin)
	if err != nil {
		return err
	}

	object := b.Bytes()

	// Perform any linkages.
	for _, fun := range p.Functions {
		// Get the function base.
		absFunName := p.Path + "." + fun.Name
		funSym := symbols[absFunName]
		if funSym == nil {
			return fmt.Errorf("internal error: failed to find symbol for %s", absFunName)
		}

		for _, link := range fun.Links {
			sym := symbols[link.Name]
			if sym == nil {
				return fmt.Errorf("internal error: failed to find symbol for %s", link.Name)
			}

			err := link.Perform(arch, object, funSym, sym.Address)
			if err != nil {
				return fmt.Errorf("%s: %v", absFunName, err)
			}
		}
	}

	err = os.WriteFile(out, object, 0755)
	if err != nil {
		return fmt.Errorf("failed to write %s: %v", out, err)
	}

	return nil
}

// encodeConstant writes the given constant to a
// section.
func encodeConstant(b *bytes.Buffer, byteOrder gobinary.ByteOrder, v constant.Value, t types.Type) error {
	switch v.Kind() {
	case constant.Bool:
		if constant.BoolVal(v) {
			b.WriteByte(1)
		} else {
			b.WriteByte(0)
		}
	case constant.Integer:
		typ := types.Underlying(t)
		switch typ {
		case types.Int, types.Int64:
			val, _ := constant.Int64Val(v)
			gobinary.Write(b, byteOrder, uint64(val))
		case types.Int32:
			val, _ := constant.Int64Val(v)
			gobinary.Write(b, byteOrder, uint32(val))
		case types.Int16:
			val, _ := constant.Int64Val(v)
			gobinary.Write(b, byteOrder, uint16(val))
		case types.Int8:
			val, _ := constant.Int64Val(v)
			gobinary.Write(b, byteOrder, uint8(val))
		case types.Uint, types.Uint64, types.Uintptr, types.UntypedInt:
			val, _ := constant.Uint64Val(v)
			gobinary.Write(b, byteOrder, uint64(val))
		case types.Uint32:
			val, _ := constant.Uint64Val(v)
			gobinary.Write(b, byteOrder, uint32(val))
		case types.Uint16:
			val, _ := constant.Uint64Val(v)
			gobinary.Write(b, byteOrder, uint16(val))
		case types.Uint8, types.Byte:
			val, _ := constant.Uint64Val(v)
			gobinary.Write(b, byteOrder, uint8(val))
		default:
			return fmt.Errorf("unexpected value kind %s with type %s", v.Kind(), t)
		}
	case constant.String:
		// TODO: add support for arrays of strings.
		return fmt.Errorf("unsupported value kind %s with type %s", v.Kind(), t)
	case constant.Array:
		values := constant.ArrayVal(v)
		element := t.(*types.Array).Element()
		for _, value := range values {
			err := encodeConstant(b, byteOrder, value, element)
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected value kind %s", v.Kind())
	}

	return nil
}
