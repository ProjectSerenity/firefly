// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package sys

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"firefly-os.dev/tools/ruse/internal/x86"
)

func TestDefaultABIs(t *testing.T) {
	arches := []*Arch{X86, X86_64}
	for _, arch := range arches {
		t.Run(arch.Name, func(t *testing.T) {
			err := arch.Validate(&arch.DefaultABI)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestABIs(t *testing.T) {
	tests := []struct {
		Name       string
		Arch       *Arch
		ABI        *ABI
		Params     []int
		Result     int
		WantParams [][]Location
		WantResult []Location
	}{
		{
			Name:   "x86 System V",
			Arch:   X86,
			ABI:    nil,
			Params: []int{1, 8, 4},
			Result: 2,
			WantParams: [][]Location{
				{Stack{Pointer: x86.ESP, Offset: +0}},
				{Stack{Pointer: x86.ESP, Offset: +4}, Stack{Pointer: x86.ESP, Offset: +8}},
				{Stack{Pointer: x86.ESP, Offset: +12}},
			},
			WantResult: []Location{
				x86.EAX,
			},
		},
		{
			Name:   "x86-64 System V",
			Arch:   X86_64,
			ABI:    nil,
			Params: []int{1, 16, 8, 4, 4, 4, 4},
			Result: 4,
			WantParams: [][]Location{
				{x86.RDI},
				{x86.RSI, x86.RDX},
				{x86.RCX},
				{x86.R8},
				{x86.R9},
				{Stack{Pointer: x86.RSP, Offset: +0}},
				{Stack{Pointer: x86.RSP, Offset: +8}},
			},
			WantResult: []Location{
				x86.RAX,
			},
		},
		{
			Name: "x86-64 inverted stack",
			Arch: X86_64,
			ABI: &ABI{
				InvertedStack: true,
			},
			Params: []int{1, 16, 8},
			Result: 16,
			WantParams: [][]Location{
				{Stack{Pointer: x86.RSP, Offset: +24}},
				{Stack{Pointer: x86.RSP, Offset: +16}, Stack{Pointer: x86.RSP, Offset: +8}},
				{Stack{Pointer: x86.RSP, Offset: +0}},
			},
			WantResult: []Location{
				Stack{Pointer: x86.RSP, Offset: +8},
				Stack{Pointer: x86.RSP, Offset: +0},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			// Check that the parameters
			// match.
			params := test.Arch.Parameters(test.ABI, test.Params)
			if diff := cmp.Diff(test.WantParams, params); diff != "" {
				t.Fatalf("Parameters(): (-want, +got)\n%s", diff)
			}

			// Do the same again to make
			// sure the implementation
			// does not mutate the arch
			// or ABI.
			params = test.Arch.Parameters(test.ABI, test.Params)
			if diff := cmp.Diff(test.WantParams, params); diff != "" {
				t.Fatalf("repeated Parameters(): (-want, +got)\n%s", diff)
			}

			// Check that the results
			// match.
			result := test.Arch.Result(test.ABI, test.Result)
			if diff := cmp.Diff(test.WantResult, result); diff != "" {
				t.Fatalf("Result(): (-want, +got)\n%s", diff)
			}

			// Do the same again to make
			// sure the implementation
			// does not mutate the arch
			// or ABI.
			result = test.Arch.Result(test.ABI, test.Result)
			if diff := cmp.Diff(test.WantResult, result); diff != "" {
				t.Fatalf("repeated Result(): (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestIsABIRegister(t *testing.T) {
	type Reg struct {
		Register Location
		Want     bool
	}

	tests := []struct {
		Arch      *Arch
		Registers []Reg
	}{
		{
			Arch: X86,
			Registers: []Reg{
				{x86.AL, true},
				{x86.R8L, false},
				{x86.AX, true},
				{x86.EAX, true},
				{x86.ESP, false},
			},
		},
		{
			Arch: X86_64,
			Registers: []Reg{
				{x86.AL, true},
				{x86.R8L, true},
				{x86.AX, true},
				{x86.EAX, true},
				{x86.ESP, false},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Arch.Name, func(t *testing.T) {
			for _, reg := range test.Registers {
				t.Run(reg.Register.String(), func(t *testing.T) {
					got := test.Arch.IsABIRegister(reg.Register)
					if got != reg.Want {
						t.Fatalf("%s.IsABIRegister(%s): got %v, want %v", test.Arch, reg.Register, got, reg.Want)
					}
				})
			}
		})
	}
}
