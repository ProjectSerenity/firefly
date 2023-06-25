// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"testing"

	"firefly-os.dev/tools/ruse/internal/x86"
)

func TestAreEquivalent(t *testing.T) {
	tests := []struct {
		Name string
		A    *TestEntry
		B    *TestEntry
		Want bool
	}{
		{
			Name: "different",
			A: &TestEntry{
				Inst:  x86.MOV_R32_M32,
				Intel: "mov r8d, dword ptr [rcx]",
			},
			B: &TestEntry{
				Inst:  x86.ADD_M32_Imm32,
				Intel: "add [rcx], 0x1234",
			},
			Want: false,
		},
		{
			Name: "specialisation match",
			A: &TestEntry{
				Inst:  x86.CMPPD_XMM1_M128_Imm5u,
				Intel: "cmppd xmm3, xmmword ptr [rcx], 0",
			},
			B: &TestEntry{
				Inst:  x86.CMPEQPD_XMM1_M128,
				Intel: "cmpeqpd xmm3, xmmword ptr [rcx]",
			},
			Want: true,
		},
		{
			Name: "specialisation mismatch",
			A: &TestEntry{
				Inst:  x86.CMPPD_XMM1_M128_Imm5u,
				Intel: "cmppd xmm3, xmmword ptr [rcx], 1",
			},
			B: &TestEntry{
				Inst:  x86.CMPEQPD_XMM1_M128,
				Intel: "cmpeqpd xmm3, xmmword ptr [rcx]",
			},
			Want: false,
		},
		{
			Name: "xchg",
			A: &TestEntry{
				Inst:  x86.XCHG_R8_M8,
				Intel: "xchg r8, [rcx]",
			},
			B: &TestEntry{
				Inst:  x86.XCHG_M8_R8,
				Intel: "xchg [rcx], r8",
			},
			Want: true,
		},
		{
			Name: "synonyms",
			A: &TestEntry{
				Inst:  x86.SETNBE_Rmr8,
				Intel: "setnbe al",
			},
			B: &TestEntry{
				Inst:  x86.SETA_Rmr8,
				Intel: "seta al",
			},
			Want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := AreEquivalent(test.A, test.B)
			if got != test.Want {
				t.Fatalf("AreEquivalent: got %v, want %v\n  A: %s\n  B: %s", got, test.Want, test.A.Intel, test.B.Intel)
			}
		})
	}
}

func TestCanonicalIntelMemory(t *testing.T) {
	tests := []struct {
		Name   string
		Memory string
		Want   string
	}{
		{
			Name:   "canonical base",
			Memory: "[eax]",
			Want:   "[eax]",
		},
		{
			Name:   "canonical displacement",
			Memory: "[0x7f]",
			Want:   "[0x7f]",
		},
		{
			Name:   "canonical full",
			Memory: "[eax+ecx*4+0x7fff]",
			Want:   "[eax+ecx*4+0x7fff]",
		},
		{
			Name:   "noncanonical base",
			Memory: "[eax+0x0]",
			Want:   "[eax]",
		},
		{
			Name:   "short noncanonical base",
			Memory: "[eax+0]",
			Want:   "[eax]",
		},
		{
			Name:   "noncanonical index",
			Memory: "[eax*1]",
			Want:   "[eax]",
		},
		{
			Name:   "noncanonical inxe",
			Memory: "[eax*1]",
			Want:   "[eax]",
		},
		{
			Name:   "noncanonical full",
			Memory: "[eax+ecx*1+0x0]",
			Want:   "[eax+ecx]",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := CanonicalIntelMemory(test.Memory)
			if got != test.Want {
				t.Fatalf("CanonicalIntelMemory(%q):\nGot:  %s\nWant: %s", test.Memory, got, test.Want)
			}
		})
	}
}

func TestCanonicaliseStringOperation(t *testing.T) {
	tests := []struct {
		Name  string
		Entry *TestEntry
		Want  string
	}{
		{
			Name: "already canonical",
			Entry: &TestEntry{
				Inst:  x86.MOVSD,
				Intel: "movsd dword ptr es:[edi], dword ptr ds:[esi]",
				Mode:  x86.Mode32,
			},
			Want: "movsd dword ptr es:[edi], dword ptr ds:[esi]",
		},
		{
			Name: "minimal",
			Entry: &TestEntry{
				Inst:  x86.MOVSD,
				Intel: "movsd",
				Mode:  x86.Mode32,
			},
			Want: "movsd dword ptr es:[edi], dword ptr ds:[esi]",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, ok := CanonicaliseStringOperation(test.Entry, true)
			if !ok {
				t.Logf("Mnemonic: %s", test.Entry.Inst.Mnemonic)
				t.Logf("Intel:    %s", test.Entry.Intel)
				t.Logf("Syntax:   %s", test.Entry.Inst.Syntax)
				t.Logf("Mode:     %d", test.Entry.Mode.Int)
				t.Fatalf("CanonicaliseStringOperation(): got false")
			}

			if got != test.Want {
				t.Logf("Mnemonic: %s", test.Entry.Inst.Mnemonic)
				t.Logf("Intel:    %s", test.Entry.Intel)
				t.Logf("Syntax:   %s", test.Entry.Inst.Syntax)
				t.Logf("Mode:     %d", test.Entry.Mode.Int)
				t.Fatalf("CanonicaliseStringOperation():\nGot:  %s\nWant: %s", got, test.Want)
			}
		})
	}
}

func TestIsSpecialisation(t *testing.T) {
	tests := []struct {
		Name    string
		General *TestEntry
		Special *TestEntry
		Want    bool
	}{
		{
			Name: "match",
			General: &TestEntry{
				Inst:  x86.CMPPD_XMM1_M128_Imm5u,
				Intel: "cmppd xmm3, xmmword ptr [rcx], 0",
			},
			Special: &TestEntry{
				Inst:  x86.CMPEQPD_XMM1_M128,
				Intel: "cmpeqpd xmm3, xmmword ptr [rcx]",
			},
			Want: true,
		},
		{
			Name: "mismatch",
			General: &TestEntry{
				Inst:  x86.CMPPD_XMM1_M128_Imm5u,
				Intel: "cmppd xmm3, xmmword ptr [rcx], 1",
			},
			Special: &TestEntry{
				Inst:  x86.CMPEQPD_XMM1_M128,
				Intel: "cmpeqpd xmm3, xmmword ptr [rcx]",
			},
			Want: false,
		},
		{
			Name: "wrong order",
			General: &TestEntry{
				Inst:  x86.CMPEQPD_XMM1_M128,
				Intel: "cmpeqpd xmm3, xmmword ptr [rcx]",
			},
			Special: &TestEntry{
				Inst:  x86.CMPPD_XMM1_M128_Imm5u,
				Intel: "cmppd xmm3, xmmword ptr [rcx], 0",
			},
			Want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := IsSpecialisation(test.Special, test.General)
			if got != test.Want {
				t.Fatalf("IsSpecialisation: got %v, want %v\n  General: %s\n  Special: %s", got, test.Want, test.General.Intel, test.Special.Intel)
			}
		})
	}
}
