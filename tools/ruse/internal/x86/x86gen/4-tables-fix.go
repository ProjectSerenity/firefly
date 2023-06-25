// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"strings"
	"unicode"
)

func dropSpaces(s string) string {
	dropSpace := func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}

		return r
	}

	return strings.Map(dropSpace, s)
}

func stringSetEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

func fixMnemonicHeadings(page int, headings []string, stats *Stats) ([]string, error) {
	var err error
	var opcode, instruction, operandEncoding, mode64, mode32, cpuid, description bool
	for i, col := range headings {
		match := func(b *bool, err *error) {
			if *err != nil {
				return
			}

			if *b {
				*err = Errorf(page, "invalid mnemonic table: header %d (%q) is a repetition (row %q)", i+1, col, headings)
			}

			*b = true
		}

		switch dropSpaces(col) {
		case "Opcode",
			"Opcode*", // Asterisk.
			"1Opcode": // Misparsed superscript 1.
			col = "Opcode"
			match(&opcode, &err)

		case "Instruction":
			col = "Instruction"
			match(&instruction, &err)

		case "OpcodeInstruction":
			stats.ListingError("p.%d: Invalid mnemonic table heading entry %q", page, dropSpaces(col))
			fallthrough
		case "Opcode/Instruction", "Opcode*/Instruction":
			col = "Opcode/Instruction"
			match(&opcode, &err)
			match(&instruction, &err)

		case "Op/En", "OpEn":
			col = "Op/En"
			match(&operandEncoding, &err)

		case "64-BitMode", "64-bitMode":
			col = "64-Bit Mode"
			match(&mode64, &err)

		case "32-BitMode",
			"Compat/LegMode",
			"Compat/1LegMode": // Misparsed superscript 1.
			col = "32-Bit Mode"
			match(&mode32, &err)

		case "64/32*bitModeSupport":
			if col == "64/32 *\nbit Mode\nSupport" {
				// The asterisk is spurious (and not visible!).
				stats.ListingError("p.%d: Spurious hidden asterisk in mnemonic table heading entry %q", page, dropSpaces(col))
			}
			fallthrough
		case "32/64bitModeSupport",
			"64/32-bitMode",
			"64/32-bitModeSupport",
			"64/32bitModeSupport":
			col = "64/32-Bit Mode"
			match(&mode64, &err)
			match(&mode32, &err)

		case "CPUID",
			"CPUIDFeatureFlag", "CPUIDFea-tureFlag":
			col = "CPUID Feature Flag"
			match(&cpuid, &err)

		case "Description",
			"DescriptionST(0)": // Misparsed superscript ST(0) on the line below.
			col = "Description"
			match(&description, &err)

		default:
			if err != nil {
				return nil, err
			}

			return nil, Errorf(page, "invalid mnemonic table: header %d (%q) is invalid (row %q)", i+1, col, headings)
		}

		headings[i] = col
	}

	if err != nil {
		return nil, err
	}

	switch false {
	case opcode:
		return nil, Errorf(page, "invalid mnemonic table: no opcode column (row %q)", headings)
	case instruction:
		return nil, Errorf(page, "invalid mnemonic table: no instruction column (row %q)", headings)
	case operandEncoding:
		// This is likely an error, but if
		// the instruction takes no parameters,
		// then its encoding may already be
		// unambiguous.
	case mode64:
		return nil, Errorf(page, "invalid mnemonic table: no 64-bit mode column (row %q)", headings)
	case mode32:
		return nil, Errorf(page, "invalid mnemonic table: no 32-bit mode column (row %q)", headings)
	case cpuid:
		// This field is optional.
	case description:
		return nil, Errorf(page, "invalid mnemonic table: no description column (row %q)", headings)
	}

	return headings, nil
}

func fixOperandEncodingTable(page int, table [][]string, stats *Stats) ([][]string, error) {
	// Normalise the heading.
	for i, col := range table[0] {
		switch col {
		case "Tuple":
			table[0][i] = "Tuple Type"

		// This is just a consequence of
		// the challenges of parsing a
		// PDF.
		case "Operand 3 1":
			table[0][i] = "Operand 3" // A mis-parsed superscript 1.
		case "Operand 1 SIB.base (r): Address of pointer":
			table[0][i] = "Operand 1"
		case "Operand 2 SIB.base (r): Address of pointer":
			table[0][i] = "Operand 2"
		}
	}

	// Add any missing tuple type.
	if (len(table[0]) == 1 && table[0][0] == "Op/En") ||
		(len(table[0]) >= 2 && table[0][0] == "Op/En" && table[0][1] == "Operand 1") {
		for i, row := range table {
			if i == 0 {
				table[i] = append([]string{"Op/En", "Tuple Type"}, row[1:]...)
			} else {
				table[i] = append([]string{row[0], "N/A"}, row[1:]...)
			}
		}
	}

	// Handle some odd cases.

	if stringSetEqual(table[0], []string{"Op/En", "Tuple Type", "Operand 1", "Operands 2—9"}) {
		// Some of the AES instructions have
		// lots of implicit arguments.
		for i, row := range table {
			if i == 0 {
				table[i] = append(row[:3], "Operand 2", "Operand 3", "Operand 4")
			} else {
				table[i] = append(row, "N/A", "N/A")
			}
		}

		return table, nil
	}

	if stringSetEqual(table[0], []string{"Op/En", "Tuple Type", "Operand 1", "Operand 2", "Operand 3", "Operand 4 RDX/EDX is implied 64/32 bits"}) {
		// The page for MULX is a little odd.
		for i, row := range table {
			if i == 0 {
				table[i][5] = "Operand 4"
			} else if len(row) == 6 && row[5] == " source" {
				table[i][5] = "Implicit EAX/RAX"
			}
		}

		return table, nil
	}

	if stringSetEqual(table[0], []string{"Op/En", "Tuple Type", "Operand 1", "Operand 2", "Operand 3", "Operands 4—5", "Operands 6—7"}) {
		// Some of the AES instructions have
		// lots of implicit arguments.
		for i, row := range table {
			if i == 0 {
				table[i] = append(row[:5], "Operand 4")
			} else if len(row) == 7 && stringSetEqual(row[4:], []string{"Implicit XMM0 (r, w)", "Implicit XMM1—2 (w)", "Implicit XMM4—6 (w)"}) {
				table[i] = append(row[:4], "N/A", "N/A")
			}
		}

		return table, nil
	}

	if stringSetEqual(table[0], []string{"Op/En", "Tuple Type", "Operand 1", "Operand 2", "Operands 3—4", "Operands 5—9"}) {
		// Some of the AES instructions have
		// lots of implicit arguments.
		for i, row := range table {
			if i == 0 {
				table[i] = append(row[:4], "Operand 3", "Operand 4")
			} else if len(row) == 6 && stringSetEqual(row[4:], []string{"Implicit XMM0—1 (r, w)", "Implicit XMM2—6 (w)"}) {
				table[i] = append(row[:4], "N/A", "N/A")
			}
		}

		return table, nil
	}

	if stringSetEqual(table[0], []string{"Op/En", "Tuple Type", "Operand 1", "Operand 2 BaseReg (R): VSIB:base,", "Operand 3", "Operand 4"}) {
		// VGATHERDPS/VGATHERDPD has too small
		// a gap between the table headings and
		// the VSIB entry in operand 2, so they
		// become merged. We split them here.
		table[0][3] = "Operand 2"

		return table, nil
	}

	if stringSetEqual(table[0], []string{"Op/En", "Tuple Type", "Operand 1 BaseReg (R): VSIB:base,", "Operand 2", "Operand 3", "Operand 4"}) {
		// VPSCATTERDD/VPSCATTERDQ/VPSCATTERQD/VPSCATTERQQ
		// has too small a gap between the table
		// headings and the VSIB entry in operand
		// 1, so they become merged. We split
		// them here.
		table[0][2] = "Operand 1"

		return table, nil
	}

	if stringSetEqual(table[0], []string{"Op/En", "Tuple Type", "Operand 1", "Operand2", "Operand3", "Operand4"}) {
		// XABORT is missing the spaces in the
		// last three operand headings.
		stats.ListingError("p.%d: Malformed instruction operand encoding table headings", page)
		table[0] = append(table[0][3:], "Operand 2", "Operand 3", "Operand 4")

		return table, nil
	}

	// Add any missing operands.

	operands := []string{"Operand 1", "Operand 2", "Operand 3", "Operand 4"}
	for i, operand := range operands {
		if len(table[0]) == 2+i {
			for j, row := range table {
				if j == 0 {
					table[j] = append(row, operand)
				} else {
					table[j] = append(row, "N/A")
				}
			}
		}
	}

	// Finally, check the heading.

	if !stringSetEqual(table[0], []string{"Op/En", "Tuple Type", "Operand 1", "Operand 2", "Operand 3", "Operand 4"}) {
		return nil, Errorf(page, "invalid instruction operand encoding table: malformed heading (%q)", table[0])
	}

	return table, nil
}
