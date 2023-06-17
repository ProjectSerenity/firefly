// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Take the Intel x86 manual and extract the list
// of instruction names in the outline and the
// document ID and date from page 1.

package main

import (
	"bytes"
	"regexp"

	"rsc.io/pdf"
)

var (
	idRegex   = regexp.MustCompile(`Order Number: ([\w-\-]+)`)
	dateRegex = regexp.MustCompile(`\b(January|February|March|April|May|June|July|August|September|October|November|December) ((19|20)[0-9][0-9])\b`)
)

// ParseMetadata parses the PDF and returns the
// document metadata.
func ParseMetadata(r *pdf.Reader) (id, date string, err error) {
	p := r.Page(1)
	text := pageContent(p)

	var length int
	for _, t := range text {
		length += len(t.S) + 1
	}

	var buf bytes.Buffer
	buf.Grow(length)
	for _, t := range text {
		buf.WriteString(t.S)
		buf.WriteByte('\n')
	}

	all := buf.String()
	m := idRegex.FindStringSubmatch(all)
	if m == nil {
		return "", "", Errorf(1, "failed to find document ID on page 1:\n%s", all)
	}

	id = m[1]

	date = dateRegex.FindString(all)
	if date == "" {
		return "", "", Errorf(1, "failed to find publishing date on page 1:\n%s", all)
	}

	return id, date, nil
}

// ParseOutline returns the list of instruction headings
// from the table of contents.
//
// This is used later to check that we found an instruction
// definition for every heading.
func ParseOutline(outline pdf.Outline) []string {
	return appendOutline(nil, outline)
}

var (
	instructionRegex = regexp.MustCompile(`\d Instructions \([A-Z](-[A-Z])?\)|VMX Instructions|Instruction SET Reference|Instruction Set Reference|SHA Extensions Reference`)
	headingRegex     = regexp.MustCompile(`^\d+\.\d+ `)
)

// appendOutline works recursively to parse the table
// of contents to identify instruction headings.
func appendOutline(list []string, outline pdf.Outline) []string {
	if instructionRegex.MatchString(outline.Title) {
		for _, child := range outline.Child {
			if !headingRegex.MatchString(child.Title) {
				list = append(list, fixDash.Replace(child.Title))
			}
		}
	}

	for _, child := range outline.Child {
		list = appendOutline(list, child)
	}

	return list
}
