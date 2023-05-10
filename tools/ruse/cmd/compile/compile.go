// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package compile compiles a set of Ruse source code.
package compile

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

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/binary"
	"firefly-os.dev/tools/ruse/binary/elf"
	"firefly-os.dev/tools/ruse/compiler"
	"firefly-os.dev/tools/ruse/parser"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

var program = filepath.Base(os.Args[0])

type binaryEncoder func(w io.Writer, bin *binary.Binary) error

// Main compiles a set of Ruse files into an executable
// binary.
func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("compile", flag.ExitOnError)

	var help bool
	var out string
	var arch *sys.Arch
	var encode binaryEncoder
	flags.BoolVar(&help, "h", false, "Show this message and exit.")
	flags.Func("arch", "The target architecture (x86-64).", func(s string) error {
		if arch != nil {
			return fmt.Errorf("-arch can only be specified once")
		}

		switch s {
		case "x86-64":
			arch = sys.X86_64
		default:
			return fmt.Errorf("unrecognised -arch: %q", s)
		}

		return nil
	})
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
	flags.StringVar(&out, "o", "", "The name of the compiled binary.")

	flags.Usage = func() {
		log.Printf("Usage:\n  %s %s OPTIONS FILE...\n\n", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(2)
	}

	err := flags.Parse(args)
	if err != nil || help {
		flags.Usage()
	}

	if arch == nil || encode == nil || out == "" {
		flags.Usage()
		os.Exit(2)
	}

	filenames := flags.Args()
	if len(filenames) == 0 {
		flags.Usage()
		os.Exit(2)
	}

	fset := token.NewFileSet()
	files := make([]*ast.File, len(filenames))
	for i, filename := range filenames {
		files[i], err = parser.ParseFile(fset, filename, nil, 0)
		if err != nil {
			return err
		}

		if files[i].Name.Name != "main" {
			return fmt.Errorf("can only compile package main yet, found %s", files[i].Name.Name)
		}
	}

	info := &types.Info{
		Types:       make(map[ast.Expression]types.TypeAndValue),
		Definitions: make(map[*ast.Identifier]types.Object),
		Uses:        make(map[*ast.Identifier]types.Object),
	}

	var config types.Config
	pkg, err := config.Check("main", fset, files, arch, info)
	if err != nil {
		return err
	}

	sizes := types.SizesFor(arch)
	p, err := compiler.Compile(fset, arch, pkg, files, info, sizes)
	if err != nil {
		return err
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

	symbols := make(map[string]*binary.Symbol)
	var table []*binary.Symbol
	var code, stringsData bytes.Buffer
	for _, fun := range p.Functions {
		prev := code.Len()
		sym := &binary.Symbol{
			Name:    p.Path + "." + fun.Name,
			Kind:    binary.SymbolFunction,
			Section: 0,
			Offset:  uintptr(prev), // Just the offset within the section for now.
		}

		err = compiler.EncodeTo(&code, fset, arch, fun)
		if err != nil {
			return err
		}

		sym.Length = code.Len() - prev
		table = append(table, sym)
		symbols[sym.Name] = sym
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
			Section: 1,
			Offset:  uintptr(stringsData.Len()), // Just the offset within the section for now.
			Length:  len(s),
		}

		stringsData.WriteString(s)
		table = append(table, sym)
		symbols[sym.Name] = sym
	}

	const page4k = 0x1000 // One 4 KiB page.
	bin := &binary.Binary{
		Arch:     arch,
		BaseAddr: 0x20_0000, // 2 MiB in.
		Sections: []*binary.Section{
			{
				Name:        "code",
				Address:     0x20_0000,
				Permissions: binary.Read | binary.Execute,
				Data:        code.Bytes(),
			},
			{
				Name:        "strings",
				Address:     0x20_0000 + uintptr(code.Len()) + uintptr(page4k-(code.Len()%page4k)), // The start of the next page after code.
				Permissions: binary.Read,
				Data:        stringsData.Bytes(),
			},
		},
	}

	var b bytes.Buffer
	err = encode(&b, bin)
	if err != nil {
		return err
	}

	object := b.Bytes()

	// Finish the symbol table.
	for _, sym := range table {
		sym.Offset += bin.Sections[sym.Section].Offset
		sym.Address = sym.Offset + bin.Sections[sym.Section].Address
	}

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

			err := link.Perform(arch, object, int(funSym.Offset), sym.Address)
			if err != nil {
				return fmt.Errorf("%s: %v", fset.Position(link.Pos), err)
			}
		}
	}

	err = os.WriteFile(out, object, 0755)
	if err != nil {
		return fmt.Errorf("failed to write %s: %v", out, err)
	}

	return nil
}
