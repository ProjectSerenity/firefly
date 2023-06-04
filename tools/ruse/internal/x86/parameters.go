// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package x86

import (
	"fmt"
)

// Parameter includes structured informattion
// about a parameter to an x86 instruction.
type Parameter struct {
	Type      ParameterType     // The parameter type.
	Encoding  ParameterEncoding // The way the operand is encoded in machien code.
	UID       string            // The unique identifier of the parameter.
	Bits      int               // The parameter size in bits.
	Syntax    string            // The Intel syntax for the parameter.
	Registers []*Register       // The set of acceptable registers (if any) for this operand.
}

func (p *Parameter) String() string {
	return fmt.Sprintf("%s %s (%s)", p.Type, p.Syntax, p.Encoding)
}

// ParameterType categories a parameter
// to an x86 instruction.
type ParameterType uint8

const (
	_                     ParameterType = iota
	TypeSignedImmediate                 // A signed integer literal.
	TypeUnsignedImmediate               // An unsigned integer literal.
	TypeRegister                        // A register selection.
	TypeStackIndex                      // An x87 FPU stack index.
	TypeRelativeAddress                 // An address offset from the instruction pointer.
	TypeFarPointer                      // A segment selector and absolute address offset pair.
	TypeMemory                          // A memory address expression.
	TypeMemoryOffset                    // A memory offset expression.
	TypeStringDst                       // A memory address for a string destination.
	TypeStringSrc                       // A memory address for a string source.
)

var ParameterTypes = map[string]ParameterType{
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

func (t ParameterType) String() string {
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

func (t ParameterType) UID() string {
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

// ParameterEncoding represents a way in
// which an x86 instruction's parameter
// is encoded (or not) in the machine
// code.
type ParameterEncoding uint8

const (
	_                        ParameterEncoding = iota
	EncodingNone                               // The parameter is required in the assembly but is not encoded.
	EncodingVEXvvvv                            // The parameter is encoded in the VEX.vvvv field of the machine code.
	EncodingRegisterModifier                   // The parameter is encoded in the opcode byte.
	EncodingStackIndex                         // The parameter is an x87 stack index, encoded in the opcode byte.
	EncodingCodeOffset                         // The parameter is encoded as a code offset after the opcode.
	EncodingModRMreg                           // The parameter is encoded in the ModR/M.reg field of the machine code.
	EncodingModRMrm                            // The parameter is encoded in the ModR/M.rm field of the machine code.
	EncodingSIB                                // The parameter is encoded in the SIB byte.
	EncodingDisplacement                       // The parameter is encoded in the displacement field of the machine code.
	EncodingImmediate                          // The parameter is encoded in the immediate field of the machine code.
	EncodingVEXis4                             // The parameter is encoded in the VEX /is4 immediate byte.
)

var ParameterEncodings = map[string]ParameterEncoding{
	"none":              EncodingNone,
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

func (e ParameterEncoding) String() string {
	switch e {
	case EncodingNone:
		return "none"
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

func (e ParameterEncoding) UID() string {
	switch e {
	case EncodingNone:
		return "EncodingNone"
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

// Define the parameters.
var (
	// Explicit unencoded register literals.
	ParamAL       = &Parameter{TypeRegister, EncodingNone, "AL", 8, "AL", []*Register{AL}}
	ParamCL       = &Parameter{TypeRegister, EncodingNone, "CL", 8, "CL", []*Register{CL}}
	ParamAX       = &Parameter{TypeRegister, EncodingNone, "AX", 16, "AX", []*Register{AX}}
	ParamDX       = &Parameter{TypeRegister, EncodingNone, "DX", 16, "DX", []*Register{DX}}
	ParamEAX      = &Parameter{TypeRegister, EncodingNone, "EAX", 32, "EAX", []*Register{EAX}}
	ParamECX      = &Parameter{TypeRegister, EncodingNone, "ECX", 32, "ECX", []*Register{ECX}}
	ParamRAX      = &Parameter{TypeRegister, EncodingNone, "RAX", 64, "RAX", []*Register{RAX}}
	ParamXMM0     = &Parameter{TypeRegister, EncodingNone, "XMM0", 128, "XMM0", []*Register{XMM0}}
	ParamES       = &Parameter{TypeRegister, EncodingNone, "ES", 16, "ES", []*Register{ES}}
	ParamCS       = &Parameter{TypeRegister, EncodingNone, "CS", 16, "CS", []*Register{CS}}
	ParamSS       = &Parameter{TypeRegister, EncodingNone, "SS", 16, "SS", []*Register{SS}}
	ParamDS       = &Parameter{TypeRegister, EncodingNone, "DS", 16, "DS", []*Register{DS}}
	ParamFS       = &Parameter{TypeRegister, EncodingNone, "FS", 16, "FS", []*Register{FS}}
	ParamGS       = &Parameter{TypeRegister, EncodingNone, "GS", 16, "GS", []*Register{GS}}
	ParamCR8      = &Parameter{TypeRegister, EncodingNone, "CR8", 64, "CR8", []*Register{CR8}}
	ParamST       = &Parameter{TypeStackIndex, EncodingNone, "ST", 80, "ST", []*Register{ST0}}
	ParamStrDst8  = &Parameter{TypeStringDst, EncodingNone, "StrDst8", 8, "[es:edi:8]", []*Register{DI, EDI, RDI}}
	ParamStrDst16 = &Parameter{TypeStringDst, EncodingNone, "StrDst16", 16, "[es:edi:16]", []*Register{DI, EDI, RDI}}
	ParamStrDst32 = &Parameter{TypeStringDst, EncodingNone, "StrDst32", 32, "[es:edi:32]", []*Register{DI, EDI, RDI}}
	ParamStrDst64 = &Parameter{TypeStringDst, EncodingNone, "StrDst64", 64, "[rdi:64]", []*Register{RDI}}
	ParamStrSrc8  = &Parameter{TypeStringSrc, EncodingNone, "StrSrc8", 8, "[ds:esi:8]", []*Register{SI, ESI, RSI}}
	ParamStrSrc16 = &Parameter{TypeStringSrc, EncodingNone, "StrSrc16", 16, "[ds:esi:16]", []*Register{SI, ESI, RSI}}
	ParamStrSrc32 = &Parameter{TypeStringSrc, EncodingNone, "StrSrc32", 32, "[ds:esi:32]", []*Register{SI, ESI, RSI}}
	ParamStrSrc64 = &Parameter{TypeStringSrc, EncodingNone, "StrSrc64", 64, "[rsi:64]", []*Register{RSI}}
	Param0        = &Parameter{TypeSignedImmediate, EncodingNone, "0", 0, "0", nil}
	Param1        = &Parameter{TypeSignedImmediate, EncodingNone, "1", 0, "1", nil}
	Param2        = &Parameter{TypeSignedImmediate, EncodingNone, "2", 0, "2", nil}
	Param3        = &Parameter{TypeSignedImmediate, EncodingNone, "3", 0, "3", nil}
	Param4        = &Parameter{TypeSignedImmediate, EncodingNone, "4", 0, "4", nil}
	Param5        = &Parameter{TypeSignedImmediate, EncodingNone, "5", 0, "5", nil}
	Param6        = &Parameter{TypeSignedImmediate, EncodingNone, "6", 0, "6", nil}
	Param7        = &Parameter{TypeSignedImmediate, EncodingNone, "7", 0, "7", nil}
	Param8        = &Parameter{TypeSignedImmediate, EncodingNone, "8", 0, "8", nil}
	Param9        = &Parameter{TypeSignedImmediate, EncodingNone, "9", 0, "9", nil}

	// VEX.vvvv register selection.
	ParamR8V  = &Parameter{TypeRegister, EncodingVEXvvvv, "R8V", 8, "r8V", Registers8bitGeneralPurpose}
	ParamR16V = &Parameter{TypeRegister, EncodingVEXvvvv, "R16V", 16, "r16V", Registers16bitGeneralPurpose}
	ParamR32V = &Parameter{TypeRegister, EncodingVEXvvvv, "R32V", 32, "r32V", Registers32bitGeneralPurpose}
	ParamR64V = &Parameter{TypeRegister, EncodingVEXvvvv, "R64V", 64, "r64V", Registers64bitGeneralPurpose}
	ParamKV   = &Parameter{TypeRegister, EncodingVEXvvvv, "KV", 16, "kV", RegistersOpmask}
	ParamXMMV = &Parameter{TypeRegister, EncodingVEXvvvv, "XMMV", 128, "xmmV", Registers128bitXMM}
	ParamYMMV = &Parameter{TypeRegister, EncodingVEXvvvv, "YMMV", 256, "ymmV", Registers256bitYMM}
	ParamZMMV = &Parameter{TypeRegister, EncodingVEXvvvv, "ZMMV", 512, "zmmV", Registers512bitZMM}

	// Registers encoded in the opcode.
	ParamR8op  = &Parameter{TypeRegister, EncodingRegisterModifier, "R8op", 8, "r8op", Registers8bitGeneralPurpose}
	ParamR16op = &Parameter{TypeRegister, EncodingRegisterModifier, "R16op", 16, "r16op", Registers16bitGeneralPurpose}
	ParamR32op = &Parameter{TypeRegister, EncodingRegisterModifier, "R32op", 32, "r32op", Registers32bitGeneralPurpose}
	ParamR64op = &Parameter{TypeRegister, EncodingRegisterModifier, "R64op", 64, "r64op", Registers64bitGeneralPurpose}

	// FPU stack index literals.
	ParamSTi = &Parameter{TypeStackIndex, EncodingStackIndex, "STi", 80, "ST(i)", RegistersStackIndices}

	// Relative or absolute address.
	ParamRel8     = &Parameter{TypeRelativeAddress, EncodingCodeOffset, "Rel8", 8, "rel8", nil}
	ParamRel16    = &Parameter{TypeRelativeAddress, EncodingCodeOffset, "Rel16", 16, "rel16", nil}
	ParamRel32    = &Parameter{TypeRelativeAddress, EncodingCodeOffset, "Rel32", 32, "rel32", nil}
	ParamPtr16v16 = &Parameter{TypeFarPointer, EncodingCodeOffset, "Ptr16v16", 32, "ptr16:16", nil}
	ParamPtr16v32 = &Parameter{TypeFarPointer, EncodingCodeOffset, "Ptr16v32", 48, "ptr16:32", nil}

	// ModR/M.reg register selection.
	ParamR8       = &Parameter{TypeRegister, EncodingModRMreg, "R8", 8, "r8", Registers8bitGeneralPurpose}
	ParamR16      = &Parameter{TypeRegister, EncodingModRMreg, "R16", 16, "r16", Registers16bitGeneralPurpose}
	ParamR32      = &Parameter{TypeRegister, EncodingModRMreg, "R32", 32, "r32", Registers32bitGeneralPurpose}
	ParamR64      = &Parameter{TypeRegister, EncodingModRMreg, "R64", 64, "r64", Registers64bitGeneralPurpose}
	ParamSreg     = &Parameter{TypeRegister, EncodingModRMreg, "Sreg", 16, "Sreg", Registers16bitSegment}
	ParamCR0toCR7 = &Parameter{TypeRegister, EncodingModRMreg, "CR0toCR7", 64, "CR0-CR7", []*Register{CR0, CR1, CR2, CR3, CR4, CR5, CR6, CR7}}
	ParamDR0toDR7 = &Parameter{TypeRegister, EncodingModRMreg, "DR0toDR7", 64, "DR0-DR7", []*Register{DR0, DR1, DR2, DR3, DR4, DR5, DR6, DR7}}
	ParamK1       = &Parameter{TypeRegister, EncodingModRMreg, "K1", 16, "k1", RegistersOpmask}
	ParamMM1      = &Parameter{TypeRegister, EncodingModRMreg, "MM1", 64, "mm1", Registers64bitMMX}
	ParamXMM1     = &Parameter{TypeRegister, EncodingModRMreg, "XMM1", 128, "xmm1", Registers128bitXMM}
	ParamYMM1     = &Parameter{TypeRegister, EncodingModRMreg, "YMM1", 256, "ymm1", Registers256bitYMM}
	ParamZMM1     = &Parameter{TypeRegister, EncodingModRMreg, "ZMM1", 512, "zmm1", Registers512bitZMM}

	// ModR/M register selection or memory address.
	ParamRmr8        = &Parameter{TypeRegister, EncodingModRMrm, "Rmr8", 8, "rmr8", Registers8bitGeneralPurpose}
	ParamRmr16       = &Parameter{TypeRegister, EncodingModRMrm, "Rmr16", 16, "rmr16", Registers16bitGeneralPurpose}
	ParamRmr32       = &Parameter{TypeRegister, EncodingModRMrm, "Rmr32", 32, "rmr32", Registers32bitGeneralPurpose}
	ParamRmr64       = &Parameter{TypeRegister, EncodingModRMrm, "Rmr64", 64, "rmr64", Registers64bitGeneralPurpose}
	ParamK2          = &Parameter{TypeRegister, EncodingModRMrm, "K2", 16, "k2", RegistersOpmask}
	ParamMM2         = &Parameter{TypeRegister, EncodingModRMrm, "MM2", 64, "mm2", Registers64bitMMX}
	ParamXMM2        = &Parameter{TypeRegister, EncodingModRMrm, "XMM2", 128, "xmm2", Registers128bitXMM}
	ParamYMM2        = &Parameter{TypeRegister, EncodingModRMrm, "YMM2", 256, "ymm2", Registers256bitYMM}
	ParamZMM2        = &Parameter{TypeRegister, EncodingModRMrm, "ZMM2", 512, "zmm2", Registers512bitZMM}
	ParamM           = &Parameter{TypeMemory, EncodingModRMrm, "M", 0, "m", nil}
	ParamM8          = &Parameter{TypeMemory, EncodingModRMrm, "M8", 8, "m8", nil}
	ParamM16         = &Parameter{TypeMemory, EncodingModRMrm, "M16", 16, "m16", nil}
	ParamM16bcst     = &Parameter{TypeMemory, EncodingModRMrm, "M16bcst", 16, "m16bcst", nil}
	ParamM32         = &Parameter{TypeMemory, EncodingModRMrm, "M32", 32, "m32", nil}
	ParamM32bcst     = &Parameter{TypeMemory, EncodingModRMrm, "M32bcst", 32, "m32bcst", nil}
	ParamM64         = &Parameter{TypeMemory, EncodingModRMrm, "M64", 64, "m64", nil}
	ParamM64bcst     = &Parameter{TypeMemory, EncodingModRMrm, "M64bcst", 64, "m64bcst", nil}
	ParamM80bcd      = &Parameter{TypeMemory, EncodingModRMrm, "M80bcd", 80, "m80bcd", nil}
	ParamM80dec      = &Parameter{TypeMemory, EncodingModRMrm, "M80dec", 80, "m80dec", nil}
	ParamM128        = &Parameter{TypeMemory, EncodingModRMrm, "M128", 128, "m128", nil}
	ParamM256        = &Parameter{TypeMemory, EncodingModRMrm, "M256", 256, "m256", nil}
	ParamM384        = &Parameter{TypeMemory, EncodingModRMrm, "M384", 384, "m384", nil}
	ParamM512        = &Parameter{TypeMemory, EncodingModRMrm, "M512", 512, "m512", nil}
	ParamM512byte    = &Parameter{TypeMemory, EncodingModRMrm, "M512byte", 4098, "m512byte", nil}
	ParamM32fp       = &Parameter{TypeMemory, EncodingModRMrm, "M32fp", 32, "m32fp", nil}
	ParamM64fp       = &Parameter{TypeMemory, EncodingModRMrm, "M64fp", 64, "m64fp", nil}
	ParamM80fp       = &Parameter{TypeMemory, EncodingModRMrm, "M80fp", 80, "m80fp", nil}
	ParamM16int      = &Parameter{TypeMemory, EncodingModRMrm, "M16int", 16, "m16int", nil}
	ParamM32int      = &Parameter{TypeMemory, EncodingModRMrm, "M32int", 32, "m32int", nil}
	ParamM64int      = &Parameter{TypeMemory, EncodingModRMrm, "M64int", 64, "m64int", nil}
	ParamM16v16      = &Parameter{TypeMemory, EncodingModRMrm, "M16v16", 32, "m16:16", nil}
	ParamM16v32      = &Parameter{TypeMemory, EncodingModRMrm, "M16v32", 48, "m16:32", nil}
	ParamM16v64      = &Parameter{TypeMemory, EncodingModRMrm, "M16v64", 80, "m16:64", nil}
	ParamM16x16      = &Parameter{TypeMemory, EncodingModRMrm, "M16x16", 32, "m16&16", nil}
	ParamM16x32      = &Parameter{TypeMemory, EncodingModRMrm, "M16x32", 48, "m16&32", nil}
	ParamM16x64      = &Parameter{TypeMemory, EncodingModRMrm, "M16x64", 80, "m16&64", nil}
	ParamM32x32      = &Parameter{TypeMemory, EncodingModRMrm, "M32x32", 64, "m32&32", nil}
	ParamM2byte      = &Parameter{TypeMemory, EncodingModRMrm, "M2byte", 16, "m2byte", nil}
	ParamM14l28byte  = &Parameter{TypeMemory, EncodingModRMrm, "M14l28byte", 224, "m14/28byte", nil}
	ParamM94l108byte = &Parameter{TypeMemory, EncodingModRMrm, "M94l108byte", 864, "m94/108byte", nil}

	ParamVm32x = &Parameter{TypeMemory, EncodingSIB, "Vm32x", 32, "vm32x", nil}
	ParamVm32y = &Parameter{TypeMemory, EncodingSIB, "Vm32y", 32, "vm32y", nil}
	ParamVm32z = &Parameter{TypeMemory, EncodingSIB, "Vm32z", 32, "vm32z", nil}
	ParamVm64x = &Parameter{TypeMemory, EncodingSIB, "Vm64x", 64, "vm64x", nil}
	ParamVm64y = &Parameter{TypeMemory, EncodingSIB, "Vm64y", 64, "vm64y", nil}
	ParamVm64z = &Parameter{TypeMemory, EncodingSIB, "Vm64z", 64, "vm64z", nil}

	// Memory values only in the displacement field.
	ParamMoffs8  = &Parameter{TypeMemoryOffset, EncodingDisplacement, "Moffs8", 8, "moffs8", nil}
	ParamMoffs16 = &Parameter{TypeMemoryOffset, EncodingDisplacement, "Moffs16", 16, "moffs16", nil}
	ParamMoffs32 = &Parameter{TypeMemoryOffset, EncodingDisplacement, "Moffs32", 32, "moffs32", nil}
	ParamMoffs64 = &Parameter{TypeMemoryOffset, EncodingDisplacement, "Moffs64", 64, "moffs64", nil}

	// Immediate values.
	ParamImm8   = &Parameter{TypeSignedImmediate, EncodingImmediate, "Imm8", 8, "imm8", nil}
	ParamImm16  = &Parameter{TypeSignedImmediate, EncodingImmediate, "Imm16", 16, "imm16", nil}
	ParamImm32  = &Parameter{TypeSignedImmediate, EncodingImmediate, "Imm32", 32, "imm32", nil}
	ParamImm64  = &Parameter{TypeSignedImmediate, EncodingImmediate, "Imm64", 64, "imm64", nil}
	ParamImm5u  = &Parameter{TypeUnsignedImmediate, EncodingImmediate, "Imm5u", 5, "imm5u", nil}
	ParamImm8u  = &Parameter{TypeUnsignedImmediate, EncodingImmediate, "Imm8u", 8, "imm8u", nil}
	ParamImm16u = &Parameter{TypeUnsignedImmediate, EncodingImmediate, "Imm16u", 16, "imm16u", nil}
	ParamImm32u = &Parameter{TypeUnsignedImmediate, EncodingImmediate, "Imm32u", 32, "imm32u", nil}
	ParamImm64u = &Parameter{TypeUnsignedImmediate, EncodingImmediate, "Imm64u", 64, "imm64u", nil}

	// VEX /is4 register selection
	ParamXMMIH = &Parameter{TypeRegister, EncodingVEXis4, "XMMIH", 128, "xmmIH", Registers128bitXMM}
	ParamYMMIH = &Parameter{TypeRegister, EncodingVEXis4, "YMMIH", 256, "ymmIH", Registers256bitYMM}
	ParamZMMIH = &Parameter{TypeRegister, EncodingVEXis4, "ZMMIH", 512, "zmmIH", Registers512bitZMM}
)

var Parameters = map[string]*Parameter{
	// Explicit unencoded register literals.
	"AL":          ParamAL,
	"CL":          ParamCL,
	"AX":          ParamAX,
	"DX":          ParamDX,
	"EAX":         ParamEAX,
	"ECX":         ParamECX,
	"RAX":         ParamRAX,
	"XMM0":        ParamXMM0,
	"ES":          ParamES,
	"CS":          ParamCS,
	"SS":          ParamSS,
	"DS":          ParamDS,
	"FS":          ParamFS,
	"GS":          ParamGS,
	"CR8":         ParamCR8,
	"ST":          ParamST,
	"[es:edi:8]":  ParamStrDst8,
	"[es:edi:16]": ParamStrDst16,
	"[es:edi:32]": ParamStrDst32,
	"[rdi:64]":    ParamStrDst64,
	"[ds:esi:8]":  ParamStrSrc8,
	"[ds:esi:16]": ParamStrSrc16,
	"[ds:esi:32]": ParamStrSrc32,
	"[rsi:64]":    ParamStrSrc64,
	"0":           Param0,
	"1":           Param1,
	"2":           Param2,
	"3":           Param3,
	"4":           Param4,
	"5":           Param5,
	"6":           Param6,
	"7":           Param7,
	"8":           Param8,
	"9":           Param9,

	// VEX.vvvv register selection.
	"r8V":  ParamR8V,
	"r16V": ParamR16V,
	"r32V": ParamR32V,
	"r64V": ParamR64V,
	"kV":   ParamKV,
	"xmmV": ParamXMMV,
	"ymmV": ParamYMMV,
	"zmmV": ParamZMMV,

	// FPU stack index literals.
	"ST(i)": ParamSTi,

	// Relative or absolute address.
	"rel8":     ParamRel8,
	"rel16":    ParamRel16,
	"rel32":    ParamRel32,
	"ptr16:16": ParamPtr16v16,
	"ptr16:32": ParamPtr16v32,

	// ModR/M.reg register selection.
	"r8":      ParamR8,
	"r16":     ParamR16,
	"r32":     ParamR32,
	"r64":     ParamR64,
	"r8op":    ParamR8op,
	"r16op":   ParamR16op,
	"r32op":   ParamR32op,
	"r64op":   ParamR64op,
	"Sreg":    ParamSreg,
	"CR0-CR7": ParamCR0toCR7,
	"DR0-DR7": ParamDR0toDR7,
	"k1":      ParamK1,
	"mm1":     ParamMM1,
	"xmm1":    ParamXMM1,
	"ymm1":    ParamYMM1,
	"zmm1":    ParamZMM1,

	// ModR/M register selection or memory address.
	"rmr8":        ParamRmr8,
	"rmr16":       ParamRmr16,
	"rmr32":       ParamRmr32,
	"rmr64":       ParamRmr64,
	"k2":          ParamK2,
	"mm2":         ParamMM2,
	"xmm2":        ParamXMM2,
	"ymm2":        ParamYMM2,
	"zmm2":        ParamZMM2,
	"m":           ParamM,
	"m8":          ParamM8,
	"m16":         ParamM16,
	"m32":         ParamM32,
	"m32bcst":     ParamM32bcst,
	"m64":         ParamM64,
	"m64bcst":     ParamM64bcst,
	"m80bcd":      ParamM80bcd,
	"m80dec":      ParamM80dec,
	"m128":        ParamM128,
	"m256":        ParamM256,
	"m384":        ParamM384,
	"m512":        ParamM512,
	"m512byte":    ParamM512byte,
	"m32fp":       ParamM32fp,
	"m64fp":       ParamM64fp,
	"m80fp":       ParamM80fp,
	"m16int":      ParamM16int,
	"m32int":      ParamM32int,
	"m64int":      ParamM64int,
	"m16:16":      ParamM16v16,
	"m16:32":      ParamM16v32,
	"m16:64":      ParamM16v64,
	"m16&16":      ParamM16x16,
	"m16&32":      ParamM16x32,
	"m16&64":      ParamM16x64,
	"m32&32":      ParamM32x32,
	"m2byte":      ParamM2byte,
	"m14/28byte":  ParamM14l28byte,
	"m94/108byte": ParamM94l108byte,

	// VSIB vector sets.
	"vm32x": ParamVm32x,
	"vm32y": ParamVm32y,
	"vm32z": ParamVm32z,
	"vm64x": ParamVm64x,
	"vm64y": ParamVm64y,
	"vm64z": ParamVm64z,

	// Memory values only in the displacement field.
	"moffs8":  ParamMoffs8,
	"moffs16": ParamMoffs16,
	"moffs32": ParamMoffs32,
	"moffs64": ParamMoffs64,

	// Immediate values.
	"imm8":   ParamImm8,
	"imm16":  ParamImm16,
	"imm32":  ParamImm32,
	"imm64":  ParamImm64,
	"imm5u":  ParamImm5u,
	"imm8u":  ParamImm8u,
	"imm16u": ParamImm16u,
	"imm32u": ParamImm32u,
	"imm64u": ParamImm64u,

	// VEX /is4 register selection
	"xmmIH": ParamXMMIH,
	"ymmIH": ParamYMMIH,
}
