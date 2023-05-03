// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"testing"

	"firefly-os.dev/tools/ruse/internal/x86"
)

func TestIsDisassemblyMatch(t *testing.T) {
	tests := []struct {
		Name   string
		Entry  *TestEntry
		Disasm string
		Code   string
		Want   bool
	}{
		{
			Name: "different",
			Entry: &TestEntry{
				Inst:  x86.MOV_R32_M32,
				Intel: "mov r8d, dword ptr [rcx]",
			},
			Disasm: "add [rcx], 0x1234",
			Code:   "6681003412",
			Want:   false,
		},
		{
			Name: "identical",
			Entry: &TestEntry{
				Inst:  x86.ADC_Rmr32_Imm32,
				Intel: "adc eax, 0x1234",
			},
			Disasm: "adc eax,  0x1234",
			Code:   "1534120000",
			Want:   true,
		},
		{
			Name: "different base",
			Entry: &TestEntry{
				Inst:  x86.ADC_Rmr8_Imm8,
				Intel: "adc ax, 255",
			},
			Disasm: "adc ax, 0xff",
			Code:   "6615ff00",
			Want:   true,
		},
		{
			Name: "implicit radix",
			Entry: &TestEntry{
				Inst:  x86.AAD,
				Intel: "aad",
			},
			Disasm: "aad 10",
			Code:   "d50a",
			Want:   true,
		},
		{
			Name: "specialisation",
			Entry: &TestEntry{
				Inst:  x86.CMPPD_XMM1_M128_Imm5u,
				Intel: "cmppd xmm3, xmmword ptr [rcx], 0x0",
			},
			Disasm: "cmpeqpd xmm3, xmmword ptr [rcx]",
			Code:   "660fc21900",
			Want:   true,
		},
		{
			Name: "blendvpd implicit arg",
			Entry: &TestEntry{
				Inst:  x86.BLENDVPD_XMM1_M128,
				Intel: "blendvpd xmm3, xmmword ptr [rcx]",
			},
			Disasm: "blendvpd xmm3, xmmword ptr [rcx], xmm0",
			Code:   "660f381519",
			Want:   true,
		},
		{
			Name: "ds relative call",
			Entry: &TestEntry{
				Inst:  x86.CALL_M32,
				Intel: "call ds:[eax]",
			},
			Disasm: "notrack call [eax]",
			Code:   "3eff10",
			Want:   true,
		},
		{
			Name: "call syntax",
			Entry: &TestEntry{
				Inst:  x86.CALL_M32,
				Intel: "call dword ptr [0x7fff]",
			},
			Disasm: "call 7fff <_start-0x4010ec>",
			Code:   "66ff16ff7f",
			Want:   true,
		},
		{
			Name: "call-far direct syntax",
			Entry: &TestEntry{
				Inst:  x86.CALL_FAR_Ptr16v32,
				Intel: "lcall 0x12, 0xfcfdfe",
			},
			Disasm: "call 0x12:0xfcfdfe",
			Code:   "9afefdfc001200",
			Want:   true,
		},
		{
			Name: "call-far indirect syntax 16",
			Entry: &TestEntry{
				Inst:  x86.CALL_FAR_M16v16,
				Intel: "lcall word ptr [bx+si]",
				Mode:  x86.Mode16,
			},
			Disasm: "call dword ptr [bx+si]",
			Code:   "ff10",
			Want:   true,
		},
		{
			Name: "call-far indirect syntax 32",
			Entry: &TestEntry{
				Inst:  x86.CALL_FAR_M16v32,
				Intel: "lcall dword ptr [eax]",
				Mode:  x86.Mode32,
			},
			Disasm: "call fword ptr [eax]",
			Code:   "ff10",
			Want:   true,
		},
		{
			Name: "equal meaning instructions",
			Entry: &TestEntry{
				Inst:  x86.SETNBE_Rmr8,
				Intel: "setnbe al",
			},
			Disasm: "seta al",
			Code:   "0f97c0",
			Want:   true,
		},
		{
			Name: "ambiguous x87 parameter order",
			Entry: &TestEntry{
				Inst:  x86.FADD_ST_STi,
				Intel: "fadd st, st(0)",
			},
			Disasm: "fadd st(0), st",
			Code:   "d8c0",
			Want:   true,
		},
		{
			Name: "implied x87 parameter pair",
			Entry: &TestEntry{
				Inst:  x86.FMULP,
				Intel: "fmulp",
			},
			Disasm: "fmulp st(1), st",
			Code:   "dec9",
			Want:   true,
		},
		{
			Name: "implied x87 single parameter",
			Entry: &TestEntry{
				Inst:  x86.FUCOM,
				Intel: "fucom",
			},
			Disasm: "fucom st(1)",
			Code:   "dde1",
			Want:   true,
		},
		{
			Name: "iretw",
			Entry: &TestEntry{
				Inst:  x86.IRET,
				Intel: "iret",
				Mode:  x86.Mode32,
			},
			Disasm: "iretw",
			Code:   "66cf",
			Want:   true,
		},
		{
			Name: "iretd",
			Entry: &TestEntry{
				Inst:  x86.IRETD,
				Intel: "iretd",
				Mode:  x86.Mode32,
			},
			Disasm: "iret",
			Code:   "cf",
			Want:   true,
		},
		{
			Name: "not iretd",
			Entry: &TestEntry{
				Inst:  x86.IRETD,
				Intel: "iretd",
				Mode:  x86.Mode32,
			},
			Disasm: "iret",
			Code:   "66cf",
			Want:   false,
		},
		{
			Name: "lar",
			Entry: &TestEntry{
				Inst:  x86.LAR_R16_Rmr16,
				Intel: "lar ax, cx",
			},
			Disasm: "lar ax, ecx",
			Code:   "0f02c1",
			Want:   true,
		},
		{
			Name: "heuristic specialisation",
			Entry: &TestEntry{
				Inst:  x86.CMPPD_XMM1_M128_Imm5u,
				Intel: "cmppd xmm0, xmmword ptr [ebp+ecx], 0",
			},
			Disasm: "cmpeqpd xmm0, xmmword ptr [ebp+ecx*1+0x0]",
			Code:   "67660fc2440d0000",
			Want:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := IsDisassemblyMatch(test.Entry, test.Disasm, test.Code)
			if got != test.Want {
				t.Fatalf("IsDisassemblyMatch(%q, %q, %q): got %v, want %v", test.Entry.Intel, test.Disasm, test.Code, got, test.Want)
			}
		})
	}
}
