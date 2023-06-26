// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package a64

import (
	"fmt"
	"strings"
)

// Register contains information about
// an A64 register, including its size
// in bits (for fixed-size register
// groups) and whether it belongs to
// any register groups.
type Register struct {
	Name    string       `json:"name"`
	Type    RegisterType `json:"-"`
	Bits    int          `json:"-"`
	Aliases []string     `json:"-"`
}

func (r *Register) String() string    { return r.Name }
func (r *Register) UpperName() string { return strings.ToUpper(r.Name) }

var (
	// General-purpose registers.
	// 32-bit registers.
	W0  = &Register{Name: "w0", Type: TypeGeneralPurpose, Bits: 32}
	W1  = &Register{Name: "w1", Type: TypeGeneralPurpose, Bits: 32}
	W2  = &Register{Name: "w2", Type: TypeGeneralPurpose, Bits: 32}
	W3  = &Register{Name: "w3", Type: TypeGeneralPurpose, Bits: 32}
	W4  = &Register{Name: "w4", Type: TypeGeneralPurpose, Bits: 32}
	W5  = &Register{Name: "w5", Type: TypeGeneralPurpose, Bits: 32}
	W6  = &Register{Name: "w6", Type: TypeGeneralPurpose, Bits: 32}
	W7  = &Register{Name: "w7", Type: TypeGeneralPurpose, Bits: 32}
	W8  = &Register{Name: "w8", Type: TypeGeneralPurpose, Bits: 32}
	W9  = &Register{Name: "w9", Type: TypeGeneralPurpose, Bits: 32}
	W10 = &Register{Name: "w10", Type: TypeGeneralPurpose, Bits: 32}
	W11 = &Register{Name: "w11", Type: TypeGeneralPurpose, Bits: 32}
	W12 = &Register{Name: "w12", Type: TypeGeneralPurpose, Bits: 32}
	W13 = &Register{Name: "w13", Type: TypeGeneralPurpose, Bits: 32}
	W14 = &Register{Name: "w14", Type: TypeGeneralPurpose, Bits: 32}
	W15 = &Register{Name: "w15", Type: TypeGeneralPurpose, Bits: 32}
	W16 = &Register{Name: "w16", Type: TypeGeneralPurpose, Bits: 32}
	W17 = &Register{Name: "w17", Type: TypeGeneralPurpose, Bits: 32}
	W18 = &Register{Name: "w18", Type: TypeGeneralPurpose, Bits: 32}
	W19 = &Register{Name: "w19", Type: TypeGeneralPurpose, Bits: 32}
	W20 = &Register{Name: "w20", Type: TypeGeneralPurpose, Bits: 32}
	W21 = &Register{Name: "w21", Type: TypeGeneralPurpose, Bits: 32}
	W22 = &Register{Name: "w22", Type: TypeGeneralPurpose, Bits: 32}
	W23 = &Register{Name: "w23", Type: TypeGeneralPurpose, Bits: 32}
	W24 = &Register{Name: "w24", Type: TypeGeneralPurpose, Bits: 32}
	W25 = &Register{Name: "w25", Type: TypeGeneralPurpose, Bits: 32}
	W26 = &Register{Name: "w26", Type: TypeGeneralPurpose, Bits: 32}
	W27 = &Register{Name: "w27", Type: TypeGeneralPurpose, Bits: 32}
	W28 = &Register{Name: "w28", Type: TypeGeneralPurpose, Bits: 32}
	W29 = &Register{Name: "w29", Type: TypeGeneralPurpose, Bits: 32}
	W30 = &Register{Name: "w30", Type: TypeGeneralPurpose, Bits: 32}
	WZR = &Register{Name: "wzr", Type: TypeGeneralPurpose, Bits: 32}
	// 64-bit registers.
	X0  = &Register{Name: "x0", Type: TypeGeneralPurpose, Bits: 64}
	X1  = &Register{Name: "x1", Type: TypeGeneralPurpose, Bits: 64}
	X2  = &Register{Name: "x2", Type: TypeGeneralPurpose, Bits: 64}
	X3  = &Register{Name: "x3", Type: TypeGeneralPurpose, Bits: 64}
	X4  = &Register{Name: "x4", Type: TypeGeneralPurpose, Bits: 64}
	X5  = &Register{Name: "x5", Type: TypeGeneralPurpose, Bits: 64}
	X6  = &Register{Name: "x6", Type: TypeGeneralPurpose, Bits: 64}
	X7  = &Register{Name: "x7", Type: TypeGeneralPurpose, Bits: 64}
	X8  = &Register{Name: "x8", Type: TypeGeneralPurpose, Bits: 64}
	X9  = &Register{Name: "x9", Type: TypeGeneralPurpose, Bits: 64}
	X10 = &Register{Name: "x10", Type: TypeGeneralPurpose, Bits: 64}
	X11 = &Register{Name: "x11", Type: TypeGeneralPurpose, Bits: 64}
	X12 = &Register{Name: "x12", Type: TypeGeneralPurpose, Bits: 64}
	X13 = &Register{Name: "x13", Type: TypeGeneralPurpose, Bits: 64}
	X14 = &Register{Name: "x14", Type: TypeGeneralPurpose, Bits: 64}
	X15 = &Register{Name: "x15", Type: TypeGeneralPurpose, Bits: 64}
	X16 = &Register{Name: "x16", Type: TypeGeneralPurpose, Bits: 64}
	X17 = &Register{Name: "x17", Type: TypeGeneralPurpose, Bits: 64}
	X18 = &Register{Name: "x18", Type: TypeGeneralPurpose, Bits: 64}
	X19 = &Register{Name: "x19", Type: TypeGeneralPurpose, Bits: 64}
	X20 = &Register{Name: "x20", Type: TypeGeneralPurpose, Bits: 64}
	X21 = &Register{Name: "x21", Type: TypeGeneralPurpose, Bits: 64}
	X22 = &Register{Name: "x22", Type: TypeGeneralPurpose, Bits: 64}
	X23 = &Register{Name: "x23", Type: TypeGeneralPurpose, Bits: 64}
	X24 = &Register{Name: "x24", Type: TypeGeneralPurpose, Bits: 64}
	X25 = &Register{Name: "x25", Type: TypeGeneralPurpose, Bits: 64}
	X26 = &Register{Name: "x26", Type: TypeGeneralPurpose, Bits: 64}
	X27 = &Register{Name: "x27", Type: TypeGeneralPurpose, Bits: 64}
	X28 = &Register{Name: "x28", Type: TypeGeneralPurpose, Bits: 64}
	X29 = &Register{Name: "x29", Type: TypeGeneralPurpose, Bits: 64}
	X30 = &Register{Name: "x30", Type: TypeGeneralPurpose, Bits: 64, Aliases: []string{"lr"}}
	XZR = &Register{Name: "xzr", Type: TypeGeneralPurpose, Bits: 64}

	// Stack pointer.
	WSP = &Register{Name: "wsp", Type: TypeStackPointer, Bits: 32}
	SP  = &Register{Name: "sp", Type: TypeStackPointer, Bits: 64}

	// Floating point and vector registers.
	// 8-bit registers.
	B0  = &Register{Name: "b0", Type: TypeFloatingPoint, Bits: 8}
	B1  = &Register{Name: "b1", Type: TypeFloatingPoint, Bits: 8}
	B2  = &Register{Name: "b2", Type: TypeFloatingPoint, Bits: 8}
	B3  = &Register{Name: "b3", Type: TypeFloatingPoint, Bits: 8}
	B4  = &Register{Name: "b4", Type: TypeFloatingPoint, Bits: 8}
	B5  = &Register{Name: "b5", Type: TypeFloatingPoint, Bits: 8}
	B6  = &Register{Name: "b6", Type: TypeFloatingPoint, Bits: 8}
	B7  = &Register{Name: "b7", Type: TypeFloatingPoint, Bits: 8}
	B8  = &Register{Name: "b8", Type: TypeFloatingPoint, Bits: 8}
	B9  = &Register{Name: "b9", Type: TypeFloatingPoint, Bits: 8}
	B10 = &Register{Name: "b10", Type: TypeFloatingPoint, Bits: 8}
	B11 = &Register{Name: "b11", Type: TypeFloatingPoint, Bits: 8}
	B12 = &Register{Name: "b12", Type: TypeFloatingPoint, Bits: 8}
	B13 = &Register{Name: "b13", Type: TypeFloatingPoint, Bits: 8}
	B14 = &Register{Name: "b14", Type: TypeFloatingPoint, Bits: 8}
	B15 = &Register{Name: "b15", Type: TypeFloatingPoint, Bits: 8}
	B16 = &Register{Name: "b16", Type: TypeFloatingPoint, Bits: 8}
	B17 = &Register{Name: "b17", Type: TypeFloatingPoint, Bits: 8}
	B18 = &Register{Name: "b18", Type: TypeFloatingPoint, Bits: 8}
	B19 = &Register{Name: "b19", Type: TypeFloatingPoint, Bits: 8}
	B20 = &Register{Name: "b20", Type: TypeFloatingPoint, Bits: 8}
	B21 = &Register{Name: "b21", Type: TypeFloatingPoint, Bits: 8}
	B22 = &Register{Name: "b22", Type: TypeFloatingPoint, Bits: 8}
	B23 = &Register{Name: "b23", Type: TypeFloatingPoint, Bits: 8}
	B24 = &Register{Name: "b24", Type: TypeFloatingPoint, Bits: 8}
	B25 = &Register{Name: "b25", Type: TypeFloatingPoint, Bits: 8}
	B26 = &Register{Name: "b26", Type: TypeFloatingPoint, Bits: 8}
	B27 = &Register{Name: "b27", Type: TypeFloatingPoint, Bits: 8}
	B28 = &Register{Name: "b28", Type: TypeFloatingPoint, Bits: 8}
	B29 = &Register{Name: "b29", Type: TypeFloatingPoint, Bits: 8}
	B30 = &Register{Name: "b30", Type: TypeFloatingPoint, Bits: 8}
	B31 = &Register{Name: "b31", Type: TypeFloatingPoint, Bits: 8}
	// 16-bit registers.
	H0  = &Register{Name: "h0", Type: TypeFloatingPoint, Bits: 16}
	H1  = &Register{Name: "h1", Type: TypeFloatingPoint, Bits: 16}
	H2  = &Register{Name: "h2", Type: TypeFloatingPoint, Bits: 16}
	H3  = &Register{Name: "h3", Type: TypeFloatingPoint, Bits: 16}
	H4  = &Register{Name: "h4", Type: TypeFloatingPoint, Bits: 16}
	H5  = &Register{Name: "h5", Type: TypeFloatingPoint, Bits: 16}
	H6  = &Register{Name: "h6", Type: TypeFloatingPoint, Bits: 16}
	H7  = &Register{Name: "h7", Type: TypeFloatingPoint, Bits: 16}
	H8  = &Register{Name: "h8", Type: TypeFloatingPoint, Bits: 16}
	H9  = &Register{Name: "h9", Type: TypeFloatingPoint, Bits: 16}
	H10 = &Register{Name: "h10", Type: TypeFloatingPoint, Bits: 16}
	H11 = &Register{Name: "h11", Type: TypeFloatingPoint, Bits: 16}
	H12 = &Register{Name: "h12", Type: TypeFloatingPoint, Bits: 16}
	H13 = &Register{Name: "h13", Type: TypeFloatingPoint, Bits: 16}
	H14 = &Register{Name: "h14", Type: TypeFloatingPoint, Bits: 16}
	H15 = &Register{Name: "h15", Type: TypeFloatingPoint, Bits: 16}
	H16 = &Register{Name: "h16", Type: TypeFloatingPoint, Bits: 16}
	H17 = &Register{Name: "h17", Type: TypeFloatingPoint, Bits: 16}
	H18 = &Register{Name: "h18", Type: TypeFloatingPoint, Bits: 16}
	H19 = &Register{Name: "h19", Type: TypeFloatingPoint, Bits: 16}
	H20 = &Register{Name: "h20", Type: TypeFloatingPoint, Bits: 16}
	H21 = &Register{Name: "h21", Type: TypeFloatingPoint, Bits: 16}
	H22 = &Register{Name: "h22", Type: TypeFloatingPoint, Bits: 16}
	H23 = &Register{Name: "h23", Type: TypeFloatingPoint, Bits: 16}
	H24 = &Register{Name: "h24", Type: TypeFloatingPoint, Bits: 16}
	H25 = &Register{Name: "h25", Type: TypeFloatingPoint, Bits: 16}
	H26 = &Register{Name: "h26", Type: TypeFloatingPoint, Bits: 16}
	H27 = &Register{Name: "h27", Type: TypeFloatingPoint, Bits: 16}
	H28 = &Register{Name: "h28", Type: TypeFloatingPoint, Bits: 16}
	H29 = &Register{Name: "h29", Type: TypeFloatingPoint, Bits: 16}
	H30 = &Register{Name: "h30", Type: TypeFloatingPoint, Bits: 16}
	H31 = &Register{Name: "h31", Type: TypeFloatingPoint, Bits: 16}
	// 32-bit registers.
	S0  = &Register{Name: "s0", Type: TypeFloatingPoint, Bits: 32}
	S1  = &Register{Name: "s1", Type: TypeFloatingPoint, Bits: 32}
	S2  = &Register{Name: "s2", Type: TypeFloatingPoint, Bits: 32}
	S3  = &Register{Name: "s3", Type: TypeFloatingPoint, Bits: 32}
	S4  = &Register{Name: "s4", Type: TypeFloatingPoint, Bits: 32}
	S5  = &Register{Name: "s5", Type: TypeFloatingPoint, Bits: 32}
	S6  = &Register{Name: "s6", Type: TypeFloatingPoint, Bits: 32}
	S7  = &Register{Name: "s7", Type: TypeFloatingPoint, Bits: 32}
	S8  = &Register{Name: "s8", Type: TypeFloatingPoint, Bits: 32}
	S9  = &Register{Name: "s9", Type: TypeFloatingPoint, Bits: 32}
	S10 = &Register{Name: "s10", Type: TypeFloatingPoint, Bits: 32}
	S11 = &Register{Name: "s11", Type: TypeFloatingPoint, Bits: 32}
	S12 = &Register{Name: "s12", Type: TypeFloatingPoint, Bits: 32}
	S13 = &Register{Name: "s13", Type: TypeFloatingPoint, Bits: 32}
	S14 = &Register{Name: "s14", Type: TypeFloatingPoint, Bits: 32}
	S15 = &Register{Name: "s15", Type: TypeFloatingPoint, Bits: 32}
	S16 = &Register{Name: "s16", Type: TypeFloatingPoint, Bits: 32}
	S17 = &Register{Name: "s17", Type: TypeFloatingPoint, Bits: 32}
	S18 = &Register{Name: "s18", Type: TypeFloatingPoint, Bits: 32}
	S19 = &Register{Name: "s19", Type: TypeFloatingPoint, Bits: 32}
	S20 = &Register{Name: "s20", Type: TypeFloatingPoint, Bits: 32}
	S21 = &Register{Name: "s21", Type: TypeFloatingPoint, Bits: 32}
	S22 = &Register{Name: "s22", Type: TypeFloatingPoint, Bits: 32}
	S23 = &Register{Name: "s23", Type: TypeFloatingPoint, Bits: 32}
	S24 = &Register{Name: "s24", Type: TypeFloatingPoint, Bits: 32}
	S25 = &Register{Name: "s25", Type: TypeFloatingPoint, Bits: 32}
	S26 = &Register{Name: "s26", Type: TypeFloatingPoint, Bits: 32}
	S27 = &Register{Name: "s27", Type: TypeFloatingPoint, Bits: 32}
	S28 = &Register{Name: "s28", Type: TypeFloatingPoint, Bits: 32}
	S29 = &Register{Name: "s29", Type: TypeFloatingPoint, Bits: 32}
	S30 = &Register{Name: "s30", Type: TypeFloatingPoint, Bits: 32}
	S31 = &Register{Name: "s31", Type: TypeFloatingPoint, Bits: 32}
	// 64-bit registers.
	D0  = &Register{Name: "d0", Type: TypeFloatingPoint, Bits: 64}
	D1  = &Register{Name: "d1", Type: TypeFloatingPoint, Bits: 64}
	D2  = &Register{Name: "d2", Type: TypeFloatingPoint, Bits: 64}
	D3  = &Register{Name: "d3", Type: TypeFloatingPoint, Bits: 64}
	D4  = &Register{Name: "d4", Type: TypeFloatingPoint, Bits: 64}
	D5  = &Register{Name: "d5", Type: TypeFloatingPoint, Bits: 64}
	D6  = &Register{Name: "d6", Type: TypeFloatingPoint, Bits: 64}
	D7  = &Register{Name: "d7", Type: TypeFloatingPoint, Bits: 64}
	D8  = &Register{Name: "d8", Type: TypeFloatingPoint, Bits: 64}
	D9  = &Register{Name: "d9", Type: TypeFloatingPoint, Bits: 64}
	D10 = &Register{Name: "d10", Type: TypeFloatingPoint, Bits: 64}
	D11 = &Register{Name: "d11", Type: TypeFloatingPoint, Bits: 64}
	D12 = &Register{Name: "d12", Type: TypeFloatingPoint, Bits: 64}
	D13 = &Register{Name: "d13", Type: TypeFloatingPoint, Bits: 64}
	D14 = &Register{Name: "d14", Type: TypeFloatingPoint, Bits: 64}
	D15 = &Register{Name: "d15", Type: TypeFloatingPoint, Bits: 64}
	D16 = &Register{Name: "d16", Type: TypeFloatingPoint, Bits: 64}
	D17 = &Register{Name: "d17", Type: TypeFloatingPoint, Bits: 64}
	D18 = &Register{Name: "d18", Type: TypeFloatingPoint, Bits: 64}
	D19 = &Register{Name: "d19", Type: TypeFloatingPoint, Bits: 64}
	D20 = &Register{Name: "d20", Type: TypeFloatingPoint, Bits: 64}
	D21 = &Register{Name: "d21", Type: TypeFloatingPoint, Bits: 64}
	D22 = &Register{Name: "d22", Type: TypeFloatingPoint, Bits: 64}
	D23 = &Register{Name: "d23", Type: TypeFloatingPoint, Bits: 64}
	D24 = &Register{Name: "d24", Type: TypeFloatingPoint, Bits: 64}
	D25 = &Register{Name: "d25", Type: TypeFloatingPoint, Bits: 64}
	D26 = &Register{Name: "d26", Type: TypeFloatingPoint, Bits: 64}
	D27 = &Register{Name: "d27", Type: TypeFloatingPoint, Bits: 64}
	D28 = &Register{Name: "d28", Type: TypeFloatingPoint, Bits: 64}
	D29 = &Register{Name: "d29", Type: TypeFloatingPoint, Bits: 64}
	D30 = &Register{Name: "d30", Type: TypeFloatingPoint, Bits: 64}
	D31 = &Register{Name: "d31", Type: TypeFloatingPoint, Bits: 64}
	// 128-bit registers.
	Q0  = &Register{Name: "q0", Type: TypeFloatingPoint, Bits: 128}
	Q1  = &Register{Name: "q1", Type: TypeFloatingPoint, Bits: 128}
	Q2  = &Register{Name: "q2", Type: TypeFloatingPoint, Bits: 128}
	Q3  = &Register{Name: "q3", Type: TypeFloatingPoint, Bits: 128}
	Q4  = &Register{Name: "q4", Type: TypeFloatingPoint, Bits: 128}
	Q5  = &Register{Name: "q5", Type: TypeFloatingPoint, Bits: 128}
	Q6  = &Register{Name: "q6", Type: TypeFloatingPoint, Bits: 128}
	Q7  = &Register{Name: "q7", Type: TypeFloatingPoint, Bits: 128}
	Q8  = &Register{Name: "q8", Type: TypeFloatingPoint, Bits: 128}
	Q9  = &Register{Name: "q9", Type: TypeFloatingPoint, Bits: 128}
	Q10 = &Register{Name: "q10", Type: TypeFloatingPoint, Bits: 128}
	Q11 = &Register{Name: "q11", Type: TypeFloatingPoint, Bits: 128}
	Q12 = &Register{Name: "q12", Type: TypeFloatingPoint, Bits: 128}
	Q13 = &Register{Name: "q13", Type: TypeFloatingPoint, Bits: 128}
	Q14 = &Register{Name: "q14", Type: TypeFloatingPoint, Bits: 128}
	Q15 = &Register{Name: "q15", Type: TypeFloatingPoint, Bits: 128}
	Q16 = &Register{Name: "q16", Type: TypeFloatingPoint, Bits: 128}
	Q17 = &Register{Name: "q17", Type: TypeFloatingPoint, Bits: 128}
	Q18 = &Register{Name: "q18", Type: TypeFloatingPoint, Bits: 128}
	Q19 = &Register{Name: "q19", Type: TypeFloatingPoint, Bits: 128}
	Q20 = &Register{Name: "q20", Type: TypeFloatingPoint, Bits: 128}
	Q21 = &Register{Name: "q21", Type: TypeFloatingPoint, Bits: 128}
	Q22 = &Register{Name: "q22", Type: TypeFloatingPoint, Bits: 128}
	Q23 = &Register{Name: "q23", Type: TypeFloatingPoint, Bits: 128}
	Q24 = &Register{Name: "q24", Type: TypeFloatingPoint, Bits: 128}
	Q25 = &Register{Name: "q25", Type: TypeFloatingPoint, Bits: 128}
	Q26 = &Register{Name: "q26", Type: TypeFloatingPoint, Bits: 128}
	Q27 = &Register{Name: "q27", Type: TypeFloatingPoint, Bits: 128}
	Q28 = &Register{Name: "q28", Type: TypeFloatingPoint, Bits: 128}
	Q29 = &Register{Name: "q29", Type: TypeFloatingPoint, Bits: 128}
	Q30 = &Register{Name: "q30", Type: TypeFloatingPoint, Bits: 128}
	Q31 = &Register{Name: "q31", Type: TypeFloatingPoint, Bits: 128}
)

var Registers = []*Register{
	// General-purpose registers.
	// 32-bit registers.
	W0, W1, W2, W3, W4, W5, W6, W7,
	W8, W9, W10, W11, W12, W13, W14, W15,
	W16, W17, W18, W19, W20, W21, W22, W23,
	W24, W25, W26, W27, W28, W29, W30,
	// 64-bit registers.
	X0, X1, X2, X3, X4, X5, X6, X7,
	X8, X9, X10, X11, X12, X13, X14, X15,
	X16, X17, X18, X19, X20, X21, X22, X23,
	X24, X25, X26, X27, X28, X29, X30,

	// Stack pointer.
	SP,

	// Floating point and vector registers.
	// 8-bit registers.
	B0, B1, B2, B3, B4, B5, B6, B7,
	B8, B9, B10, B11, B12, B13, B14, B15,
	B16, B17, B18, B19, B20, B21, B22, B23,
	B24, B25, B26, B27, B28, B29, B30, B31,
	// 16-bit registers.
	H0, H1, H2, H3, H4, H5, H6, H7,
	H8, H9, H10, H11, H12, H13, H14, H15,
	H16, H17, H18, H19, H20, H21, H22, H23,
	H24, H25, H26, H27, H28, H29, H30, H31,
	// 32-bit registers.
	S0, S1, S2, S3, S4, S5, S6, S7,
	S8, S9, S10, S11, S12, S13, S14, S15,
	S16, S17, S18, S19, S20, S21, S22, S23,
	S24, S25, S26, S27, S28, S29, S30, S31,
	// 64-bit registers.
	D0, D1, D2, D3, D4, D5, D6, D7,
	D8, D9, D10, D11, D12, D13, D14, D15,
	D16, D17, D18, D19, D20, D21, D22, D23,
	D24, D25, D26, D27, D28, D29, D30, D31,
	// 128-bit registers.
	Q0, Q1, Q2, Q3, Q4, Q5, Q6, Q7,
	Q8, Q9, Q10, Q11, Q12, Q13, Q14, Q15,
	Q16, Q17, Q18, Q19, Q20, Q21, Q22, Q23,
	Q24, Q25, Q26, Q27, Q28, Q29, Q30, Q31,
}

var (
	// W contains the 32-bit general
	// purpose registers.
	W = []*Register{
		W0, W1, W2, W3, W4, W5, W6, W7,
		W8, W9, W10, W11, W12, W13, W14, W15,
		W16, W17, W18, W19, W20, W21, W22, W23,
		W24, W25, W26, W27, W28, W29, W30,
	}

	// X contains the 64-bit general
	// purpose registers.
	X = []*Register{
		X0, X1, X2, X3, X4, X5, X6, X7,
		X8, X9, X10, X11, X12, X13, X14, X15,
		X16, X17, X18, X19, X20, X21, X22, X23,
		X24, X25, X26, X27, X28, X29, X30,
	}

	// W_X contains all general purpose
	// registers.
	W_X = []*Register{
		W0, W1, W2, W3, W4, W5, W6, W7,
		W8, W9, W10, W11, W12, W13, W14, W15,
		W16, W17, W18, W19, W20, W21, W22, W23,
		W24, W25, W26, W27, W28, W29, W30,
		X0, X1, X2, X3, X4, X5, X6, X7,
		X8, X9, X10, X11, X12, X13, X14, X15,
		X16, X17, X18, X19, X20, X21, X22, X23,
		X24, X25, X26, X27, X28, X29, X30,
	}

	// W_WSP contains the 32-bit general
	// purpose registers and stack pointer.
	W_WSP = []*Register{
		W0, W1, W2, W3, W4, W5, W6, W7,
		W8, W9, W10, W11, W12, W13, W14, W15,
		W16, W17, W18, W19, W20, W21, W22, W23,
		W24, W25, W26, W27, W28, W29, W30,
		WSP,
	}

	// X_SP contains the 64-bit general
	// purpose registers and stack pointer.
	X_SP = []*Register{
		X0, X1, X2, X3, X4, X5, X6, X7,
		X8, X9, X10, X11, X12, X13, X14, X15,
		X16, X17, X18, X19, X20, X21, X22, X23,
		X24, X25, X26, X27, X28, X29, X30,
		SP,
	}

	// B contains the 8-bit floating
	// point registers.
	B = []*Register{
		B0, B1, B2, B3, B4, B5, B6, B7,
		B8, B9, B10, B11, B12, B13, B14, B15,
		B16, B17, B18, B19, B20, B21, B22, B23,
		B24, B25, B26, B27, B28, B29, B30, B31,
	}

	// H contains the 16-bit floating
	// point registers.
	H = []*Register{
		H0, H1, H2, H3, H4, H5, H6, H7,
		H8, H9, H10, H11, H12, H13, H14, H15,
		H16, H17, H18, H19, H20, H21, H22, H23,
		H24, H25, H26, H27, H28, H29, H30, H31,
	}

	// S contains the 32-bit floating
	// point registers.
	S = []*Register{
		S0, S1, S2, S3, S4, S5, S6, S7,
		S8, S9, S10, S11, S12, S13, S14, S15,
		S16, S17, S18, S19, S20, S21, S22, S23,
		S24, S25, S26, S27, S28, S29, S30, S31,
	}

	// D contains the 64-bit floating
	// point registers.
	D = []*Register{
		D0, D1, D2, D3, D4, D5, D6, D7,
		D8, D9, D10, D11, D12, D13, D14, D15,
		D16, D17, D18, D19, D20, D21, D22, D23,
		D24, D25, D26, D27, D28, D29, D30, D31,
	}

	// Q contains the 128-bit floating
	// point registers.
	Q = []*Register{
		Q0, Q1, Q2, Q3, Q4, Q5, Q6, Q7,
		Q8, Q9, Q10, Q11, Q12, Q13, Q14, Q15,
		Q16, Q17, Q18, Q19, Q20, Q21, Q22, Q23,
		Q24, Q25, Q26, Q27, Q28, Q29, Q30, Q31,
	}
)

// RegisterSizes maps the names of fixed-size
// register names (lower case) to their size
// in bits.
var RegisterSizes = make(map[string]int)

var RegistersByName = make(map[string]*Register)

func init() {
	for _, reg := range Registers {
		RegisterSizes[reg.Name] = reg.Bits
		RegistersByName[reg.Name] = reg
	}
}

// RegisterType categorises an x86
// register.
type RegisterType uint8

const (
	_ RegisterType = iota
	TypeGeneralPurpose
	TypeStackPointer
	TypeFloatingPoint
)

func (t RegisterType) String() string {
	switch t {
	case TypeGeneralPurpose:
		return "general purpose register"
	case TypeStackPointer:
		return "stack pointer register"
	case TypeFloatingPoint:
		return "floating point register"
	default:
		return fmt.Sprintf("RegisterType(%d)", t)
	}
}

var RegisterTypes = map[string]RegisterType{
	"general purpose register": TypeGeneralPurpose,
	"stack pointer register":   TypeStackPointer,
	"floating point register":  TypeFloatingPoint,
}
