// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseInstruction(t *testing.T) {
	tests := []struct {
		Name string
		File string
		Want []*Instruction
	}{
		{
			Name: "ABS",
			File: "abs",
			Want: []*Instruction{
				{
					Family:   "ABS",
					Mnemonic: "abs",
					Form:     "ABS_32_dp_1src",
					Encoding: &Encoding{
						Bits: 0b0101_1010_1100_0000_0010_0000_0000_0000,
						Args: []Arg{
							{Name: "Wd", Syntax: "W", Index: &Variable{Name: "Rd", Width: 5, Shift: 0}},
							{Name: "Wn", Syntax: "W", Index: &Variable{Name: "Rn", Width: 5, Shift: 5}},
						},
					},
				},
				{
					Family:   "ABS",
					Mnemonic: "abs",
					Form:     "ABS_64_dp_1src",
					Encoding: &Encoding{
						Bits: 0b1101_1010_1100_0000_0010_0000_0000_0000,
						Args: []Arg{
							{Name: "Xd", Syntax: "X", Index: &Variable{Name: "Rd", Width: 5, Shift: 0}},
							{Name: "Xn", Syntax: "X", Index: &Variable{Name: "Rn", Width: 5, Shift: 5}},
						},
					},
				},
			},
		},
		{
			Name: "AUTIA",
			File: "autia",
			Want: []*Instruction{
				{
					Family:   "AUTIA",
					Mnemonic: "autia",
					Form:     "AUTIA_64P_dp_1src",
					Encoding: &Encoding{
						Bits: 0b1101_1010_1100_0001_0001_0000_0000_0000,
						Args: []Arg{
							{Name: "Xd", Syntax: "X", Index: &Variable{Name: "Rd", Width: 5, Shift: 0}},
							{Name: "Xn|SP", Syntax: "X_SP", Index: &Variable{Name: "Rn", Width: 5, Shift: 5}},
						},
					},
				},
				{
					Family:   "AUTIA",
					Mnemonic: "autiza",
					Form:     "AUTIZA_64Z_dp_1src",
					Encoding: &Encoding{
						Bits: 0b1101_1010_1100_0001_0011_0011_1110_0000,
						Args: []Arg{
							{Name: "Xd", Syntax: "X", Index: &Variable{Name: "Rd", Width: 5, Shift: 0}},
						},
					},
				},
				{
					Family:   "AUTIA",
					Mnemonic: "autia1716",
					Form:     "AUTIA1716_HI_hints",
					Encoding: &Encoding{
						Bits: 0b1101_0101_0000_0011_0010_0001_1001_1111,
					},
				},
				{
					Family:   "AUTIA",
					Mnemonic: "autiasp",
					Form:     "AUTIASP_HI_hints",
					Encoding: &Encoding{
						Bits: 0b1101_0101_0000_0011_0010_0011_1011_1111,
					},
				},
				{
					Family:   "AUTIA",
					Mnemonic: "autiaz",
					Form:     "AUTIAZ_HI_hints",
					Encoding: &Encoding{
						Bits: 0b1101_0101_0000_0011_0010_0011_1001_1111,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			name := filepath.Join("testdata", test.File+".xml")
			f, err := os.Open(name)
			if err != nil {
				t.Fatalf("failed to open %s: %v", name, err)
			}

			defer f.Close()

			got, err := ParseInstruction(f, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Fatalf("ParseInstruction(): (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestParseTemplate(t *testing.T) {
	tests := []struct {
		Name     string
		Text     []string
		Mnemonic string
		Args     []Arg
	}{
		{
			Name: "AUTIAZ",
			Text: []string{
				`<text>AUTIAZ</text>`,
			},
			Mnemonic: "AUTIAZ",
		},
		{
			Name: "ABS",
			Text: []string{
				`<text>ABS  </text>`,
				`<a link="sa_wd" hover="32-bit general-purpose destination register (field &quot;Rd&quot;)">&lt;Wd&gt;</a>`,
				`<text>, </text>`,
				`<a link="sa_wn" hover="32-bit general-purpose source register (field &quot;Rn&quot;)">&lt;Wn&gt;</a>`,
			},
			Mnemonic: "ABS",
			Args: []Arg{
				{Name: "Wd", Syntax: "W", Index: &Variable{Name: "Rd"}},
				{Name: "Wn", Syntax: "W", Index: &Variable{Name: "Rn"}},
			},
		},
		{
			Name: "ABS (SIMD)",
			Text: []string{
				`<text>ABS  </text>`,
				`<a link="sa_v" hover="Width specifier (field &quot;size&quot;) [D]">&lt;V&gt;</a>`,
				`<a link="sa_d" hover="SIMD&amp;FP destination register number (field &quot;Rd&quot;)">&lt;d&gt;</a>`,
				`<text>, </text>`,
				`<a link="sa_v" hover="Width specifier (field &quot;size&quot;) [D]">&lt;V&gt;</a>`,
				`<a link="sa_n" hover="SIMD&amp;FP source register number (field &quot;Rn&quot;)">&lt;n&gt;</a>`,
			},
			Mnemonic: "ABS",
			Args: []Arg{
				{Name: "Vd", Syntax: "V", Size: &Variable{Name: "size"}, Index: &Variable{Name: "Rd"}},
				{Name: "Vn", Syntax: "V", Size: &Variable{Name: "size"}, Index: &Variable{Name: "Rn"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			text := strings.Join(test.Text, "")
			mnemonic, args, err := parseTemplate(text)
			if err != nil {
				t.Fatalf("failed to parse template %q: %v", text, err)
			}

			if mnemonic != test.Mnemonic {
				t.Errorf("parseTemplate(%q):\nGot mnemonic  %q\nWant mnemonic %q", text, mnemonic, test.Mnemonic)
			}

			if diff := cmp.Diff(test.Args, args); diff != "" {
				t.Fatalf("parseTemplate(%q): (-want, +got)\n%s", text, diff)
			}
		})
	}
}
