// Copyright 2024 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package disasm disassembles a Ruse rpkg or executable binary.
package disasm

import (
	"bytes"
	"context"
	"debug/elf"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"golang.org/x/arch/x86/x86asm"

	"firefly-os.dev/tools/ruse/compiler"
	"firefly-os.dev/tools/ruse/rpkg"
)

var program = filepath.Base(os.Args[0])

// Main links together a set of Ruse rpkg files into an executable
// binary.
func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("disasm", flag.ExitOnError)

	var help bool
	var mode int
	var sections, symbols *regexp.Regexp
	flags.BoolVar(&help, "h", false, "Show this message and exit.")
	flags.IntVar(&mode, "mode", 64, "The CPU mode to assume while disassembling.")
	flags.Func("sections", "A regular expression selecting which sections to disassemble (defaults to 'code').", func(s string) error {
		if sections != nil {
			return fmt.Errorf("-sections provided more than once")
		}

		var err error
		sections, err = regexp.Compile(s)
		return err
	})
	flags.Func("symbols", "A regular expression selecting which symbols to disassemble (defaults to '.+').", func(s string) error {
		if symbols != nil {
			return fmt.Errorf("-symbols provided more than once")
		}

		var err error
		symbols, err = regexp.Compile(s)
		return err
	})

	flags.Usage = func() {
		log.Printf("Usage:\n  %s %s OPTIONS FILE\n\n", program, flags.Name())
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

	filename := filenames[0]

	printed := false
	var disasm bytes.Buffer
	disassemble := func(pc uintptr, src []byte, section, symbol string, symname x86asm.SymLookup) error {
		// Apply our filters.
		if sections != nil && !sections.MatchString(section) {
			return nil
		}

		if symbols != nil && !symbols.MatchString(symbol) {
			return nil
		}

		// TODO: support other architectures.

		disasm.Reset()
		if printed {
			disasm.WriteByte('\n')
		}
		printed = true

		fmt.Fprintf(&disasm, "%016x <%s>:\n", pc, symbol)
		for len(src) > 0 {
			inst, err := x86asm.Decode(src, mode)
			if err != nil {
				return err
			}

			size := inst.Len
			if size == 0 {
				size = 1
			}

			fmt.Fprintf(&disasm, "  %06x:\t% -21x\t%s\n", pc, src[:size], x86asm.IntelSyntax(inst, uint64(pc), symname))

			src = src[size:]
			pc += uintptr(size)
		}

		_, err := w.Write(disasm.Bytes())
		return err
	}

	if filepath.Ext(filename) == ".rpkg" {
		// We know how to handle RPKG files.
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read rpkg %q: %v", filename, err)
		}

		d, err := rpkg.NewDecoder(data)
		if err != nil {
			return fmt.Errorf("failed to parse rpkg %q: %v", filename, err)
		}

		// Pull all the data from the package.
		// The order of these steps is important.

		_, err = d.ABIs()
		if err != nil {
			return fmt.Errorf("failed to parse rpkg %q: %v", filename, err)
		}

		_, err = d.Sections()
		if err != nil {
			return fmt.Errorf("failed to parse rpkg %q: %v", filename, err)
		}

		_, err = d.Strings()
		if err != nil {
			return fmt.Errorf("failed to parse rpkg %q: %v", filename, err)
		}

		_, err = d.Imports()
		if err != nil {
			return fmt.Errorf("failed to parse rpkg %q: %v", filename, err)
		}

		_, err = d.Types()
		if err != nil {
			return fmt.Errorf("failed to parse rpkg %q: %v", filename, err)
		}

		symbols, _, err := d.Symbols()
		if err != nil {
			return fmt.Errorf("failed to parse rpkg %q: %v", filename, err)
		}

		for _, sym := range symbols {
			if sym.Kind != rpkg.SymKindFunction {
				continue
			}

			section := sym.SectionName
			if section == "" {
				section = "code" // This is the default.
			}

			symbol := sym.AbsoluteName()
			src := sym.Value.(compiler.MachineCode)
			err = disassemble(0, []byte(src), section, symbol, nil)
			if err != nil {
				return fmt.Errorf("failed to disassemble rpkg %q: %v", filename, err)
			}
		}

		return nil
	}

	// For now, we assume ELF.
	// TODO: add more binary formats.
	f, err := elf.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open %q: %v", filename, err)
	}

	defer f.Close()

	codeSectionIndex := -1
	if sections == nil && symbols == nil {
		for i, sect := range f.Sections {
			if sect.Name == "code" {
				codeSectionIndex = i
				break
			}
		}
	}

	syms, err := f.Symbols()
	if err != nil {
		return fmt.Errorf("failed to read symbol table: %v", err)
	}

	// Process the symbol table.
	slices.SortFunc(syms, func(a, b elf.Symbol) int {
		if a.Value < b.Value {
			return -1
		}

		if a.Value > b.Value {
			return +1
		}

		return 0
	})

	symname := func(pc uint64) (string, uint64) {
		// Iterate through the symbols backwards,
		// so that we return the symbol closest
		// to the address.
		for i := len(syms) - 1; i >= 0; i-- {
			sym := syms[i]
			if sym.Value > pc {
				continue
			}

			return sym.Name, sym.Value
		}

		return "", 0
	}

	sectionData := make([][]byte, len(f.Sections))
	for _, sym := range syms {
		if codeSectionIndex != -1 && int(sym.Section) != codeSectionIndex {
			continue
		}

		section := f.Sections[sym.Section].Name
		symbol := sym.Name

		data := sectionData[sym.Section]
		if data == nil {
			data, err = f.Sections[sym.Section].Data()
			if err != nil {
				return fmt.Errorf("failed to read section %q: %v", section, err)
			}

			sectionData[sym.Section] = data
		}

		offset := sym.Value - f.Sections[sym.Section].Addr
		err = disassemble(uintptr(sym.Value), data[offset:offset+sym.Size], section, symbol, symname)
		if err != nil {
			return fmt.Errorf("failed to disassemble %q: %v", filename, err)
		}
	}

	return nil
}
