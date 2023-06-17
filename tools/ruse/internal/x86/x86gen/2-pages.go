// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Take the Intel x86 manual PDF and split it
// into separate pages of text.

package main

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"rsc.io/pdf"
)

func matchContains(t pdf.Text, font string, size float64, substr string) bool {
	return t.Font == font && math.Abs(t.FontSize-size) < 0.1 && strings.Contains(t.S, substr)
}

func matchExact(t pdf.Text, font string, size float64, substr string) bool {
	return t.Font == font && math.Abs(t.FontSize-size) < 0.1 && t.S == substr
}

func sameFont(f1, f2 string) bool {
	f1 = strings.TrimSuffix(f1, ",Italic")
	f1 = strings.TrimSuffix(f1, "-Italic")
	f2 = strings.TrimSuffix(f1, ",Italic")
	f2 = strings.TrimSuffix(f1, "-Italic")

	return strings.TrimSuffix(f1, ",Italic") == strings.TrimSuffix(f2, ",Italic") ||
		f1 == "Symbol" || f2 == "Symbol" ||
		f1 == "TimesNewRoman" || f2 == "TimesNewRoman"
}

// ReadInstructionPages returns the set of text
// for an instruction, starting at the given
// page and continuing to (but not including)
// the page where the next instruction begins.
//
// The page where the next instruction begins
// is also returned.
func ReadInstructionPages(r *pdf.Reader, page int, debug bool) (name string, pages []Page, nextPage int) {
	p := r.Page(page)
	page++
	text := pageContent(p)
	if debug {
		fmt.Println("DEBUG first page", page-1)
	}

	// We expect the page to have the text
	// "INSTRUCTION SET REFERENCE {range}"
	// or "{feature} EXTENSIONS" as the
	// section heading.
	//
	// If not, this is probably not an
	// instruction page, so we just return
	// the next page number.
	badPage := func(text []pdf.Text) bool {
		if len(text) == 0 {
			return true
		}

		if (!matchContains(text[0], "NeoSansIntel", 9, "INSTRUCTION") || !matchContains(text[0], "NeoSansIntel", 9, "REFERENCE")) &&
			!matchContains(text[0], "NeoSansIntel", 9, "EXTENSIONS") {
			return true
		}

		return false
	}

	if badPage(text) {
		return "", nil, page
	}

	// Skip over the section heading.
	text = text[1:]

	// Each instruction should start with
	// the instruction name as a heading.
	// We treat these as the boundary
	// between instructions.
	instructionName := func(text []pdf.Text) (name string, rest []pdf.Text) {
		// F2XM1 has a superscript 'x' in
		// its description, which confuses
		// title parsing, so we handle that
		// here.
		if len(text) >= 2 && matchExact(text[0], "NeoSansIntelMedium", 9.6, "x") && matchExact(text[1], "NeoSansIntelMedium", 12, "F2XM1—Compute 2 –1") {
			text[1].S = "F2XM1—Compute 2^x-1"
			text = text[1:]
		}

		isInstHeadline := func(s string) bool {
			return strings.Contains(s, "—") ||
				strings.Contains(s, " - ") ||
				strings.Contains(s, "PTEST- Logical Compare")
		}

		if len(text) == 0 || !matchContains(text[0], "NeoSansIntelMedium", 12, "") || !isInstHeadline(text[0].S) {
			return "", text
		}

		name = text[0].S
		text = text[1:]
		for len(text) > 0 && matchContains(text[0], "NeoSansIntelMedium", 12, "") {
			name += " " + text[0].S
			text = text[1:]
		}

		name = fixDash.Replace(name)
		name = strings.Replace(name, "SparsePrefetch", "Sparse Prefetch", 1) // The name is too long and gets compressed.

		return name, text
	}

	// Check that this is indeed an
	// instruction listing.
	name, text = instructionName(text)
	if name == "" {
		// We are either part-way through another
		// instruction, or on a page in between
		// instructions. To save repeated effort
		// later, we just advance to the next
		// instruction.
		numPages := r.NumPage()
		for page < numPages {
			p = r.Page(page)
			page++
			text := pageContent(p)
			if badPage(text) {
				return "", nil, page
			}

			text = text[1:]
			other, text := instructionName(text)
			if other != "" {
				// Backtrack to the previous page,
				// as we're now pointing to the
				// page after where this instruction
				// starts.
				page--
				return "", nil, page
			}
		}

		return "", nil, page
	}

	// Each page should have a footer
	// with the instruction name, the
	// section-page reference, and the
	// document name.
	//
	// The three are on the same line,
	// but their horizontal position
	// flips on alternate pages.
	//
	// The section-page is hard to
	// predict, but if we see "Vol"
	// and the instruction name in
	// the final three texts, both
	// with the same Y position, we
	// trim all trailing texts with
	// the same Y.
	trimTrailer := func(name string, text []pdf.Text) []pdf.Text {
		prefix, _, _ := strings.Cut(name, "-")
		last3 := text
		if len(last3) > 3 {
			last3 = last3[len(last3)-3:]
		}

		var y float64
		var foundVol, foundName bool
		for _, t := range last3 {
			if matchContains(t, "NeoSansIntel", 8, "Vol") {
				foundVol = true
			} else if matchContains(t, "NeoSansIntel", 8, prefix) {
				foundName = true
			} else {
				continue
			}

			if y == 0 {
				y = t.Y
			} else if y != t.Y {
				// There's a mismatch. Default
				// to leaving all text in place.
				return text
			}
		}

		if !foundVol || !foundName || y == 0 {
			return text
		}

		for i, t := range text {
			if t.Y == y {
				// Stop here.
				return text[:i]
			}
		}

		return text
	}

	text = trimTrailer(name, text)
	pages = append(pages, Page{Page: page - 1, Text: text})
	if debug {
		fmt.Printf("Name: %s\n", name)
		for _, t := range text {
			fmt.Printf("%v\n", t)
		}
	}

	// Now we just keep advancing until
	// we reach the page with the next
	// instruction, or until we reach
	// the end of the doc.
	numPages := r.NumPage()
	for page < numPages {
		p = r.Page(page)
		page++
		text := pageContent(p)
		if badPage(text) {
			return name, pages, page
		}

		text = text[1:]
		other, text := instructionName(text)
		if other != "" {
			// Backtrack to the previous page,
			// as we're now pointing to the
			// page after where this instruction
			// starts.
			page--
			return name, pages, page
		}

		text = trimTrailer(name, text)
		pages = append(pages, Page{Page: page - 1, Text: text})
		if debug {
			fmt.Println("DEBUG later page", page-1)
			for _, t := range text {
				fmt.Printf("%v\n", t)
			}
		}
	}

	return name, pages, page
}

func findPhrases(chars []pdf.Text) (phrases []pdf.Text) {
	// Sort by Y coordinate and normalize.
	const nudge = 1
	sort.Sort(pdf.TextVertical(chars))
	old := -100000.0
	for i, c := range chars {
		if c.Y != old && math.Abs(old-c.Y) < nudge {
			chars[i].Y = old
		} else {
			old = c.Y
		}
	}

	// Sort by Y coordinate, breaking ties with X.
	// This will bring letters in a single word together.
	sort.Sort(pdf.TextVertical(chars))

	// Loop over chars.
	for i := 0; i < len(chars); {
		// Find all chars on line.
		j := i + 1
		for j < len(chars) && chars[j].Y == chars[i].Y {
			j++
		}

		var end float64
		// Split line into words (really, phrases).
		for k := i; k < j; {
			ck := &chars[k]
			s := ck.S
			end = ck.X + ck.W
			charSpace := ck.FontSize / 6
			wordSpace := ck.FontSize * 2 / 3
			l := k + 1
			for l < j {
				// Grow word.
				cl := &chars[l]
				if sameFont(cl.Font, ck.Font) && cl.FontSize == ck.FontSize && cl.X <= end+charSpace {
					s += cl.S
					end = cl.X + cl.W
					l++
					continue
				}

				// Add space to phrase before next word.
				if sameFont(cl.Font, ck.Font) && cl.FontSize == ck.FontSize && cl.X <= end+wordSpace {
					s += " " + cl.S
					end = cl.X + cl.W
					l++
					continue
				}

				break
			}

			f := ck.Font
			f = strings.TrimSuffix(f, ",Italic")
			f = strings.TrimSuffix(f, "-Italic")
			phrases = append(phrases, pdf.Text{Font: f, FontSize: ck.FontSize, X: ck.X, Y: ck.Y, W: end, S: s})
			k = l
		}

		i = j
	}

	return phrases
}

// pageContent pre-processes the page, returning its
// simplified content.
func pageContent(p pdf.Page) (text []pdf.Text) {
	content := p.Content()
	for i, t := range content.Text {
		if matchContains(t, "Symbol", 11, "≠") {
			t.Font = "NeoSansIntel"
			t.FontSize = 9
			content.Text[i] = t
		}

		if t.S == "*" ||
			t.S == "**" ||
			t.S == "***" ||
			(t.S == "," && t.Font == "Arial" && t.FontSize < 9) ||
			(t.S == "1" && t.Font == "Arial") {
			t.Font = "NeoSansIntel"
			t.FontSize = 9
			if i+1 < len(content.Text) {
				t.Y = content.Text[i+1].Y
			}
			content.Text[i] = t
		}
	}

	text = findPhrases(content.Text)
	for i, t := range text {
		if matchContains(t, "NeoSansIntel", 8, ".WIG") || matchContains(t, "NeoSansIntel", 8, "AVX2") {
			t.FontSize = 9
			text[i] = t
		}
		if t.Font == "NeoSansIntel-Medium" {
			t.Font = "NeoSansIntelMedium"
			text[i] = t
		}
		if t.Font == "NeoSansIntel-Italic" {
			t.Font = "NeoSansIntel,Italic"
			text[i] = t
		}
	}

	return text
}
