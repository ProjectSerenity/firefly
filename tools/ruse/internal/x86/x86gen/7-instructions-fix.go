// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"strconv"
	"strings"
	"unicode"

	"firefly-os.dev/tools/ruse/internal/x86"
)

func dropNumbers(s string) string {
	dropNumber := func(r rune) rune {
		if unicode.IsDigit(r) {
			return -1
		}

		return r
	}

	return strings.Map(dropNumber, s)
}

func onlyNumbers(s string) string {
	keepNumber := func(r rune) rune {
		if !unicode.IsDigit(r) {
			return -1
		}

		return r
	}

	return strings.Map(keepNumber, s)
}
func fixInstruction(stats *Stats, inst *x86.Instruction) error {
	// General corrections.
	switch inst.Syntax {
	case "HRESET imm8, <EAX>":
		// The manual gives an operand encoding
		// of ModRM:r/m, N/A, which should be
		// Immediate, Implicit.
		if inst.Operands[0].Encoding != x86.EncodingImmediate {
			stats.ListingError()
			inst.Operands[0].Encoding = x86.EncodingImmediate
		}

		if inst.Operands[1].Encoding != x86.EncodingImplicit {
			stats.ListingError()
			inst.Operands[1].Encoding = x86.EncodingImplicit
		}
	case "IN AL, imm8", "IN AX, imm8", "IN EAX, imm8":
		// The manual gives an operand encoding
		// of Immediate, N/A, which should be
		// Implicit, Immediate.
		if inst.Operands[0].Encoding == x86.EncodingImmediate {
			stats.ListingError()
			inst.Operands[0].Encoding = x86.EncodingNone
		}

		if inst.Operands[1].Encoding != x86.EncodingImmediate {
			stats.ListingError()
			inst.Operands[1].Encoding = x86.EncodingImmediate
		}
	case "NOP", "NOP r16/m16", "NOP r32/m32":
		// The manual has been updated to include
		// NP in its encoding, but still uses the
		// 66 prefix in examples. We assume the NP
		// is incorrect and remove it.
		stats.InstructionError()
		inst.Encoding.NoVEXPrefixes = false
		inst.Encoding.Syntax = strings.TrimPrefix(inst.Encoding.Syntax, "NP ")
	case "SENDUIPI r64":
		// The manual gives an operand encoding of
		// ModRM:reg, which should be ModRM:r/m,
		// as it also has /6, which goes in the
		// ModRM:reg slot.
		if inst.Operands[0].Encoding != x86.EncodingModRMrm {
			stats.ListingError()
			inst.Operands[0].Encoding = x86.EncodingModRMrm
		}
	}

	// Determine the data size if necessary.
	switch inst.Mnemonic {
	case "AAA",
		"CMPSB", "INSB", "LODSB", "MOVSB", "OUTSB", "SCASB", "STOSB",
		"PEXTRB", "PINSRB",
		"VPBROADCASTB",
		"VPCOMPRESSB",
		"VPEXPANDB",
		"VPEXTRB",
		"VPINSRB":
		inst.DataSize = 8
	case "AAD", "AAM",
		"CBW", "CWD",
		"CMPSW", "INSW", "LODSW", "MOVSW", "OUTSW", "SCASW", "STOSW",
		"IRET",
		"PEXTRW", "PINSRW",
		"POPA", "POPF", "PUSHA", "PUSHF",
		"VADDSH",
		"VCMPSH",
		"VCOMISH",
		"VCVTSD2SH", "VCVTSI2SH", "VCVTSS2SH", "VCVTUSI2SH",
		"VDIVSH",
		"VFCMADDCSH",
		"VFCMULCSH",
		"VFMADD132SH", "VFMADD213SH", "VFMADD231SH",
		"VFMADDCSH",
		"VFMSUB132SH", "VFMSUB213SH", "VFMSUB231SH",
		"VFMULCSH",
		"VFNMADD132SH", "VFNMADD213SH", "VFNMADD231SH",
		"VFNMSUB132SH", "VFNMSUB213SH", "VFNMSUB231SH",
		"VFPCLASSSH",
		"VGETEXPSH",
		"VGETMANTSH",
		"VMAXSH",
		"VMINSH",
		"VMOVSH",
		"VMOVW",
		"VMULSH",
		"VPBROADCASTW",
		"VPCOMPRESSW",
		"VPEXPANDW",
		"VPEXTRW",
		"VPINSRW",
		"VRCPSH",
		"VREDUCESH",
		"VRNDSCALESH",
		"VRSQRT14SH", "VRSQRT28SH", "VRSQRTSH", "VSQRTSH",
		"VSCALEFSH",
		"VSUBSH",
		"VUCOMISH":
		inst.DataSize = 16
	case "CWDE",
		"CMPSD", "INSD", "LODSD", "MOVSD", "OUTSD", "SCASD", "STOSD",
		"IRETD",
		"MOVD",
		"PEXTRD", "PINSRD",
		"POPAD", "POPFD", "PUSHAD", "PUSHFD",
		"VPEXTRD",
		"VADDSS",
		"VBROADCASTSS",
		"VCMPSS",
		"VCOMISS",
		"VCOMPRESSPS",
		"VCVTSH2SS",
		"VCVTSS2SD",
		"VDIVSS",
		"VEXPANDPS",
		"VEXTRACTPS",
		"VFIXUPIMMSS",
		"VFMADD132SS", "VFMADD213SS", "VFMADD231SS",
		"VFMSUB132SS", "VFMSUB213SS", "VFMSUB231SS",
		"VFNMADD132SS", "VFNMADD213SS", "VFNMADD231SS",
		"VFNMSUB132SS", "VFNMSUB213SS", "VFNMSUB231SS",
		"VFPCLASSSS",
		"VGATHERDPS",
		"VGATHERPF0DPD", "VGATHERPF0DPS", "VGATHERPF0QPD", "VGATHERPF0QPS",
		"VGATHERPF1DPD", "VGATHERPF1DPS", "VGATHERPF1QPD", "VGATHERPF1QPS",
		"VGATHERQPS",
		"VGETEXPSS",
		"VGETMANTSS",
		"VINSERTPS",
		"VMAXSS",
		"VMINSS",
		"VMOVD",
		"VMOVSS",
		"VMULSS",
		"VPBROADCASTD",
		"VPCOMPRESSD",
		"VPEXPANDD",
		"VPGATHERDD", "VPGATHERQD",
		"VPINSRD",
		"VPSCATTERDD", "VPSCATTERDQ",
		"VRANGESS",
		"VRCP14SS", "VRCP28SS",
		"VREDUCESS",
		"VRNDSCALESS",
		"VRSQRT14SS", "VRSQRT28SS",
		"VSCALEFSS",
		"VSCATTERDPD", "VSCATTERDPS",
		"VSQRTSS",
		"VSUBSS",
		"VUCOMISS",
		"WRSSD", "WRUSSD":
		inst.DataSize = 32
	case "CDQ", "CDQE",
		"CMPSQ", "INSQ", "LODSQ", "MOVSQ", "OUTSQ", "SCASQ", "STOSQ",
		"IRETQ",
		"MOVQ",
		"PEXTRQ", "PINSRQ",
		"POPFQ", "PUSHFQ",
		"VPEXTRQ",
		"VADDSD",
		"VBROADCASTSD",
		"VCMPSD",
		"VCOMISD",
		"VCOMPRESSPD",
		"VCVTSD2SS", "VCVTSH2SD",
		"VDIVSD",
		"VEXPANDPD",
		"VFIXUPIMMSD",
		"VFMADD132SD", "VFMADD213SD", "VFMADD231SD",
		"VFMSUB132SD", "VFMSUB213SD", "VFMSUB231SD",
		"VFNMADD132SD", "VFNMADD213SD", "VFNMADD231SD",
		"VFNMSUB132SD", "VFNMSUB213SD", "VFNMSUB231SD",
		"VFPCLASSSD",
		"VGATHERDPD", "VGATHERQPD",
		"VGETEXPSD",
		"VGETMANTSD",
		"VPINSRQ",
		"VMAXSD",
		"VMINSD",
		"VMOVHPD",
		"VMOVLPD",
		"VMOVQ",
		"VMOVSD",
		"VMULSD",
		"VPBROADCASTQ",
		"VPCOMPRESSQ",
		"VPEXPANDQ",
		"VPGATHERDQ", "VPGATHERQQ",
		"VPSCATTERQD", "VPSCATTERQQ",
		"VRANGESD",
		"VRCP14SD", "VRCP28SD",
		"VREDUCESD",
		"VRNDSCALESD",
		"VRSQRT14SD", "VRSQRT28SD",
		"VSCALEFSD",
		"VSCATTERQPD", "VSCATTERQPS",
		"VSQRTSD",
		"VSUBSD",
		"VUCOMISD",
		"WRSSQ", "WRUSSQ":
		inst.DataSize = 64
	case "VINSERTF32X4", "VINSERTF64X2",
		"VINSERTI32X4", "VINSERTI64X2",
		"VPCLMULQDQ":
		inst.DataSize = 128
	case "VINSERTF32X8", "VINSERTF64X4",
		"VINSERTI32X8", "VINSERTI64X4":
		inst.DataSize = 256
	}
	switch inst.Syntax {
	case "JMP rel8":
		inst.DataSize = 8
	case "CALL m16:16", "CALL ptr16:16", "CALL r16/m16",
		"JMP m16:16", "JMP ptr16:16", "JMP r16/m16",
		"MOV r32, Sreg":
		inst.DataSize = 16
	case "CALL m16:32", "CALL ptr16:32", "CALL r32/m32",
		"JMP m16:32", "JMP ptr16:32", "JMP r32/m32",
		"MOV CR0-CR7, r32", "MOV DR0-DR7, r32",
		"MOVSS xmm1, xmm2", "MOVSS xmm1, m32", "MOVSS xmm2/m32, xmm1":
		inst.DataSize = 32
		switch inst.Mnemonic {
		case "MOVSS":
			inst.OperandSize = false
		}
	case "CALL m16:64", "CALL r64/m64",
		"JMP m16:64", "JMP r64/m64",
		"MOVSD xmm1, xmm2", "MOVSD xmm1, m64", "MOVSD xmm1/m64, xmm2":
		inst.DataSize = 64
		switch inst.Mnemonic {
		case "MOVSD":
			inst.OperandSize = false
		}
	case "VCVTPD2DQ xmm1, xmm2/m128", "VCVTPD2DQ xmm1 {k1}{z}, xmm2/m128/m64bcst",
		"VCVTPD2PS xmm1, xmm2/m128", "VCVTPD2PS xmm1 {k1}{z}, xmm2/m128/m64bcst",
		"VCVTTPD2DQ xmm1, xmm2/m128", "VCVTTPD2DQ xmm1 {k1}{z}, xmm2/m128/m64bcst":
		inst.DataSize = 128
	case "VCVTPD2DQ xmm1, ymm2/m256", "VCVTPD2DQ xmm1 {k1}{z}, ymm2/m256/m64bcst",
		"VCVTPD2PS xmm1, ymm2/m256", "VCVTPD2PS xmm1 {k1}{z}, ymm2/m256/m64bcst",
		"VCVTTPD2DQ xmm1, ymm2/m256", "VCVTTPD2DQ xmm1 {k1}{z}, ymm2/m256/m64bcst":
		inst.DataSize = 256
	case "VCVTPD2DQ ymm1 {k1}{z}, zmm2/m512/m64bcst{er}",
		"VCVTPD2PS ymm1 {k1}{z}, zmm2/m512/m64bcst{er}",
		"VCVTTPD2DQ ymm1 {k1}{z}, zmm2/m512/m64bcst{er}":
		inst.DataSize = 512
	}

	// Some instructions get the
	// operand size marking wrong.
	switch inst.Mnemonic {
	case "CBW", "CWD", "CWDE",
		"IRET", "IRETD",
		"POPA", "POPAD",
		"POPF", "POPFD",
		"PUSHA", "PUSHAD",
		"PUSHF", "PUSHFD",
		"SLDT",
		"STR":
		inst.OperandSize = true
	case "CALL", "JMP":
		if strings.Contains(inst.Operands[0].Name, "ptr16") {
			inst.OperandSize = true
		}
	case "POP", "PUSH", "XCHG":
		if strings.Contains(inst.Syntax, "r8") || strings.Contains(inst.Syntax, "r64") {
			inst.OperandSize = false
		}
	}

	// Ensure we don't pretend you
	// can use REX prefixes in 16-bit
	// or 32-bit mode.
	if inst.Encoding.REX {
		inst.Mode32 = false
		inst.Mode16 = false
	}

	// Ensure we don't leave tuple
	// types in non-EVEX instructions.
	if inst.TupleType != x86.TupleNone && !inst.Encoding.EVEX {
		stats.ListingError()
		inst.TupleType = x86.TupleNone
	}

	// Fix the individual operands.
	for i := range inst.Operands {
		err := fixOperand(inst.Page, inst.Operands[i], stats)
		if err != nil {
			return Errorf(inst.Page, "instruction %s  %s has operand %d (%q) with %v", inst.Encoding.Syntax, inst.Syntax, i+1, inst.Operands[i].Name, err)
		}

		// We can't use 64-bit
		// general purpose registers
		// in 16-bit or 32-bit mode.
		if inst.Operands[i] != nil && inst.Operands[i].Name == "r64" {
			inst.Mode32 = false
			inst.Mode16 = false
		}
	}

	// These instructions have a
	// data size equal to the size
	// of the given operand index.
	switch inst.Mnemonic {
	// Index 0 (first operand).
	case "CMP", "CMPXCHG",
		"CVTSD2SI",
		"CVTSS2SI",
		"CVTTSD2SI",
		"CVTTSS2SI",
		"DEC", "DIV", "IDIV", "INC", "IMUL", "MUL", "MULX",
		"FADD", "FCOM", "FCOMP", "FDIV", "FDIVR",
		"FIADD", "FICOM", "FICOMP", "FIDIV", "FIDIVR",
		"FILD", "FIMUL", "FIST", "FISTP", "FISTTP",
		"FISUB", "FISUBR", "FLD", "FMUL", "FSTP", "FSTTP",
		"FST", "FSUB", "FSUBR",
		"IN",
		"JA", "JAE", "JB", "JBE",
		"JCXZ", "JECXZ", "JRCXZ",
		"JC", "JE",
		"JG", "JGE",
		"JL", "JLE",
		"JNA", "JNAE",
		"JNB", "JNBE",
		"JNC", "JNE",
		"JNG", "JNGE",
		"JNL", "JNLE",
		"JNO", "JNP",
		"JNS", "JNZ",
		"JO",
		"JP", "JPE",
		"JPO",
		"JS", "JZ",
		"MOVDIRI",
		"MOVSX", "MOVZX", "MOV",
		"NEG", "NOT", "OR", "XOR",
		"PDEP", "PEXT",
		"POP", "PUSH",
		"PTWRITE",
		"RCL", "RCR", "ROL", "ROR", "RORX",
		"RDFSBASE", "RDGSBASE",
		"SAL", "SAR", "SARX", "SHL", "SHLX", "SHR", "SHRX",
		"SBB", "SUB",
		"TEST",
		"VAESDEC", "VAESDECLAST",
		"VAESENC", "VAESENCLAST",
		"VCVTSD2SI", "VCVTSD2USI",
		"VCVTSH2SI", "VCVTSH2USI",
		"VCVTSS2SI", "VCVTSS2USI",
		"VCVTTSD2SI", "VCVTTSD2USI",
		"VCVTTSH2SI", "VCVTTSH2USI",
		"VCVTTSS2SI", "VCVTTSS2USI",
		"VMOVDQA64", "VMOVDQU8", "VMOVDQU32", "VMOVDQU64",
		"VSCATTERPF0DPD", "VSCATTERPF0DPS", "VSCATTERPF0QPD", "VSCATTERPF0QPS",
		"VSCATTERPF1DPD", "VSCATTERPF1DPS", "VSCATTERPF1QPD", "VSCATTERPF1QPS",
		"WRFSBASE", "WRGSBASE",
		"XADD", "XCHG":
		if inst.DataSize == 0 && inst.Operands[0] != nil {
			inst.DataSize = inst.Operands[0].Bits
		}
	// Index 1 (second operand).
	case "CRC32",
		"CVTSI2SD",
		"CVTSI2SS",
		"OUT",
		"VPSADBW":
		if inst.DataSize == 0 && inst.Operands[1] != nil {
			inst.DataSize = inst.Operands[1].Bits
		}
	// Index 2 (third operand).
	case "VCVTSI2SD",
		"VCVTSI2SS",
		"VCVTUSI2SD",
		"VCVTUSI2SS":
		if inst.DataSize == 0 && inst.Operands[2] != nil {
			inst.DataSize = inst.Operands[2].Bits
		}
	}

	// See whether we can use the
	// operand data to determine
	// the instruction data size.
	if inst.DataSize == 0 && (inst.OperandSize || inst.Encoding.REX) && inst.Operands[0] != nil {
		inst.DataSize = inst.Operands[0].Bits
	}
	switch inst.Mnemonic {
	// Uniform arithmetic instructions.
	case "ADCX", "ADOX", "ANDN",
		"BEXTR", "BLSI", "BLSMSK", "BLSR", "BSF", "BSR", "BSWAP", "BZHI":
		if inst.DataSize == 0 {
			for i, op := range inst.Operands {
				if op == nil {
					break
				}

				if inst.DataSize != 0 && op.Bits != 0 && inst.DataSize != op.Bits {
					return Errorf(inst.Page, "found arithmetic instruction %q with data size %d and operand %d (%q) size %d", inst.Syntax, inst.DataSize, i, op.Name, op.Bits)
				}

				if inst.DataSize == 0 {
					inst.DataSize = op.Bits
				}
			}
		}
	// Arithmetic instructions that
	// operate on the largest operand
	// size.
	case "ADC", "ADD", "AND",
		"BT", "BTC", "BTR", "BTS":
		if inst.DataSize == 0 {
			for _, op := range inst.Operands {
				if op != nil && inst.DataSize < op.Bits {
					inst.DataSize = op.Bits
				}
			}
		}
	}

	// Remove unwanted data sizes if
	// necessary.
	switch inst.Mnemonic {
	case "INCSSPQ":
		inst.DataSize = 0
	}

	// Operand post-corrections.
	switch inst.Syntax {
	case "AAD imm8",
		"AAM imm8",
		"INT imm8",
		"OUT imm8, AL", "OUT imm8, AX", "OUT imm8, EAX":
		inst.Operands[0].Syntax = "imm8u"
		inst.Operands[0].UID = "Imm8u"
		inst.Operands[0].Type = x86.TypeUnsignedImmediate
	case "CMPPD xmm1, xmm2/m128, imm8",
		"CMPPS xmm1, xmm2/m128, imm8",
		"CMPSD xmm1, xmm2/m64, imm8",
		"CMPSS xmm1, xmm2/m32, imm8":
		inst.Operands[2].Syntax = "imm5u"
		inst.Operands[2].UID = "Imm5u"
		inst.Operands[2].Type = x86.TypeUnsignedImmediate
		inst.Operands[2].Bits = 5
	case "ENTER imm16, imm8":
		inst.Operands[0].Syntax = "imm16u"
		inst.Operands[0].UID = "Imm16u"
		inst.Operands[0].Type = x86.TypeUnsignedImmediate
		inst.Operands[1].Syntax = "imm5u"
		inst.Operands[1].UID = "Imm5u"
		inst.Operands[1].Type = x86.TypeUnsignedImmediate
		inst.Operands[1].Bits = 5
	case "IN AL, imm8", "IN AX, imm8", "IN EAX, imm8",
		"MOV r8/m8, imm8", "MOV r8, imm8":
		inst.Operands[1].Syntax = "imm8u"
		inst.Operands[1].UID = "Imm8u"
		inst.Operands[1].Type = x86.TypeUnsignedImmediate
	case "LDDQU xmm1, m":
		inst.Operands[1].Syntax = "m128"
		inst.Operands[1].UID = "M128"
		inst.Operands[1].Bits = 128
	case "RET imm16":
		inst.Operands[0].Syntax = "imm16u"
		inst.Operands[0].UID = "Imm16u"
		inst.Operands[0].Type = x86.TypeUnsignedImmediate
	}
	switch inst.Mnemonic {
	case "RCL", "RCR", "ROL", "ROR",
		"SAL", "SAR", "SHL", "SHR":
		// The rotate/shift is unsigned.
		if inst.Operands[1].Type == x86.TypeSignedImmediate {
			inst.Operands[1].Syntax += "u"
			inst.Operands[1].UID += "u"
			inst.Operands[1].Type = x86.TypeUnsignedImmediate
		}
	case "VCMPPD", "VCMPPS", "VCMPSD", "VCMPSS":
		if inst.MinArgs == 4 && (inst.Operands[3].Syntax == "imm8" || inst.Operands[3].Syntax == "imm5u") {
			inst.Operands[3].Syntax = "imm5u"
			inst.Operands[3].UID = "Imm5u"
			inst.Operands[3].Type = x86.TypeUnsignedImmediate
			inst.Operands[3].Bits = 5
		}
	}

	// Identify far calls/jumps/returns.
	switch inst.Mnemonic {
	case "CALL":
		// We need to differentiate
		// near and far calls.
		if strings.IndexByte(inst.Operands[0].Name, ':') > 0 {
			inst.Mnemonic += "-FAR"
			inst.Syntax = strings.Replace(inst.Syntax, "CALL", "CALL-FAR", 1)
		}
	case "JMP":
		// We need to differentiate
		// near and far jump.
		if strings.IndexByte(inst.Operands[0].Name, ':') > 0 {
			inst.Mnemonic += "-FAR"
			inst.Syntax = strings.Replace(inst.Syntax, "JMP", "JMP-FAR", 1)
		}
	case "RET":
		// We need to differentiate
		// near and far returns.
		if inst.Encoding.Syntax == "CB" || inst.Encoding.Syntax == "CA iw" {
			inst.Mnemonic += "-FAR"
			inst.Syntax = strings.Replace(inst.Syntax, "RET", "RET-FAR", 1)
		}
	}

	// Make the mnemonic lower case,
	// as we use it in the assembler.
	inst.Mnemonic = strings.ToLower(inst.Mnemonic)

	// Derive the instruction UID.
	uid := genUID(inst, false)

	// Check the UID is a valid Go
	// identifier.
	x, err := parser.ParseExpr(uid)
	if _, ok := x.(*ast.Ident); err != nil || !ok {
		return Errorf(inst.Page, "instruction %s has invalid UID %q: not a valid identifier", inst.Syntax, uid)
	}

	inst.UID = uid

	// Categorise the parameters.
	var (
		registerInVEXvvvv  string
		registerModifier   string
		stackIndex         string
		codeOffset         string
		registerInModRMreg string
		registerInModRMrm  string
		sib                string

		codeOffsets []int
		immediates  []int
		memory      string
	)

	for _, op := range inst.Operands {
		if op == nil {
			break
		}

		switch op.Type {
		case x86.TypeSignedImmediate, x86.TypeUnsignedImmediate:
			if op.Encoding == x86.EncodingImmediate {
				bits := op.Bits
				if bits == 5 {
					bits = 8 // 5-bit values are stored in 8 bits.
				}

				immediates = append(immediates, bits)
			}
		case x86.TypeMemory:
			if memory != "" {
				return Errorf(inst.Page, "instruction %s (%s): found memory values %s and %s", inst.Syntax, inst.Encoding.Syntax, memory, op.Syntax)
			}

			memory = op.Syntax
		}

		switch op.Encoding {
		case x86.EncodingVEXvvvv:
			if registerInVEXvvvv != "" {
				return Errorf(inst.Page, "instruction %s (%s): found registers %s and %s encoded in VEX.vvvv", inst.Syntax, inst.Encoding.Syntax, registerInVEXvvvv, op.Syntax)
			}

			registerInVEXvvvv = op.Syntax
		case x86.EncodingRegisterModifier:
			if registerModifier != "" {
				return Errorf(inst.Page, "instruction %s (%s): found register modifier %s and %s encoded in the opcode", inst.Syntax, inst.Encoding.Syntax, registerModifier, op.Syntax)
			}

			registerModifier = op.Syntax
		case x86.EncodingStackIndex:
			if stackIndex != "" {
				return Errorf(inst.Page, "instruction %s (%s): found x87 FPU stack indices %s and %s encoded in the opcode", inst.Syntax, inst.Encoding.Syntax, stackIndex, op.Syntax)
			}

			stackIndex = op.Syntax
		case x86.EncodingCodeOffset:
			if codeOffset != "" {
				return Errorf(inst.Page, "instruction %s (%s): found relative code offsets %s and %s encoded after the opcode", inst.Syntax, inst.Encoding.Syntax, codeOffset, op.Syntax)
			}

			codeOffset = op.Syntax
			codeOffsets = append(codeOffsets, op.Bits)
		case x86.EncodingModRMreg:
			if registerInModRMreg != "" {
				return Errorf(inst.Page, "instruction %s (%s): found registers %s and %s encoded in ModR/M.reg", inst.Syntax, inst.Encoding.Syntax, registerInModRMreg, op.Syntax)
			}

			registerInModRMreg = op.Syntax
		case x86.EncodingModRMrm:
			if registerInModRMrm != "" {
				return Errorf(inst.Page, "instruction %s (%s): found registers %s and %s encoded in ModR/M.rm", inst.Syntax, inst.Encoding.Syntax, registerInModRMrm, op.Syntax)
			}

			registerInModRMrm = op.Syntax
		case x86.EncodingSIB:
			if sib != "" {
				return Errorf(inst.Page, "instruction %s (%s): found memory %s and %s encoded in SIB", inst.Syntax, inst.Encoding.Syntax, sib, op.Syntax)
			}

			sib = op.Syntax
		}
	}

	// Get some extra details from the encoding
	// string, as there are some details we
	// don't extract generally.
	var (
		codeOffsetSizes []int
		immediateSizes  []int
	)

	clauses := strings.Fields(inst.Encoding.Syntax)
	for _, clause := range clauses {
		switch clause {
		case "cb":
			codeOffsetSizes = append(codeOffsetSizes, 8)
		case "cw":
			codeOffsetSizes = append(codeOffsetSizes, 16)
		case "cd":
			codeOffsetSizes = append(codeOffsetSizes, 32)
		case "cp":
			codeOffsetSizes = append(codeOffsetSizes, 48)
		case "co":
			codeOffsetSizes = append(codeOffsetSizes, 64)
		case "ct":
			codeOffsetSizes = append(codeOffsetSizes, 80)
		case "ib":
			immediateSizes = append(immediateSizes, 8)
		case "iw":
			immediateSizes = append(immediateSizes, 16)
		case "id":
			immediateSizes = append(immediateSizes, 32)
		case "io":
			immediateSizes = append(immediateSizes, 64)
		}
	}

	// Rationalise the parameters with
	// the broader encoding.

	if inst.Encoding.RegisterModifier != 0 && stackIndex == "" && registerModifier == "" {
		return Errorf(inst.Page, "instruction %s (%s): found register opcode modifier but no x87 FPU stack index or register parameter", inst.Syntax, inst.Encoding.Syntax)
	}

	if sib != "" && !inst.Encoding.SIB {
		return Errorf(inst.Page, "instruction %s (%s) found parameter %s but instruction encoding is missing /sib", inst.Syntax, inst.Encoding.Syntax, sib)
	}

	if inst.TupleType == x86.Tuple1Scalar && inst.DataSize == 0 {
		return Errorf(inst.Page, "instruction %s (%s) has tuple type %s but no data operation size", inst.Syntax, inst.Encoding.Syntax, inst.TupleType)
	}

	switch {
	case inst.Encoding.CodeOffset && codeOffset == "":
		return Errorf(inst.Page, "instruction %s (%s): a relative code offset is required but no relative offset parameters are included", inst.Syntax, inst.Encoding.Syntax)
	case !inst.Encoding.CodeOffset && codeOffset != "":
		return Errorf(inst.Page, "instruction %s (%s): no relative code offset is expected but relative offset parameter %q is included", inst.Syntax, inst.Encoding.Syntax, codeOffset)
	}

	if len(codeOffsets) != len(codeOffsetSizes) {
		return Errorf(inst.Page, "instruction %s (%s): found %d code offset parameters but expected %d from the encoding %s", inst.Syntax, inst.Encoding.Syntax, len(codeOffsets), len(codeOffsetSizes), inst.Encoding.Syntax)
	}

	for i := range codeOffsets {
		if codeOffsets[i] != codeOffsetSizes[i] {
			return Errorf(inst.Page, "instruction %s (%s): code offset parameter %d of %d has size %d bits but expected %d bits from the encoding %s", inst.Syntax, inst.Encoding.Syntax, i+1, len(codeOffsets), codeOffsets[i], codeOffsetSizes[i], inst.Encoding.Syntax)
		}
	}

	if inst.Encoding.ModRMreg != 0 && registerInModRMreg != "" {
		return Errorf(inst.Page, "instruction %s (%s): found register parameter %s and fixed value /%d encoded in ModR/M.reg", inst.Syntax, inst.Encoding.Syntax, registerInModRMreg, inst.Encoding.ModRMreg-1)
	}

	if registerInModRMreg != "" || inst.Encoding.ModRMreg != 0 || registerInModRMrm != "" || inst.Encoding.ModRMrm != 0 {
		inst.Encoding.ModRM = true
	}

	if registerInModRMreg != "" && registerInModRMrm != "" && !strings.Contains(inst.Encoding.Syntax, "/r") {
		stats.InstructionError()
		inst.Encoding.Syntax += " /r"
		inst.Encoding.ModRM = true
	}

	if len(immediates) != len(immediateSizes) {
		return Errorf(inst.Page, "instruction %s (%s): found %d immediate parameters but expected %d from the encoding %s", inst.Syntax, inst.Encoding.Syntax, len(immediates), len(immediateSizes), inst.Encoding.Syntax)
	}

	for i := range immediates {
		if immediates[i] != immediateSizes[i] {
			return Errorf(inst.Page, "instruction %s (%s): immediate parameter %d of %d has size %d bits but expected %d bits from the encoding %s", inst.Syntax, inst.Encoding.Syntax, i+1, len(immediates), immediates[i], immediateSizes[i], inst.Encoding.Syntax)
		}
	}

	return nil
}

func genUID(inst *x86.Instruction, includeImplied bool) string {
	var b strings.Builder
	b.WriteString(strings.Replace(strings.ToUpper(inst.Mnemonic), "-", "_", 1))

	switch inst.Mnemonic {
	case "bndmov", "vmload", "vmrun", "vmsave":
		// These instructions have ambiguous
		// versions because the same encoding
		// has different meanings in 32-bit
		// and 64-bit mode.
		if inst.Mode32 {
			b.WriteString("32")
		} else {
			b.WriteString("64")
		}
	case "pop":
		if inst.Operands[0].Name == "FS" || inst.Operands[0].Name == "GS" {
			// POP on FS and GS
			// has three versions with
			// different stack sizes.
			// We make them unambiguous by
			// appending the stack size.
			if inst.Mode64 && inst.Mode32 {
				b.WriteString("16")
			} else if inst.Mode32 {
				b.WriteString("32")
			} else {
				b.WriteString("64")
			}
		}
	}

	// Add the operands to the UID.
	end := inst.MinArgs
	if includeImplied {
		end = inst.MaxArgs
	}

	for i := 0; i < end; i++ {
		op := inst.Operands[i]
		b.WriteByte('_')
		b.WriteString(op.UID)
		if op.Name == "m32bcst" || op.Name == "m64bcst" {
			// These forms can be used for different
			// vector size instructions. We append
			// the vector size to disambiguate them.
			fmt.Fprintf(&b, "%d", inst.Encoding.VectorSize())
		}
	}

	if inst.Encoding.REX {
		b.WriteString("_REX")
	}

	if inst.Encoding.VEX {
		b.WriteString("_VEX")
	}

	if inst.Encoding.EVEX {
		b.WriteString("_EVEX")
		b.WriteString(strconv.Itoa(inst.Encoding.VectorSize()))
	}

	return b.String()
}

func fixOperand(page int, op *x86.Operand, stats *Stats) error {
	if op == nil {
		return nil
	}

	// Check the encoding and
	// name are consistent,
	// determine the operand
	// type, and its UID.
	switch op.Encoding {
	case x86.EncodingNone:
		switch op.Name {
		case "AL", "CL", "AX", "DX", "EAX", "ECX", "RAX", "CR8", "XMM0",
			"ES", "CS", "SS", "DS", "FS", "GS":
			op.UID = op.Name
			op.Type = x86.TypeRegister
		case "ST", "ST(0)":
			op.UID = "ST"
			op.Type = x86.TypeStackIndex
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			op.UID = op.Name
			op.Type = x86.TypeUnsignedImmediate
		case "[es:edi:8]", "[es:edi:16]", "[es:edi:32]", "[rdi:64]":
			op.UID = "StrDst" + onlyNumbers(op.Name)
			op.Type = x86.TypeStringDst
		case "[ds:esi:8]", "[ds:esi:16]", "[ds:esi:32]", "[rsi:64]":
			op.UID = "StrSrc" + onlyNumbers(op.Name)
			op.Type = x86.TypeStringSrc
		default:
			return Errorf(page, "invalid encoding %q", op.Encoding)
		}
	case x86.EncodingImplicit:
		switch op.Name {
		case "<EAX>", "<ECX>", "<EDX>", "<RAX>", "<XMM0>":
			op.UID = strings.TrimSuffix(strings.TrimPrefix(op.Name, "<"), ">")
			op.Type = x86.TypeRegister
		default:
			return Errorf(page, "invalid encoding %q", op.Encoding)
		}
	case x86.EncodingVEXvvvv:
		switch op.Name {
		case "r8", "r16", "r32", "r64":
			op.UID = strings.ToUpper(op.Name) + "V"
			op.Type = x86.TypeRegister
		case "k1", "k2", "k3",
			"tmm", "tmm1", "tmm2", "tmm3",
			"xmm", "xmm1", "xmm2", "xmm3", "xmm4",
			"ymm", "ymm1", "ymm2", "ymm3", "ymm4",
			"zmm", "zmm1", "zmm2", "zmm3", "zmm4":
			op.UID = strings.ToUpper(dropNumbers(op.Name)) + "V"
			op.Type = x86.TypeRegister
		default:
			return Errorf(page, "invalid encoding %q", op.Encoding)
		}
	case x86.EncodingRegisterModifier:
		switch op.Name {
		case "r8", "r16", "r32", "r64":
			op.UID = strings.ToUpper(op.Name) + "op"
			op.Type = x86.TypeRegister
		default:
			return Errorf(page, "invalid encoding %q", op.Encoding)
		}
	case x86.EncodingStackIndex:
		switch op.Name {
		case "ST(i)":
			op.UID = "STi"
			op.Type = x86.TypeStackIndex
		default:
			return Errorf(page, "invalid encoding %q", op.Encoding)
		}
	case x86.EncodingCodeOffset:
		switch op.Name {
		case "rel8", "rel16", "rel32":
			op.UID = "R" + op.Name[1:]
			op.Type = x86.TypeRelativeAddress
		case "ptr16:16", "ptr16:32":
			op.UID = "Ptr16v" + op.Name[6:]
			op.Type = x86.TypeFarPointer
		default:
			return Errorf(page, "invalid encoding %q", op.Encoding)
		}
	case x86.EncodingModRMreg:
		switch op.Name {
		case "AL", "CL", "AX", "DX", "EAX", "ECX", "RAX", "CR8", "XMM0",
			"ES", "CS", "SS", "DS", "FS", "GS":
			op.UID = op.Name
			op.Type = x86.TypeRegister
		case "r8", "r16", "r32", "r64":
			op.UID = strings.ToUpper(op.Name)
			op.Type = x86.TypeRegister
		case "Sreg":
			op.UID = op.Name
			op.Type = x86.TypeRegister
		case "CR0-CR7",
			"DR0-DR7":
			op.UID = strings.Replace(op.Name, "-", "to", 1)
			op.Type = x86.TypeRegister
		case "bnd", "bnd1", "bnd2", "bnd3",
			"k1", "k2", "k3",
			"mm", "mm1", "mm2",
			"tmm", "tmm1", "tmm2", "tmm3",
			"xmm", "xmm1", "xmm2", "xmm3", "xmm4",
			"ymm", "ymm1", "ymm2", "ymm3", "ymm4",
			"zmm", "zmm1", "zmm2", "zmm3", "zmm4":
			op.UID = strings.ToUpper(dropNumbers(op.Name)) + "1"
			op.Type = x86.TypeRegister
		default:
			return Errorf(page, "invalid encoding %q", op.Encoding)
		}
	case x86.EncodingModRMrm:
		switch op.Name {
		case "AL", "CL", "AX", "DX", "EAX", "ECX", "RAX", "CR8", "XMM0",
			"ES", "CS", "SS", "DS", "FS", "GS":
			op.UID = op.Name
			op.Type = x86.TypeRegister
		case "r8", "r16", "r32", "r64":
			op.UID = "Rm" + op.Name
			op.Type = x86.TypeRegister
		case "bnd", "bnd1", "bnd2", "bnd3",
			"k1", "k2", "k3",
			"mm", "mm1", "mm2",
			"tmm", "tmm1", "tmm2", "tmm3",
			"xmm", "xmm1", "xmm2", "xmm3", "xmm4",
			"ymm", "ymm1", "ymm2", "ymm3", "ymm4",
			"zmm", "zmm1", "zmm2", "zmm3", "zmm4":
			op.UID = strings.ToUpper(dropNumbers(op.Name)) + "2"
			op.Type = x86.TypeRegister
		case "m", "m8", "m16", "m32", "m64", "m80", "m128", "m256", "m384", "m512",
			"m16fp", "m32fp", "m64fp", "m80fp",
			"m16int", "m32int", "m64int",
			"m2byte", "m512byte",
			"m80bcd", "m80dec",
			"m16bcst", "m32bcst", "m64bcst":
			op.UID = "M" + op.Name[1:]
			op.Type = x86.TypeMemory
		case "m16&16", "m16&32", "m16&64":
			op.UID = "M16x" + op.Name[4:]
			op.Type = x86.TypeMemory
		case "m32&32":
			op.UID = "M32x" + op.Name[4:]
			op.Type = x86.TypeMemory
		case "m16:16", "m16:32", "m16:64":
			op.UID = "M16v" + op.Name[4:]
			op.Type = x86.TypeMemory
		case "m14/28byte", "m94/108byte":
			op.UID = "M" + strings.Replace(op.Name[1:], "/", "l", 1)
			op.Type = x86.TypeMemory
		case "mib":
			// This isn't wrong, but we choose to
			// be more precise, so we don't record
			// an error.
			op.UID = "Mib"
			op.Type = x86.TypeMemory
			op.Encoding = x86.EncodingSIB
		default:
			return Errorf(page, "invalid encoding %q", op.Encoding)
		}
	case x86.EncodingSIB:
		switch op.Name {
		case "mib":
			op.UID = "Mib"
			op.Type = x86.TypeMemory
		case "vm32x", "vm32y", "vm32z", "vm64x", "vm64y", "vm64z":
			op.UID = "V" + op.Name[1:]
			op.Type = x86.TypeMemory
		default:
			return Errorf(page, "invalid encoding %q", op.Encoding)
		}
	case x86.EncodingDisplacement:
		switch op.Name {
		case "moffs8", "moffs16", "moffs32", "moffs64":
			op.UID = "M" + op.Name[1:]
			op.Type = x86.TypeMemoryOffset
		default:
			return Errorf(page, "invalid encoding %q", op.Encoding)
		}
	case x86.EncodingImmediate:
		switch op.Name {
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			op.UID = op.Name
			op.Type = x86.TypeSignedImmediate
		case "imm8", "imm16", "imm32", "imm64":
			op.UID = "I" + op.Name[1:]
			op.Type = x86.TypeSignedImmediate
		default:
			return Errorf(page, "invalid encoding %q", op.Encoding)
		}
	case x86.EncodingVEXis4:
		switch op.Name {
		case "tmm", "tmm1", "tmm2", "tmm3",
			"xmm", "xmm1", "xmm2", "xmm3", "xmm4",
			"ymm", "ymm1", "ymm2", "ymm3", "ymm4",
			"zmm", "zmm1", "zmm2", "zmm3", "zmm4":
			op.UID = strings.ToUpper(dropNumbers(op.Name)) + "IH"
			op.Type = x86.TypeRegister
		default:
			return Errorf(page, "invalid encoding %q", op.Encoding)
		}
	default:
		return Errorf(page, "invalid encoding %q", op.Encoding)
	}

	// Determine the size in bits.
	switch op.Name {
	case "AL", "CL",
		"r8",
		"rel8",
		"m8", "moffs8",
		"[es:edi:8]", "[ds:esi:8]",
		"imm8":
		op.Bits = 8
	case "AX", "DX",
		"ES", "CS", "SS", "DS", "FS", "GS",
		"r16", "Sreg",
		"rel16",
		"m16", "m16fp", "m16int", "m2byte", "m16bcst", "moffs16",
		"[es:edi:16]", "[ds:esi:16]",
		"imm16":
		op.Bits = 16
	case "EAX", "ECX",
		"r32",
		"rel32",
		"ptr16:16",
		"m32", "m32fp", "m32int", "m16&16", "m16:16", "m32bcst", "moffs32",
		"[es:edi:32]", "[ds:esi:32]",
		"vm32x", "vm32y", "vm32z",
		"imm32",
		"<EAX>", "<ECX>", "<EDX>":
		op.Bits = 32
	case "ptr16:32",
		"m16&32", "m16:32":
		op.Bits = 48
	case "RAX", "CR8",
		"r64",
		"k1", "k2", "k3",
		"mm", "mm1", "mm2",
		"CR0-CR7", "DR0-DR7",
		"m64", "m64fp", "m64int", "m32&32", "m64bcst", "moffs64",
		"[rdi:64]", "[rsi:64]",
		"vm64x", "vm64y", "vm64z",
		"imm64":
		op.Bits = 64
	case "ST",
		"ST(0)", "ST(i)",
		"m80fp", "m80bcd", "m80dec", "m16&64", "m16:64":
		op.Bits = 80
	case "XMM0",
		"xmm", "xmm1", "xmm2", "xmm3", "xmm4",
		"m128",
		"<XMM0>":
		op.Bits = 128
	case "m14/28byte":
		op.Bits = 8 * 28
	case "ymm", "ymm1", "ymm2", "ymm3", "ymm4",
		"m256":
		op.Bits = 256
	case "m384":
		op.Bits = 384
	case "zmm", "zmm1", "zmm2", "zmm3", "zmm4",
		"m512":
		op.Bits = 512
	case "m94/108byte":
		op.Bits = 8 * 108
	case "m512byte":
		op.Bits = 8 * 512
	}

	if op.Type == 0 {
		return Errorf(page, "invalid type %q", op.Type)
	}

	if op.UID == "" {
		return Errorf(page, "invalid UID %q", op.UID)
	}

	op.Registers = registersByOperandUID[op.UID]
	if op.Registers == nil && op.Type == x86.TypeRegister {
		return Errorf(page, "failed to find registers for %q", op.UID)
	}

	return nil
}
