// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"testing"

	"firefly-os.dev/tools/ruse/internal/x86"

	"github.com/google/go-cmp/cmp"
)

func TestParameterCombinations(t *testing.T) {
	tests := []struct {
		Name string
		Args []string
		Want [][]*x86.Parameter
	}{
		{
			Name: "none",
			Args: nil,
			Want: nil,
		},
		{
			Name: "simple",
			Args: []string{"AL", "r8"},
			Want: [][]*x86.Parameter{
				{x86.ParamAL, x86.ParamR8},
			},
		},
		{
			Name: "single split",
			Args: []string{"AL", "r/m8"},
			Want: [][]*x86.Parameter{
				{x86.ParamAL, x86.ParamRmr8},
				{x86.ParamAL, x86.ParamM8},
			},
		},
		{
			Name: "multi split",
			Args: []string{"AL", "r/m8", "xmm2/m128"},
			Want: [][]*x86.Parameter{
				{x86.ParamAL, x86.ParamRmr8, x86.ParamXMM2},
				{x86.ParamAL, x86.ParamRmr8, x86.ParamM128},
				{x86.ParamAL, x86.ParamM8, x86.ParamXMM2},
				{x86.ParamAL, x86.ParamM8, x86.ParamM128},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := ParameterCombinations(test.Args)
			if err != nil {
				t.Fatalf("ParameterCombinations(%q): got unexpected error: %v", test.Args, err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Fatalf("ParameterCombinations(%q): (-want, +got)\n%s", test.Args, diff)
			}
		})
	}
}
