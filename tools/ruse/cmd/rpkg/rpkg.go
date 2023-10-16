// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package rpkg prints debug information about a Ruse rpkg file.
package rpkg

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"go/constant"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"firefly-os.dev/tools/ruse/compiler"
	"firefly-os.dev/tools/ruse/rpkg"
)

var program = filepath.Base(os.Args[0])

// Main prints debug information about a Ruse rpkg file.
func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("rpkg", flag.ExitOnError)

	var help, header, imports, exports, types, symbols, sections, strings, linkages, functions bool
	all := [...]*bool{
		&header,
		&imports,
		&exports,
		&types,
		&symbols,
		&sections,
		&strings,
		&linkages,
		&functions,
	}

	flags.BoolVar(&help, "h", false, "Show this message and exit.")
	flags.BoolVar(&header, "header", true, "Print information about the rpkg header.")
	flags.BoolVar(&imports, "imports", false, "Print the list of imported package names.")
	flags.BoolVar(&exports, "exports", false, "Print the list of exported symbols.")
	flags.BoolVar(&types, "types", false, "Print the set of types defined.")
	flags.BoolVar(&symbols, "symbols", false, "Print the set of symbols defined.")
	flags.BoolVar(&sections, "sections", false, "Print the set of sections defined.")
	flags.BoolVar(&strings, "strings", false, "Print the set of strings defined.")
	flags.BoolVar(&linkages, "linkages", false, "Print the set of linkages defined.")
	flags.BoolVar(&functions, "functions", false, "Print the set of functions defined.")
	flags.BoolFunc("all", "Print all information.", func(s string) error {
		v, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}

		if v {
			for _, b := range all {
				*b = true
			}
		}

		return nil
	})

	flags.Usage = func() {
		log.Printf("Usage:\n  %s %s [OPTIONS] RPKG\n\n", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(2)
	}

	err := flags.Parse(args)
	if err != nil || help {
		flags.Usage()
	}

	filenames := flags.Args()
	if len(filenames) != 1 {
		flags.Usage()
		os.Exit(2)
	}

	name := filenames[0]
	data, err := os.ReadFile(filenames[0])
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", name, err)
	}

	d, err := rpkg.NewDecoder(data)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", name, err)
	}

	var numSections int
	for _, b := range []bool{header, imports, exports, types, symbols, sections, strings, linkages, functions} {
		if b {
			numSections++
		}
	}

	var printSectionHeadings bool
	switch numSections {
	case 0:
		return nil
	case 1:
		printSectionHeadings = false
	default:
		printSectionHeadings = true
	}

	printSection := func(s string) {
		if printSectionHeadings {
			fmt.Printf("%s:\n", s)
		}
	}

	printText := func(format string, v ...any) {
		if printSectionHeadings {
			fmt.Printf("\t"+format, v...)
		} else {
			fmt.Printf(format, v...)
		}
	}

	if header {
		hdr := d.Header()
		fmt.Printf("architecture: %s\n", hdr.Architecture)
		fmt.Printf("rpkg version: %d\n", hdr.Version)
		fmt.Printf("checksum:     %x\n", hdr.Checksum)
		fmt.Printf("package name: %s\n", hdr.PackageName)
		fmt.Printf("sections:\n")
		fmt.Printf("\timports offset:  %d\n", hdr.ImportsOffset)
		fmt.Printf("\texports offset:  %d\n", hdr.ExportsOffset)
		fmt.Printf("\ttypes offset:    %d\n", hdr.TypesOffset)
		fmt.Printf("\tsymbols offset:  %d\n", hdr.SymbolsOffset)
		fmt.Printf("\tABIs offset:     %d\n", hdr.ABIsOffset)
		fmt.Printf("\tsections offset: %d\n", hdr.SectionsOffset)
		fmt.Printf("\tstrings offset:  %d\n", hdr.StringsOffset)
		fmt.Printf("\tlinkages offset: %d\n", hdr.LinkagesOffset)
		fmt.Printf("\tcode offset:     %d\n", hdr.CodeOffset)
		fmt.Printf("\tchecksum offset: %d\n", hdr.ChecksumOffset)
	}

	// The order here matters.
	gotStrings, err := d.Strings()
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", name, err)
	}

	gotImports, err := d.Imports()
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", name, err)
	}

	_, err = d.ABIs()
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", name, err)
	}

	gotSections, err := d.Sections()
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", name, err)
	}

	gotTypes, err := d.Types()
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", name, err)
	}

	gotSymbols, _, err := d.Symbols()
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", name, err)
	}

	gotExports, err := d.Exports()
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", name, err)
	}

	gotLinkages, err := d.Linkages()
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", name, err)
	}

	if imports {
		printSection("imports")
		for _, imp := range gotImports {
			printText("%s\n", imp)
		}
	}

	if exports {
		printSection("exports")
		for _, exp := range gotExports {
			printText("%s\n", exp.Name())
		}
	}

	if types {
		printSection("types")
		for i, typ := range gotTypes {
			if i == 0 && typ == nil {
				continue
			}

			printText("%s\n", typ)
		}
	}

	if symbols {
		printSection("symbols")
		for _, sym := range gotSymbols {
			switch sym.Kind {
			case rpkg.SymKindFunction:
				// The type is already printed in parentheses,
				// so there's no need to add more. Also, the
				// data isn't meaningfully printable.
				printText("%s %s %s\n", sym.Kind, sym.AbsoluteName(), sym.Type)
			case rpkg.SymKindStringConstant:
				// We want to quote the string.
				v := sym.Value.(constant.Value)
				s := constant.StringVal(v)
				printText("%s %s (%s): %q\n", sym.Kind, sym.AbsoluteName(), sym.Type, s)
			default:
				printText("%s %s (%s): %v\n", sym.Kind, sym.AbsoluteName(), sym.Type, sym.Value)
			}
		}
	}

	if sections {
		printSection("sections")
		for _, section := range gotSections {
			var fixed string
			if section.FixedAddr() {
				fixed = " (fixed)"
			}

			sect := section.Section()
			printText("%#016x %s %s%s\n", sect.Address, sect.Permissions, sect.Name, fixed)
		}
	}

	if strings {
		printSection("strings")
		for i, str := range gotStrings {
			if i == 0 && str == "" {
				continue
			}

			printText("%q\n", str)
		}
	}

	if linkages {
		printSection("linkages")
		for _, link := range gotLinkages {
			printText("%s: %s (%s) at offset %d (address %#x)\n", link.Source, link.Target, link.Type, link.Offset, link.Address)
		}
	}

	if functions {
		printSection("functions")
		printed := 0
		for _, sym := range gotSymbols {
			if printed != 0 {
				fmt.Println()
			}

			switch sym.Kind {
			case rpkg.SymKindFunction:
				// The type is already printed in parentheses,
				// so there's no need to add more. Also, the
				// data isn't meaningfully printable.
				printText("%s %s\n", sym.Name, sym.Type)
				for _, link := range sym.Links {
					printText("Link %s for %s at %d\n", link.Type, link.Name, link.Offset)
				}

				printText("%s", hex.Dump([]byte(sym.Value.(compiler.MachineCode))))
				printed++
			}
		}
	}

	return nil
}
