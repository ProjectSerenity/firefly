// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"firefly-os.dev/tools/ruse/internal/x86"
)

func TestGenerateTestEntries(t *testing.T) {
	tests := []struct {
		Name  string
		Insts []*x86.Instruction
		Want  []*TestEntry
	}{
		{
			Name: "solo",
			Insts: []*x86.Instruction{
				// This is simple, as it only has one form.
				x86.POP_ES,
			},
			Want: []*TestEntry{
				{
					Inst:  x86.POP_ES,
					Code:  "07",
					Mode:  x86.Mode16,
					Ruse:  "(pop es)",
					Intel: "pop es",
				},
				{
					Inst:  x86.POP_ES,
					Code:  "07",
					Mode:  x86.Mode32,
					Ruse:  "(pop es)",
					Intel: "pop es",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := GenerateTestEntries(test.Insts)
			if err != nil {
				t.Fatalf("failed to generate tests: %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Fatalf("GenerateTestEntries(): (-want, +got)\n%s", diff)
			}
		})
	}
}
