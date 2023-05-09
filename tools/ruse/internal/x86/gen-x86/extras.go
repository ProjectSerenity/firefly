// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"firefly-os.dev/tools/ruse/internal/x86"
)

func enc(s string) *x86.Encoding {
	encoding, err := x86.ParseEncoding(s)
	if err != nil {
		panic(err.Error())
	}

	return encoding
}

func params(v ...*x86.Parameter) []*x86.Parameter {
	return v
}

// Extras contains additional instructions
// not included in Go's x86.csv. These are
// a mix of AMD-specific instructions like
// VMLOAD and alternative mnemonics, such
// as MOVS m8, m8 (which is in Go's x86.csv
// as MOVSB).
var Extras = []*x86.Instruction{
	// Explicit-operands version of BLENDVPD/BLENDVPS/PBLENDVB.
	{Mnemonic: "blendvpd", Syntax: "BLENDVPD xmm1, xmm2/m128, XMM0", Encoding: enc("66 0F 38 15 /r"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2, x86.ParamXMM0), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "blendvpd", Syntax: "BLENDVPD xmm1, xmm2/m128, XMM0", Encoding: enc("66 0F 38 15 /r"), Parameters: params(x86.ParamXMM1, x86.ParamM128, x86.ParamXMM0), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "blendvps", Syntax: "BLENDVPS xmm1, xmm2/m128, XMM0", Encoding: enc("66 0F 38 14 /r"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2, x86.ParamXMM0), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "blendvps", Syntax: "BLENDVPS xmm1, xmm2/m128, XMM0", Encoding: enc("66 0F 38 14 /r"), Parameters: params(x86.ParamXMM1, x86.ParamM128, x86.ParamXMM0), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "pblendvb", Syntax: "PBLENDVB xmm1, xmm2/m128, XMM0", Encoding: enc("66 0F 38 10 /r"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2, x86.ParamXMM0), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "pblendvb", Syntax: "PBLENDVB xmm1, xmm2/m128, XMM0", Encoding: enc("66 0F 38 10 /r"), Parameters: params(x86.ParamXMM1, x86.ParamM128, x86.ParamXMM0), Mode16: true, Mode32: true, Mode64: true},

	// Clear the global interrupt flag (AMD-V).
	{Mnemonic: "clgi", Syntax: "CLGI", Encoding: enc("0F 01 DD"), Mode16: true, Mode32: true, Mode64: true},

	// Specialised mnemonics for CMPPD xmm1, xmm2/m128, X.
	{Mnemonic: "cmpeqpd", Syntax: "CMPEQPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 00"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpeqpd", Syntax: "CMPEQPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 00"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpltpd", Syntax: "CMPLTPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 01"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpltpd", Syntax: "CMPLTPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 01"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmplepd", Syntax: "CMPLEPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 02"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmplepd", Syntax: "CMPLEPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 02"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpunordpd", Syntax: "CMPUNORDPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 03"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpunordpd", Syntax: "CMPUNORDPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 03"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpneqpd", Syntax: "CMPNEQPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 04"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpneqpd", Syntax: "CMPNEQPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 04"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnltpd", Syntax: "CMPNLTPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 05"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnltpd", Syntax: "CMPNLTPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 05"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnlepd", Syntax: "CMPNLEPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 06"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnlepd", Syntax: "CMPNLEPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 06"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpordpd", Syntax: "CMPORDPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 07"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpordpd", Syntax: "CMPORDPD xmm1, xmm2/m128", Encoding: enc("66 0F C2 /r 07"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},

	// Specialised mnemonics for CMPPS xmm1, xmm2/m128, X.
	{Mnemonic: "cmpeqps", Syntax: "CMPEQPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 00"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpeqps", Syntax: "CMPEQPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 00"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpltps", Syntax: "CMPLTPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 01"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpltps", Syntax: "CMPLTPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 01"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpleps", Syntax: "CMPLEPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 02"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpleps", Syntax: "CMPLEPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 02"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpunordps", Syntax: "CMPUNORDPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 03"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpunordps", Syntax: "CMPUNORDPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 03"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpneqps", Syntax: "CMPNEQPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 04"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpneqps", Syntax: "CMPNEQPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 04"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnltps", Syntax: "CMPNLTPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 05"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnltps", Syntax: "CMPNLTPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 05"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnleps", Syntax: "CMPNLEPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 06"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnleps", Syntax: "CMPNLEPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 06"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpordps", Syntax: "CMPORDPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 07"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpordps", Syntax: "CMPORDPS xmm1, xmm2/m128", Encoding: enc("0F C2 /r 07"), Parameters: params(x86.ParamXMM1, x86.ParamM128), Mode16: true, Mode32: true, Mode64: true},

	// Specialised mnemonics for CMPSD xmm1, xmm2/m64, X.
	{Mnemonic: "cmpeqsd", Syntax: "CMPEQSD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 00"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpeqsd", Syntax: "CMPEQSD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 00"), Parameters: params(x86.ParamXMM1, x86.ParamM64), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpltsd", Syntax: "CMPLTSD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 01"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpltsd", Syntax: "CMPLTSD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 01"), Parameters: params(x86.ParamXMM1, x86.ParamM64), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmplesd", Syntax: "CMPLESD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 02"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmplesd", Syntax: "CMPLESD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 02"), Parameters: params(x86.ParamXMM1, x86.ParamM64), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpunordsd", Syntax: "CMPUNORDSD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 03"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpunordsd", Syntax: "CMPUNORDSD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 03"), Parameters: params(x86.ParamXMM1, x86.ParamM64), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpneqsd", Syntax: "CMPNEQSD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 04"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpneqsd", Syntax: "CMPNEQSD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 04"), Parameters: params(x86.ParamXMM1, x86.ParamM64), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnltsd", Syntax: "CMPNLTSD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 05"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnltsd", Syntax: "CMPNLTSD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 05"), Parameters: params(x86.ParamXMM1, x86.ParamM64), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnlesd", Syntax: "CMPNLESD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 06"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnlesd", Syntax: "CMPNLESD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 06"), Parameters: params(x86.ParamXMM1, x86.ParamM64), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpordsd", Syntax: "CMPORDSD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 07"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpordsd", Syntax: "CMPORDSD xmm1, xmm2/m64", Encoding: enc("F2 0F C2 /r 07"), Parameters: params(x86.ParamXMM1, x86.ParamM64), Mode16: true, Mode32: true, Mode64: true},

	// Specialised mnemonics for CMPSS xmm1, xmm2/m32, X.
	{Mnemonic: "cmpeqss", Syntax: "CMPEQSS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 00"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpeqss", Syntax: "CMPEQSS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 00"), Parameters: params(x86.ParamXMM1, x86.ParamM32), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpltss", Syntax: "CMPLTSS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 01"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpltss", Syntax: "CMPLTSS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 01"), Parameters: params(x86.ParamXMM1, x86.ParamM32), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpless", Syntax: "CMPLESS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 02"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpless", Syntax: "CMPLESS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 02"), Parameters: params(x86.ParamXMM1, x86.ParamM32), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpunordss", Syntax: "CMPUNORDSS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 03"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpunordss", Syntax: "CMPUNORDSS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 03"), Parameters: params(x86.ParamXMM1, x86.ParamM32), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpneqss", Syntax: "CMPNEQSS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 04"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpneqss", Syntax: "CMPNEQSS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 04"), Parameters: params(x86.ParamXMM1, x86.ParamM32), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnltss", Syntax: "CMPNLTSS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 05"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnltss", Syntax: "CMPNLTSS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 05"), Parameters: params(x86.ParamXMM1, x86.ParamM32), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnless", Syntax: "CMPNLESS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 06"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpnless", Syntax: "CMPNLESS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 06"), Parameters: params(x86.ParamXMM1, x86.ParamM32), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpordss", Syntax: "CMPORDSS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 07"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "cmpordss", Syntax: "CMPORDSS xmm1, xmm2/m32", Encoding: enc("F3 0F C2 /r 07"), Parameters: params(x86.ParamXMM1, x86.ParamM32), Mode16: true, Mode32: true, Mode64: true},

	// Explicit-operands versions of CMPSB, CMPSW, CMPSD, CMPSQ.
	{Mnemonic: "cmps", Syntax: "CMPS [ds:esi:8], [es:edi:8]", Encoding: enc("A6"), Parameters: params(x86.ParamStrSrc8, x86.ParamStrDst8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "cmps", Syntax: "CMPS [ds:esi:16], [es:edi:16]", Encoding: enc("A7"), Parameters: params(x86.ParamStrSrc16, x86.ParamStrDst16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "cmps", Syntax: "CMPS [ds:esi:32], [es:edi:32]", Encoding: enc("A7"), Parameters: params(x86.ParamStrSrc32, x86.ParamStrDst32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "cmps", Syntax: "CMPS [rsi:64], [rdi:64]", Encoding: enc("REX.W A7"), Parameters: params(x86.ParamStrSrc64, x86.ParamStrDst64), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},
	{Mnemonic: "cmpsb", Syntax: "CMPSB [ds:esi:8], [es:edi:8]", Encoding: enc("A6"), Parameters: params(x86.ParamStrSrc8, x86.ParamStrDst8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "cmpsw", Syntax: "CMPSW [ds:esi:16], [es:edi:16]", Encoding: enc("A7"), Parameters: params(x86.ParamStrSrc16, x86.ParamStrDst16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "cmpsd", Syntax: "CMPSD [ds:esi:32], [es:edi:32]", Encoding: enc("A7"), Parameters: params(x86.ParamStrSrc32, x86.ParamStrDst32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "cmpsq", Syntax: "CMPSQ [rsi:64], [rdi:64]", Encoding: enc("REX.W A7"), Parameters: params(x86.ParamStrSrc64, x86.ParamStrDst64), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},

	// The 64-bit version of ENQCMD(S) r32/r64, m512.
	{Mnemonic: "enqcmd", Syntax: "ENQCMD r32/r64, m512", Encoding: enc("F2 0F 38 F8 !(11):rrr:bbb /r"), Parameters: params(x86.ParamR64, x86.ParamM512), Mode64: true},
	{Mnemonic: "enqcmds", Syntax: "ENQCMDS r32/r64, m512", Encoding: enc("F2 0F 38 F8 !(11):rrr:bbb /r"), Parameters: params(x86.ParamR64, x86.ParamM512), Mode64: true},

	// Extract field from register.
	{Mnemonic: "extrq", Syntax: "EXTRQ xmm2, imm8, imm8", Encoding: enc("66 0F 78 /0 ib ib"), Parameters: params(x86.ParamXMM2, x86.ParamImm8, x86.ParamImm8), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "extrq", Syntax: "EXTRQ xmm1, xmm2", Encoding: enc("66 0F 79 /r"), Parameters: params(x86.ParamXMM1, x86.ParamXMM2), Mode16: true, Mode32: true, Mode64: true},

	// Fast exit multimedia state.
	{Mnemonic: "femms", Syntax: "FEMMS", Encoding: enc("0F 0E"), Mode16: true, Mode32: true, Mode64: true},

	// Push loge2 / +0.0 onto the FPU register stack.
	{Mnemonic: "fldln2", Syntax: "FLDLN2", Encoding: enc("D9 ED"), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "fldz", Syntax: "FLDZ", Encoding: enc("D9 EE"), Mode16: true, Mode32: true, Mode64: true},

	// Perform an SMX function.
	{Mnemonic: "getsec", Syntax: "GETSEC", Encoding: enc("NP 0F 37"), Mode16: true, Mode32: true, Mode64: true},

	// Explicit-operand versions of INSB, INSW, INSD, INSQ.
	{Mnemonic: "ins", Syntax: "INS [es:edi:8], DX", Encoding: enc("6C"), Parameters: params(x86.ParamStrDst8, x86.ParamDX), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "ins", Syntax: "INS [es:edi:16], DX", Encoding: enc("6D"), Parameters: params(x86.ParamStrDst16, x86.ParamDX), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "ins", Syntax: "INS [es:edi:32], DX", Encoding: enc("6D"), Parameters: params(x86.ParamStrDst32, x86.ParamDX), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "insb", Syntax: "INSB [es:edi:8], DX", Encoding: enc("6C"), Parameters: params(x86.ParamStrDst8, x86.ParamDX), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "insw", Syntax: "INSW [es:edi:16], DX", Encoding: enc("6D"), Parameters: params(x86.ParamStrDst16, x86.ParamDX), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "insd", Syntax: "INSD [es:edi:32], DX", Encoding: enc("6D"), Parameters: params(x86.ParamStrDst32, x86.ParamDX), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},

	// Explicit-operand version of LODSB, LODSW, LODSD, LODSQ, both with and without the implicit destination accumulator register.
	{Mnemonic: "lods", Syntax: "LODS [ds:esi:8]", Encoding: enc("AC"), Parameters: params(x86.ParamStrSrc8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "lods", Syntax: "LODS AL, [ds:esi:8]", Encoding: enc("AC"), Parameters: params(x86.ParamAL, x86.ParamStrSrc8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "lods", Syntax: "LODS [ds:esi:16]", Encoding: enc("AD"), Parameters: params(x86.ParamStrSrc16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "lods", Syntax: "LODS AX, [ds:esi:16]", Encoding: enc("AD"), Parameters: params(x86.ParamAX, x86.ParamStrSrc16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "lods", Syntax: "LODS [ds:esi:32]", Encoding: enc("AD"), Parameters: params(x86.ParamStrSrc32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "lods", Syntax: "LODS EAX, [ds:esi:32]", Encoding: enc("AD"), Parameters: params(x86.ParamEAX, x86.ParamStrSrc32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "lods", Syntax: "LODS [rsi:64]", Encoding: enc("REX.W AD"), Parameters: params(x86.ParamStrSrc64), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},
	{Mnemonic: "lods", Syntax: "LODS RAX, [rsi:64]", Encoding: enc("REX.W AD"), Parameters: params(x86.ParamRAX, x86.ParamStrSrc64), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},
	{Mnemonic: "lodsb", Syntax: "LODSB [ds:esi:8]", Encoding: enc("AC"), Parameters: params(x86.ParamStrSrc8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "lodsb", Syntax: "LODSB AL, [ds:esi:8]", Encoding: enc("AC"), Parameters: params(x86.ParamAL, x86.ParamStrSrc8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "lodsw", Syntax: "LODSW [ds:esi:16]", Encoding: enc("AD"), Parameters: params(x86.ParamStrSrc16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "lodsw", Syntax: "LODSW AX, [ds:esi:16]", Encoding: enc("AD"), Parameters: params(x86.ParamAX, x86.ParamStrSrc16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "lodsd", Syntax: "LODSD [ds:esi:32]", Encoding: enc("AD"), Parameters: params(x86.ParamStrSrc32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "lodsd", Syntax: "LODSD EAX, [ds:esi:32]", Encoding: enc("AD"), Parameters: params(x86.ParamEAX, x86.ParamStrSrc32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "lodsq", Syntax: "LODSQ [rsi:64]", Encoding: enc("REX.W AD"), Parameters: params(x86.ParamStrSrc64), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},
	{Mnemonic: "lodsq", Syntax: "LODSQ RAX, [rsi:64]", Encoding: enc("REX.W AD"), Parameters: params(x86.ParamRAX, x86.ParamStrSrc64), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},

	// Explicit-operand version of MOVSB, MOVSW, MOVSD, MOVSQ.
	{Mnemonic: "movs", Syntax: "MOVS [es:edi:8], [ds:esi:8]", Encoding: enc("A4"), Parameters: params(x86.ParamStrDst8, x86.ParamStrSrc8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "movs", Syntax: "MOVS [es:edi:16], [ds:esi:16]", Encoding: enc("A5"), Parameters: params(x86.ParamStrDst16, x86.ParamStrSrc16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "movs", Syntax: "MOVS [es:edi:32], [ds:esi:32]", Encoding: enc("A5"), Parameters: params(x86.ParamStrDst32, x86.ParamStrSrc32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "movs", Syntax: "MOVS [rdi:64], [rsi:64]", Encoding: enc("REX.W A5"), Parameters: params(x86.ParamStrDst64, x86.ParamStrSrc64), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},
	{Mnemonic: "movsb", Syntax: "MOVSB [es:edi:8], [ds:esi:8]", Encoding: enc("A4"), Parameters: params(x86.ParamStrDst8, x86.ParamStrSrc8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "movsw", Syntax: "MOVSW [es:edi:16], [ds:esi:16]", Encoding: enc("A5"), Parameters: params(x86.ParamStrDst16, x86.ParamStrSrc16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "movsd", Syntax: "MOVSD [es:edi:32], [ds:esi:32]", Encoding: enc("A5"), Parameters: params(x86.ParamStrDst32, x86.ParamStrSrc32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "movsq", Syntax: "MOVSQ [rdi:64], [rsi:64]", Encoding: enc("REX.W A5"), Parameters: params(x86.ParamStrDst64, x86.ParamStrSrc64), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},

	// The 64-bit version of MOVDIR64B r16/r32/r64, m512.
	{Mnemonic: "movdir64b", Syntax: "MOVDIR64B r16/r32/r64, m512", Encoding: enc("66 0F 38 F8 /r"), Parameters: params(x86.ParamR64, x86.ParamM512), Mode64: true},

	// Do nothing.
	{Mnemonic: "nop", Syntax: "NOP m16", Encoding: enc("0F 1F /0"), Parameters: params(x86.ParamM16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "nop", Syntax: "NOP m32", Encoding: enc("0F 1F /0"), Parameters: params(x86.ParamM32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},

	// Explicit-operand version of OUTSB, OUTSW, OUTSD, OUTSQ.
	{Mnemonic: "outs", Syntax: "OUTS DX, [ds:esi:8]", Encoding: enc("6E"), Parameters: params(x86.ParamDX, x86.ParamStrSrc8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "outs", Syntax: "OUTS DX, [ds:esi:16]", Encoding: enc("6F"), Parameters: params(x86.ParamDX, x86.ParamStrSrc16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "outs", Syntax: "OUTS DX, [ds:esi:32]", Encoding: enc("6F"), Parameters: params(x86.ParamDX, x86.ParamStrSrc32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "outsb", Syntax: "OUTSB DX, [ds:esi:8]", Encoding: enc("6E"), Parameters: params(x86.ParamDX, x86.ParamStrSrc8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "outsw", Syntax: "OUTSW DX, [ds:esi:16]", Encoding: enc("6F"), Parameters: params(x86.ParamDX, x86.ParamStrSrc16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "outsd", Syntax: "OUTSD DX, [ds:esi:32]", Encoding: enc("6F"), Parameters: params(x86.ParamDX, x86.ParamStrSrc32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},

	// Versions of POP/PUSH ES/CS/SS/DS/FS/GS that increment the stack pointer by 2/4/8.
	{Mnemonic: "popw", Syntax: "POPW ES", Encoding: enc("07"), Parameters: params(x86.ParamES), Mode16: true, Mode32: true, Mode64: false, OperandSize: true, DataSize: 16},
	{Mnemonic: "popw", Syntax: "POPW SS", Encoding: enc("17"), Parameters: params(x86.ParamSS), Mode16: true, Mode32: true, Mode64: false, OperandSize: true, DataSize: 16},
	{Mnemonic: "popw", Syntax: "POPW DS", Encoding: enc("1F"), Parameters: params(x86.ParamDS), Mode16: true, Mode32: true, Mode64: false, OperandSize: true, DataSize: 16},
	{Mnemonic: "popw", Syntax: "POPW FS", Encoding: enc("0F A1"), Parameters: params(x86.ParamFS), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "popw", Syntax: "POPW GS", Encoding: enc("0F A9"), Parameters: params(x86.ParamGS), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "popd", Syntax: "POPD ES", Encoding: enc("07"), Parameters: params(x86.ParamES), Mode16: false, Mode32: true, Mode64: false, OperandSize: true, DataSize: 32},
	{Mnemonic: "popd", Syntax: "POPD SS", Encoding: enc("17"), Parameters: params(x86.ParamSS), Mode16: false, Mode32: true, Mode64: false, OperandSize: true, DataSize: 32},
	{Mnemonic: "popd", Syntax: "POPD DS", Encoding: enc("1F"), Parameters: params(x86.ParamDS), Mode16: false, Mode32: true, Mode64: false, OperandSize: true, DataSize: 32},
	{Mnemonic: "popd", Syntax: "POPD FS", Encoding: enc("0F A1"), Parameters: params(x86.ParamFS), Mode16: false, Mode32: true, Mode64: false, OperandSize: true, DataSize: 32},
	{Mnemonic: "popd", Syntax: "POPD GS", Encoding: enc("0F A9"), Parameters: params(x86.ParamGS), Mode16: false, Mode32: true, Mode64: false, OperandSize: true, DataSize: 32},
	{Mnemonic: "popq", Syntax: "POPQ FS", Encoding: enc("REX.W 0F A1"), Parameters: params(x86.ParamFS), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},
	{Mnemonic: "popq", Syntax: "POPQ GS", Encoding: enc("REX.W 0F A9"), Parameters: params(x86.ParamGS), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},
	{Mnemonic: "pushw", Syntax: "PUSHW ES", Encoding: enc("06"), Parameters: params(x86.ParamES), Mode16: true, Mode32: true, Mode64: false, OperandSize: true, DataSize: 16},
	{Mnemonic: "pushw", Syntax: "PUSHW CS", Encoding: enc("0E"), Parameters: params(x86.ParamCS), Mode16: true, Mode32: true, Mode64: false, OperandSize: true, DataSize: 16},
	{Mnemonic: "pushw", Syntax: "PUSHW SS", Encoding: enc("16"), Parameters: params(x86.ParamSS), Mode16: true, Mode32: true, Mode64: false, OperandSize: true, DataSize: 16},
	{Mnemonic: "pushw", Syntax: "PUSHW DS", Encoding: enc("1E"), Parameters: params(x86.ParamDS), Mode16: true, Mode32: true, Mode64: false, OperandSize: true, DataSize: 16},
	{Mnemonic: "pushw", Syntax: "PUSHW FS", Encoding: enc("0F A0"), Parameters: params(x86.ParamFS), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "pushw", Syntax: "PUSHW GS", Encoding: enc("0F A8"), Parameters: params(x86.ParamGS), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "pushd", Syntax: "PUSHD ES", Encoding: enc("06"), Parameters: params(x86.ParamES), Mode16: true, Mode32: true, Mode64: false, OperandSize: true, DataSize: 32},
	{Mnemonic: "pushd", Syntax: "PUSHD CS", Encoding: enc("0E"), Parameters: params(x86.ParamCS), Mode16: true, Mode32: true, Mode64: false, OperandSize: true, DataSize: 32},
	{Mnemonic: "pushd", Syntax: "PUSHD SS", Encoding: enc("16"), Parameters: params(x86.ParamSS), Mode16: true, Mode32: true, Mode64: false, OperandSize: true, DataSize: 32},
	{Mnemonic: "pushd", Syntax: "PUSHD DS", Encoding: enc("1E"), Parameters: params(x86.ParamDS), Mode16: true, Mode32: true, Mode64: false, OperandSize: true, DataSize: 32},
	{Mnemonic: "pushd", Syntax: "PUSHD FS", Encoding: enc("0F A0"), Parameters: params(x86.ParamFS), Mode16: true, Mode32: true, Mode64: false, OperandSize: true, DataSize: 32},
	{Mnemonic: "pushd", Syntax: "PUSHD GS", Encoding: enc("0F A8"), Parameters: params(x86.ParamGS), Mode16: true, Mode32: true, Mode64: false, OperandSize: true, DataSize: 32},
	{Mnemonic: "pushq", Syntax: "PUSHQ FS", Encoding: enc("REX.W 0F A0"), Parameters: params(x86.ParamFS), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},
	{Mnemonic: "pushq", Syntax: "PUSHQ GS", Encoding: enc("REX.W 0F A8"), Parameters: params(x86.ParamGS), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},

	// Explicit expressions of PUSH imm16 and PUSH imm32.
	{Mnemonic: "push", Syntax: "PUSH imm16", Encoding: enc("68 iw"), Parameters: params(x86.ParamImm16), Mode16: true, Mode32: false, Mode64: false, DataSize: 16},
	{Mnemonic: "push", Syntax: "PUSH imm32", Encoding: enc("68 id"), Parameters: params(x86.ParamImm32), Mode16: false, Mode32: true, Mode64: true, DataSize: 32},
	{Mnemonic: "pushw", Syntax: "PUSHW imm16", Encoding: enc("68 iw"), Parameters: params(x86.ParamImm16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "pushd", Syntax: "PUSHD imm32", Encoding: enc("68 id"), Parameters: params(x86.ParamImm32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},

	// Explicit-operand version of SCASB, SCASW, SCASD, SCASQ, both with and without the implicit destination accumulator register.
	{Mnemonic: "scas", Syntax: "SCAS [es:edi:8]", Encoding: enc("AE"), Parameters: params(x86.ParamStrDst8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "scas", Syntax: "SCAS AL, [es:edi:8]", Encoding: enc("AE"), Parameters: params(x86.ParamAL, x86.ParamStrDst8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "scas", Syntax: "SCAS [es:edi:16]", Encoding: enc("AF"), Parameters: params(x86.ParamStrDst16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "scas", Syntax: "SCAS AX, [es:edi:16]", Encoding: enc("AF"), Parameters: params(x86.ParamAX, x86.ParamStrDst16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "scas", Syntax: "SCAS [es:edi:32]", Encoding: enc("AF"), Parameters: params(x86.ParamStrDst32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "scas", Syntax: "SCAS EAX, [es:edi:32]", Encoding: enc("AF"), Parameters: params(x86.ParamEAX, x86.ParamStrDst32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "scas", Syntax: "SCAS [rdi:64]", Encoding: enc("REX.W AF"), Parameters: params(x86.ParamStrDst64), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},
	{Mnemonic: "scas", Syntax: "SCAS RAX, [rdi:64]", Encoding: enc("REX.W AF"), Parameters: params(x86.ParamRAX, x86.ParamStrDst64), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},
	{Mnemonic: "scasb", Syntax: "SCASB [es:edi:8]", Encoding: enc("AE"), Parameters: params(x86.ParamStrDst8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "scasb", Syntax: "SCASB AL, [es:edi:8]", Encoding: enc("AE"), Parameters: params(x86.ParamAL, x86.ParamStrDst8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "scasw", Syntax: "SCASW [es:edi:16]", Encoding: enc("AF"), Parameters: params(x86.ParamStrDst16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "scasw", Syntax: "SCASW AX, [es:edi:16]", Encoding: enc("AF"), Parameters: params(x86.ParamAX, x86.ParamStrDst16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "scasd", Syntax: "SCASD [es:edi:32]", Encoding: enc("AF"), Parameters: params(x86.ParamStrDst32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "scasd", Syntax: "SCASD EAX, [es:edi:32]", Encoding: enc("AF"), Parameters: params(x86.ParamEAX, x86.ParamStrDst32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "scasq", Syntax: "SCASQ [rdi:64]", Encoding: enc("REX.W AF"), Parameters: params(x86.ParamStrDst64), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},
	{Mnemonic: "scasq", Syntax: "SCASQ RAX, [rdi:64]", Encoding: enc("REX.W AF"), Parameters: params(x86.ParamRAX, x86.ParamStrDst64), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},

	// Verifiable startup of trusted software (AMD-V).
	{Mnemonic: "skinit", Syntax: "SKINIT EAX", Encoding: enc("0F 01 DE"), Parameters: params(x86.ParamEAX), Mode16: false, Mode32: true, Mode64: false},

	// Store the local descriptor table register.
	{Mnemonic: "sldt", Syntax: "SLDT m16", Encoding: enc("0F 00 /0"), Parameters: params(x86.ParamM16), Mode16: true, Mode32: true, Mode64: true},

	// Store the machine status word.
	{Mnemonic: "smsw", Syntax: "SMSW m16", Encoding: enc("0F 01 /4"), Parameters: params(x86.ParamM16), Mode16: true, Mode32: true, Mode64: true},

	// Set the global interrupt flag (AMD-V).
	{Mnemonic: "stgi", Syntax: "STGI", Encoding: enc("0F 01 DC"), Mode16: true, Mode32: true, Mode64: true},

	// Explicit-operand version of STOSB, STOSW, STOSD, STOSQ, both with and without the implicit destination accumulator register.
	{Mnemonic: "stos", Syntax: "STOS [es:edi:8]", Encoding: enc("AA"), Parameters: params(x86.ParamStrDst8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "stos", Syntax: "STOS [es:edi:8], AL", Encoding: enc("AA"), Parameters: params(x86.ParamStrDst8, x86.ParamAL), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "stos", Syntax: "STOS [es:edi:16]", Encoding: enc("AB"), Parameters: params(x86.ParamStrDst16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "stos", Syntax: "STOS [es:edi:16], AX", Encoding: enc("AB"), Parameters: params(x86.ParamStrDst16, x86.ParamAX), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "stos", Syntax: "STOS [es:edi:32]", Encoding: enc("AB"), Parameters: params(x86.ParamStrDst32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "stos", Syntax: "STOS [es:edi:32], EAX", Encoding: enc("AB"), Parameters: params(x86.ParamStrDst32, x86.ParamEAX), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "stos", Syntax: "STOS [rdi:64]", Encoding: enc("REX.W AB"), Parameters: params(x86.ParamStrDst64), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},
	{Mnemonic: "stos", Syntax: "STOS [rdi:64], RAX", Encoding: enc("REX.W AB"), Parameters: params(x86.ParamStrDst64, x86.ParamRAX), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},
	{Mnemonic: "stosb", Syntax: "STOSB [es:edi:8]", Encoding: enc("AA"), Parameters: params(x86.ParamStrDst8), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "stosb", Syntax: "STOSB [es:edi:8], AL", Encoding: enc("AA"), Parameters: params(x86.ParamStrDst8, x86.ParamAL), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 8},
	{Mnemonic: "stosw", Syntax: "STOSW [es:edi:16]", Encoding: enc("AB"), Parameters: params(x86.ParamStrDst16), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "stosw", Syntax: "STOSW [es:edi:16], AX", Encoding: enc("AB"), Parameters: params(x86.ParamStrDst16, x86.ParamAX), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 16},
	{Mnemonic: "stosd", Syntax: "STOSD [es:edi:32]", Encoding: enc("AB"), Parameters: params(x86.ParamStrDst32), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "stosd", Syntax: "STOSD [es:edi:32], EAX", Encoding: enc("AB"), Parameters: params(x86.ParamStrDst32, x86.ParamEAX), Mode16: true, Mode32: true, Mode64: true, OperandSize: true, DataSize: 32},
	{Mnemonic: "stosq", Syntax: "STOSQ [rdi:64]", Encoding: enc("REX.W AB"), Parameters: params(x86.ParamStrDst64), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},
	{Mnemonic: "stosq", Syntax: "STOSQ [rdi:64], RAX", Encoding: enc("REX.W AB"), Parameters: params(x86.ParamStrDst64, x86.ParamRAX), Mode16: false, Mode32: false, Mode64: true, OperandSize: true, DataSize: 64},

	// Store the task register.
	{Mnemonic: "str", Syntax: "STR m16", Encoding: enc("0F 00 /1"), Parameters: params(x86.ParamM16), Mode16: true, Mode32: true, Mode64: true},

	// The 64-bit version of UMONITOR rmr16/rmr32/rmr64.
	{Mnemonic: "umonitor", Syntax: "UMONITOR rmr16/rmr32/rmr64", Encoding: enc("F3 0F AE /6"), Parameters: params(x86.ParamRmr64), Mode64: true},

	// Call into the VMM (VT-X).
	{Mnemonic: "vmcall", Syntax: "VMCALL", Encoding: enc("0F 01 C1"), Mode16: true, Mode32: true, Mode64: true},

	// Clear virtual machine control structure (VT-X).
	{Mnemonic: "vmclear", Syntax: "VMCLEAR m64", Encoding: enc("66 0F C7 /6"), Parameters: params(x86.ParamM64), Mode16: true, Mode32: true, Mode64: true},

	// Launch virtual machine managed by current VMCS (VT-X).
	{Mnemonic: "vmlaunch", Syntax: "VMLAUNCH", Encoding: enc("0F 01 C2"), Mode16: true, Mode32: true, Mode64: true},

	// Load a VM's state (AMD-V).
	{Mnemonic: "vmload", Syntax: "VMLOAD EAX", Encoding: enc("0F 01 DA"), Parameters: params(x86.ParamEAX), Mode16: false, Mode32: true, Mode64: false, AddressSize: true},
	{Mnemonic: "vmload", Syntax: "VMLOAD RAX", Encoding: enc("0F 01 DA"), Parameters: params(x86.ParamRAX), Mode16: false, Mode32: false, Mode64: true, AddressSize: true},

	// Call into the VMM (AMD-V).
	{Mnemonic: "vmmcall", Syntax: "VMMCALL", Encoding: enc("0F 01 D9"), Mode16: true, Mode32: true, Mode64: true},

	// Load/store pointer to virtual machine control structure (VT-X).
	{Mnemonic: "vmptrld", Syntax: "VMPTRLD m64", Encoding: enc("NP 0F C7 /6"), Parameters: params(x86.ParamM64), Mode16: true, Mode32: true, Mode64: true},
	{Mnemonic: "vmptrst", Syntax: "VMPTRST m64", Encoding: enc("NP 0F C7 /7"), Parameters: params(x86.ParamM64), Mode16: true, Mode32: true, Mode64: true},

	// Resume virtual machine managed by current VMCS (VT-X).
	{Mnemonic: "vmresume", Syntax: "VMRESUME", Encoding: enc("0F 01 C3"), Mode16: true, Mode32: true, Mode64: true},

	// Perform a VM enter (AMD-V).
	{Mnemonic: "vmrun", Syntax: "VMRUN EAX", Encoding: enc("0F 01 D8"), Parameters: params(x86.ParamEAX), Mode16: false, Mode32: true, Mode64: false, AddressSize: true},
	{Mnemonic: "vmrun", Syntax: "VMRUN RAX", Encoding: enc("0F 01 D8"), Parameters: params(x86.ParamRAX), Mode16: false, Mode32: false, Mode64: true, AddressSize: true},

	// Save a VM's state (AMD-V).
	{Mnemonic: "vmsave", Syntax: "VMSAVE EAX", Encoding: enc("0F 01 DB"), Parameters: params(x86.ParamEAX), Mode16: false, Mode32: true, Mode64: false, AddressSize: true},
	{Mnemonic: "vmsave", Syntax: "VMSAVE RAX", Encoding: enc("0F 01 DB"), Parameters: params(x86.ParamRAX), Mode16: false, Mode32: false, Mode64: true, AddressSize: true},

	// Leave VMX operation (VT-X).
	{Mnemonic: "vmxoff", Syntax: "VMXOFF", Encoding: enc("0F 01 C4"), Mode16: true, Mode32: true, Mode64: true},

	// Enter VMX root operation (VT-X).
	{Mnemonic: "vmxon", Syntax: "VMXON m64", Encoding: enc("F3 0F C7 /6"), Parameters: params(x86.ParamM64), Mode16: true, Mode32: true, Mode64: true},
}
