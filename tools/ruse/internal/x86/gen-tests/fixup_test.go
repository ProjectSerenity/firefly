// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"testing"

	"firefly-os.dev/tools/ruse/internal/x86"
)

func TestFixupInstruction(t *testing.T) {
	tests := []struct {
		Name  string
		Entry *TestEntry
		Inst  instructionPair
		Want  string
	}{
		{
			Name: "adc correct",
			Entry: &TestEntry{
				Inst:  x86.ADC_Rmr8_R8,
				Intel: "adc al, bl",
				Code:  "10d8",
			},
			Inst: instructionPair{
				From: instruction{"", "12"},
				To:   instruction{"", "10"},
			},
			Want: "10d8",
		},
		{
			Name: "adc wrong",
			Entry: &TestEntry{
				Inst:  x86.ADC_Rmr8_R8,
				Intel: "adc al, bl",
				Code:  "12c3",
			},
			Inst: instructionPair{
				From: instruction{"", "12"},
				To:   instruction{"", "10"},
			},
			Want: "10d8",
		},
		{
			Name: "movaps correct",
			Entry: &TestEntry{
				Inst:  x86.MOVAPS_XMM1_XMM2,
				Intel: "movaps xmm3, xmm2",
				Code:  "0f28da",
			},
			Inst: instructionPair{
				From: instruction{"", "0f29"},
				To:   instruction{"", "0f28"},
			},
			Want: "0f28da",
		},
		{
			Name: "movaps wrong",
			Entry: &TestEntry{
				Inst:  x86.MOVAPS_XMM1_XMM2,
				Intel: "movaps xmm3, xmm2",
				Code:  "0f29d3",
			},
			Inst: instructionPair{
				From: instruction{"", "0f29"},
				To:   instruction{"", "0f28"},
			},
			Want: "0f28da",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			orig := test.Entry.Code
			fixupInstruction(test.Entry, test.Inst)
			if test.Entry.Code != test.Want {
				t.Fatalf("fixupInstruction(%q, %#v):\n  Got:  %s\n  Want: %s", orig, test.Inst, test.Entry.Code, test.Want)
			}

			test.Entry.Code = orig
			FixupEntry(test.Entry)
			if test.Entry.Code != test.Want {
				t.Fatalf("Fixup(%q):\n  Got:  %s\n  Want: %s", orig, test.Entry.Code, test.Want)
			}
		})
	}
}
