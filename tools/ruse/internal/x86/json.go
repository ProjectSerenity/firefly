// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package x86

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type jsonEncoding struct {
	// The textual representation.
	Syntax string `json:"syntax"`

	// Legacy prefixes.
	PrefixOpcodes     []int `json:"prefixOpcodes,omitempty"`
	NoVEXPrefixes     bool  `json:"noVexPrefixes,omitempty"`
	NoRepPrefixes     bool  `json:"noRepPrefixes,omitempty"`
	MandatoryPrefixes []int `json:"mandatoryPrefixes,omitempty"`

	// REX prefixes.
	REX   bool `json:"rex,omitempty"`
	REX_R bool `json:"rexR,omitempty"`
	REX_W bool `json:"rexW,omitempty"`

	// VEX prefixes.
	VEX       bool  `json:"vex,omitempty"`
	VEX_L     bool  `json:"vexL,omitempty"`
	VEXpp     uint8 `json:"vexPp,omitempty"`
	VEXm_mmmm uint8 `json:"vexMmmmm,omitempty"`
	VEX_W     bool  `json:"vexW,omitempty"`
	VEX_WIG   bool  `json:"vexWig,omitempty"`
	VEXis4    bool  `json:"vexIs4,omitempty"`

	// EVEX prefixes.
	EVEX     bool `json:"evex,omitempty"`
	EVEX_Lp  bool `json:"evexLp,omitempty"`
	Mask     bool `json:"mask,omitempty"`
	Zero     bool `json:"zero,omitempty"`
	Rounding bool `json:"rounding,omitempty"`
	Suppress bool `json:"suppress,omitempty"`

	// Opcode data.
	Opcode           []int `json:"opcode"`
	RegisterModifier int   `json:"registerModifier,omitempty"`
	StackIndex       int   `json:"stackIndex,omitempty"`

	// Code offset after the opcode.
	CodeOffset bool `json:"codeOffset,omitempty"`

	// ModR/M byte.
	ModRM    bool  `json:"modRm,omitempty"`
	ModRMmod uint8 `json:"modRmMod,omitempty"`
	ModRMreg uint8 `json:"modRmReg,omitempty"`
	ModRMrm  uint8 `json:"modRmRm,omitempty"`

	// Vector SIB.
	VSIB bool `json:"vsib,omitempty"`

	// Implied immediates.
	ImpliedImmediate string `json:"impliedImmediate,omitempty"`
}

func (e *Encoding) MarshalJSON() ([]byte, error) {
	j := jsonEncoding{
		Syntax: e.Syntax,

		// PrefixOpcodes is handled separately.
		NoVEXPrefixes: e.NoVEXPrefixes,
		NoRepPrefixes: e.NoRepPrefixes,
		// MandatoryPrefixes is handled separately.

		REX:   e.REX,
		REX_R: e.REX_R,
		REX_W: e.REX_W,

		VEX:       e.VEX,
		VEX_L:     e.VEX_L,
		VEXpp:     e.VEXpp,
		VEXm_mmmm: e.VEXm_mmmm,
		VEX_W:     e.VEX_W,
		VEX_WIG:   e.VEX_WIG,
		VEXis4:    e.VEXis4,

		EVEX:     e.EVEX,
		EVEX_Lp:  e.EVEX_Lp,
		Mask:     e.Mask,
		Zero:     e.Zero,
		Rounding: e.Rounding,
		Suppress: e.Suppress,

		// Opcode is handled separately.
		RegisterModifier: e.RegisterModifier,
		StackIndex:       e.StackIndex,

		CodeOffset: e.CodeOffset,

		ModRM:    e.ModRM,
		ModRMmod: e.ModRMmod,
		ModRMreg: e.ModRMreg,
		ModRMrm:  e.ModRMrm,

		VSIB: e.VSIB,

		// ImpliedImmediate is handled separately.
	}

	if len(e.PrefixOpcodes) > 0 {
		j.PrefixOpcodes = make([]int, len(e.PrefixOpcodes))
		for i, op := range e.PrefixOpcodes {
			j.PrefixOpcodes[i] = int(op)
		}
	}

	if len(e.MandatoryPrefixes) > 0 {
		j.MandatoryPrefixes = make([]int, len(e.MandatoryPrefixes))
		for i, prefix := range e.MandatoryPrefixes {
			j.MandatoryPrefixes[i] = int(prefix)
		}
	}

	if len(e.Opcode) > 0 {
		j.Opcode = make([]int, len(e.Opcode))
		for i, op := range e.Opcode {
			j.Opcode[i] = int(op)
		}
	}

	if len(e.ImpliedImmediate) > 0 {
		j.ImpliedImmediate = hex.EncodeToString(e.ImpliedImmediate)
	}

	return json.Marshal(j)
}

func (e *Encoding) UnmarshalJSON(data []byte) error {
	var j jsonEncoding
	err := json.Unmarshal(data, &j)
	if err != nil {
		return err
	}

	var mandatoryPrefixes []Prefix
	var prefixOpcodes, opcode, impliedImmediate []byte
	if len(j.PrefixOpcodes) > 0 {
		prefixOpcodes = make([]byte, len(j.PrefixOpcodes))
		for i, b := range j.PrefixOpcodes {
			if b < 0 || 0xff < b {
				return fmt.Errorf("invalid prefix opcode %d: exceeds 8-bit unsigned integer", b)
			}

			prefixOpcodes[i] = byte(b)
		}
	}

	if len(j.MandatoryPrefixes) > 0 {
		mandatoryPrefixes = make([]Prefix, len(j.MandatoryPrefixes))
		for i, b := range j.MandatoryPrefixes {
			if b < 0 || 0xff < b {
				return fmt.Errorf("invalid mandatory prefixes %d: exceeds 8-bit unsigned integer", b)
			}

			mandatoryPrefixes[i] = Prefix(b)
		}
	}

	if len(j.Opcode) > 0 {
		opcode = make([]byte, len(j.Opcode))
		for i, b := range j.Opcode {
			if b < 0 || 0xff < b {
				return fmt.Errorf("invalid opcode %d: exceeds 8-bit unsigned integer", b)
			}

			opcode[i] = byte(b)
		}
	}

	if len(j.ImpliedImmediate) > 0 {
		impliedImmediate, err = hex.DecodeString(j.ImpliedImmediate)
		if err != nil {
			return fmt.Errorf("failed to decode impliedImmediate: %v", err)
		}
	}

	*e = Encoding{
		Syntax: j.Syntax,

		PrefixOpcodes:     prefixOpcodes,
		NoVEXPrefixes:     j.NoVEXPrefixes,
		NoRepPrefixes:     j.NoRepPrefixes,
		MandatoryPrefixes: mandatoryPrefixes,

		REX:   j.REX,
		REX_R: j.REX_R,
		REX_W: j.REX_W,

		VEX:       j.VEX,
		VEX_L:     j.VEX_L,
		VEXpp:     j.VEXpp,
		VEXm_mmmm: j.VEXm_mmmm,
		VEX_W:     j.VEX_W,
		VEX_WIG:   j.VEX_WIG,
		VEXis4:    j.VEXis4,

		EVEX:     j.EVEX,
		EVEX_Lp:  j.EVEX_Lp,
		Mask:     j.Mask,
		Zero:     j.Zero,
		Rounding: j.Rounding,
		Suppress: j.Suppress,

		Opcode:           opcode,
		RegisterModifier: j.RegisterModifier,
		StackIndex:       j.StackIndex,

		CodeOffset: j.CodeOffset,

		ModRM:    j.ModRM,
		ModRMmod: j.ModRMmod,
		ModRMreg: j.ModRMreg,
		ModRMrm:  j.ModRMrm,

		VSIB: j.VSIB,

		ImpliedImmediate: impliedImmediate,
	}

	return nil
}

type jsonParameter struct {
	Type      string   `json:"type"`
	Encoding  string   `json:"encoding"`
	UID       string   `json:"uid"`
	Bits      int      `json:"bits,omitempty"`
	Syntax    string   `json:"syntax"`
	Registers []string `json:"registers,omitempty"`
}

func (p *Parameter) MarshalJSON() ([]byte, error) {
	j := jsonParameter{
		Type:     p.Type.String(),
		Encoding: p.Encoding.String(),
		UID:      p.UID,
		Syntax:   p.Syntax,
		Bits:     p.Bits,
	}

	if len(p.Registers) > 0 {
		j.Registers = make([]string, len(p.Registers))
		for i, reg := range p.Registers {
			j.Registers[i] = reg.Name
		}
	}

	return json.Marshal(j)
}

func (p *Parameter) UnmarshalJSON(data []byte) error {
	var j jsonParameter
	err := json.Unmarshal(data, &j)
	if err != nil {
		return err
	}

	typ, ok := ParameterTypes[j.Type]
	if !ok {
		return fmt.Errorf("unrecognised parameter type: %q", j.Type)
	}

	encoding, ok := ParameterEncodings[j.Encoding]
	if !ok {
		return fmt.Errorf("unrecognised parameter encoding: %q", j.Encoding)
	}

	var registers []*Register
	if len(j.Registers) > 0 {
		registers = make([]*Register, len(j.Registers))
		for i, name := range j.Registers {
			reg, ok := RegistersByName[name]
			if !ok {
				return fmt.Errorf("unrecognised register name: %q", name)
			}

			registers[i] = reg
		}
	}

	*p = Parameter{
		Type:      typ,
		Encoding:  encoding,
		UID:       j.UID,
		Syntax:    j.Syntax,
		Bits:      j.Bits,
		Registers: registers,
	}

	return nil
}
