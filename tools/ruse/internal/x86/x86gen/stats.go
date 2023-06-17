// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"strconv"
	"time"
)

var ignoredStats Stats

type Stats struct {
	t0 time.Time

	FirstPage             int
	LastPage              int
	Listings              int
	ListingErrors         int
	Instructions          int
	InstructionErrors     int
	InstructionForms      int
	ExtraInstructions     int
	ExtraInstructionForms int
	TotalInstructions     int
	TotalInstructionForms int
}

func (s *Stats) notnil() *Stats {
	if s != nil {
		return s
	}

	return &ignoredStats
}

func (s *Stats) Start()                { s.notnil().t0 = time.Now() }
func (s *Stats) Listing()              { s.notnil().Listings++ }
func (s *Stats) ListingError()         { s.notnil().ListingErrors++ }
func (s *Stats) InstructionError()     { s.notnil().InstructionErrors++ }
func (s *Stats) InstructionForm()      { s.notnil().InstructionForms++ }
func (s *Stats) ExtraInstruction()     { s.notnil().ExtraInstructions++ }
func (s *Stats) ExtraInstructionForm() { s.notnil().ExtraInstructionForms++ }

func (s *Stats) Instruction(page int) {
	s = s.notnil()
	s.Instructions++
	if s.FirstPage == 0 {
		s.FirstPage = page
	}

	if s.LastPage < page {
		s.LastPage = page
	}
}

func (s *Stats) String() string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "Found %s instruction listings on pages %s - %s.\n", humaniseNumber(s.Listings), humaniseNumber(s.FirstPage), humaniseNumber(s.LastPage))
	fmt.Fprintf(&b, "Found %s instructions.\n", humaniseNumber(s.Instructions))
	fmt.Fprintf(&b, "Found %s unique instruction forms.\n", humaniseNumber(s.InstructionForms))
	fmt.Fprintf(&b, "Added %s instructions.\n", humaniseNumber(s.ExtraInstructions))
	fmt.Fprintf(&b, "Added %s unique instruction forms.\n", humaniseNumber(s.ExtraInstructionForms))
	fmt.Fprintf(&b, "Found %s total instructions.\n", humaniseNumber(s.Instructions+s.ExtraInstructions))
	fmt.Fprintf(&b, "Found %s total unique instruction forms.\n", humaniseNumber(s.InstructionForms+s.ExtraInstructionForms))
	fmt.Fprintf(&b, "Fixed %s instruction listing errors.\n", humaniseNumber(s.ListingErrors))
	fmt.Fprintf(&b, "Fixed %s instruction errors.\n", humaniseNumber(s.InstructionErrors))
	fmt.Fprintf(&b, "Fixed %s total errors.\n", humaniseNumber(s.ListingErrors+s.InstructionErrors))
	if !s.t0.IsZero() {
		fmt.Fprintf(&b, "Runtime: %s.\n", time.Since(s.t0).Round(time.Second))
	}

	return b.String()
}

func humaniseNumber(v int) string {
	prefix, suffix := strconv.Itoa(v), ""
	for len(prefix) > 3 {
		suffix = "," + prefix[len(prefix)-3:] + suffix
		prefix = prefix[:len(prefix)-3]
	}

	return prefix + suffix
}
