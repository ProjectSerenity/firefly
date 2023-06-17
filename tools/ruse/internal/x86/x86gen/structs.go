// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"rsc.io/pdf"

	"firefly-os.dev/tools/ruse/internal/x86"
)

// Page represents a page of text in the Intel
// x86 manual PDF.
type Page struct {
	Page int
	Text []pdf.Text
}

// Table reprsents an encoding table as a series of
// rows of text.
type Table struct {
	Page int
	Rows [][]string
}

// Mnemonic represents an entry
// in a mnemonic table in the
// manual.
//
// Example:
//
//	VCOMPRESSPD - Store Sparse Packed Double Precision Floating-Point Values Into Dense
//	Memory
//
//	| Opcode/                        | Op / | 64/32    | CPUID    | Description                                     |
//	| Instruction                    | En   | bit Mode | Feature  |                                                 |
//	|                                |      | Support  | Flag     |                                                 |
//	+--------------------------------+------+----------+----------+-------------------------------------------------+
//	| EVEX.128.66.0F38.W1 8A /r      | A    | V/V      | AVX512VL | Compress packed double precision floating-point |
//	| VCOMPRESSPD xmm1/m128 {k1}{z}, |      |          | AVX512F  | values from xmm2 to xmm1/m128 using writemask   |
//	| xmm2                           |      |          |          | k1.                                             |
//	| EVEX.256.66.0F38.W1 8A /r      | A    | V/V      | AVX512VL | Compress packed double precision floating-point |
//	| VCOMPRESSPD ymm1/m256 {k1}{z}, |      |          | AVX512F  | values from ymm2 to ymm1/m256 using writemask   |
//	| ymm2                           |      |          |          | k1.                                             |
//	| EVEX.512.66.0F38.W1 8A /r      | A    | V/V      | AVX512F  | Compress packed double precision floating-point |
//	| VCOMPRESSPD zmm1/m512 {k1}{z}, |      |          |          | values from zmm2 using control mask k1 to       |
//	| zmm2                           |      |          |          | zmm1/m512.                                      |
type Mnemonic struct {
	Page            int
	Opcode          string
	Instruction     string
	OperandEncoding string
	Mode64          string
	Mode32          string
	Mode16          string
	CPUID           string
	Description     string

	// Indicates that multiple
	// instruction forms can be
	// selected using the operand
	// size override prefix.
	OperandSize bool
	AddressSize bool
	DataSize    int // Optional data operation size.
}

// OperandEncoding contains the
// information from an operand
// encoding table entry in the
// manual.
//
// Example:
//
//	Instruction Operand Encoding
//
//	| Op/En | Tuple Type    | Operand 1     | Operand 2     | Operand 3 | Operand 4 |
//	+-------+---------------+---------------+---------------+-----------+-----------+
//	|   A   |       N/A     | ModRM:reg (w) | ModRM:r/m (r) |    N/A    |    N/A    |
//	|   B   | Tuple1 Scalar | ModRM:reg (w) | ModRM:r/m (r) |    N/A    |    N/A    |
//	|   C   |     Tuple2    | ModRM:reg (w) | ModRM:r/m (r) |    N/A    |    N/A    |
//	|   D   |     Tuple4    | ModRM:reg (w) | ModRM:r/m (r) |    N/A    |    N/A    |
//	|   E   |     Tuple8    | ModRM:reg (w) | ModRM:r/m (r) |    N/A    |    N/A    |
type OperandEncoding struct {
	Page      int
	Encoding  string
	TupleType string
	Operands  [4]string
}

// Listing contains the textual description of a set
// of instructions.
type Listing struct {
	Page  int
	Pages int

	Name                 string
	MnemonicTable        []Mnemonic
	OperandEncodingTable []OperandEncoding
}

// Spec represents a completed instruction
// specification, combining a mnemonic table
// entry and an operand encoding table entry.
type Spec struct {
	M *Mnemonic
	E *OperandEncoding
}

// Instruction contains the information to describe
// a complete instruction form.
type Instruction struct {
	Page      int           `json:"page,omitempty"`
	Mnemonic  string        `json:"mnemonic"`
	UID       string        `json:"uid"`
	Syntax    string        `json:"syntax"`
	Encoding  *x86.Encoding `json:"encoding"`
	TupleType x86.TupleType `json:"tupletype,omitempty"`

	MinArgs  int         `json:"minArgs"`
	MaxArgs  int         `json:"maxArgs"`
	Operands [4]*Operand `json:"operands"`

	Mode64 bool `json:"mode64"`
	Mode32 bool `json:"mode32"`
	Mode16 bool `json:"mode16"`

	CPUID []string `json:"cpuid,omitempty"`

	OperandSize bool `json:"operandSize,omitempty"`
	AddressSize bool `json:"addressSize,omitempty"`
	DataSize    int  `json:"dataSize,omitempty"`
}

// Operand describes one operand to an instruction.
type Operand struct {
	Name      string                `json:"name"`
	Syntax    string                `json:"syntax"`
	UID       string                `json:"uid"`
	Type      x86.ParameterType     `json:"type"`
	Encoding  x86.ParameterEncoding `json:"encoding"`
	Bits      int                   `json:"bits,omitempty"`
	Registers []*x86.Register       `json:"registers,omitempty"`
}

var registersByOperandUID = map[string][]*x86.Register{
	"AL":    {x86.AL},
	"CL":    {x86.CL},
	"AX":    {x86.AX},
	"DX":    {x86.DX},
	"EAX":   {x86.EAX},
	"ECX":   {x86.ECX},
	"EDX":   {x86.EDX},
	"RAX":   {x86.RAX},
	"CR8":   {x86.CR8},
	"XMM0":  {x86.XMM0},
	"ST":    {x86.ST0},
	"ST(0)": {x86.ST0},

	"StrDst8":  {x86.DI, x86.EDI, x86.RDI},
	"StrDst16": {x86.DI, x86.EDI, x86.RDI},
	"StrDst32": {x86.DI, x86.EDI, x86.RDI},
	"StrDst64": {x86.RDI},
	"StrSrc8":  {x86.SI, x86.ESI, x86.RSI},
	"StrSrc16": {x86.SI, x86.ESI, x86.RSI},
	"StrSrc32": {x86.SI, x86.ESI, x86.RSI},
	"StrSrc64": {x86.RSI},

	"ES": {x86.ES},
	"CS": {x86.CS},
	"SS": {x86.SS},
	"DS": {x86.DS},
	"FS": {x86.FS},
	"GS": {x86.GS},

	"R8V":  x86.Registers8bitGeneralPurpose,
	"R16V": x86.Registers16bitGeneralPurpose,
	"R32V": x86.Registers32bitGeneralPurpose,
	"R64V": x86.Registers64bitGeneralPurpose,
	"KV":   x86.RegistersOpmask,
	"TMMV": x86.RegistersTMM,
	"XMMV": x86.Registers128bitXMM,
	"YMMV": x86.Registers256bitYMM,
	"ZMMV": x86.Registers512bitZMM,

	"R8op":  x86.Registers8bitGeneralPurpose,
	"R16op": x86.Registers16bitGeneralPurpose,
	"R32op": x86.Registers32bitGeneralPurpose,
	"R64op": x86.Registers64bitGeneralPurpose,

	"STi": x86.RegistersStackIndices,

	"R8":       x86.Registers8bitGeneralPurpose,
	"R16":      x86.Registers16bitGeneralPurpose,
	"R32":      x86.Registers32bitGeneralPurpose,
	"R64":      x86.Registers64bitGeneralPurpose,
	"Sreg":     x86.Registers16bitSegment,
	"CR0toCR7": {x86.CR0, x86.CR1, x86.CR2, x86.CR3, x86.CR4, x86.CR5, x86.CR6, x86.CR7},
	"DR0toDR7": {x86.DR0, x86.DR1, x86.DR2, x86.DR3, x86.DR4, x86.DR5, x86.DR6, x86.DR7},
	"K1":       x86.RegistersOpmask,
	"BND1":     {x86.BND0, x86.BND1, x86.BND2},
	"MM1":      x86.Registers64bitMMX,
	"TMM1":     x86.RegistersTMM,
	"XMM1":     x86.Registers128bitXMM,
	"YMM1":     x86.Registers256bitYMM,
	"ZMM1":     x86.Registers512bitZMM,

	"Rmr8":  x86.Registers8bitGeneralPurpose,
	"Rmr16": x86.Registers16bitGeneralPurpose,
	"Rmr32": x86.Registers32bitGeneralPurpose,
	"Rmr64": x86.Registers64bitGeneralPurpose,
	"K2":    x86.RegistersOpmask,
	"BND2":  {x86.BND0, x86.BND1, x86.BND2},
	"MM2":   x86.Registers64bitMMX,
	"TMM2":  x86.RegistersTMM,
	"XMM2":  x86.Registers128bitXMM,
	"YMM2":  x86.Registers256bitYMM,
	"ZMM2":  x86.Registers512bitZMM,

	"XMMIH": x86.Registers128bitXMM,
	"YMMIH": x86.Registers256bitYMM,
	"ZMMIH": x86.Registers512bitZMM,
}
