// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// The bulk of the code for encoding x86
// instructions.

package compiler

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	"firefly-os.dev/tools/ruse/internal/x86"
	"firefly-os.dev/tools/ruse/ssafir"
	"firefly-os.dev/tools/ruse/token"
)

const (
	// Section 2.1.5, table 2.2, Mod column.
	modDerefenceRegister      byte = 0b00
	modSmallDisplacedRegister byte = 0b01
	modLargeDisplacedRegister byte = 0b10
	modRegister               byte = 0b11

	// Section 2.1.5, table 2.2, Effective address column.
	rmSIB                byte = 0b100
	rmDisplacementOnly32 byte = 0b101
	rmDisplacementOnly16 byte = 0b110

	// Section 2.1.5, table 2.3, Base row.
	sibStackPointerBase byte = 0b100
	sibNoBase           byte = 0b101
	sibNoIndex          byte = 0b100
)

func encodeX86(w io.Writer, fset *token.FileSet, fun *ssafir.Function) error {
	write := false
	b, ok := w.(*bytes.Buffer)
	if !ok {
		write = true
		b = new(bytes.Buffer)
	}

	errorf := func(pos token.Pos, format string, v ...any) error {
		position := fset.Position(pos)
		return errors.New(fmt.Sprintf("%s: ", position) + fmt.Sprintf(format, v...))
	}

	mode, ok := fun.Extra.(x86.Mode)
	if !ok {
		return errorf(fun.Entry.Pos, "internal error: x86 function %q has no CPU mode", fun.Name)
	}

	var code x86.Code
	for _, block := range fun.Blocks {
		for _, v := range block.Values {
			info := v.Op.Info()
			if info.Virtual {
				continue
			}

			data, ok := v.Extra.(*x86InstructionData)
			if !ok {
				position := fset.Position(v.Pos)
				return fmt.Errorf("%s: internal error: expression compiled to non-instruction value %v", position, v)
			}

			err := data.Encode(&code, mode)
			if err != nil {
				return errorf(v.Pos, "%v", err)
			}

			code.EncodeTo(b)
		}
	}

	if write {
		_, err := w.Write(b.Bytes())
		if err != nil {
			return err
		}
	}

	return nil
}

// Encode follows the rules in the x86-64 manual, volume 2A,
// chapters 2 and 3, to encode an instruction.
func (data *x86InstructionData) Encode(code *x86.Code, mode x86.Mode) (err error) {
	*code = x86.Code{} // Reset.
	code.VEX.Default()
	code.EVEX.Default()

	seenPrefix := make(map[x86.Prefix]bool)
	addPrefix := func(prefix x86.Prefix) {
		if !seenPrefix[prefix] {
			seenPrefix[prefix] = true
			code.AddPrefix(prefix)
		}
	}

	// Start with the mandatory encoding
	// details defined in the instruction.

	copy(code.PrefixOpcodes[:], data.Inst.Encoding.PrefixOpcodes)
	for _, prefix := range data.Inst.Encoding.MandatoryPrefixes {
		addPrefix(prefix)
	}

	if data.Inst.Encoding.REX || data.REX_W {
		code.REX.SetOn()
	}

	code.REX.SetR(data.Inst.Encoding.REX_R)
	code.REX.SetW(data.REX_W || data.Inst.Encoding.REX_W)
	code.SetL(data.Inst.Encoding.VEX_L)
	code.SetPP(data.Inst.Encoding.VEXpp)
	code.SetM_MMMM(data.Inst.Encoding.VEXm_mmmm)
	code.SetW(data.Inst.Encoding.VEX_W)
	code.EVEX.SetOn(data.Inst.Encoding.EVEX)
	code.EVEX.SetLp(data.Inst.Encoding.EVEX_Lp)
	code.EVEX.SetZ(data.Zero)
	code.EVEX.SetAAA(data.Mask)
	code.EVEX.SetBr(data.Broadcast)

	// Store the opcode.
	code.OpcodeLen = copy(code.Opcode[:], data.Inst.Encoding.Opcode)

	if data.Inst.Encoding.ModRMreg != 0 {
		code.ModRM.SetReg(data.Inst.Encoding.ModRMreg - 1)
	}

	// Then include everything specified
	// in the assembly.

	// Prefixes are straightforward.
	for _, b := range data.Prefixes {
		addPrefix(b)
	}

	// Check for an address with a segment override,
	// making a segment prefix necessary.
	//
	// We also check for an address with a non-standard
	// size, making the address override prefix
	// necessary.
	for i, param := range data.Inst.Parameters {
		if param.Type != x86.TypeMemory {
			continue
		}

		addr := data.Args[i].(*x86.Memory)

		var bits int
		switch {
		case addr.Base == nil && addr.Index == nil:
			// We don't try to guess the address
			// size without a register being involved.
			// Hopefully this is fine.
		case addr.Base == nil:
			bits = addr.Index.Bits
		case addr.Index == nil:
			bits = addr.Base.Bits
		case addr.Base.Bits == addr.Index.Bits:
			bits = addr.Base.Bits
		default:
			return fmt.Errorf("invalid assembly directive: invalid argument %d: found address with base %s and index %s of different sizes", i, addr.Base, addr.Index)
		}

		if 8 < bits && bits < 64 && bits != int(mode.Int) { // We ignore large registers.
			addPrefix(x86.PrefixAddressSize)
		}

		// Check for an explicit segment.
		if addr.Segment == nil {
			continue
		}

		switch addr.Segment {
		case x86.ES:
			addPrefix(x86.PrefixES)
		case x86.CS:
			addPrefix(x86.PrefixCS)
		case x86.SS:
			addPrefix(x86.PrefixSS)
		case x86.DS:
			addPrefix(x86.PrefixDS)
		case x86.FS:
			addPrefix(x86.PrefixFS)
		case x86.GS:
			addPrefix(x86.PrefixGS)
		default:
			return fmt.Errorf("invalid assembly directive: invalid argument %d: found address with invalid segment %s", i, addr.Segment)
		}

		// We can't have more than one
		// address, so we can stop here.
		break
	}

	// Check for a data operation with a
	// non-standard size, making the operand
	// override prefix necessary.
	if 8 < data.Inst.DataSize && data.Inst.DataSize <= 64 && data.Inst.OperandSize {
		var defaultBits int
		switch mode.Int {
		case 16:
			defaultBits = 16
		case 32, 64:
			defaultBits = 32
		}

		if data.Inst.DataSize == 64 {
			code.REX.SetOn()
			code.REX.SetW(true)
		} else if data.Inst.DataSize != defaultBits && !data.Inst.Encoding.NoVEXPrefixes {
			addPrefix(x86.PrefixOperandSize)
		}
	}

	for i, param := range data.Inst.Parameters {
		var err error
		switch param.Encoding {
		case x86.EncodingNone:
			// Nothing to do, unless this is a
			// string operation, in which case
			// we may need to add an address
			// size override.
			if param.Type == x86.TypeStringDst || param.Type == x86.TypeStringSrc {
				reg := data.Args[i].(*x86.Register)
				if reg.Bits != int(mode.Int) {
					addPrefix(x86.PrefixAddressSize)
				}
			}
		case x86.EncodingVEXvvvv:
			arg := data.Args[i].(*x86.Register)
			vp, vvvv := arg.VEXvvvv()
			code.EVEX.SetVp(vp)
			code.VEX.SetVVVV(vvvv)
			code.EVEX.SetVVVV(vvvv)
		case x86.EncodingRegisterModifier:
			arg := data.Args[i].(*x86.Register)
			idx := data.Inst.Encoding.RegisterModifier - 1
			_, rex, rexB, reg := arg.ModRM()
			code.REX.SetREX(rex)
			code.SetB(rexB)
			code.Opcode[idx] += reg
		case x86.EncodingStackIndex:
			mod := data.Args[i].(uint8)
			idx := data.Inst.Encoding.StackIndex - 1
			code.Opcode[idx] += mod
		case x86.EncodingCodeOffset:
			arg := data.Args[i].(uint64)
			switch param.Bits {
			case 8:
				if param.Type == x86.TypeRelativeAddress {
					rel := int8(uint8(arg))
					rel -= 1 + int8(code.Len())
					arg = uint64(int64(rel))
				}
				code.CodeOffset[code.CodeOffsetLen] = uint8(arg)
				code.CodeOffsetLen += 1
			case 16:
				if param.Type == x86.TypeRelativeAddress {
					rel := int16(uint16(arg))
					rel -= 2 + int16(code.Len())
					arg = uint64(int64(rel))
				}
				binary.LittleEndian.PutUint16(code.CodeOffset[code.CodeOffsetLen:], uint16(arg))
				code.CodeOffsetLen += 2
			case 32:
				if param.Type == x86.TypeRelativeAddress {
					rel := int32(uint32(arg))
					rel -= 4 + int32(code.Len())
					arg = uint64(int64(rel))
				}
				binary.LittleEndian.PutUint32(code.CodeOffset[code.CodeOffsetLen:], uint32(arg))
				code.CodeOffsetLen += 4
			case 48:
				binary.LittleEndian.PutUint16(code.CodeOffset[code.CodeOffsetLen:], uint16(arg))
				code.CodeOffsetLen += 2
				binary.LittleEndian.PutUint32(code.CodeOffset[code.CodeOffsetLen:], uint32(arg>>16))
				code.CodeOffsetLen += 4
			default:
				panic(fmt.Sprintf("unsupported code offset: %d bits", param.Bits))
			}
		case x86.EncodingModRMreg:
			arg := data.Args[i].(*x86.Register)
			evexR, rex, rexR, reg := arg.ModRM()
			code.EVEX.SetRp(evexR)
			code.REX.SetREX(rex)
			code.SetR(rexR)
			code.ModRM.SetReg(reg)
		case x86.EncodingModRMrm:
			switch arg := data.Args[i].(type) {
			case *x86.Register:
				evexX, rex, rexB, reg := arg.ModRM()
				code.EVEX.SetX(evexX)
				code.REX.SetREX(rex)
				code.SetB(rexB)
				code.ModRM.SetMod(modRegister)
				code.ModRM.SetRM(reg)
			case *x86.Memory:
				err = data.encodeMemory(code, mode, arg)
				if err != nil {
					return fmt.Errorf("invalid argument %d: %v", i, err)
				}
			default:
				return fmt.Errorf("invalid argument %d: %s encoding specified for unexpected type %T", i, param.Encoding, data.Args[i])
			}
		case x86.EncodingDisplacement:
			arg := data.Args[i].(*x86.Memory)
			_, _, err := data.addDisplacement(code, arg.Base, mode, param.Type == x86.TypeMemoryOffset, arg.Displacement)
			if err != nil {
				return fmt.Errorf("invalid argument %d: %v", i, err)
			}
			// No ModR/M byte for a memory offset.
		case x86.EncodingImmediate:
			arg := data.Args[i].(uint64)
			switch param.Bits {
			case 5, 8:
				code.Immediate[code.ImmediateLen] = uint8(arg)
				code.ImmediateLen += 1
			case 16:
				binary.LittleEndian.PutUint16(code.Immediate[code.ImmediateLen:], uint16(arg))
				code.ImmediateLen += 2
			case 32:
				binary.LittleEndian.PutUint32(code.Immediate[code.ImmediateLen:], uint32(arg))
				code.ImmediateLen += 4
			case 64:
				binary.LittleEndian.PutUint64(code.Immediate[code.ImmediateLen:], arg)
				code.ImmediateLen += 8
			default:
				panic(fmt.Sprintf("unsupported immediate: %d bits", param.Bits))
			}
		case x86.EncodingVEXis4:
			arg := data.Args[i].(*x86.Register)
			is4 := arg.VEXis4()
			code.Immediate[code.ImmediateLen] = is4
			code.ImmediateLen += 1
		default:
			return fmt.Errorf("invalid argument %d: unrecognised encoding %d", i, param.Encoding)
		}
	}

	if !code.VEX.On() {
		// We default to setting VEX.vvvv = 0b1111. We undo that if VEX is unused.
		code.VEX.Reset()
		code.EVEX.Reset()
	} else if code.EVEX.On() {
		code.VEX.Reset()
		code.REX = 0
	} else {
		code.EVEX.Reset()
		code.REX = 0
	}

	if !code.REX.On() {
		code.REX = 0
	}

	if code.ModRM != 0 || data.Inst.Encoding.ModRM {
		code.UseModRM = true
	}

	// Finally, append any implied immediate.
	code.ImmediateLen += copy(code.Immediate[code.ImmediateLen:], data.Inst.Encoding.ImpliedImmediate)

	return nil
}

// addDisplacement is a helper function for encoding a
// memory address displacement.
func (data *x86InstructionData) addDisplacement(code *x86.Code, base *x86.Register, mode x86.Mode, isMoffset bool, displ int64) (mod, rm byte, err error) {
	size := int(mode.Int)
	if base != nil && base.Bits != 0 {
		size = base.Bits
	}

	switch size {
	case 16:
		rm = rmDisplacementOnly16
	case 32, 64:
		rm = rmDisplacementOnly32
	}

	// Determine whether to compress the displacement.
	N, err := data.Inst.DisplacementCompression(data.Broadcast)
	if err != nil {
		return 0, 0, err
	}

	compressed := displ / N
	if displ%N != 0 {
		compressed = displ
	}

	switch {
	case base != nil && math.MinInt8 <= compressed && compressed <= math.MaxInt8 && displ%N == 0:
		mod = modSmallDisplacedRegister
		code.Displacement[code.DisplacementLen] = uint8(int8(compressed))
		code.DisplacementLen += 1
	case size == 16 && ((math.MinInt16 <= displ && displ <= math.MaxInt16) || (0 <= displ && displ <= math.MaxUint16)):
		mod = modLargeDisplacedRegister
		binary.LittleEndian.PutUint16(code.Displacement[code.DisplacementLen:], uint16(int16(displ)))
		code.DisplacementLen += 2
	case (size == 32 || size == 64) && ((math.MinInt32 <= displ && displ <= math.MaxInt32) || (0 <= displ && displ <= math.MaxUint32)):
		mod = modLargeDisplacedRegister
		binary.LittleEndian.PutUint32(code.Displacement[code.DisplacementLen:], uint32(int32(displ)))
		code.DisplacementLen += 4
	case mode.Int == 64 && base == nil && isMoffset: // No need to bounds check here, as the value is already in a 64-bit value.
		mod = modDerefenceRegister
		binary.LittleEndian.PutUint64(code.Displacement[code.DisplacementLen:], uint64(displ))
		code.DisplacementLen += 8
	default:
		return 0, 0, fmt.Errorf("invalid displacement %#x for mode %d", displ, mode.Int)
	}

	if base == nil {
		mod = modDerefenceRegister
	}

	return mod, rm, nil
}

// Encode follows the rules in the x86-64 manual, volume 2A,
// chapters 2 and 3, to encode a memory reference.
func (data *x86InstructionData) encodeMemory(code *x86.Code, mode x86.Mode, m *x86.Memory) (err error) {
	convertScale := func(err *error, scale uint8) uint8 {
		if *err != nil {
			return 0
		}

		switch scale {
		case 1:
			return 0
		case 2:
			return 1
		case 4:
			return 2
		case 8:
			return 3
		default:
			*err = fmt.Errorf("invalid scale %d", scale)
			return 0
		}
	}

	// See https://blog.yossarian.net/2020/06/13/How-x86-addresses-memory
	base := m.Base != nil
	index := m.Index != nil
	scale := m.Scale != 0
	displacement := m.Displacement != 0
	switch {
	case base && index && scale && displacement:
		code.ModRM.SetRM(rmSIB)
		rex, rexB, base := m.Base.Base()
		code.REX.SetREX(rex)
		code.SetB(rexB)
		code.SIB.SetBase(base)
		_, rex, rexX, idx := m.Index.ModRM()
		code.REX.SetREX(rex)
		code.SetX(rexX)
		code.SIB.SetIndex(idx)
		scale := convertScale(&err, m.Scale)
		code.SIB.SetScale(scale)
		mod, _, err := data.addDisplacement(code, m.Base, mode, false, m.Displacement)
		code.ModRM.SetMod(mod)
		if err != nil {
			return err
		}
	case index && scale && displacement:
		code.ModRM.SetRM(rmSIB)
		code.SIB.SetBase(sibNoBase)
		_, rex, rexX, idx := m.Index.ModRM()
		code.REX.SetREX(rex)
		code.SetX(rexX)
		code.SIB.SetIndex(idx)
		scale := convertScale(&err, m.Scale)
		code.SIB.SetScale(scale)
		mod, _, err := data.addDisplacement(code, m.Base, mode, false, m.Displacement)
		code.ModRM.SetMod(mod)
		if err != nil {
			return err
		}
	case base && index && scale:
		code.ModRM.SetRM(rmSIB)
		rex, rexB, base := m.Base.Base()
		code.REX.SetREX(rex)
		code.SetB(rexB)
		code.SIB.SetBase(base)
		switch m.Base {
		case x86.BP, x86.EBP, x86.RBP:
			mod, _, err := data.addDisplacement(code, m.Base, mode, false, m.Displacement)
			code.ModRM.SetMod(mod)
			if err != nil {
				return err
			}
		}
		_, rex, rexX, idx := m.Index.ModRM()
		code.REX.SetREX(rex)
		code.SetX(rexX)
		code.SIB.SetIndex(idx)
		scale := convertScale(&err, m.Scale)
		code.SIB.SetScale(scale)
	case base && index && displacement:
		code.ModRM.SetRM(rmSIB)
		rex, rexB, base := m.Base.Base()
		code.REX.SetREX(rex)
		code.SetB(rexB)
		code.SIB.SetBase(base)
		_, rex, rexX, idx := m.Index.ModRM()
		code.REX.SetREX(rex)
		code.SetX(rexX)
		code.SIB.SetIndex(idx)
		mod, _, err := data.addDisplacement(code, m.Base, mode, false, m.Displacement)
		code.ModRM.SetMod(mod)
		if err != nil {
			return err
		}
	case index && scale:
		code.ModRM.SetRM(rmSIB)
		code.SIB.SetBase(sibNoBase)
		_, rex, rexX, idx := m.Index.ModRM()
		code.REX.SetREX(rex)
		code.SetX(rexX)
		code.SIB.SetIndex(idx)
		scale := convertScale(&err, m.Scale)
		code.SIB.SetScale(scale)
		// With no base, we still have to
		// include a 32-bit displacement.
		binary.LittleEndian.PutUint32(code.Displacement[code.DisplacementLen:], 0)
		code.DisplacementLen += 4
	case base && displacement:
		switch m.Base {
		case x86.ESP, x86.RSP:
			// This has to go in the SIB byte instead.
			code.ModRM.SetRM(rmSIB)
			code.SIB.SetBase(sibStackPointerBase)
			code.SIB.SetIndex(sibNoIndex)
			mod, _, err := data.addDisplacement(code, m.Base, mode, false, m.Displacement)
			code.ModRM.SetMod(mod)
			if err != nil {
				return err
			}
		default:
			mod, _, err := data.addDisplacement(code, m.Base, mode, false, m.Displacement)
			code.ModRM.SetMod(mod)
			if err != nil {
				return err
			}
			rex, rexB, base := m.Base.Base()
			code.REX.SetREX(rex)
			code.SetB(rexB)
			code.ModRM.SetRM(base)
		}
	case base && index:
		code.ModRM.SetRM(rmSIB)
		rex, rexB, base := m.Base.Base()
		code.REX.SetREX(rex)
		code.SetB(rexB)
		code.SIB.SetBase(base)
		switch m.Base {
		case x86.BP, x86.EBP, x86.RBP:
			mod, _, err := data.addDisplacement(code, m.Base, mode, false, m.Displacement)
			code.ModRM.SetMod(mod)
			if err != nil {
				return err
			}
		}
		_, rex, rexX, idx := m.Index.ModRM()
		code.REX.SetREX(rex)
		code.SetX(rexX)
		code.SIB.SetIndex(idx)
		code.SIB.SetScale(0)
	case base:
		rex, rexB, base := m.Base.Base()
		code.REX.SetREX(rex)
		code.SetB(rexB)
		code.ModRM.SetRM(base)
		mod := modDerefenceRegister
		switch m.Base {
		case x86.BP, x86.EBP, x86.RBP:
			mod, _, err = data.addDisplacement(code, m.Base, mode, false, m.Displacement)
			if err != nil {
				return err
			}
		case x86.ESP, x86.RSP:
			// This has to go in the SIB byte instead.
			code.SIB.SetBase(sibStackPointerBase)
			code.SIB.SetIndex(sibNoIndex)
		}
		code.ModRM.SetMod(mod)
	case !base && !index && !scale && !displacement:
		// This is just a zero displacement.
		fallthrough
	case displacement:
		if mode.Int == 64 {
			// This has to go in the SIB byte (see 2.2.1.6).
			code.ModRM.SetRM(rmSIB)
			code.SIB.SetBase(sibNoBase)
			code.SIB.SetIndex(sibNoIndex)
			code.SIB.SetScale(0)
			mod, _, err := data.addDisplacement(code, m.Base, mode, false, m.Displacement)
			code.ModRM.SetMod(mod)
			if err != nil {
				return err
			}
		} else {
			mod, rm, err := data.addDisplacement(code, m.Base, mode, false, m.Displacement)
			code.ModRM.SetMod(mod)
			code.ModRM.SetRM(rm)
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("internal error: malformed x86-64 address: %#v", m)
	}

	return nil
}
