// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Parse mnemonic and operand encoding tables into
// structured data, grouped into listings.

package main

import (
	"errors"
	"fmt"
	"strings"

	"rsc.io/pdf"
)

func respace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

var ErrAllIgnored = errors.New("all mnemonic table entries were ignored")

// ParsePage parses the PDF at the given page number,
// returning information on any instructions starting
// on that page. Subsequent pages will be parsed as
// needed to complete the instruction.
//
// ParsePage also returns the next page after the
// instruction.
func ParsePage(r *pdf.Reader, page int, stats *Stats, debug bool) (listing *Listing, nextPage int, err error) {
	name, pages, next := ReadInstructionPages(r, page, debug)
	if name == "" {
		return nil, next, nil
	}

	mnemonics, encodings := SplitInstructionPages(pages, stats, debug)
	if len(mnemonics) == 0 {
		return nil, next, Errorf(page, "found no mnemonic table for instruction %q", name)
	}

	stats.Listing()
	listing = &Listing{
		Page:  page,
		Pages: next - page,
		Name:  name,
	}

	ignored := false
	for i, page := range mnemonics {
		if debug {
			fmt.Printf("Mnemonic text %d:\n", i+1)
			for _, t := range page.Text {
				fmt.Println(t)
			}
		}

		table, err := ExtractMnemonicTable(page, stats, debug)
		if err != nil {
			return nil, next, Errorf(page.Page, "failed to extract mnemonic table %d: %v", i+1, err)
		}

		entries, err := ParseMnemonicTable(table, stats, debug)
		if err == ErrAllIgnored {
			ignored = true
			continue
		}

		if err != nil {
			return nil, next, Errorf(page.Page, "failed to parse mnemonic table %d: %v", i+1, err)
		}

		listing.MnemonicTable = append(listing.MnemonicTable, entries...)
	}

	// Look for cases where two successive
	// instructions are identical, except
	// that the second has a REX/REX.W
	// prefix. If so, we drop the second
	// instruction, as it can be derived
	// at runtime.
	trimmedMnemonics := make([]Mnemonic, 0, len(listing.MnemonicTable))
	for i, m := range listing.MnemonicTable {
		if i == 0 {
			trimmedMnemonics = append(trimmedMnemonics, m)
			continue
		}

		trimmed := strings.Replace(m.Opcode, "REX.W ", "", 1)
		trimmed = strings.Replace(trimmed, "REX ", "", 1)
		trimmed = strings.Replace(trimmed, "+ ", "", 1)

		p := listing.MnemonicTable[i-1]
		if trimmed == p.Opcode && m.Instruction == p.Instruction && m.OperandEncoding == p.OperandEncoding && m.Mode64 == p.Mode64 {
			// Skip this one.
			continue
		}

		trimmedMnemonics = append(trimmedMnemonics, m)
	}
	listing.MnemonicTable = trimmedMnemonics

	// Check whether we have any groups
	// of mnemonics that vary only by
	// their operand size, suggesting
	// that they are selected using the
	// operand size override prefix.
	//
	// Start by building up a mapping
	// based on the opcode.
	opcodes := make(map[string][]*Mnemonic)
	for i, mnemonic := range listing.MnemonicTable {
		opcode := mnemonic.Opcode
		if strings.Contains(opcode, "VEX") {
			continue
		}

		// We also ignore any code offset
		// suffix, since this is influenced
		// by the operand size override
		// prefix.
		opcode = strings.TrimSuffix(opcode, " cw")
		opcode = strings.TrimSuffix(opcode, " cd")

		// Likewise with immediate sizes.
		opcode = strings.TrimSuffix(opcode, " iw")
		opcode = strings.TrimSuffix(opcode, " id")

		// Likewise with opcode registers.
		opcode = strings.TrimSuffix(opcode, "+rw")
		opcode = strings.TrimSuffix(opcode, "+rd")
		opcodes[opcode] = append(opcodes[opcode], &listing.MnemonicTable[i])
	}

	// Now find the groups and check
	// their mode compatibilities
	// overlap.
	for _, group := range opcodes {
		if len(group) < 2 {
			continue
		}

		// Turn the mode compatibilities
		// into a bitmap.
		const (
			mode64 = 1 << iota
			mode32
			mode16
		)

		modes := make([]uint8, len(group))
		for i, m := range group {
			var mode uint8
			if m.Mode64 == "Valid" {
				mode |= mode64
			}
			if m.Mode32 == "Valid" {
				mode |= mode32
			}
			if m.Mode16 == "Valid" {
				mode |= mode16
			}

			modes[i] = mode
		}

		for i := range modes {
			for j := i + 1; j < len(modes); j++ {
				// The instruction mnemonics must
				// also be the same, or we will
				// erroneously include aliased
				// instructions like JZ and JE.
				mnemonic1, _, _ := strings.Cut(group[i].Instruction, " ")
				mnemonic2, _, _ := strings.Cut(group[j].Instruction, " ")
				if modes[i]&modes[j] != 0 && mnemonic1 == mnemonic2 {
					group[i].OperandSize = true
					group[j].OperandSize = true
				}
			}
		}
	}

	for i, page := range encodings {
		if debug {
			fmt.Printf("Operand encoding text %d:\n", i+1)
			for _, t := range page.Text {
				fmt.Println(t)
			}
		}

		table, err := ExtractEncodingTable(page, stats, debug)
		if err != nil {
			return nil, next, Errorf(page.Page, "failed to extract operand encoding table %d: %v", i+1, err)
		}

		entries, err := ParseOperandEncodingTable(table, stats, debug)
		if err != nil {
			return nil, next, Errorf(page.Page, "failed to parse operand encoding table %d: %v", i+1, err)
		}

		listing.OperandEncodingTable = append(listing.OperandEncodingTable, entries...)
	}

	if ignored && len(listing.MnemonicTable) == 0 {
		return listing, next, ErrAllIgnored
	}

	return listing, next, nil
}

func ParseMnemonicTable(table *Table, stats *Stats, debug bool) ([]Mnemonic, error) {
	if table == nil || len(table.Rows) == 0 {
		return nil, nil
	}

	ignored := false
	headings := table.Rows[0]
	var out []Mnemonic
	for i, row := range table.Rows {
		if i == 0 {
			continue
		}

		if len(row) != len(headings) {
			return nil, Errorf(table.Page, "invalid mnemonic table: row %d has the wrong number of columns: want %d, got %d (%q)", i+1, len(headings), len(row), row)
		}

		var opcode, instruction, operandEncoding, mode64, mode32, cpuid, description string
		for j, col := range row {
			switch headings[j] {
			case "Opcode":
				opcode = col
			case "Instruction":
				instruction = col
			case "Opcode/Instruction":
				// Find the split.
				i := strings.IndexByte(col, '\n')
				if i < 0 {
					return nil, Errorf(table.Page, "invalid mnemonic table: row %d has invalid opcode/instruction value %q (%q)", i+1, col, row)
				}

				opcode = respace(col[:i])
				instruction = respace(col[i+1:])
			case "Op/En":
				operandEncoding = col
			case "64-Bit Mode":
				mode64 = col
			case "32-Bit Mode":
				mode32 = col
			case "64/32-Bit Mode":
				switch col {
				case "VV":
					stats.InstructionError()
					col = "V/V"
				}

				parts := strings.Split(col, "/")
				if len(parts) != 2 {
					return nil, Errorf(table.Page, "invalid mnemonic table: row %d has invalid 64/32-bit mode value %q (%q)", i+1, col, row)
				}

				mode64 = parts[0]
				mode32 = parts[1]
			case "CPUID Feature Flag":
				cpuid = col
			case "Description":
				description = col
			default:
				return nil, Errorf(table.Page, "invalid mnemonic table: row %d has invalid heading %d (%q)", i+1, j+1, headings[j])
			}
		}

		opcode = respace(opcode)
		instruction = respace(instruction)
		if ignoreInstruction[instruction] {
			ignored = true
			continue
		}

		mnemonic := Mnemonic{
			Page:            table.Page,
			Opcode:          opcode,
			Instruction:     instruction,
			OperandEncoding: operandEncoding,
			Mode64:          mode64,
			Mode32:          mode32,
			CPUID:           cpuid,
			Description:     description,
		}

		stats.Instruction(table.Page)
		err := mnemonic.fix(stats)
		if err != nil {
			return nil, Errorf(table.Page, "invalid mnemonic table: row %d (%q) has %v", i+1, row, err)
		}

		out = append(out, mnemonic)
	}

	if ignored && len(out) == 0 {
		return nil, ErrAllIgnored
	}

	return out, nil
}

func ParseOperandEncodingTable(table *Table, stats *Stats, debug bool) ([]OperandEncoding, error) {
	if table == nil || len(table.Rows) == 0 {
		return nil, nil
	}

	var err error
	table.Rows, err = fixOperandEncodingTable(table.Page, table.Rows, stats)
	if err != nil {
		return nil, err
	}

	var out []OperandEncoding
	for i, row := range table.Rows {
		if i == 0 {
			continue
		}

		if len(row) != 6 {
			return nil, Errorf(table.Page, "invalid instruction operand encoding table: row %d is invalid (%q)", i+1, row)
		}

		operandEncoding := OperandEncoding{
			Page:      table.Page,
			Encoding:  row[0],
			TupleType: row[1],
			Operands: [4]string{
				row[2],
				row[3],
				row[4],
				row[5],
			},
		}

		err = operandEncoding.fix(stats)
		if err != nil {
			return nil, Errorf(table.Page, "invalid instruction operand encoding table: row %d (%q) has %v", i+1, row, err)
		}

		out = append(out, operandEncoding)
	}

	return out, nil
}

// IgnoredInstructions lists the instruction syntaxes
// that we ignore when parsing mnemonic tables. These
// are forms that have a misleading encoding, such as
// appearing to take an arbitrary memory address when
// actually the address used is fixed.
var IgnoredInstructions = []string{
	// The memory address is actually fixed.
	"CMPS m16, m16",
	"CMPS m32, m32",
	"CMPS m64, m64",
	"CMPS m8, m8",
	// The general form is sufficient.
	"ENTER imm16, 0",
	"ENTER imm16,1",
	// GETSEC is a single instruction
	// with multiple leaf forms. We add
	// the general form separately.
	"GETSEC[CAPABILITIES]",
	"GETSEC[ENTERACCS]",
	"GETSEC[EXITAC]",
	"GETSEC[SENTER]",
	"GETSEC[SEXIT]",
	"GETSEC[PARAMETERS]",
	"GETSEC[SMCTRL]",
	"GETSEC[WAKEUP]",
	// The memory address is actually fixed.
	"INS m16, DX",
	"INS m32, DX",
	"INS m8, DX",
	// We ignore prefixes.
	"LOCK",
	// The memory address is actually fixed.
	"LODS m16",
	"LODS m32",
	"LODS m64",
	"LODS m8",
	"MOVS m16, m16",
	"MOVS m32, m32",
	"MOVS m64, m64",
	"MOVS m8, m8",
	"OUTS DX, m16",
	"OUTS DX, m32",
	"OUTS DX, m8",
	// We ignore forms that are actually
	// prefixes, not mnemonics.
	"REP INS m16, DX",
	"REP INS m32, DX",
	"REP INS m8, DX",
	"REP INS r/m32, DX",
	"REP LODS AL",
	"REP LODS AX",
	"REP LODS EAX",
	"REP LODS RAX",
	"REP MOVS m16, m16",
	"REP MOVS m32, m32",
	"REP MOVS m64, m64",
	"REP MOVS m8, m8",
	"REP OUTS DX, m16",
	"REP OUTS DX, m32",
	"REP OUTS DX, m8",
	"REP OUTS DX, r/m16",
	"REP OUTS DX, r/m32",
	"REP OUTS DX, r/m8",
	"REP STOS m16",
	"REP STOS m32",
	"REP STOS m64",
	"REP STOS m8",
	"REPE CMPS m16, m16",
	"REPE CMPS m32, m32",
	"REPE CMPS m64, m64",
	"REPE CMPS m8, m8",
	"REPE SCAS m16",
	"REPE SCAS m32",
	"REPE SCAS m64",
	"REPE SCAS m8",
	"REPNE CMPS m16, m16",
	"REPNE CMPS m32, m32",
	"REPNE CMPS m64, m64",
	"REPNE CMPS m8, m8",
	"REPNE SCAS m16",
	"REPNE SCAS m32",
	"REPNE SCAS m64",
	"REPNE SCAS m8",
	// The memory address is actually fixed.
	"SCAS m16",
	"SCAS m32",
	"SCAS m64",
	"SCAS m8",
	"STOS m16",
	"STOS m32",
	"STOS m64",
	"STOS m8",
	"XLAT m8",
	// We ignore prefixes.
	"XACQUIRE",
	"XRELEASE",
}

var ignoreInstruction map[string]bool

func init() {
	ignoreInstruction = make(map[string]bool)
	for _, inst := range IgnoredInstructions {
		ignoreInstruction[inst] = true
	}
}
