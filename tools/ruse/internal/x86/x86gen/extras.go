// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"firefly-os.dev/tools/ruse/internal/x86"
)

// ExtraOperands contains additional
// operands for existing instructions.
//
// Each entry maps the instruction UID
// to four operand pointers. Any non-nil
// operand pointers should be installed
// into the instruction.
var ExtraOperands = map[string][4]*Operand{
	// Implicit strings in CMPS.
	"CMPSB": {
		0: {Name: "m8", Syntax: "[ds:esi:8]", UID: "StrSrc8", Type: x86.TypeStringSrc, Encoding: x86.EncodingNone, Bits: 8},
		1: {Name: "m8", Syntax: "[es:edi:8]", UID: "StrDst8", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 8},
	},
	"CMPSW": {
		0: {Name: "m16", Syntax: "[ds:esi:16]", UID: "StrSrc16", Type: x86.TypeStringSrc, Encoding: x86.EncodingNone, Bits: 16},
		1: {Name: "m16", Syntax: "[es:edi:16]", UID: "StrDst16", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 16},
	},
	"CMPSD": {
		0: {Name: "m32", Syntax: "[ds:esi:32]", UID: "StrSrc32", Type: x86.TypeStringSrc, Encoding: x86.EncodingNone, Bits: 32},
		1: {Name: "m32", Syntax: "[es:edi:32]", UID: "StrDst32", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 32},
	},
	"CMPSQ_REX": {
		0: {Name: "m64", Syntax: "[rsi:64]", UID: "StrSrc64", Type: x86.TypeStringSrc, Encoding: x86.EncodingNone, Bits: 64},
		1: {Name: "m64", Syntax: "[rdi:64]", UID: "StrDst64", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 64},
	},

	// Implicit strings in INS.
	"INSB": {
		0: {Name: "m8", Syntax: "[es:edi:8]", UID: "StrDst8", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 8},
		1: {Name: "DX", Syntax: "DX", UID: "DX", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 16},
	},
	"INSW": {
		0: {Name: "m16", Syntax: "[es:edi:16]", UID: "StrDst16", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 16},
		1: {Name: "DX", Syntax: "DX", UID: "DX", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 16},
	},
	"INSD": {
		0: {Name: "m32", Syntax: "[es:edi:32]", UID: "StrDst32", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 32},
		1: {Name: "DX", Syntax: "DX", UID: "DX", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 16},
	},

	// Implicit strings in LODS.
	"LODSB": {
		0: {Name: "AL", Syntax: "AL", UID: "AL", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 8},
		1: {Name: "m8", Syntax: "[ds:esi:8]", UID: "StrSrc8", Type: x86.TypeStringSrc, Encoding: x86.EncodingNone, Bits: 8},
	},
	"LODSW": {
		0: {Name: "AX", Syntax: "AX", UID: "AX", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 8},
		1: {Name: "m16", Syntax: "[ds:esi:16]", UID: "StrSrc16", Type: x86.TypeStringSrc, Encoding: x86.EncodingNone, Bits: 16},
	},
	"LODSD": {
		0: {Name: "EAX", Syntax: "EAX", UID: "EAX", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 8},
		1: {Name: "m32", Syntax: "[ds:esi:32]", UID: "StrSrc32", Type: x86.TypeStringSrc, Encoding: x86.EncodingNone, Bits: 32},
	},
	"LODSQ_REX": {
		0: {Name: "RAX", Syntax: "RAX", UID: "RAX", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 8},
		1: {Name: "m64", Syntax: "[rsi:64]", UID: "StrSrc64", Type: x86.TypeStringSrc, Encoding: x86.EncodingNone, Bits: 64},
	},

	// Implicit strings in MOVS.
	"MOVSB": {
		0: {Name: "m8", Syntax: "[es:edi:8]", UID: "StrDst8", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 8},
		1: {Name: "m8", Syntax: "[ds:esi:8]", UID: "StrSrc8", Type: x86.TypeStringSrc, Encoding: x86.EncodingNone, Bits: 8},
	},
	"MOVSW": {
		0: {Name: "m16", Syntax: "[es:edi:16]", UID: "StrDst16", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 16},
		1: {Name: "m16", Syntax: "[ds:esi:16]", UID: "StrSrc16", Type: x86.TypeStringSrc, Encoding: x86.EncodingNone, Bits: 16},
	},
	"MOVSD": {
		0: {Name: "m32", Syntax: "[es:edi:32]", UID: "StrDst32", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 32},
		1: {Name: "m32", Syntax: "[ds:esi:32]", UID: "StrSrc32", Type: x86.TypeStringSrc, Encoding: x86.EncodingNone, Bits: 32},
	},
	"MOVSQ_REX": {
		0: {Name: "m64", Syntax: "[rdi:64]", UID: "StrDst64", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 64},
		1: {Name: "m64", Syntax: "[rsi:64]", UID: "StrSrc64", Type: x86.TypeStringSrc, Encoding: x86.EncodingNone, Bits: 64},
	},

	// Implicit strings in OUTS.
	"OUTSB": {
		0: {Name: "DX", Syntax: "DX", UID: "DX", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 16},
		1: {Name: "m8", Syntax: "[ds:esi:8]", UID: "StrSrc8", Type: x86.TypeStringSrc, Encoding: x86.EncodingNone, Bits: 8},
	},
	"OUTSW": {
		0: {Name: "DX", Syntax: "DX", UID: "DX", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 16},
		1: {Name: "m16", Syntax: "[ds:esi:16]", UID: "StrSrc16", Type: x86.TypeStringSrc, Encoding: x86.EncodingNone, Bits: 16},
	},
	"OUTSD": {
		0: {Name: "DX", Syntax: "DX", UID: "DX", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 16},
		1: {Name: "m32", Syntax: "[ds:esi:32]", UID: "StrSrc32", Type: x86.TypeStringSrc, Encoding: x86.EncodingNone, Bits: 32},
	},

	// Implicit strings in SCAS.
	"SCASB": {
		0: {Name: "AL", Syntax: "AL", UID: "AL", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 8},
		1: {Name: "m8", Syntax: "[es:edi:8]", UID: "StrDst8", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 8},
	},
	"SCASW": {
		0: {Name: "AX", Syntax: "AX", UID: "AX", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 8},
		1: {Name: "m16", Syntax: "[es:edi:16]", UID: "StrDst16", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 16},
	},
	"SCASD": {
		0: {Name: "EAX", Syntax: "EAX", UID: "EAX", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 8},
		1: {Name: "m32", Syntax: "[es:edi:32]", UID: "StrDst32", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 32},
	},
	"SCASQ_REX": {
		0: {Name: "RAX", Syntax: "RAX", UID: "RAX", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 8},
		1: {Name: "m64", Syntax: "[rdi:64]", UID: "StrDst64", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 64},
	},

	// Implicit strings in STOS.
	"STOSB": {
		0: {Name: "m8", Syntax: "[es:edi:8]", UID: "StrDst8", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 8},
		1: {Name: "AL", Syntax: "AL", UID: "AL", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 8},
	},
	"STOSW": {
		0: {Name: "m16", Syntax: "[es:edi:16]", UID: "StrDst16", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 16},
		1: {Name: "AX", Syntax: "AX", UID: "AX", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 8},
	},
	"STOSD": {
		0: {Name: "m32", Syntax: "[es:edi:32]", UID: "StrDst32", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 32},
		1: {Name: "EAX", Syntax: "EAX", UID: "EAX", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 8},
	},
	"STOSQ_REX": {
		0: {Name: "m64", Syntax: "[rdi:64]", UID: "StrDst64", Type: x86.TypeStringDst, Encoding: x86.EncodingNone, Bits: 64},
		1: {Name: "RAX", Syntax: "RAX", UID: "RAX", Type: x86.TypeRegister, Encoding: x86.EncodingNone, Bits: 8},
	},
}

// Extras contains fake instruction
// listings for some instructions
// not included in the Intel x86
// manual.
var Extras = []*Listing{
	{
		// Clear the global interrupt flag (AMD-V).
		MnemonicTable: []Mnemonic{{Opcode: "0F 01 DD", Instruction: "CLGI", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"}},
	},

	{
		MnemonicTable: []Mnemonic{
			// Specialised mnemonics for CMPPD xmm1, xmm2/m128, X.
			{Opcode: "66 0F C2 /r 00", Instruction: "CMPEQPD xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "66 0F C2 /r 01", Instruction: "CMPLTPD xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "66 0F C2 /r 02", Instruction: "CMPLEPD xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "66 0F C2 /r 03", Instruction: "CMPUNORDPD xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "66 0F C2 /r 04", Instruction: "CMPNEQPD xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "66 0F C2 /r 05", Instruction: "CMPNLTPD xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "66 0F C2 /r 06", Instruction: "CMPNLEPD xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "66 0F C2 /r 07", Instruction: "CMPORDPD xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},

			// Specialised mnemonics for CMPPS xmm1, xmm2/m128, X.
			{Opcode: "0F C2 /r 00", Instruction: "CMPEQPS xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "0F C2 /r 01", Instruction: "CMPLTPS xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "0F C2 /r 02", Instruction: "CMPLEPS xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "0F C2 /r 03", Instruction: "CMPUNORDPS xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "0F C2 /r 04", Instruction: "CMPNEQPS xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "0F C2 /r 05", Instruction: "CMPNLTPS xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "0F C2 /r 06", Instruction: "CMPNLEPS xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "0F C2 /r 07", Instruction: "CMPORDPS xmm1, xmm2/m128", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},

			// Specialised mnemonics for CMPSD xmm1, xmm2/m64, X.
			{Opcode: "F2 0F C2 /r 00", Instruction: "CMPEQSD xmm1, xmm2/m64", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "F2 0F C2 /r 01", Instruction: "CMPLTSD xmm1, xmm2/m64", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "F2 0F C2 /r 02", Instruction: "CMPLESD xmm1, xmm2/m64", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "F2 0F C2 /r 03", Instruction: "CMPUNORDSD xmm1, xmm2/m64", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "F2 0F C2 /r 04", Instruction: "CMPNEQSD xmm1, xmm2/m64", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "F2 0F C2 /r 05", Instruction: "CMPNLTSD xmm1, xmm2/m64", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "F2 0F C2 /r 06", Instruction: "CMPNLESD xmm1, xmm2/m64", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "F2 0F C2 /r 07", Instruction: "CMPORDSD xmm1, xmm2/m64", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},

			// Specialised mnemonics for CMPSS xmm1, xmm2/m32, X.
			{Opcode: "F3 0F C2 /r 00", Instruction: "CMPEQSS xmm1, xmm2/m32", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "F3 0F C2 /r 01", Instruction: "CMPLTSS xmm1, xmm2/m32", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "F3 0F C2 /r 02", Instruction: "CMPLESS xmm1, xmm2/m32", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "F3 0F C2 /r 03", Instruction: "CMPUNORDSS xmm1, xmm2/m32", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "F3 0F C2 /r 04", Instruction: "CMPNEQSS xmm1, xmm2/m32", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "F3 0F C2 /r 05", Instruction: "CMPNLTSS xmm1, xmm2/m32", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "F3 0F C2 /r 06", Instruction: "CMPNLESS xmm1, xmm2/m32", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "F3 0F C2 /r 07", Instruction: "CMPORDSS xmm1, xmm2/m32", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
		},
		OperandEncodingTable: []OperandEncoding{{Encoding: "A", Operands: [4]string{"ModRM:reg", "ModRM:r/m", "N/A", "N/A"}}},
	},

	{
		// Explicit-operands versions of CMPS.
		MnemonicTable: []Mnemonic{
			{Opcode: "A6", Instruction: "CMPS [ds:esi:8], [es:edi:8]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", DataSize: 8},
			{Opcode: "A7", Instruction: "CMPS [ds:esi:16], [es:edi:16]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "A7", Instruction: "CMPS [ds:esi:32], [es:edi:32]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 32},
			{Opcode: "REX.W A7", Instruction: "CMPS [rsi:64], [rdi:64]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Invalid", Mode16: "Invalid", OperandSize: true, DataSize: 64},
		},
		OperandEncodingTable: []OperandEncoding{{Encoding: "A", Operands: [4]string{"None", "None", "N/A", "N/A"}}},
	},

	{
		// Extract field from register.
		MnemonicTable: []Mnemonic{
			{Opcode: "66 0F 78 /0 ib ib", Instruction: "EXTRQ xmm2, imm8, imm8", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "66 0F 79 /r", Instruction: "EXTRQ xmm1, xmm2", OperandEncoding: "B", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
		},
		OperandEncodingTable: []OperandEncoding{
			{Encoding: "A", Operands: [4]string{"ModRM:r/m", "Immediate", "Immediate", "N/A"}},
			{Encoding: "B", Operands: [4]string{"ModRM:reg", "ModRM:r/m", "N/A", "N/A"}},
		},
	},

	{
		// Fast exit multimedia state.
		MnemonicTable: []Mnemonic{{Opcode: "0F 0E", Instruction: "FEMMS", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"}},
	},

	{
		// Free floating point register and pop stack.
		MnemonicTable:        []Mnemonic{{Opcode: "DF C0+i", OperandEncoding: "A", Instruction: "FFREEP ST(i)", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"}},
		OperandEncodingTable: []OperandEncoding{{Encoding: "A", Operands: [4]string{"ST(i)", "N/A", "N/A", "N/A"}}},
	},

	{
		// Perform an SMX function.
		MnemonicTable: []Mnemonic{{Opcode: "NP 0F 37", Instruction: "GETSEC", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"}},
	},

	{
		// Explicit-operands versions of INS.
		MnemonicTable: []Mnemonic{
			{Opcode: "6C", Instruction: "INS [es:edi:8], DX", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", DataSize: 8},
			{Opcode: "6D", Instruction: "INS [es:edi:16], DX", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "6D", Instruction: "INS [es:edi:32], DX", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 32},
		},
		OperandEncodingTable: []OperandEncoding{{Encoding: "A", Operands: [4]string{"None", "None", "N/A", "N/A"}}},
	},

	{
		// Explicit-operands versions of LODS.
		MnemonicTable: []Mnemonic{
			{Opcode: "AC", Instruction: "LODS AL, [ds:esi:8]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", DataSize: 8},
			{Opcode: "AD", Instruction: "LODS AX, [ds:esi:16]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "AD", Instruction: "LODS EAX, [ds:esi:32]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 32},
			{Opcode: "REX.W AD", Instruction: "LODS RAX, [rsi:64]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Invalid", Mode16: "Invalid", OperandSize: true, DataSize: 64},
		},
		OperandEncodingTable: []OperandEncoding{{Encoding: "A", Operands: [4]string{"None", "None", "N/A", "N/A"}}},
	},

	{
		// Explicit-operands versions of MOVS.
		MnemonicTable: []Mnemonic{
			{Opcode: "A4", Instruction: "MOVS [es:edi:8], [ds:esi:8]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", DataSize: 8},
			{Opcode: "A5", Instruction: "MOVS [es:edi:16], [ds:esi:16]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "A5", Instruction: "MOVS [es:edi:32], [ds:esi:32]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 32},
			{Opcode: "REX.W A5", Instruction: "MOVS [rdi:64], [rsi:64]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Invalid", Mode16: "Invalid", OperandSize: true, DataSize: 64},
		},
		OperandEncodingTable: []OperandEncoding{{Encoding: "A", Operands: [4]string{"None", "None", "N/A", "N/A"}}},
	},

	{
		// Explicit-operands versions of OUTS.
		MnemonicTable: []Mnemonic{
			{Opcode: "6E", Instruction: "OUTS DX, [ds:esi:8]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", DataSize: 8},
			{Opcode: "6F", Instruction: "OUTS DX, [ds:esi:16]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "6F", Instruction: "OUTS DX, [ds:esi:32]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 32},
		},
		OperandEncodingTable: []OperandEncoding{{Encoding: "A", Operands: [4]string{"None", "None", "N/A", "N/A"}}},
	},

	{
		MnemonicTable: []Mnemonic{
			// Versions of POP/PUSH ES/CS/SS/DS/FS/GS that increment the stack pointer by 2/4/8.
			{Opcode: "07", Instruction: "POPW ES", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "17", Instruction: "POPW SS", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "1F", Instruction: "POPW DS", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "0F A1", Instruction: "POPW FS", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "0F A9", Instruction: "POPW GS", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "07", Instruction: "POPD ES", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Invalid", OperandSize: true, DataSize: 32},
			{Opcode: "17", Instruction: "POPD SS", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Invalid", OperandSize: true, DataSize: 32},
			{Opcode: "1F", Instruction: "POPD DS", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Invalid", OperandSize: true, DataSize: 32},
			{Opcode: "0F A1", Instruction: "POPD FS", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Invalid", OperandSize: true, DataSize: 32},
			{Opcode: "0F A9", Instruction: "POPD GS", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Invalid", OperandSize: true, DataSize: 32},
			{Opcode: "REX.W 0F A1", Instruction: "POPQ FS", OperandEncoding: "A", Mode64: "Valid", Mode32: "Invalid", Mode16: "Invalid", OperandSize: true, DataSize: 64},
			{Opcode: "REX.W 0F A9", Instruction: "POPQ GS", OperandEncoding: "A", Mode64: "Valid", Mode32: "Invalid", Mode16: "Invalid", OperandSize: true, DataSize: 64},
			{Opcode: "06", Instruction: "PUSHW ES", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "0E", Instruction: "PUSHW CS", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "16", Instruction: "PUSHW SS", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "1E", Instruction: "PUSHW DS", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "0F A0", Instruction: "PUSHW FS", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "0F A8", Instruction: "PUSHW GS", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "06", Instruction: "PUSHD ES", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 32},
			{Opcode: "0E", Instruction: "PUSHD CS", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 32},
			{Opcode: "16", Instruction: "PUSHD SS", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 32},
			{Opcode: "1E", Instruction: "PUSHD DS", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 32},
			{Opcode: "0F A0", Instruction: "PUSHD FS", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 32},
			{Opcode: "0F A8", Instruction: "PUSHD GS", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 32},
			{Opcode: "REX.W 0F A0", Instruction: "PUSHQ FS", OperandEncoding: "A", Mode64: "Valid", Mode32: "Invalid", Mode16: "Invalid", OperandSize: true, DataSize: 64},
			{Opcode: "REX.W 0F A8", Instruction: "PUSHQ GS", OperandEncoding: "A", Mode64: "Valid", Mode32: "Invalid", Mode16: "Invalid", OperandSize: true, DataSize: 64},
		},
		OperandEncodingTable: []OperandEncoding{{Encoding: "A", Operands: [4]string{"None", "N/A", "N/A", "N/A"}}},
	},

	{
		MnemonicTable: []Mnemonic{
			// Explicit expressions of PUSH imm16 and PUSH imm32.
			{Opcode: "68 iw", Instruction: "PUSHW imm16", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "68 id", Instruction: "PUSHD imm32", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 32},
		},
		OperandEncodingTable: []OperandEncoding{{Encoding: "A", Operands: [4]string{"Immediate", "N/A", "N/A", "N/A"}}},
	},

	{
		// Explicit-operands verdions of SCAS.
		MnemonicTable: []Mnemonic{
			{Opcode: "AE", Instruction: "SCAS AL, [es:edi:8]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", DataSize: 8},
			{Opcode: "AF", Instruction: "SCAS AX, [es:edi:16]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "AF", Instruction: "SCAS EAX, [es:edi:32]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 32},
			{Opcode: "REX.W AF", Instruction: "SCAS RAX, [rdi:64]", OperandEncoding: "A", Mode64: "Valid", Mode32: "Invalid", Mode16: "Invalid", OperandSize: true, DataSize: 64},
		},
		OperandEncodingTable: []OperandEncoding{{Encoding: "A", Operands: [4]string{"None", "None", "N/A", "N/A"}}},
	},

	{
		MnemonicTable: []Mnemonic{
			// Set the global interrupt flag (AMD-V).
			{Opcode: "0F 01 DC", Instruction: "STGI", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
		},
	},

	{
		// Explicit-operands verdions of STOS.
		MnemonicTable: []Mnemonic{
			{Opcode: "AA", Instruction: "STOS [es:edi:8], AL", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", DataSize: 8},
			{Opcode: "AB", Instruction: "STOS [es:edi:16], AX", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 16},
			{Opcode: "AB", Instruction: "STOS [es:edi:32], EAX", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid", OperandSize: true, DataSize: 32},
			{Opcode: "REX.W AB", Instruction: "STOS [rdi:64], RAX", OperandEncoding: "A", Mode64: "Valid", Mode32: "Invalid", Mode16: "Invalid", OperandSize: true, DataSize: 64},
		},
		OperandEncodingTable: []OperandEncoding{{Encoding: "A", Operands: [4]string{"None", "None", "N/A", "N/A"}}},
	},

	{
		// Intel VT-X.
		MnemonicTable: []Mnemonic{
			// Call into the VMM (VT-X).
			{Opcode: "0F 01 C1", Instruction: "VMCALL", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},

			// Clear virtual machine control structure (VT-X).
			{Opcode: "66 0F C7 /6", Instruction: "VMCLEAR m64", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},

			// Launch virtual machine managed by current VMCS (VT-X).
			{Opcode: "0F 01 C2", Instruction: "VMLAUNCH", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},

			// Load/store pointer to virtual machine control structure (VT-X).
			{Opcode: "NP 0F C7 /6", Instruction: "VMPTRLD m64", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
			{Opcode: "NP 0F C7 /7", Instruction: "VMPTRST m64", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},

			// Resume virtual machine managed by current VMCS (VT-X).
			{Opcode: "0F 01 C3", Instruction: "VMRESUME", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},

			// Leave VMX operation (VT-X).
			{Opcode: "0F 01 C4", Instruction: "VMXOFF", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},

			// Enter VMX root operation (VT-X).
			{Opcode: "F3 0F C7 /6", Instruction: "VMXON m64", OperandEncoding: "A", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},
		},
		OperandEncodingTable: []OperandEncoding{{Encoding: "A", Operands: [4]string{"ModRM:r/m", "N/A", "N/A", "N/A"}}},
	},

	{
		// AMD-V.
		MnemonicTable: []Mnemonic{
			// Verifiable startup of trusted software (AMD-V).
			{Opcode: "0F 01 DE", Instruction: "SKINIT <EAX>", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Invalid"},

			// Load a VM's state (AMD-V).
			{Opcode: "0F 01 DA", Instruction: "VMLOAD <EAX>", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Invalid", AddressSize: true},
			{Opcode: "0F 01 DA", Instruction: "VMLOAD <RAX>", OperandEncoding: "A", Mode64: "Valid", Mode32: "Invalid", Mode16: "Invalid", AddressSize: true},

			// Call into the VMM (AMD-V).
			{Opcode: "0F 01 D9", Instruction: "VMMCALL", Mode64: "Valid", Mode32: "Valid", Mode16: "Valid"},

			// Perform a VM enter (AMD-V).
			{Opcode: "0F 01 D8", Instruction: "VMRUN <EAX>", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Invalid", AddressSize: true},
			{Opcode: "0F 01 D8", Instruction: "VMRUN <RAX>", OperandEncoding: "A", Mode64: "Valid", Mode32: "Invalid", Mode16: "Invalid", AddressSize: true},

			// Save a VM's state (AMD-V).
			{Opcode: "0F 01 DB", Instruction: "VMSAVE <EAX>", OperandEncoding: "A", Mode64: "Invalid", Mode32: "Valid", Mode16: "Invalid", AddressSize: true},
			{Opcode: "0F 01 DB", Instruction: "VMSAVE <RAX>", OperandEncoding: "A", Mode64: "Valid", Mode32: "Invalid", Mode16: "Invalid", AddressSize: true},
		},
		OperandEncodingTable: []OperandEncoding{{Encoding: "A", Operands: [4]string{"Implicit", "N/A", "N/A", "N/A"}}},
	},
}
