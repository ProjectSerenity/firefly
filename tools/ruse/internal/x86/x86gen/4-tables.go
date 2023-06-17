// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Take a set of mnemonnic and operand encoding
// table text and extract them into table data.

package main

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"rsc.io/pdf"
)

func halfMissing(x []string) bool {
	n := 0
	for _, s := range x {
		if s == "" {
			n++
		}
	}

	return n >= len(x)/2
}

// ExtractMnemonicTable processes the texts containing
// a mnemonic table and return the text as a series of
// rows.
func ExtractMnemonicTable(page Page, stats *Stats, debug bool) (table *Table, err error) {
	// Trim any trailing notes.
	sort.Sort(pdf.TextVertical(page.Text))
	for i := len(page.Text) - 1; i >= 0; i-- {
		if matchExact(page.Text[i], "NeoSansIntelMedium", 7.98, "1") {
			// This is a misparsed superscript 1
			// after "Instruction Operand Encoding".
			if debug {
				fmt.Println("Trimming a trailing superscript 1 from mnemonic table.")
			}

			page.Text = page.Text[:i]
			continue
		}

		if matchExact(page.Text[i], "NeoSansIntelMedium", 9, "NOTES:") || matchExact(page.Text[i], "NeoSansIntel", 9, "NOTES:") {
			if debug {
				fmt.Printf("Trimming %d lines of trailing notes from mnemonic table.\n", len(page.Text)-i)
			}

			page.Text = page.Text[:i]
			break
		}

		if (page.Text[i].Font != "NeoSansIntel" && page.Text[i].Font != "Verdana") || page.Text[i].FontSize > 9 {
			// Probably different text.
			break
		}
	}

	sort.Sort(pdf.TextHorizontal(page.Text))

	const nudge = 1

	old := -100000.0
	var col []float64
	for i, t := range page.Text {
		if t.Font != "NeoSansIntelMedium" || t.FontSize < 8 { // Only headings count, ignore supertext.
			continue
		}
		if t.X != old && math.Abs(old-t.X) < nudge {
			page.Text[i].X = old
		} else if t.X != old {
			old = t.X
			col = append(col, old)
		}
	}

	if len(col) == 0 {
		return nil, nil
	}

	sort.Sort(pdf.TextVertical(page.Text))

	y := -100000.0
	var rows [][]string
	var line []string
	bold := -1
	for _, t := range page.Text {
		if t.FontSize < 8 {
			// This is superscript, which we
			// ignore.
			continue
		}

		if t.Y != y {
			rows = append(rows, make([]string, len(col)))
			line = rows[len(rows)-1]
			y = t.Y
			if t.Font == "NeoSansIntelMedium" {
				bold = len(rows) - 1
			}
		}

		i := 0
		for i+1 < len(col) && col[i+1] <= t.X+nudge {
			i++
		}

		if line[i] != "" {
			line[i] += " "
		}

		line[i] += t.S
	}

	table = &Table{Page: page.Page}
	for i, t := range rows {
		if 0 < i && i <= bold || bold < i && halfMissing(t) {
			// merge with earlier line
			last := table.Rows[len(table.Rows)-1]
			for j, s := range t {
				if s != "" {
					last[j] += "\n" + s
				}
			}
		} else {
			table.Rows = append(table.Rows, t)
		}
	}

	if bold >= 0 {
		var err error
		table.Rows[0], err = fixMnemonicHeadings(page.Page, table.Rows[0], stats)
		if err != nil {
			return nil, err
		}
	}

	return table, nil
}

// ExtractEncodingTable processes the texts containing
// an encoding table and return the text as a series of
// rows.
func ExtractEncodingTable(page Page, stats *Stats, debug bool) (table *Table, err error) {
	sort.Sort(pdf.TextVertical(page.Text))
	var col []float64
	center := func(t pdf.Text) float64 {
		return t.X + t.W/2
	}

	for _, t := range page.Text {
		if matchContains(t, "NeoSansIntel", 9, "Op/En") || matchContains(t, "NeoSansIntel", 9, "Operand") ||
			matchContains(t, "NeoSansIntelMedium", 9, "Op/En") || matchContains(t, "NeoSansIntelMedium", 9, "Operand") ||
			matchExact(t, "NeoSansIntel", 9, "Tuple") || matchExact(t, "NeoSansIntel", 9, "Tuple Type") ||
			matchExact(t, "NeoSansIntelMedium", 9, "Tuple") || matchExact(t, "NeoSansIntelMedium", 9, "Tuple Type") {
			if debug {
				fmt.Printf("column %d at %.2f: %v\n", len(col), center(t), t)
			}

			col = append(col, center(t))
		}
	}

	if len(col) == 0 {
		return nil, nil
	}

	const nudge = 35

	y := -100000.0
	var rows [][]string
	var line []string
	for _, t := range page.Text {
		if t.Y != y {
			rows = append(rows, make([]string, len(col)))
			line = rows[len(rows)-1]
			y = t.Y
		}

		i := 0
		x := center(t)
		for i+1 < len(col) && col[i+1] <= x+nudge {
			i++
		}

		if debug {
			fmt.Printf("text at %.2f: %v => %d\n", x, t, i)
		}

		if line[i] == "ModRM:r/m (r)" && t.S == "VEX.vvvv (r) /" {
			stats.InstructionError()
			continue // Ambiguous table for VMOVHLPS.
		}

		if line[i] != "" {
			line[i] += " "
		}

		t.S = strings.Replace(t.S, ", ModRM:[7:6] must be 11b", "", 1)
		t.S = strings.Replace(t.S, ", ModRM:[7:6] must not be 11b", "", 1)
		if prefix, suffix, found := strings.Cut(t.S, "ModRM:rm"); found {
			stats.InstructionError()
			t.S = prefix + "ModRM:r/m" + suffix
		}

		line[i] += t.S
	}

	table = &Table{Page: page.Page, Rows: rows[:0]}
	for _, line := range rows {
		if line[0] == "" && len(table.Rows) > 0 {
			last := table.Rows[len(table.Rows)-1]
			for i, col := range line {
				if col != "" && col != "VEX.vvvv (r) /" { // Ambiguous table for VMOVHLPS.
					last[i] += " " + col
				}
			}

			continue
		}

		table.Rows = append(table.Rows, line)
	}

	return table, nil
}
