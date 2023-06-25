// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Tools for correcting cases where simple
// instruction forms are ambiguous and the
// assembler has chosen the alternative
// form.

package main

import (
	"strings"

	"firefly-os.dev/tools/ruse/internal/x86"
)

type instruction struct {
	MandatoryPrefix string
	Opcode          string
}

type instructionPair struct {
	From, To instruction
}

// Easily reversible instruction forms. See FixupEntry.
var reversible = map[string]instructionPair{
	"ADC r8/m8, r8":             {From: instruction{"", "12"}, To: instruction{"", "10"}},
	"ADC r8, r8/m8":             {From: instruction{"", "10"}, To: instruction{"", "12"}},
	"ADC r16/m16, r16":          {From: instruction{"", "13"}, To: instruction{"", "11"}},
	"ADC r16, r16/m16":          {From: instruction{"", "11"}, To: instruction{"", "13"}},
	"ADC r32/m32, r32":          {From: instruction{"", "13"}, To: instruction{"", "11"}},
	"ADC r32, r32/m32":          {From: instruction{"", "11"}, To: instruction{"", "13"}},
	"ADC r64/m64, r64":          {From: instruction{"", "13"}, To: instruction{"", "11"}},
	"ADC r64, r64/m64":          {From: instruction{"", "11"}, To: instruction{"", "13"}},
	"ADD r8/m8, r8":             {From: instruction{"", "02"}, To: instruction{"", "00"}},
	"ADD r8, r8/m8":             {From: instruction{"", "00"}, To: instruction{"", "02"}},
	"ADD r16/m16, r16":          {From: instruction{"", "03"}, To: instruction{"", "01"}},
	"ADD r16, r16/m16":          {From: instruction{"", "01"}, To: instruction{"", "03"}},
	"ADD r32/m32, r32":          {From: instruction{"", "03"}, To: instruction{"", "01"}},
	"ADD r32, r32/m32":          {From: instruction{"", "01"}, To: instruction{"", "03"}},
	"ADD r64/m64, r64":          {From: instruction{"", "03"}, To: instruction{"", "01"}},
	"ADD r64, r64/m64":          {From: instruction{"", "01"}, To: instruction{"", "03"}},
	"AND r8/m8, r8":             {From: instruction{"", "22"}, To: instruction{"", "20"}},
	"AND r8, r8/m8":             {From: instruction{"", "20"}, To: instruction{"", "22"}},
	"AND r16/m16, r16":          {From: instruction{"", "23"}, To: instruction{"", "21"}},
	"AND r16, r16/m16":          {From: instruction{"", "21"}, To: instruction{"", "23"}},
	"AND r32/m32, r32":          {From: instruction{"", "23"}, To: instruction{"", "21"}},
	"AND r32, r32/m32":          {From: instruction{"", "21"}, To: instruction{"", "23"}},
	"AND r64/m64, r64":          {From: instruction{"", "23"}, To: instruction{"", "21"}},
	"AND r64, r64/m64":          {From: instruction{"", "21"}, To: instruction{"", "23"}},
	"CMP r8/m8, r8":             {From: instruction{"", "3a"}, To: instruction{"", "38"}},
	"CMP r8, r8/m8":             {From: instruction{"", "38"}, To: instruction{"", "3a"}},
	"CMP r16/m16, r16":          {From: instruction{"", "3b"}, To: instruction{"", "39"}},
	"CMP r16, r16/m16":          {From: instruction{"", "39"}, To: instruction{"", "3b"}},
	"CMP r32/m32, r32":          {From: instruction{"", "3b"}, To: instruction{"", "39"}},
	"CMP r32, r32/m32":          {From: instruction{"", "39"}, To: instruction{"", "3b"}},
	"CMP r64/m64, r64":          {From: instruction{"", "3b"}, To: instruction{"", "39"}},
	"CMP r64, r64/m64":          {From: instruction{"", "39"}, To: instruction{"", "3b"}},
	"MOV r8/m8, r8":             {From: instruction{"", "8a"}, To: instruction{"", "88"}},
	"MOV r8, r8/m8":             {From: instruction{"", "88"}, To: instruction{"", "8a"}},
	"MOV r16/m16, r16":          {From: instruction{"", "8b"}, To: instruction{"", "89"}},
	"MOV r16, r16/m16":          {From: instruction{"", "89"}, To: instruction{"", "8b"}},
	"MOV r32/m32, r32":          {From: instruction{"", "8b"}, To: instruction{"", "89"}},
	"MOV r32, r32/m32":          {From: instruction{"", "89"}, To: instruction{"", "8b"}},
	"MOV r64/m64, r64":          {From: instruction{"", "8b"}, To: instruction{"", "89"}},
	"MOV r64, r64/m64":          {From: instruction{"", "89"}, To: instruction{"", "8b"}},
	"OR r8/m8, r8":              {From: instruction{"", "0a"}, To: instruction{"", "08"}},
	"OR r8, r8/m8":              {From: instruction{"", "08"}, To: instruction{"", "0a"}},
	"OR r16/m16, r16":           {From: instruction{"", "0b"}, To: instruction{"", "09"}},
	"OR r16, r16/m16":           {From: instruction{"", "09"}, To: instruction{"", "0b"}},
	"OR r32/m32, r32":           {From: instruction{"", "0b"}, To: instruction{"", "09"}},
	"OR r32, r32/m32":           {From: instruction{"", "09"}, To: instruction{"", "0b"}},
	"OR r64/m64, r64":           {From: instruction{"", "0b"}, To: instruction{"", "09"}},
	"OR r64, r64/m64":           {From: instruction{"", "09"}, To: instruction{"", "0b"}},
	"SBB r8/m8, r8":             {From: instruction{"", "1a"}, To: instruction{"", "18"}},
	"SBB r8, r8/m8":             {From: instruction{"", "18"}, To: instruction{"", "1a"}},
	"SBB r16/m16, r16":          {From: instruction{"", "1b"}, To: instruction{"", "19"}},
	"SBB r16, r16/m16":          {From: instruction{"", "19"}, To: instruction{"", "1b"}},
	"SBB r32/m32, r32":          {From: instruction{"", "1b"}, To: instruction{"", "19"}},
	"SBB r32, r32/m32":          {From: instruction{"", "19"}, To: instruction{"", "1b"}},
	"SBB r64/m64, r64":          {From: instruction{"", "1b"}, To: instruction{"", "19"}},
	"SBB r64, r64/m64":          {From: instruction{"", "19"}, To: instruction{"", "1b"}},
	"SUB r8/m8, r8":             {From: instruction{"", "2a"}, To: instruction{"", "28"}},
	"SUB r8, r8/m8":             {From: instruction{"", "28"}, To: instruction{"", "2a"}},
	"SUB r16/m16, r16":          {From: instruction{"", "2b"}, To: instruction{"", "29"}},
	"SUB r16, r16/m16":          {From: instruction{"", "29"}, To: instruction{"", "2b"}},
	"SUB r32/m32, r32":          {From: instruction{"", "2b"}, To: instruction{"", "29"}},
	"SUB r32, r32/m32":          {From: instruction{"", "29"}, To: instruction{"", "2b"}},
	"SUB r64/m64, r64":          {From: instruction{"", "2b"}, To: instruction{"", "29"}},
	"SUB r64, r64/m64":          {From: instruction{"", "29"}, To: instruction{"", "2b"}},
	"XCHG r8/m8, r8":            {From: instruction{"", "86"}, To: instruction{"", "86"}},
	"XCHG r16/m16, r16":         {From: instruction{"", "87"}, To: instruction{"", "87"}},
	"XCHG r32/m32, r32":         {From: instruction{"", "87"}, To: instruction{"", "87"}},
	"XCHG r64/m64, r64":         {From: instruction{"", "87"}, To: instruction{"", "87"}},
	"XOR r8/m8, r8":             {From: instruction{"", "32"}, To: instruction{"", "30"}},
	"XOR r8, r8/m8":             {From: instruction{"", "30"}, To: instruction{"", "32"}},
	"XOR r16/m16, r16":          {From: instruction{"", "33"}, To: instruction{"", "31"}},
	"XOR r16, r16/m16":          {From: instruction{"", "31"}, To: instruction{"", "33"}},
	"XOR r32/m32, r32":          {From: instruction{"", "33"}, To: instruction{"", "31"}},
	"XOR r32, r32/m32":          {From: instruction{"", "31"}, To: instruction{"", "33"}},
	"XOR r64/m64, r64":          {From: instruction{"", "33"}, To: instruction{"", "31"}},
	"XOR r64, r64/m64":          {From: instruction{"", "31"}, To: instruction{"", "33"}},
	"MOVAPD xmm1, xmm2/m128":    {From: instruction{"66", "0f29"}, To: instruction{"66", "0f28"}},
	"MOVAPD xmm2/m128, xmm1":    {From: instruction{"66", "0f28"}, To: instruction{"66", "0f29"}},
	"MOVAPS xmm1, xmm2/m128":    {From: instruction{"", "0f29"}, To: instruction{"", "0f28"}},
	"MOVAPS xmm2/m128, xmm1":    {From: instruction{"", "0f28"}, To: instruction{"", "0f29"}},
	"MOVDQA xmm1, xmm2/m128":    {From: instruction{"66", "0f7f"}, To: instruction{"66", "0f6f"}},
	"MOVDQA xmm2/m128, xmm1":    {From: instruction{"66", "0f6f"}, To: instruction{"66", "0f7f"}},
	"MOVDQU xmm1, xmm2/m128":    {From: instruction{"f3", "0f7f"}, To: instruction{"f3", "0f6f"}},
	"MOVDQU xmm2/m128, xmm1":    {From: instruction{"f3", "0f6f"}, To: instruction{"f3", "0f7f"}},
	"MOVQ mm, mm/m64":           {From: instruction{"", "0f7f"}, To: instruction{"", "0f6f"}},
	"MOVQ mm, r64/m64":          {From: instruction{"", "0f7e"}, To: instruction{"", "0f6e"}},
	"MOVQ mm/m64, mm":           {From: instruction{"", "0f6f"}, To: instruction{"", "0f7f"}},
	"MOVQ r64/m64, mm":          {From: instruction{"", "0f6e"}, To: instruction{"", "0f7e"}},
	"MOVQ xmm1, xmm2/m64":       {From: instruction{"66", "0fd6"}, To: instruction{"f3", "0f7e"}},
	"MOVQ xmm2/m64, xmm1":       {From: instruction{"f3", "0f7e"}, To: instruction{"66", "0fd6"}},
	"MOVSD xmm1, xmm2":          {From: instruction{"", "0f11"}, To: instruction{"", "0f10"}},
	"MOVSD xmm1/m64, xmm2":      {From: instruction{"", "0f10"}, To: instruction{"", "0f11"}},
	"MOVSS xmm1, xmm2/m32":      {From: instruction{"f3", "0f11"}, To: instruction{"f3", "0f10"}},
	"MOVSS xmm2/m32, xmm1":      {From: instruction{"f3", "0f10"}, To: instruction{"f3", "0f11"}},
	"MOVUPD xmm1, xmm2/m128":    {From: instruction{"66", "0f11"}, To: instruction{"66", "0f10"}},
	"MOVUPD xmm2/m128, xmm1":    {From: instruction{"66", "0f10"}, To: instruction{"66", "0f11"}},
	"MOVUPS xmm1, xmm2/m128":    {From: instruction{"", "0f11"}, To: instruction{"", "0f10"}},
	"MOVUPS xmm2/m128, xmm1":    {From: instruction{"", "0f10"}, To: instruction{"", "0f11"}},
	"PEXTRW r32/m16, xmm, imm8": {From: instruction{"66", "0fc5"}, To: instruction{"66", "0f3a15"}},
}

// FixupEntry applies common fixes to ambiguous Intel
// assmebly that meets a pair of reverisble instruction
// forms.
//
// For example, an attempt to demonstrate the instruction
// `ADC r/m8, r8` with the Intel assembly `adc al, bl`
// may lead to the assembler choosing the instruction form
// `ADC r8, r/m8`, as the two are equivalent. In this case,
// calling `swap2ByteInstruction(entry, "10") would identify
// the mismatch and correct it by replacing the opcode (12)
// with 10 and reversing the modR/M byte to swap the arguments.
func FixupEntry(entry *TestEntry) {
	if inst, ok := reversible[entry.Inst.Syntax]; ok {
		switch entry.Inst {
		case x86.XCHG_M8_R8,
			x86.XCHG_M16_R16,
			x86.XCHG_M32_R32,
			x86.XCHG_M64_R64_REX:
			// These forms are already in the right
			// order and XCHG is reversible so we
			// can't detect this from the opcode.
			return
		}

		fixupInstruction(entry, inst)
		return
	}

	fixupUD1(entry)

	switch entry.Inst.Syntax {
	case "CALL-FAR m16:16", "CALL-FAR m16:32":
		if entry.Mode.String != strings.TrimPrefix(entry.Inst.Syntax, "CALL-FAR m16:") && !strings.HasPrefix(entry.Code, "66") {
			entry.Code = "66" + entry.Code
		}
	}
}

func fixupUD1(entry *TestEntry) {
	// Clang erroneously emits operand size
	// override prefixes for UD1, so we remove
	// them here.
	if entry.Inst.Mnemonic != "UD1" {
		return
	}

	i := strings.Index(entry.Code, "0fb9") // Opcode.
	if i < 0 || i%2 != 0 {
		panic("invalid code for UD1 (bad opcode): " + entry.Code)
	}

	j := strings.Index(entry.Code[:i], "66")
	for j >= 0 && j%2 != 0 && j < i {
		k := strings.Index(entry.Code[j+1:i], "66")
		if k < 0 {
			return
		}

		j = j + 1 + k
	}

	if j < 0 || j%2 != 0 || j >= i {
		return
	}

	entry.Code = entry.Code[:j] + entry.Code[j+2:]
}

// fixupInstruction is a helper. This works by checking for
// cases where the generated machine code does not start
// with the given opcode byte (optionally preceded by an
// operand size override prefix), due to an alternative
// form being chosen.
//
// For example, an attempt to demonstrate the instruction
// `ADC r/m8, r8` with the Intel assembly `adc al, bl`
// may lead to the assembler choosing the instruction form
// `ADC r8, r/m8`, as the two are equivalent. In this case,
// calling `swap2ByteInstruction(entry, "10") would identify
// the mismatch and correct it by replacing the opcode (12)
// with 10 and reversing the modR/M byte to swap the arguments.
func fixupInstruction(entry *TestEntry, inst instructionPair) {
	if len(entry.Code) < 2 {
		return
	}

	s, ok := strings.CutPrefix(entry.Code, inst.From.MandatoryPrefix)
	if !ok {
		return
	}

	var prefix string
	switch s[:2] {
	case "f2", "f3", "66":
		prefix, s = s[:2], s[2:]
	}

	// REX prefixes, which need to be
	// flipped.
	var rex string
	if len(s) > 2 && s[0] == '4' && !strings.HasPrefix(inst.From.Opcode, "4") {
		rex, s = flipREX(s[:2]), s[2:]
	}

	rest, ok := strings.CutPrefix(s, inst.From.Opcode)
	if ok {
		entry.Code = inst.To.MandatoryPrefix + prefix + rex + inst.To.Opcode + flipRegRM(rest[:2]) + rest[2:]
	}
}

// A helper to invert a REX prefix.
// That is, bits R and B are swapped
// but any other bits are preserved.
func flipREX(rex string) string {
	switch rex {
	case "40": // REX (no change)
	case "4d": // REX.WRB (no change)
	case "4c": // REX.WR -> REX.WB
		rex = "49"
	case "49": // REX.WB -> REX.WR
		rex = "4c"
	case "48": // REX.W (no change)
	case "45": // REX.RB (no change)
	case "44": // REX.R -> REX.B
		rex = "41"
	case "41": // REX.B -> REX.R
		rex = "44"
	default:
		panic("unsupported REX prefix: " + rex)
	}

	return rex
}

// A helper to swap the Reg and R/M
// fields of a ModR/M byte in hex
// form.
func flipRegRM(s string) string {
	if len(s) != 2 {
		panic(s)
	}

	// Decode the hex byte.
	const hashtable = "0123456789abcdef"
	const reverseHexTable = "" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\xff\xff\xff\xff\xff\xff" +
		"\xff\x0a\x0b\x0c\x0d\x0e\x0f\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\x0a\x0b\x0c\x0d\x0e\x0f\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff"

	n1 := reverseHexTable[s[0]]
	n2 := reverseHexTable[s[1]]
	if n1 > 0xf || n2 > 0xf {
		panic(s)
	}

	v := (n1 << 4) | n2

	// The Mod field (the top two bits)
	// stays the same, but we swap the
	// bottom pair of three bits.
	v = (v & 0b1100_0000) |
		((v & 0b0011_1000) >> 3) |
		((v & 0b0000_0111) << 3)

	return string([]byte{hashtable[v>>4], hashtable[v&0xf]})
}
