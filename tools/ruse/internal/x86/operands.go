// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package x86

import (
	"encoding/json"
	"fmt"
)

// Operand describes one operand to an instruction.
type Operand struct {
	Name      string          `json:"name"`                // The operand's name in the manual.
	Syntax    string          `json:"syntax"`              // The consistent syntax for the operand.
	UID       string          `json:"uid"`                 // The unique identifier of the operand.
	Type      OperandType     `json:"type"`                // The operand type.
	Encoding  OperandEncoding `json:"encoding"`            // The way the operand is encoded in machine code.
	Bits      int             `json:"bits,omitempty"`      // The operand size in bits.
	Registers []*Register     `json:"registers,omitempty"` // The set of acceptable registers (if any) for this operand.
}

func (op *Operand) String() string {
	return fmt.Sprintf("%s %s (%s)", op.Type, op.Syntax, op.Encoding)
}

// OperandType categories a parameter
// to an x86 instruction.
type OperandType uint8

const (
	_                     OperandType = iota
	TypeSignedImmediate               // A signed integer literal.
	TypeUnsignedImmediate             // An unsigned integer literal.
	TypeRegister                      // A register selection.
	TypeStackIndex                    // An x87 FPU stack index.
	TypeRelativeAddress               // An address offset from the instruction pointer.
	TypeFarPointer                    // A segment selector and absolute address offset pair.
	TypeMemory                        // A memory address expression.
	TypeMemoryOffset                  // A memory offset expression.
	TypeStringDst                     // A memory address for a string destination.
	TypeStringSrc                     // A memory address for a string source.
)

var ParameterTypes = map[string]OperandType{
	"signed immediate":   TypeSignedImmediate,
	"unsigned immediate": TypeUnsignedImmediate,
	"register":           TypeRegister,
	"stack index":        TypeStackIndex,
	"relative address":   TypeRelativeAddress,
	"far pointer":        TypeFarPointer,
	"memory":             TypeMemory,
	"memory offset":      TypeMemoryOffset,
	"string destination": TypeStringDst,
	"string source":      TypeStringSrc,
}

func (t OperandType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

func (t *OperandType) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}

	got, ok := ParameterTypes[s]
	if !ok {
		return fmt.Errorf("invalid parameter type %q", s)
	}

	*t = got

	return nil
}

func (t OperandType) String() string {
	switch t {
	case TypeSignedImmediate:
		return "signed immediate"
	case TypeUnsignedImmediate:
		return "unsigned immediate"
	case TypeRegister:
		return "register"
	case TypeStackIndex:
		return "stack index"
	case TypeRelativeAddress:
		return "relative address"
	case TypeFarPointer:
		return "far pointer"
	case TypeMemory:
		return "memory"
	case TypeMemoryOffset:
		return "memory offset"
	case TypeStringDst:
		return "string destination"
	case TypeStringSrc:
		return "string source"
	default:
		return fmt.Sprintf("ParameterType(%d)", t)
	}
}

func (t OperandType) UID() string {
	switch t {
	case TypeSignedImmediate:
		return "TypeSignedImmediate"
	case TypeUnsignedImmediate:
		return "TypeUnsignedImmediate"
	case TypeRegister:
		return "TypeRegister"
	case TypeStackIndex:
		return "TypeStackIndex"
	case TypeRelativeAddress:
		return "TypeRelativeAddress"
	case TypeFarPointer:
		return "TypeFarPointer"
	case TypeMemory:
		return "TypeMemory"
	case TypeMemoryOffset:
		return "TypeMemoryOffset"
	case TypeStringDst:
		return "TypeStringDst"
	case TypeStringSrc:
		return "TypeStringSrc"
	default:
		return fmt.Sprintf("ParameterType(%d)", t)
	}
}

// OperandEncoding represents a way in
// which an x86 instruction's parameter
// is encoded (or not) in the machine
// code.
type OperandEncoding uint8

const (
	_                        OperandEncoding = iota
	EncodingNone                             // The parameter is required in the assembly but is not encoded.
	EncodingImplicit                         // The parameter is optional in the assembly and is not encoded.
	EncodingVEXvvvv                          // The parameter is encoded in the VEX.vvvv field of the machine code.
	EncodingRegisterModifier                 // The parameter is encoded in the opcode byte.
	EncodingStackIndex                       // The parameter is an x87 stack index, encoded in the opcode byte.
	EncodingCodeOffset                       // The parameter is encoded as a code offset after the opcode.
	EncodingModRMreg                         // The parameter is encoded in the ModR/M.reg field of the machine code.
	EncodingModRMrm                          // The parameter is encoded in the ModR/M.rm field of the machine code.
	EncodingSIB                              // The parameter is encoded in the SIB byte.
	EncodingDisplacement                     // The parameter is encoded in the displacement field of the machine code.
	EncodingImmediate                        // The parameter is encoded in the immediate field of the machine code.
	EncodingVEXis4                           // The parameter is encoded in the VEX /is4 immediate byte.
)

var ParameterEncodings = map[string]OperandEncoding{
	"none":              EncodingNone,
	"implicit":          EncodingImplicit,
	"VEX.vvvv":          EncodingVEXvvvv,
	"register modifier": EncodingRegisterModifier,
	"stack index":       EncodingStackIndex,
	"code offset":       EncodingCodeOffset,
	"ModR/M reg":        EncodingModRMreg,
	"ModR/M r/m":        EncodingModRMrm,
	"SIB":               EncodingSIB,
	"displacement":      EncodingDisplacement,
	"immediate":         EncodingImmediate,
	"VEX /is4":          EncodingVEXis4,
}

func (e OperandEncoding) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

func (e *OperandEncoding) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}

	got, ok := ParameterEncodings[s]
	if !ok {
		return fmt.Errorf("invalid parameter encoding %q", s)
	}

	*e = got

	return nil
}

func (e OperandEncoding) String() string {
	switch e {
	case EncodingNone:
		return "none"
	case EncodingImplicit:
		return "implicit"
	case EncodingVEXvvvv:
		return "VEX.vvvv"
	case EncodingRegisterModifier:
		return "register modifier"
	case EncodingStackIndex:
		return "stack index"
	case EncodingCodeOffset:
		return "code offset"
	case EncodingModRMreg:
		return "ModR/M reg"
	case EncodingModRMrm:
		return "ModR/M r/m"
	case EncodingSIB:
		return "SIB"
	case EncodingDisplacement:
		return "displacement"
	case EncodingImmediate:
		return "immediate"
	case EncodingVEXis4:
		return "VEX /is4"
	default:
		return fmt.Sprintf("ParameterEncoding(%d)", e)
	}
}

func (e OperandEncoding) UID() string {
	switch e {
	case EncodingNone:
		return "EncodingNone"
	case EncodingImplicit:
		return "EncodingImplicit"
	case EncodingVEXvvvv:
		return "EncodingVEXvvvv"
	case EncodingRegisterModifier:
		return "EncodingRegisterModifier"
	case EncodingStackIndex:
		return "EncodingStackIndex"
	case EncodingCodeOffset:
		return "EncodingCodeOffset"
	case EncodingModRMreg:
		return "EncodingModRMreg"
	case EncodingModRMrm:
		return "EncodingModRMrm"
	case EncodingSIB:
		return "EncodingSIB"
	case EncodingDisplacement:
		return "EncodingDisplacement"
	case EncodingImmediate:
		return "EncodingImmediate"
	case EncodingVEXis4:
		return "EncodingVEXis4"
	default:
		panic("unrecodgnised parameter encoding: " + e.String())
	}
}
