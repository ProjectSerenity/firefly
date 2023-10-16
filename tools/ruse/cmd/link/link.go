// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package link links a set of Ruse rpkg files into an executable binary.
package link

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/constant"
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
	"firefly-os.dev/tools/ruse/rpkg"
	"firefly-os.dev/tools/ruse/types"
)

var program = filepath.Base(os.Args[0])

type binaryEncoder func(w io.Writer, bin *binary.Binary) error

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

	const (
		codeSection    = "code"
		stringsSection = "strings"
		rpkgsSection   = "rpkgs"
	)

	type explicitSection struct {
		binary.Section
		FixedAddr bool
	}

	// Prepare the list of sections.
	sections := []*explicitSection{
		{
			Section: binary.Section{
				Name:        codeSection,
				Permissions: binary.Read | binary.Execute,
			},
		},
		{
			Section: binary.Section{
				Name:        stringsSection,
				Permissions: binary.Read,
			},
		},
		{
			Section: binary.Section{
				Name:        rpkgsSection,
				Permissions: binary.Read,
			},
		},
	}

	sectionIndices := make(map[string]int)
	for i, section := range sections {
		sectionIndices[section.Name] = i
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

	// Build the symbol table.
	symbols := make(map[string]*binary.Symbol)
	var main *binary.Symbol
	var table []*binary.Symbol
	var code, stringsData bytes.Buffer
	for _, fun := range p.Functions {
		prev := code.Len()
		sym := &binary.Symbol{
			Name:    p.Path + "." + fun.Name,
			Kind:    binary.SymbolFunction,
			Section: sectionIndices[codeSection],
			Offset:  uintptr(prev), // Just the offset within the section for now.
		}

		err = compiler.EncodeTo(&code, nil, arch, fun)
		if err != nil {
			return err
		}

		sym.Length = code.Len() - prev
		table = append(table, sym)
		symbols[sym.Name] = sym
		if fun.Name == "main" {
			if main != nil {
				return fmt.Errorf("main function: unexpectedly found a second main function")
			}

			main = sym
		}
	}

	for _, con := range p.Constants {
		val := con.Value()
		if val.Kind() != constant.String {
			// Non-string constants are inlined.
			continue
		}

		s := constant.StringVal(val)
		sym := &binary.Symbol{
			Name:    p.Path + "." + con.Name(),
			Kind:    binary.SymbolString,
			Section: sectionIndices[stringsSection],
			Offset:  uintptr(stringsData.Len()), // Just the offset within the section for now.
			Length:  len(s),
		}

		stringsData.WriteString(s)
		table = append(table, sym)
		symbols[sym.Name] = sym
	}

	for _, lit := range p.Literals {
		val := lit.Value()
		if val.Kind() != constant.String {
			// Non-string constants are inlined.
			continue
		}

		s := constant.StringVal(val)
		sym := &binary.Symbol{
			Name:    "." + s,
			Kind:    binary.SymbolString,
			Section: sectionIndices[stringsSection],
			Offset:  uintptr(stringsData.Len()), // Just the offset within the section for now.
			Length:  len(s),
		}

		stringsData.WriteString(s)
		table = append(table, sym)
		symbols[sym.Name] = sym
	}

	var rpkgsData cryptobyte.Builder
	if provenance {
		rpkgsData.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes([]byte(p.Path))
		})
		rpkgsData.AddBytes(checksum)
	}

	addPackage := func(p *compiler.Package, checksum []byte) error {
		for _, fun := range p.Functions {
			prev := code.Len()
			sym := &binary.Symbol{
				Name:    p.Path + "." + fun.Name,
				Kind:    binary.SymbolFunction,
				Section: sectionIndices[codeSection],
				Offset:  uintptr(prev), // Just the offset within the section for now.
			}

			err = compiler.EncodeTo(&code, nil, arch, fun)
			if err != nil {
				return err
			}

			sym.Length = code.Len() - prev
			table = append(table, sym)
			symbols[sym.Name] = sym
		}

		for _, con := range p.Constants {
			val := con.Value()
			if val == nil || val.Kind() != constant.String {
				// Non-string constants are inlined.
				continue
			}

			s := constant.StringVal(val)
			sym := &binary.Symbol{
				Name:    p.Path + "." + con.Name(),
				Kind:    binary.SymbolString,
				Section: sectionIndices[stringsSection],
				Offset:  uintptr(stringsData.Len()), // Just the offset within the section for now.
				Length:  len(s),
			}

			stringsData.WriteString(s)
			table = append(table, sym)
			symbols[sym.Name] = sym
		}

		for _, lit := range p.Literals {
			val := lit.Value()
			if val.Kind() != constant.String {
				// Non-string constants are inlined.
				continue
			}

			s := constant.StringVal(val)
			sym := &binary.Symbol{
				Name:    "." + s,
				Kind:    binary.SymbolString,
				Section: sectionIndices[stringsSection],
				Offset:  uintptr(stringsData.Len()), // Just the offset within the section for now.
				Length:  len(s),
			}

			stringsData.WriteString(s)
			table = append(table, sym)
			symbols[sym.Name] = sym
		}

		if provenance {
			rpkgsData.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
				b.AddBytes([]byte(p.Path))
			})
			rpkgsData.AddBytes(checksum)
		}

		return nil
	}

	// Add the dependencies, checking
	// that we have all the imports we
	// need.
	seenPackages := make(map[string]bool)
	needPackages := make(map[string]bool)
	seenPackages[p.Path] = true
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

		if err := addPackage(p, checksum); err != nil {
			return err
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

			if err := addPackage(p, checksum); err != nil {
				return err
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
	if p.BaseAddr != nil {
		addr, err := strconv.ParseUint(p.BaseAddr.Value, 0, 64)
		if err != nil {
			return fmt.Errorf("internal error: failed to parse package base address %q: %v", p.BaseAddr.Value, err)
		}

		baseAddr = uintptr(addr)
	}

	addSection := func(section *explicitSection) {
		if len(section.Data) == 0 {
			return
		}

		nextAddr := baseAddr
		if lastAddr > baseAddr {
			nextAddr = nextPage(lastAddr)
		}

		if !section.FixedAddr {
			section.Address = nextAddr
		}

		bin.Sections = append(bin.Sections, &section.Section)
		lastAddr = nextAddr + uintptr(len(section.Data)) - 1
	}

	sections[sectionIndices[codeSection]].Data = code.Bytes()
	sections[sectionIndices[stringsSection]].Data = stringsData.Bytes()
	sections[sectionIndices[rpkgsSection]].Data = rpkgsData.BytesOrPanic()

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
