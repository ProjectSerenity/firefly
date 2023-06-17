// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Split a set of pages of an instruction into page
// structures for the mnemonic and operand encoding
// tables. Any other text is dropped.

package main

import (
	"sort"

	"rsc.io/pdf"
)

// SplitInstructionPages takes the set of text for an
// instruction and returns three sets of text. The
// first is any mnemonic tables, the second is any
// sets of operand encoding tables, and the third is
// all subsequent text.
func SplitInstructionPages(pages []Page, stats *Stats, debug bool) (mnemonicTables, encodingTables []Page) {
	// The mnemonic table starts straight
	// away, but if we fail to find any
	// other sections, we assume this is
	// all `rest`.
	mnemonics := true
	encodings := false
	for _, page := range pages {
		// Make a copy so we can sort it
		// vertically without affecting
		// the original.
		page.Text = append([]pdf.Text(nil), page.Text...)
		sort.Sort(pdf.TextVertical(page.Text))

	scanPage:
		for i, t := range page.Text {
			switch {
			case mnemonics:
				// "Instruction Operand Encoding" marks
				// the start of the operand encoding
				// table. If there is no encoding table,
				// we instead look for "Description",
				// which should be the next section
				// after the encoding table.
				if matchContains(t, "NeoSansIntelMedium", 10, "Instruction Operand Encoding") {
					if i > 0 {
						prev := Page{Page: page.Page, Text: page.Text[:i]}
						mnemonicTables = append(mnemonicTables, prev)
					}

					page.Text = page.Text[i+1:] // Skip over the encoding heading.
					mnemonics = false
					encodings = true
					goto scanPage
				} else if matchContains(t, "NeoSansIntelMedium", 10, "Description") || matchContains(t, "NeoSansIntelMedium", 9, "Description") {
					// We may also see "Description" in the
					// mnemonic table header row. If this
					// is within 20 pixels of the top-most
					// element, we assume it's in the heading
					// and continue.
					if page.Text[0].Y-t.Y <= 20 {
						continue
					}

					if i > 0 {
						prev := Page{Page: page.Page, Text: page.Text[:i]}
						mnemonicTables = append(mnemonicTables, prev)
					}

					return mnemonicTables, nil
				}
			case encodings:
				// Now we just look for the "Description"
				// after the encoding table.
				if matchExact(t, "Verdana", 9, "TESTUI copies the current value of the user interrupt flag (UIF) into EFLAGS.CF. This instruction can be executed") {
					// The "Description" heading is missing from TESTUI.
					stats.ListingError()
					if i > 0 {
						prev := Page{Page: page.Page, Text: page.Text[:i]}
						encodingTables = append(encodingTables, prev)
					}

					return mnemonicTables, encodingTables
				}

				if matchContains(t, "NeoSansIntelMedium", 10, "Description") || matchContains(t, "NeoSansIntelMedium", 9, "Description") {
					if i > 0 {
						prev := Page{Page: page.Page, Text: page.Text[:i]}
						encodingTables = append(encodingTables, prev)
					}

					return mnemonicTables, encodingTables
				}
			}
		}

		// Now we just append the
		// (remaining) text to the
		// relevant set of text.
		switch {
		case mnemonics:
			mnemonicTables = append(mnemonicTables, page)
		case encodings:
			encodingTables = append(encodingTables, page)
		}
	}

	if mnemonics {
		// We still haven't seen any
		// other sections, so this is
		// probably not a mnemonic table.
		return nil, nil
	}

	return mnemonicTables, encodingTables
}
