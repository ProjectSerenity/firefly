// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package x86 contains structured information on the
// x86 instruction set architecture.
package x86

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Mode represents an x86
// CPU mode, as a number
// of bits.
type Mode struct {
	Int    uint8
	String string
}

var (
	Mode16 = Mode{16, "16"}
	Mode32 = Mode{32, "32"}
	Mode64 = Mode{64, "64"}
	Modes  = []Mode{Mode16, Mode32, Mode64}
)

// Instruction includes structured informatin
// about an x86 instruction.
type Instruction struct {
	Page      int       `json:"page,omitempty"`      // The page in the manual where this is defined.
	Mnemonic  string    `json:"mnemonic"`            // The Intel name for the instruction, in upper case.
	UID       string    `json:"uid"`                 // A unique identifier for the instruction.
	Syntax    string    `json:"syntax"`              // The original Intel syntax for the instruction.
	Encoding  *Encoding `json:"encoding"`            // The information on how to encode the instruction.
	TupleType TupleType `json:"tupleType,omitempty"` // Any tuple type.

	MinArgs  int         `json:"minArgs"`
	MaxArgs  int         `json:"maxArgs"`
	Operands [4]*Operand `json:"operands"` // Any parameters to the instruction.

	Mode64 bool `json:"mode64"` // Whether the instruction is supported in 64-bit mode.
	Mode32 bool `json:"mode32"` // Whether the instruction is supported in 32-bit mode.
	Mode16 bool `json:"mode16"` // Whether the instruction is supported in 16-bit mode.

	CPUID []string `json:"cpuid,omitempty"` // CPUID feature flags required (comma-separated).

	OperandSize bool `json:"operandSize,omitempty"` // Whether this instruction uses the operand size override prefix.
	AddressSize bool `json:"addressSize,omitempty"` // Whether this instruction uses the address size override prefix.
	DataSize    int  `json:"dataSize,omitempty"`    // Data operation size in bits.
}

// Supports returns whether inst is supported
// in the given CPU mode.
func (inst *Instruction) Supports(mode Mode) bool {
	switch mode.Int {
	case 16:
		return inst.Mode16
	case 32:
		return inst.Mode32
	case 64:
		return inst.Mode64
	default:
		panic("invalid mode " + mode.String)
	}
}

// HasCPUID returns whether inst's CPUID
// contains the given feature.
func (inst *Instruction) HasCPUID(feature string) bool {
	for _, got := range inst.CPUID {
		if got == feature {
			return true
		}
	}

	return false
}

// DisplacementCompression returns
// the value N for the instruction,
// as described in Intel x86 manuals,
// Volume 2A, Section 2.7.5.
func (inst *Instruction) DisplacementCompression(broadcast bool) (n int64, err error) {
	var inputSize int64
	if inst.Encoding.VEX_W {
		inputSize = 64
	} else {
		inputSize = 32
	}

	vectorSize := int64(inst.Encoding.VectorSize())
	if vectorSize == 0 || !inst.Encoding.EVEX {
		return 1, nil
	}

	switch inst.TupleType {
	case TupleNone:
		return 1, nil
	case TupleFull:
		if broadcast {
			return inputSize / 4, nil
		}

		return vectorSize / 8, nil
	case TupleHalf:
		if broadcast {
			return 4, nil
		}

		return vectorSize / 16, nil
	case TupleFullMem:
		return vectorSize / 8, nil
	case Tuple1Scalar:
		if inst.DataSize == 0 {
			panic("instruction " + inst.UID + " has tuple type Tuple1 Scalar but no data size")
		}

		inputSize = int64(inst.DataSize)
		return inputSize / 8, nil
	case Tuple1Fixed:
		return inputSize / 8, nil
	case Tuple2:
		return inputSize / 4, nil
	case Tuple4:
		return inputSize / 2, nil
	case Tuple8:
		return inputSize / 1, nil
	case TupleHalfMem:
		return vectorSize / 16, nil
	case TupleQuarterMem:
		return vectorSize / 32, nil
	case TupleEighthMem:
		return vectorSize / 64, nil
	case TupleMem128:
		return 16, nil
	case TupleMOVDDUP:
		switch vectorSize {
		case 128:
			return 8, nil
		case 256:
			return 32, nil
		case 512:
			return 64, nil
		}
	default:
		return 1, fmt.Errorf("unknown tuple type: %s", inst.TupleType)
	}

	panic("unreachable")
}

// FromJSON returns the set of x86
// instructions contained in the
// given x86.json data.
func FromJSON(r io.Reader) (insts []*Instruction, uids map[string]*Instruction, err error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()

	uids = make(map[string]*Instruction)
	for {
		inst := new(Instruction)
		err = dec.Decode(inst)
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, nil, err
		}

		insts = append(insts, inst)
		uids[inst.UID] = inst
	}

	return insts, uids, nil
}

// Code provides helper functionality
// for reading and writing machine
// code.
type Code struct {
	PrefixOpcodes   [5]byte    // Any mandatory opcodes used as prefixes (eg fwait). Unused opcodes are zero.
	Prefixes        [14]Prefix // Any legacy prefix bytes. Unused prefixes are zero.
	REX             REX        // Any REX prefix.
	VEX             VEX        // Any VEX prefix.
	EVEX            EVEX       // Any EVEX prefix.
	Opcode          [3]byte    // The opcode bytes.
	OpcodeLen       int        // The number of bytes of opcode.
	CodeOffset      [8]byte    // Any code offset applied to the instruction pointer.
	CodeOffsetLen   int        // The number of bytes of code offset to use.
	ModRM           ModRM      // Any ModR/M byte.
	UseModRM        bool       // Encode the ModR/M byte, even if zero.
	SIB             SIB        // Any Scale/Index/Base byte.
	Displacement    [8]byte    // Any memory address displacement.
	DisplacementLen int        // The number of bytes of address displacement.
	Immediate       [8]byte    // Any immediate integer literals.
	ImmediateLen    int        // The number of immediate bytes to use.
}

// prefixOpcodesLen returns the number of
// prefix opcode bytes.
func (c *Code) prefixOpcodesLen() int {
	for i, b := range c.PrefixOpcodes {
		if b == 0 {
			return i
		}
	}

	return len(c.PrefixOpcodes)
}

// prefixesLen returns the number of legacy
// prefix bytes.
func (c *Code) prefixesLen() int {
	for i, b := range c.Prefixes {
		if b == 0 {
			return i
		}
	}

	return len(c.Prefixes)
}

// AddPrefix appends a legacy prefix byte.
func (c *Code) AddPrefix(prefix Prefix) {
	for i, p := range c.Prefixes {
		if p == 0 {
			c.Prefixes[i] = prefix
			return
		}
	}
}

// Helper methods for setting common
// fields.

func (c *Code) SetR(b bool) {
	c.REX.SetR(b)
	c.VEX.SetR(!b)
	c.EVEX.SetR(!b)
}

func (c *Code) SetX(b bool) {
	c.REX.SetX(b)
	c.VEX.SetX(!b)
	c.EVEX.SetX(!b)
}

func (c *Code) SetB(b bool) {
	c.REX.SetB(b)
	c.VEX.SetB(!b)
	c.EVEX.SetB(!b)
}

func (c *Code) SetL(b bool) {
	c.VEX.SetL(b)
	c.EVEX.SetL(b)
}

func (c *Code) SetPP(b byte) {
	c.VEX.SetPP(b)
	c.EVEX.SetPP(b)
}

func (c *Code) SetM_MMMM(b byte) {
	c.VEX.SetM_MMMM(b)
	c.EVEX.SetMMM(b)
}

func (c *Code) SetW(b bool) {
	c.VEX.SetW(b)
	c.EVEX.SetW(b)
}

// Len returns c's length as a number of
// bytes.
func (c *Code) Len() int {
	var n int
	n += c.prefixOpcodesLen()
	n += c.prefixesLen()
	if c.REX != 0 {
		n++
	}
	if c.VEX.On() {
		if c.VEX.Can2Byte() {
			n += 2
		} else {
			n += 3
		}
	}
	if c.EVEX.On() {
		n += 4
	}
	n += c.OpcodeLen
	n += c.CodeOffsetLen
	if c.UseModRM {
		n++
	}
	if c.SIB != 0 {
		n++
	}
	n += c.DisplacementLen
	n += c.ImmediateLen
	return n
}

// EncodeTo appends the machine code to
// b.
func (c *Code) EncodeTo(b *bytes.Buffer) {
	b.Write(c.PrefixOpcodes[:c.prefixOpcodesLen()])
	for _, p := range c.Prefixes {
		if p == 0 {
			break
		}

		b.WriteByte(byte(p))
	}
	if c.REX != 0 {
		b.WriteByte(byte(c.REX))
	}
	if c.VEX.On() {
		if c.VEX.Can2Byte() {
			prefix, p0 := c.VEX.Encode2Byte()
			b.WriteByte(prefix)
			b.WriteByte(p0)
		} else {
			prefix, p0, p1 := c.VEX.Encode3Byte()
			b.WriteByte(prefix)
			b.WriteByte(p0)
			b.WriteByte(p1)
		}
	}
	if c.EVEX.On() {
		prefix, p0, p1, p2 := c.EVEX.Encode()
		b.WriteByte(prefix)
		b.WriteByte(p0)
		b.WriteByte(p1)
		b.WriteByte(p2)
	}
	b.Write(c.Opcode[:c.OpcodeLen])
	b.Write(c.CodeOffset[:c.CodeOffsetLen])
	if c.UseModRM {
		b.WriteByte(byte(c.ModRM))
	}
	if c.SIB != 0 {
		b.WriteByte(byte(c.SIB))
	}
	b.Write(c.Displacement[:c.DisplacementLen])
	b.Write(c.Immediate[:c.ImmediateLen])
}

// String returns a textual description
// of the machine code.
func (c *Code) String() string {
	first := true
	var s strings.Builder
	join := func() {
		if !first {
			s.WriteString(", ")
		}

		first = false
	}

	s.WriteByte('{')
	if l := c.prefixOpcodesLen(); l > 0 {
		first = false
		fmt.Fprintf(&s, "PrefixOpcodes: [% x]", c.PrefixOpcodes[:l])
	}
	if l := c.prefixesLen(); l > 0 {
		first = false
		fmt.Fprintf(&s, "Prefixes: [% x]", c.Prefixes[:l])
	}
	if c.REX != 0 {
		join()
		s.WriteString("REX: ")
		s.WriteString(c.REX.String())
	}
	if c.VEX.On() {
		join()
		s.WriteString("VEX: ")
		s.WriteString(c.VEX.String())
	}
	if c.EVEX.On() {
		join()
		s.WriteString("EVEX: ")
		s.WriteString(c.EVEX.String())
	}
	if c.OpcodeLen > 0 {
		join()
		fmt.Fprintf(&s, "Opcode: [% x]", c.Opcode[:c.OpcodeLen])
	}
	if c.CodeOffsetLen > 0 {
		join()
		fmt.Fprintf(&s, "CodeOffset: [% x]", c.CodeOffset[:c.CodeOffsetLen])
	}
	if c.UseModRM {
		join()
		s.WriteString("ModR/M: ")
		s.WriteString(c.ModRM.String())
	}
	if c.SIB != 0 {
		join()
		s.WriteString("SIB: ")
		s.WriteString(c.SIB.String())
	}
	if c.DisplacementLen > 0 {
		join()
		fmt.Fprintf(&s, "Displacement: [% x]", c.Displacement[:c.DisplacementLen])
	}
	if c.ImmediateLen > 0 {
		join()
		fmt.Fprintf(&s, "Immediate: [% x]", c.Immediate[:c.ImmediateLen])
	}
	s.WriteByte('}')

	return s.String()
}

// TypleType contains an EVEX instruction
// typle kind, as defined in Intel x86,
// Volume 2A, Section 2.6.5.
type TupleType uint8

const (
	TupleNone TupleType = iota
	TupleFull
	TupleHalf
	TupleFullMem
	Tuple1Scalar
	Tuple1Fixed
	Tuple2
	Tuple4
	Tuple8
	TupleHalfMem
	TupleQuarterMem
	TupleEighthMem
	TupleMem128
	TupleMOVDDUP
)

var TupleTypes = map[string]TupleType{
	"":              TupleNone,
	"None":          TupleNone,
	"Full":          TupleFull,
	"Half":          TupleHalf,
	"Full Mem":      TupleFullMem,
	"Tuple1 Scalar": Tuple1Scalar,
	"Tuple1 Fixed":  Tuple1Fixed,
	"Tuple2":        Tuple2,
	"Tuple4":        Tuple4,
	"Tuple8":        Tuple8,
	"Half Mem":      TupleHalfMem,
	"Quarter Mem":   TupleQuarterMem,
	"Eighth Mem":    TupleEighthMem,
	"Mem128":        TupleMem128,
	"MOVDDUP":       TupleMOVDDUP,
}

func (t TupleType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

func (t *TupleType) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}

	got, ok := TupleTypes[s]
	if !ok {
		return fmt.Errorf("invalid tuple type %q", s)
	}

	*t = got

	return nil
}

func (t TupleType) UID() string {
	switch t {
	case TupleNone:
		return "TupleNone"
	case TupleFull:
		return "TupleFull"
	case TupleHalf:
		return "TupleHalf"
	case TupleFullMem:
		return "TupleFullMem"
	case Tuple1Scalar:
		return "Tuple1Scalar"
	case Tuple1Fixed:
		return "Tuple1Fixed"
	case Tuple2:
		return "Tuple2"
	case Tuple4:
		return "Tuple4"
	case Tuple8:
		return "Tuple8"
	case TupleHalfMem:
		return "TupleHalfMem"
	case TupleQuarterMem:
		return "TupleQuarterMem"
	case TupleEighthMem:
		return "TupleEighthMem"
	case TupleMem128:
		return "TupleMem128"
	case TupleMOVDDUP:
		return "TupleMOVDDUP"
	default:
		return fmt.Sprintf("TupleType(%d)", t)
	}
}

func (t TupleType) String() string {
	switch t {
	case TupleNone:
		return "None"
	case TupleFull:
		return "Full"
	case TupleHalf:
		return "Half"
	case TupleFullMem:
		return "Full Mem"
	case Tuple1Scalar:
		return "Tuple1 Scalar"
	case Tuple1Fixed:
		return "Tuple1 Fixed"
	case Tuple2:
		return "Tuple2"
	case Tuple4:
		return "Tuple4"
	case Tuple8:
		return "Tuple8"
	case TupleHalfMem:
		return "Half Mem"
	case TupleQuarterMem:
		return "Quarter Mem"
	case TupleEighthMem:
		return "Eighth Mem"
	case TupleMem128:
		return "Mem128"
	case TupleMOVDDUP:
		return "MOVDDUP"
	default:
		return fmt.Sprintf("TupleType(%d)", t)
	}
}

// Prefix represents a legacy x86 prefix.
type Prefix byte

const (
	PrefixLock        Prefix = 0xf0
	PrefixRepeatNot   Prefix = 0xf2
	PrefixRepeat      Prefix = 0xf3
	PrefixCS          Prefix = 0x2e
	PrefixSS          Prefix = 0x36
	PrefixDS          Prefix = 0x3e
	PrefixES          Prefix = 0x26
	PrefixFS          Prefix = 0x64
	PrefixGS          Prefix = 0x65
	PrefixUnlikely    Prefix = 0x2e
	PrefixLikely      Prefix = 0x3e
	PrefixOperandSize Prefix = 0x66
	PrefixAddressSize Prefix = 0x67
)

func (p Prefix) String() string {
	switch p {
	case PrefixLock:
		return "lock"
	case PrefixRepeatNot:
		return "repnz/repne"
	case PrefixRepeat:
		return "rep/repe/repz"
	case PrefixCS:
		return "cs/unlikely"
	case PrefixSS:
		return "ss"
	case PrefixDS:
		return "ds/likely"
	case PrefixES:
		return "es"
	case PrefixFS:
		return "fs"
	case PrefixGS:
		return "gs"
	case PrefixOperandSize:
		return "data16/data32"
	case PrefixAddressSize:
		return "addr16/addr32"
	default:
		return fmt.Sprintf("Prefix(%#02x)", byte(p))
	}
}

// b2i is a helper function to convert
// a boolean to an integer. The result
// is one if `b` is true and 0 otherwise.
func (v *VEX) b2i(b bool) byte {
	if b {
		return 1
	}

	return 0
}

// VEX provides helper functionality
// for reading and writing a VEX
// prefix.
//
// We always store VEX prefixes in
// the 3-byte form but can export
// to the 2-byte form.
type VEX [2]byte

// Intel x86 manuals, Volume 2A,
// Section 2.3.5, Table 2-9.
//
// 3-byte form:
//
// 	| 7  6  5  4   3  2  1  0 |
// 	+-------------------------|
// 	| 1  1  0  0   0  1  0  0 | // 0xc4 prefix.
// 	| R  X  B  m   m  m  m  m | // P0.
// 	| W  v  v  v   v  L  p  p | // P1.
//
// 2-byte form:
//
// 	| 7  6  5  4   3  2  1  0 |
// 	+-------------------------|
// 	| 1  1  0  0   0  1  0  1 | // 0xc5 prefix.
// 	| R  v  v  v   v  L  p  p | // P0.

// P0.
func (v VEX) R() bool      { return ((v[0] >> 7) & 1) == 1 }
func (v VEX) X() bool      { return ((v[0] >> 6) & 1) == 1 }
func (v VEX) B() bool      { return ((v[0] >> 5) & 1) == 1 }
func (v VEX) M_MMMM() byte { return v[0] & 0b1_1111 }

// P1.
func (v VEX) W() bool    { return ((v[1] >> 7) & 1) == 1 }
func (v VEX) VVVV() byte { return (v[1] >> 3) & 0b1111 }
func (v VEX) L() bool    { return ((v[1] >> 2) & 1) == 1 }
func (v VEX) PP() byte   { return v[1] & 0b11 }

// P0.
func (v *VEX) SetR(b bool)      { v[0] = v[0]&0b0111_1111 | (v.b2i(b) << 7) }
func (v *VEX) SetX(b bool)      { v[0] = v[0]&0b1011_1111 | (v.b2i(b) << 6) }
func (v *VEX) SetB(b bool)      { v[0] = v[0]&0b1101_1111 | (v.b2i(b) << 5) }
func (v *VEX) SetM_MMMM(b byte) { v[0] = v[0]&0b1110_0000 | (b & 0b1_1111) }

// P1.
func (v *VEX) SetW(b bool)    { v[1] = v[1]&0b0111_1111 | (v.b2i(b) << 7) }
func (v *VEX) SetVVVV(b byte) { v[1] = v[1]&0b1000_0111 | ((b & 0b1111) << 3) }
func (v *VEX) SetL(b bool)    { v[1] = v[1]&0b1111_1011 | (v.b2i(b) << 2) }
func (v *VEX) SetPP(b byte)   { v[1] = v[1]&0b1111_1100 | (b & 0b11) }

func (v VEX) On() bool {
	return v.M_MMMM() != 0 // This is a reserved value so it shouldn't occur legitimately.
}

func (v *VEX) Reset() {
	v[0] = 0
	v[1] = 0
}

// Default resets the VEX prefix to its
// default state, which includes vvvv
// being set to 0b1111.
//
// If no m_mmmm field is set, the prefix
// will not count as active, according to
// VEX.On.
func (v *VEX) Default() {
	// These fields are inverted, so they
	// default to set.
	v.SetR(true)
	v.SetX(true)
	v.SetB(true)
	v.SetVVVV(0b1111)
}

func (v VEX) Can2Byte() bool {
	return v.X() && v.B() && !v.W() && v.M_MMMM() == 0b0_0001
}

func (v VEX) Encode2Byte() (b1, b2 byte) {
	// We're working on a copy, so we
	// can make changes safely. This
	// simplifies the encoding process.
	v.SetW(v.R())
	return 0xc5, v[1]
}

func (v VEX) Encode3Byte() (b1, b2, b3 byte) {
	// Simple.
	return 0xc4, v[0], v[1]
}

func (v VEX) String() string {
	return fmt.Sprintf("{R: %b, X: %b, B: %b, m-mmmm: %05b, W: %v, vvvv: %04b, L: %b, pp: %02b}",
		v.b2i(v.R()), v.b2i(v.X()), v.b2i(v.B()), v.M_MMMM(),
		v.W(), v.VVVV(), v.b2i(v.L()), v.PP())
}

// EVEX provides helper functionality
// for reading and writing an EVEX
// prefix.
type EVEX [3]byte

// b2i is a helper function to convert
// a boolean to an integer. The result
// is one if `b` is true and 0 otherwise.
func (p *EVEX) b2i(b bool) byte {
	if b {
		return 1
	}

	return 0
}

// Intel x86 manuals, Volume 2A,
// Section 2.6.1, Table 2-11.
//
// 	| 7  6  5  4   3  2  1  0 |
// 	+-------------------------|
// 	| 0  1  1  0   0  0  1  0 | // 0x62 prefix.
// 	| R  X  B  R'  0  m  m  m | // P0.
// 	| W  v  v  v   v  1  p  p | // P1.
// 	| z  L' L  b   V' a  a  a | // P2.

// P0.
func (p EVEX) R() bool   { return ((p[0] >> 7) & 1) == 1 }
func (p EVEX) X() bool   { return ((p[0] >> 6) & 1) == 1 }
func (p EVEX) B() bool   { return ((p[0] >> 5) & 1) == 1 }
func (p EVEX) Rp() bool  { return ((p[0] >> 4) & 1) == 1 }
func (p EVEX) MMM() byte { return p[0] & 0b111 }

// P1.
func (p EVEX) W() bool    { return ((p[1] >> 7) & 1) == 1 }
func (p EVEX) VVVV() byte { return (p[1] >> 3) & 0b1111 }
func (p EVEX) PP() byte   { return p[1] & 0b11 }

// P2.
func (p EVEX) Z() bool   { return ((p[2] >> 7) & 1) == 1 }
func (p EVEX) Lp() bool  { return ((p[2] >> 6) & 1) == 1 }
func (p EVEX) L() bool   { return ((p[2] >> 5) & 1) == 1 }
func (p EVEX) Br() bool  { return ((p[2] >> 4) & 1) == 1 }
func (p EVEX) Vp() bool  { return ((p[2] >> 3) & 1) == 1 }
func (p EVEX) AAA() byte { return p[2] & 0b111 }

// P0.
func (p *EVEX) SetR(b bool)   { p[0] = p[0]&0b0111_1111 | (p.b2i(b) << 7) }
func (p *EVEX) SetX(b bool)   { p[0] = p[0]&0b1011_1111 | (p.b2i(b) << 6) }
func (p *EVEX) SetB(b bool)   { p[0] = p[0]&0b1101_1111 | (p.b2i(b) << 5) }
func (p *EVEX) SetRp(b bool)  { p[0] = p[0]&0b1110_1111 | (p.b2i(b) << 4) }
func (p *EVEX) SetMMM(b byte) { p[0] = p[0]&0b1111_1000 | (b & 0b111) }

// P1.
func (p *EVEX) SetW(b bool)    { p[1] = p[1]&0b0111_1111 | (p.b2i(b) << 7) }
func (p *EVEX) SetVVVV(b byte) { p[1] = p[1]&0b1000_0111 | ((b & 0b1111) << 3) }
func (p *EVEX) SetPP(b byte)   { p[1] = p[1]&0b1111_1100 | (b & 0b11) }

// P2.
func (p *EVEX) SetZ(b bool)   { p[2] = p[2]&0b0111_1111 | (p.b2i(b) << 7) }
func (p *EVEX) SetLp(b bool)  { p[2] = p[2]&0b1011_1111 | (p.b2i(b) << 6) }
func (p *EVEX) SetL(b bool)   { p[2] = p[2]&0b1101_1111 | (p.b2i(b) << 5) }
func (p *EVEX) SetBr(b bool)  { p[2] = p[2]&0b1110_1111 | (p.b2i(b) << 4) }
func (p *EVEX) SetVp(b bool)  { p[2] = p[2]&0b1111_0111 | (p.b2i(b) << 3) }
func (p *EVEX) SetAAA(b byte) { p[2] = p[2]&0b1111_1000 | (b & 0b111) }

func (p EVEX) On() bool      { return ((p[1] >> 2) & 1) == 1 }
func (p *EVEX) SetOn(b bool) { p[1] = p[1]&0b1111_1011 | (p.b2i(b) << 2) }

func (p *EVEX) Reset() {
	p[0] = 0
	p[1] = 0
	p[2] = 0
}

// Default resets the EVEX prefix to its
// default state, which includes vvvv
// being set to 0b1111.
//
// If no mm field is set, the prefix will
// not count as active, according to
// EVEX.Required.
func (p *EVEX) Default() {
	// These fields are inverted, so they
	// default to set.
	p.SetR(true)
	p.SetX(true)
	p.SetB(true)
	p.SetRp(true)
	p.SetVVVV(0b1111)
	p.SetVp(true)
}

func (p EVEX) Encode() (prefix, p0, p1, p2 byte) {
	return 0x62, p[0], p[1], p[2]
}

func (p EVEX) String() string {
	return fmt.Sprintf("{R: %b, X: %b, B: %b, R': %b, mm: %02b // W: %b, vvvv: %04b, pp: %02b // z: %b, L': %b, L: %b, b: %b, V': %b, aaa: %03b}",
		p.b2i(p.R()), p.b2i(p.X()), p.b2i(p.B()), p.b2i(p.Rp()), p.MMM(),
		p.b2i(p.W()), p.VVVV(), p.PP(),
		p.b2i(p.Z()), p.b2i(p.Lp()), p.b2i(p.L()), p.b2i(p.Br()), p.b2i(p.Vp()), p.AAA())
}

// REX provides helper functionality
// for reading and writing a REX
// prefix byte.
type REX byte

// b2i is a helper function to convert
// a boolean to an integer. The result
// is one if `b` is true and 0 otherwise.
func (r *REX) b2i(b bool) REX {
	if b {
		return 1
	}

	return 0
}

// Intel x86 manuals, Volume 2A,
// Section 2.2.1.2, Table 2-4.
//
// 	| 7  6  5  4   3  2  1  0 |
// 	+-------------------------|
// 	| 0  1  0  0   W  R  X  B |

func (r REX) On() bool       { return ((r >> 6) & 1) == 1 }
func (r REX) W() bool        { return ((r >> 3) & 1) == 1 }
func (r REX) R() bool        { return ((r >> 2) & 1) == 1 }
func (r REX) X() bool        { return ((r >> 1) & 1) == 1 }
func (r REX) B() bool        { return ((r >> 0) & 1) == 1 }
func (r *REX) SetOn()        { *r |= (1 << 6) }
func (r *REX) SetREX(b bool) { *r |= (r.b2i(b) << 6) }
func (r *REX) SetW(b bool)   { *r = (*r & 0b11110111) | (r.b2i(b) << 3) }
func (r *REX) SetR(b bool)   { *r = (*r & 0b11111011) | (r.b2i(b) << 2) }
func (r *REX) SetX(b bool)   { *r = (*r & 0b11111101) | (r.b2i(b) << 1) }
func (r *REX) SetB(b bool)   { *r = (*r & 0b11111110) | (r.b2i(b) << 0) }

func (r REX) String() string {
	out := make([]byte, 8)
	at := func(i int, zero, one byte) byte {
		if ((r >> (7 - i)) & 1) == 1 {
			return one
		}

		return zero
	}

	out[0] = at(0, '0', '1')
	out[1] = at(1, '0', '1')
	out[2] = at(2, '0', '1')
	out[3] = at(3, '0', '1')
	out[4] = at(4, '0', 'W')
	out[5] = at(5, '0', 'R')
	out[6] = at(6, '0', 'X')
	out[7] = at(7, '0', 'B')

	return string(out)
}

// ModRM provides helper functionality
// for reading and writing a ModR/M
// byte.
type ModRM byte

const (
	ModRMmod00 ModRM = 0b00_000_000
	ModRMmod01 ModRM = 0b01_000_000
	ModRMmod10 ModRM = 0b10_000_000
	ModRMmod11 ModRM = 0b11_000_000

	// Section 2.1.5, table 2.2, Mod column.
	ModRMmodDereferenceRegister    = ModRMmod00
	ModRMmodSmallDisplacedRegister = ModRMmod01
	ModRMmodLargeDisplacedRegister = ModRMmod10
	ModRMmodRegister               = ModRMmod11

	ModRMreg000 ModRM = 0b00_000_000
	ModRMreg001 ModRM = 0b00_001_000
	ModRMreg010 ModRM = 0b00_010_000
	ModRMreg011 ModRM = 0b00_011_000
	ModRMreg100 ModRM = 0b00_100_000
	ModRMreg101 ModRM = 0b00_101_000
	ModRMreg110 ModRM = 0b00_110_000
	ModRMreg111 ModRM = 0b00_111_000

	ModRMrm000 ModRM = 0b00_000_000
	ModRMrm001 ModRM = 0b00_000_001
	ModRMrm010 ModRM = 0b00_000_010
	ModRMrm011 ModRM = 0b00_000_011
	ModRMrm100 ModRM = 0b00_000_100
	ModRMrm101 ModRM = 0b00_000_101
	ModRMrm110 ModRM = 0b00_000_110
	ModRMrm111 ModRM = 0b00_000_111

	// Section 2.1.5, table 2.2, Effective address column.
	ModRMrmSIB                = ModRMrm100
	ModRMrmDisplacementOnly32 = ModRMrm101
	ModRMrmDisplacementOnly16 = ModRMrm110
)

func (m ModRM) Mod() byte      { return byte(m&0b11000000) >> 6 }
func (m ModRM) Reg() byte      { return byte(m&0b00111000) >> 3 }
func (m ModRM) RM() byte       { return byte(m&0b00000111) >> 0 }
func (m *ModRM) SetMod(b byte) { *m = (*m & 0b00111111) | ((ModRM(b) & 0b11) << 6) }
func (m *ModRM) SetReg(b byte) { *m = (*m & 0b11000111) | ((ModRM(b) & 0b111) << 3) }
func (m *ModRM) SetRM(b byte)  { *m = (*m & 0b11111000) | ((ModRM(b) & 0b111) << 0) }

func (m ModRM) String() string {
	return fmt.Sprintf("{Mod: %02b, Reg: %03b, R/M: %03b}", m.Mod(), m.Reg(), m.RM())
}

// SIB provides helper functionality
// for reading and writing a SIB
// byte.
type SIB byte

const (
	SIBscale00 SIB = 0b00_000_000
	SIBscale01 SIB = 0b01_000_000
	SIBscale10 SIB = 0b10_000_000
	SIBscale11 SIB = 0b11_000_000

	SIBscale1 = SIBscale00
	SIBscale2 = SIBscale01
	SIBscale4 = SIBscale10
	SIBscale8 = SIBscale11

	SIBindex000 SIB = 0b00_000_000
	SIBindex001 SIB = 0b00_001_000
	SIBindex010 SIB = 0b00_010_000
	SIBindex011 SIB = 0b00_011_000
	SIBindex100 SIB = 0b00_100_000
	SIBindex101 SIB = 0b00_101_000
	SIBindex110 SIB = 0b00_110_000
	SIBindex111 SIB = 0b00_111_000

	// Section 2.1.5, table 2.3, Index column.
	SIBindexNone = SIBindex100

	SIBbase000 SIB = 0b00_000_000
	SIBbase001 SIB = 0b00_000_001
	SIBbase010 SIB = 0b00_000_010
	SIBbase011 SIB = 0b00_000_011
	SIBbase100 SIB = 0b00_000_100
	SIBbase101 SIB = 0b00_000_101
	SIBbase110 SIB = 0b00_000_110
	SIBbase111 SIB = 0b00_000_111

	// Section 2.1.5, table 2.3, Base row.
	SIBbaseStackPointer = SIBbase100
	SIBbaseNone         = SIBbase101
)

func (s SIB) Scale() byte      { return byte(s&0b11000000) >> 6 }
func (s SIB) Index() byte      { return byte(s&0b00111000) >> 3 }
func (s SIB) Base() byte       { return byte(s&0b00000111) >> 0 }
func (s *SIB) SetScale(b byte) { *s = (*s & 0b00111111) | ((SIB(b) & 0b11) << 6) }
func (s *SIB) SetIndex(b byte) { *s = (*s & 0b11000111) | ((SIB(b) & 0b111) << 3) }
func (s *SIB) SetBase(b byte)  { *s = (*s & 0b11111000) | ((SIB(b) & 0b111) << 0) }

func (m SIB) String() string {
	return fmt.Sprintf("{Scale: %02b, Index: %03b, Base: %03b}", m.Scale(), m.Index(), m.Base())
}
