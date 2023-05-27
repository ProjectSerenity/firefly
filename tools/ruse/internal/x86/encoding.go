// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package x86

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Encoding includes the textual description of
// an x86 instruction's encoding, as described
// in the Intel manuals, plus a structured
// representation of the same information.
type Encoding struct {
	// The textual representation.
	Syntax string

	// Legacy prefixes.
	PrefixOpcodes     []byte   // Any opcodes that must prefix the instruction (such as fwait).
	NoVEXPrefixes     bool     // Whether non-mandatory prefixes 66, F2, and F3 are forbidden.
	NoRepPrefixes     bool     // Whether non-mandatory prefixes F2 and F3 are forbidden.
	MandatoryPrefixes []Prefix // Any mandatory prefixes that precede the opcode.

	// REX prefixes.
	REX   bool // Whether a REX prefix is always required.
	REX_R bool // Whether a REX prefix is always required with REX.R set.
	REX_W bool // Whether a REX prefix is always required with REX.W set.

	// VEX prefixes.
	VEX       bool  // Whether a VEX prefix is always required.
	VEX_L     bool  // Any VEX.L value.
	VEXpp     uint8 // Any VEX.pp value that should be included (2 bits).
	VEXm_mmmm uint8 // Any VEX.m_mmmm value that should be included (5 bits).
	VEX_W     bool  // Any VEX.W value.
	VEX_WIG   bool  // Whether to ignore VEX.W.
	VEXis4    bool  // Whether a register is expected in the 4-bit immediate.

	// EVEX prefixes.
	EVEX     bool // Whether an EVEX prefix is always required.
	EVEX_Lp  bool // Any EVEX.L' value.
	Mask     bool // Any EVEX opmask support.
	Zero     bool // Any EVEX zeroing support.
	Rounding bool // Any EVEX embedded rounding support.
	Suppress bool // Any EVEX suppress all exceptions support.

	// Opcode data.
	Opcode           []byte // One or more opcode bytes.
	RegisterModifier int    // The opcode byte index where the register is encoded without a ModR/M byte, plus one. Zero for no modifier.
	StackIndex       int    // The opcode byte index where the FPU stack index is encoded, plus one. Zero for no index.

	// Code offset after the opcode.
	CodeOffset bool // Whether a code offset is expected.

	// ModR/M byte.
	ModRM    bool  // Whether a ModR/M byte is always required.
	ModRMmod uint8 // Any fixed value used as the ModR/M byte's mod field, plus one. Zero for no value. Five for any value except 0b11.
	ModRMreg uint8 // Any fixed value used as the ModR/M byte's reg field, plus one. Zero for no value.
	ModRMrm  uint8 // Any fixed value used as the ModR/M byte's r/m field, plus one. Zero for no value.

	// Vector SIB.
	VSIB bool // Whether the instruction uses the Vector SIB.

	// Immediates.
	ImpliedImmediate []byte // An immediate value implied by the encoding string.
}

// VectorSize returns the instruction's vector size,
// if any.
func (e *Encoding) VectorSize() int {
	if !e.VEX && !e.EVEX {
		return 0
	}

	L := e.VEX_L
	Lp := e.EVEX_Lp
	switch {
	case !L && !Lp:
		return 128
	case L && !Lp:
		return 256
	case !L && Lp:
		return 512
	default:
		panic(fmt.Sprintf("invalid VEX encoding: L: %v, L': %v", L, Lp))
	}
}

// MachineCodeMatch indicates whether a machine code
// sequence matched an instruction encoding, according
// to Encoding.MatchesMachineCode.
type MachineCodeMatch uint8

const (
	Match MachineCodeMatch = iota
	MismatchNoMachineCode
	MismatchNoOpcode
	MismatchNoPrefixOpcode
	MismatchForbiddenVEXPrefix
	MismatchForbiddenRepPrefix
	MismatchMissingMandatoryPrefix
	MismatchMissingREXPrefix
	MismatchMissingREX_R
	MismatchMissingREX_W
	MismatchMissingVEXPrefix
	MismatchTruncatedVEXPrefix
	MismatchUnexpected2ByteVEXPrefix
	MismatchMissingVEXm_mmmm
	MismatchMissingVEX_W
	MismatchMissingVEX_L
	MismatchMissingVEXpp
	MismatchMissingEVEXPrefix
	MismatchTruncatedEVEXPrefix
	MismatchWrongOpcode
	MismatchWrongModifiedOpcode
	MismatchMissingModRM
	MismatchWrongModRMreg
	MismatchMissingImpliedImmediate
	MismatchWrongImpliedImmediate
)

func (m MachineCodeMatch) String() string {
	switch m {
	case Match:
		return "match"
	case MismatchNoMachineCode:
		return "no machine code"
	case MismatchNoOpcode:
		return "no opcode"
	case MismatchNoPrefixOpcode:
		return "no prefix opcode"
	case MismatchForbiddenVEXPrefix:
		return "forbidden VEX prefix"
	case MismatchForbiddenRepPrefix:
		return "forbidden rep prefix"
	case MismatchMissingMandatoryPrefix:
		return "missing mandatory prefix"
	case MismatchMissingREXPrefix:
		return "missing REX prefix"
	case MismatchMissingREX_R:
		return "missing REX.R"
	case MismatchMissingREX_W:
		return "missing REX.W"
	case MismatchMissingVEXPrefix:
		return "missing VEX prefix"
	case MismatchTruncatedVEXPrefix:
		return "truncated VEX prefix"
	case MismatchUnexpected2ByteVEXPrefix:
		return "unexpected 2-byte VEX prefix"
	case MismatchMissingVEXm_mmmm:
		return "missing VEX.m_mmmm"
	case MismatchMissingVEX_W:
		return "missing VEX.W"
	case MismatchMissingVEX_L:
		return "missing VEX.L"
	case MismatchMissingVEXpp:
		return "missing VEX.pp"
	case MismatchMissingEVEXPrefix:
		return "missing EVEX prefix"
	case MismatchTruncatedEVEXPrefix:
		return "truncated EVEX prefix"
	case MismatchWrongOpcode:
		return "wrong opcode"
	case MismatchWrongModifiedOpcode:
		return "wrong modified opcode"
	case MismatchMissingModRM:
		return "missing Mod/RM byte"
	case MismatchWrongModRMreg:
		return "wrong ModR/M.reg"
	case MismatchMissingImpliedImmediate:
		return "missing implied immediate"
	case MismatchWrongImpliedImmediate:
		return "wrong implied immediate"
	default:
		return fmt.Sprintf("MachineCodeMatch(%d)", m)
	}
}

// MatchesMachineCode indicates whether the given
// machine code could be produced by encoding this
// instruction. This is not a perfect process, as
// it only uses the prefixes and opcodes, so missing
// or incorrect operands will not be identified.
func (e *Encoding) MatchesMachineCode(code []byte) MachineCodeMatch {
	// We must have at least some machine code.
	if len(code) == 0 {
		return MismatchNoMachineCode
	}

	// Make sure that we have any mandatory
	// prefix opcodes.
	var ok bool
	code, ok = bytes.CutPrefix(code, e.PrefixOpcodes)
	if !ok {
		return MismatchNoPrefixOpcode
	}

	// Next, we isolate the prefixes and check
	// them against the encoding.

	var prefixes []Prefix
	for i, prefix := range code {
		switch Prefix(prefix) {
		case PrefixLock,
			PrefixRepeatNot,
			PrefixRepeat,
			PrefixCS,
			PrefixSS,
			PrefixDS,
			PrefixES,
			PrefixFS,
			PrefixGS,
			PrefixOperandSize,
			PrefixAddressSize:
			prefixes = append(prefixes, Prefix(prefix))
			continue
		}

		code = code[i:]
		break
	}

	// Check we don't have any forbidden prefixes.
	for _, prefix := range prefixes {
		switch prefix {
		case 0x66, 0xf2, 0xf3:
			if e.NoVEXPrefixes || e.VEX {
				return MismatchForbiddenVEXPrefix
			}
		}

		switch prefix {
		case 0xf2, 0xf3:
			if e.NoRepPrefixes {
				return MismatchForbiddenRepPrefix
			}
		}
	}

	// Check we have all the mandatory prefixes.
	for _, want := range e.MandatoryPrefixes {
		ok = false
		for _, got := range prefixes {
			if got == want {
				ok = true
				break
			}
		}

		if !ok {
			return MismatchMissingMandatoryPrefix
		}
	}

	if len(code) == 0 {
		return MismatchNoOpcode
	}

	// Check for any mandatory REX prefix.
	if e.REX || e.REX_R || e.REX_W {
		rex := REX(code[0])
		code = code[1:]
		if rex>>4 != 0b0100 {
			// Not a REX prefix.
			return MismatchMissingREXPrefix
		}

		if e.REX_R && !rex.R() {
			// REX.R unset.
			return MismatchMissingREX_R
		}

		if e.REX_W && !rex.W() {
			// REX.W unset.
			return MismatchMissingREX_W
		}
	} else if !e.VEX && code[0]>>4 == 4 && e.Opcode[0]>>4 != 4 {
		// Looks like this is an optional REX prefix.
		rex := REX(code[0])
		code = code[1:]
		if rex>>4 != 0b0100 {
			// This isn't a valid REX prefix,
			// probably an opcode mismatch.
			return MismatchWrongOpcode
		}
	}

	// Check for an (E)VEX prefix.
	if e.EVEX && code[0] == 0x62 {
		// EVEX.
		if len(code) < 5 {
			return MismatchTruncatedEVEXPrefix
		}

		evex := (*EVEX)(code[1:4])
		code = code[4:]
		if evex.MMM() != e.VEXm_mmmm {
			return MismatchMissingVEXm_mmmm
		}

		if evex.W() != e.VEX_W {
			return MismatchMissingVEX_W
		}

		if evex.L() != e.VEX_L {
			return MismatchMissingVEX_L
		}

		if evex.PP() != e.VEXpp {
			return MismatchMissingVEXpp
		}
	} else if e.VEX || e.EVEX {
		var prefixLength int
		switch code[0] {
		case 0xc4:
			prefixLength = 3
		case 0xc5:
			prefixLength = 2
		default:
			// Invalid VEX prefix.
			if e.EVEX {
				return MismatchMissingEVEXPrefix
			}

			return MismatchMissingVEXPrefix
		}

		if len(code) <= prefixLength {
			// Not enough space for a VEX prefix and at least one opcode byte.
			return MismatchTruncatedVEXPrefix
		}

		vex := code[:prefixLength]
		code = code[prefixLength:]
		if prefixLength == 2 && (e.VEX_W || e.VEXm_mmmm != 0b0_0001) {
			// This isn't allowed to use a 2-byte
			// VEX prefix.
			return MismatchUnexpected2ByteVEXPrefix
		}

		// In a 3-byte prefix, we can
		// check m_mmmm and W.
		if prefixLength == 3 && vex[1]&0b1_1111 != e.VEXm_mmmm {
			return MismatchMissingVEXm_mmmm
		}
		if prefixLength == 3 && ((vex[2]>>7)&1 == 1) != e.VEX_W {
			return MismatchMissingVEX_W
		}

		// The last byte always includes
		// L and pp.
		last := vex[prefixLength-1]
		if ((last>>2)&1 == 1) != e.VEX_L {
			return MismatchMissingVEX_L
		}
		if last&0b11 != e.VEXpp {
			return MismatchMissingVEXpp
		}
	}

	// Finally, we check the opcode, which
	// may have a modifier.
	if len(code) < len(e.Opcode) {
		return MismatchNoOpcode
	}

	opcode := code[:len(e.Opcode)]
	code = code[len(e.Opcode):]
	switch {
	case e.RegisterModifier != 0:
		// Any opcode bytes before the
		// modifier should still be the
		// same.
		idx := e.RegisterModifier - 1
		if !bytes.Equal(opcode[:idx], e.Opcode[:idx]) {
			return MismatchWrongOpcode
		}

		// The modified opcode byte can
		// be up to 7 more than the base.
		if opcode[idx] < e.Opcode[idx] || e.Opcode[idx]+7 < opcode[idx] {
			return MismatchWrongModifiedOpcode
		}
	case e.StackIndex != 0:
		// Any opcode bytes before the
		// modifier should still be the
		// same.
		idx := e.StackIndex - 1
		if !bytes.Equal(opcode[:idx], e.Opcode[:idx]) {
			return MismatchWrongOpcode
		}

		// The modified opcode byte can
		// be up to 7 more than the base.
		if opcode[idx] < e.Opcode[idx] || e.Opcode[idx]+7 < opcode[idx] {
			return MismatchWrongModifiedOpcode
		}
	default:
		// Plain opcode.
		if !bytes.Equal(opcode, e.Opcode) {
			return MismatchWrongOpcode
		}
	}

	// Check whether we still have space
	// for any mandatory ModR/M byte.
	if e.ModRM && len(code) == 0 {
		return MismatchMissingModRM
	}

	// Check that any fixed ModR/M.reg
	// field is present.
	if e.ModRMreg != 0 {
		modrm := code[0]
		code = code[1:]
		modrmReg := (modrm >> 3) & 0b111
		if modrmReg != e.ModRMreg-1 {
			println(modrm, modrmReg, e.ModRMreg-1)
			return MismatchWrongModRMreg
		}
	}

	// Check any implied immediate value.
	if len(e.ImpliedImmediate) > len(code) {
		return MismatchMissingImpliedImmediate
	}
	if len(e.ImpliedImmediate) > 0 && !bytes.HasSuffix(code, e.ImpliedImmediate) {
		return MismatchWrongImpliedImmediate
	}

	return Match
}

// ParseEncoding processes the textual description
// of an x86 instruction's encoding, producing
// a structured representation of the same
// information.
func ParseEncoding(s string) (*Encoding, error) {
	// From the Intel x86 manuals, Volume 2A, section
	// 3.1.1.1:
	//
	// - NP: Indicates the use of 66/F2/F3 prefixes (beyond those already part of the instructions opcode) are not
	//   allowed with the instruction. Such use will either cause an invalid-opcode exception (#UD) or result in the
	//   encoding for a different instruction.
	// - NFx: Indicates the use of F2/F3 prefixes (beyond those already part of the instructions opcode) are not
	//   allowed with the instruction. Such use will either cause an invalid-opcode exception (#UD) or result in the
	//   encoding for a different instruction.
	// - REX.W: Indicates the use of a REX prefix that affects operand size or instruction semantics. The ordering of
	//   the REX prefix and other optional/mandatory instruction prefixes are discussed Chapter 2. Note that REX
	//   prefixes that promote legacy instructions to 64-bit behavior are not listed explicitly in the opcode column.
	// - /digit: A digit between 0 and 7 indicates that the ModR/M byte of the instruction uses only the r/m (register
	//   or memory) operand. The reg field contains the digit that provides an extension to the instruction's opcode.
	// - /r: Indicates that the ModR/M byte of the instruction contains a register operand and an r/m operand.
	// - cb, cw, cd, cp, co, ct: A 1-byte (cb), 2-byte (cw), 4-byte (cd), 6-byte (cp), 8-byte (co) or 10-byte (ct) value
	//   following the opcode. This value is used to specify a code offset and possibly a new value for the code segment
	//   register.
	// - ib, iw, id, io: A 1-byte (ib), 2-byte (iw), 4-byte (id) or 8-byte (io) immediate operand to the instruction that
	//   follows the opcode, ModR/M bytes or scale-indexing bytes. The opcode determines if the operand is a signed
	//   value. All words, doublewords and quadwords are given with the low-order byte first.
	// - +rb, +rw, +rd, +ro: Indicated the lower 3 bits of the opcode byte is used to encode the register operand
	//   without a modR/M byte. The instruction lists the corresponding hexadecimal value of the opcode byte with low
	//   3 bits as 000b. In non-64-bit mode, a register code, from 0 through 7, is added to the hexadecimal value of the
	//   opcode byte. In 64-bit mode, indicates the four bit field of REX.b and opcode[2:0] field encodes the register
	//   operand of the instruction. “+ro” is applicable only in 64-bit mode. See Table 3-1 for the codes.
	// - +i: A number used in floating-point instructions when one of the operands is ST(i) from the FPU register stack.
	//   The number i (which can range from 0 to 7) is added to the hexadecimal byte given at the left of the plus sign
	//   to form a single opcode byte.

	e := &Encoding{
		Syntax: s,
	}

	// Start with any prefixes.
	parts := strings.Fields(s)
prefixes:
	for i, clause := range parts {
		switch clause {
		case "NP":
			e.NoVEXPrefixes = true
		case "NFx":
			e.NoRepPrefixes = true
		case "REX":
			e.REX = true
		case "REX.R":
			e.REX = true
			e.REX_R = true
		case "REX.W":
			e.REX = true
			e.REX_W = true
		case "F0": // LOCK.
			e.MandatoryPrefixes = append(e.MandatoryPrefixes, 0xf0)
		case "F2": // REPNE/REPNZ or BND.
			e.MandatoryPrefixes = append(e.MandatoryPrefixes, 0xf2)
		case "F3": // REP or REPE/REPZ.
			e.MandatoryPrefixes = append(e.MandatoryPrefixes, 0xf3)
		case "2E": // CS or unlikely.
			e.MandatoryPrefixes = append(e.MandatoryPrefixes, 0x2e)
		case "36": // SS.
			e.MandatoryPrefixes = append(e.MandatoryPrefixes, 0x36)
		case "3E": // DS or likely.
			e.MandatoryPrefixes = append(e.MandatoryPrefixes, 0x3e)
		case "26": // ES.
			e.MandatoryPrefixes = append(e.MandatoryPrefixes, 0x26)
		case "64": // FS.
			e.MandatoryPrefixes = append(e.MandatoryPrefixes, 0x64)
		case "65": // GS.
			e.MandatoryPrefixes = append(e.MandatoryPrefixes, 0x65)
		case "66": // operand size.
			e.MandatoryPrefixes = append(e.MandatoryPrefixes, 0x66)
		case "67": // address size.
			e.MandatoryPrefixes = append(e.MandatoryPrefixes, 0x67)
		case "9B": // Prefix opcode: fwait.
			if len(parts[i:]) > 1 {
				e.PrefixOpcodes = append(e.PrefixOpcodes, 0x9b)
				continue
			}

			// If it's not a prefix opcode, we
			// stop here.
			fallthrough
		default:
			parts = parts[i:]
			break prefixes
		}
	}

	// Some specialised instructions
	// hard-code an immediate value
	// that is used in the more general
	// instruction form to select the
	// special form. This is represented
	// in the encoding as a hex value
	// after /r, in place of ib.
	//
	// To ensure that we put it in the
	// immediate field and not the
	// opcode, we track whether we've
	// seen /r yet.
	seenSlashR := false

	// Parse the remaining encoding
	// to identify the different fields.
	for _, clause := range parts {
		// See section 3.1.1.1.
		switch {
		case strings.HasSuffix(clause, "+rb"), strings.HasSuffix(clause, "+rw"), strings.HasSuffix(clause, "+rd"), strings.HasSuffix(clause, "+ro"):
			opcode, _, ok := strings.Cut(clause, "+")
			if !ok {
				return nil, fmt.Errorf("failed to parse opcode register modifier %q", clause)
			}

			b, err := strconv.ParseUint(opcode, 16, 8)
			if err != nil {
				return nil, fmt.Errorf("invalid opcode register modifier clause %q: %v", clause, err)
			}

			e.Opcode = append(e.Opcode, byte(b))
			e.RegisterModifier = len(e.Opcode)
			continue
		case strings.HasSuffix(clause, "+i"):
			opcode := strings.TrimSuffix(clause, "+i")
			b, err := strconv.ParseUint(opcode, 16, 8)
			if err != nil {
				return nil, fmt.Errorf("invalid FPU stack index clause %q: %v", clause, err)
			}

			e.Opcode = append(e.Opcode, byte(b))
			e.StackIndex = len(e.Opcode)
			continue
		}

		// Handle EVEX clauses, as they're complex.
		if strings.HasPrefix(clause, "EVEX.") {
			e.EVEX = true
			parts := strings.Split(clause, ".")
			for _, part := range parts {
				switch part {
				case "EVEX":
				case "NDS", "NDD", "DDS":
					// The NDS/NDD/DDS terms can be ignored,
					// as their information is also encoded
					// in the parameter details.
				case "128", "LLIG":
					e.VEX_L = false
					e.EVEX_Lp = false
				case "256":
					e.VEX_L = true
					e.EVEX_Lp = false
				case "512":
					e.VEX_L = false
					e.EVEX_Lp = true
				case "NP":
					e.NoVEXPrefixes = true
				case "66":
					e.VEXpp = 0b01
				case "F3":
					e.VEXpp = 0b10
				case "F2":
					e.VEXpp = 0b11
				case "0F":
					e.VEXm_mmmm = 0b0_0001
				case "0F38":
					e.VEXm_mmmm = 0b0_0010
				case "0F3A":
					e.VEXm_mmmm = 0b0_0011
				case "MAP5":
					e.VEXm_mmmm = 0b0_0101
				case "MAP6":
					e.VEXm_mmmm = 0b0_0110
				case "WIG":
					e.VEX_WIG = true
					fallthrough
				case "W0":
					e.VEX_W = false
				case "W1":
					e.VEX_W = true
				default:
					return nil, fmt.Errorf("invalid encoding clause %s: bad EVEX clause %q", clause, part)
				}
			}

			// Check mandatory fields.
			if e.VEXm_mmmm == 0 {
				return nil, fmt.Errorf("invalid encoding clause %s: missing EVEX.mm", clause)
			}

			continue
		}

		// Handle VEX clauses, as they're complex.
		if strings.HasPrefix(clause, "VEX.") {
			e.VEX = true
			parts := strings.Split(clause, ".")
			for _, part := range parts {
				switch part {
				case "VEX":
				case "NDS", "NDD", "DDS":
					// The NDS/NDD/DDS terms can be ignored,
					// as their information is also encoded
					// in the parameter details.
				case "128", "L0", "LZ", "LIG":
					e.VEX_L = false
				case "256", "L1":
					e.VEX_L = true
				case "NP":
					e.VEXpp = 0b00
				case "66":
					e.VEXpp = 0b01
				case "F3":
					e.VEXpp = 0b10
				case "F2":
					e.VEXpp = 0b11
				case "0F":
					e.VEXm_mmmm = 0b0_0001
				case "0F38":
					e.VEXm_mmmm = 0b0_0010
				case "0F3A":
					e.VEXm_mmmm = 0b0_0011
				case "WIG":
					e.VEX_WIG = true
					fallthrough
				case "W0":
					e.VEX_W = false
				case "W1":
					e.VEX_W = true
				default:
					return nil, fmt.Errorf("invalid encoding clause %s: bad VEX clause %q", clause, part)
				}
			}

			// Check mandatory fields.
			if e.VEXm_mmmm == 0 {
				return nil, fmt.Errorf("invalid encoding clause %s: missing VEX.m_mmmm", clause)
			}

			continue
		}

		// Handle fixed ModR/M clauses, as they're complex.
		if strings.Contains(clause, ":") {
			fields := strings.Split(clause, ":")
			if len(fields) != 3 {
				return nil, fmt.Errorf("invalid encoding clause %s: failed to parse ModR/M fields", clause)
			}

			switch fields[0] {
			case "11":
				e.ModRMmod = 0b11 + 1
			case "!(11)":
				e.ModRMmod = 5 // Any value except 0b11.
			default:
				return nil, fmt.Errorf("invalid encoding clause %s: invalid ModR/M.mod field %q", clause, fields[0])
			}

			switch fields[1] {
			case "rrr":
				e.ModRMreg = 0 // Any value.
			default:
				n, err := strconv.ParseUint(fields[1], 2, 8)
				if err != nil {
					return nil, fmt.Errorf("invalid encoding clause %s: invalid ModR/M.reg field %q: %v", clause, fields[1], err)
				}

				if n > 0b111 {
					return nil, fmt.Errorf("invalid encoding clause %s: invalid ModR/M.reg field %q: exceeds bounds", clause, fields[1])
				}

				e.ModRMreg = uint8(n) + 1
			}

			switch fields[2] {
			case "bbb":
				e.ModRMrm = 0 // Any value.
			default:
				n, err := strconv.ParseUint(fields[2], 2, 8)
				if err != nil {
					return nil, fmt.Errorf("invalid encoding clause %s: invalid ModR/M.r/m field %q: %v", clause, fields[2], err)
				}

				if n > 0b111 {
					return nil, fmt.Errorf("invalid encoding clause %s: invalid ModR/M.r/m field %q: exceeds bounds", clause, fields[2])
				}

				e.ModRMrm = uint8(n) + 1
			}

			e.ModRM = true

			continue
		}

		switch clause {
		// Unused syntax.
		case "+":
		// Opcode extensions.
		case "/0", "/1", "/2", "/3", "/4", "/5", "/6", "/7":
			digit := byte(clause[1] - '0')
			e.ModRMreg = digit + 1
			e.ModRM = true
		// R/M operand.
		case "/r":
			// Make sure we always include the
			// ModR/M byte, even if zero.
			e.ModRM = true
			seenSlashR = true
		// Code offset.
		case "cb", "cw", "cd", "cp", "co", "ct":
			if e.CodeOffset {
				return nil, fmt.Errorf("invalid encoding clause: unexpected second code offset clause %q", clause)
			}

			e.CodeOffset = true
		// Immediate values.
		case "ib", "iw", "id", "io":
			// Nothing to do here, as
			// the information is also
			// in the parameters.
		case "/is4":
			if e.VEXis4 {
				return nil, fmt.Errorf("invalid encoding clause: unexpected second %q clause", clause)
			}

			e.VEXis4 = true
		case "/vsib":
			e.VSIB = true
		default:
			b, err := strconv.ParseUint(clause, 16, 8)
			if err != nil {
				return nil, fmt.Errorf("bad encoding syntax %q: failed to handle encoding clause %q", s, clause)
			}

			if seenSlashR {
				e.ImpliedImmediate = append(e.ImpliedImmediate, byte(b))
			} else {
				e.Opcode = append(e.Opcode, byte(b))
			}
		}
	}

	return e, nil
}
