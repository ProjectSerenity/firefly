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

	var help bool
	var out string
	var encode binaryEncoder
	flags.BoolVar(&help, "h", false, "Show this message and exit.")
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
	arch, p, err := rpkg.Decode(info, data)
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
