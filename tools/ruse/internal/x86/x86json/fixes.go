// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"regexp"
	"strings"

	"firefly-os.dev/tools/ruse/internal/x86/x86csv"
)

var (
	cmovRegex = regexp.MustCompile(`(CMOV[A-Z]+) r([0-9]+)`)
	jxxRegex  = regexp.MustCompile(`(J[A-Z]+) rel([0-9]+)`)
)

// Returns whether to skip the given instruction.
func SkipInstruction(mnemonic string, inst *x86csv.Inst) bool {
	// Drop removed instruction families.
	if strings.Contains(inst.CPUID, "MPX") {
		return true
	}

	// AMX is weird; add support later.
	if strings.Contains(inst.CPUID, "AMX") {
		return true
	}

	// This clashes with another form of
	// the same instruction.
	switch inst.Intel {
	case "MOV r32/m16, Sreg",
		"MOV r/m16, Sreg",
		"MOV r/m32, Sreg":
		return true
	}

	// This instruction form is undefined.
	if inst.Intel == "BSWAP r16op" {
		return true
	}

	// These REX forms are redundant.
	switch inst.Intel {
	case "MOV Sreg, r/m16", "MOV r/m16, Sreg", "MOV r64/m16, Sreg",
		"SAL r/m8, CL", "SAL r/m8, 1", "SAL r/m8, imm8",
		"SETA r/m8", "SETAE r/m8", "SETB r/m8", "SETBE r/m8",
		"SETC r/m8", "SETE r/m8", "SETG r/m8", "SETGE r/m8",
		"SETL r/m8", "SETLE r/m8", "SETNA r/m8", "SETNAE r/m8",
		"SETNB r/m8", "SETNBE r/m8", "SETNC r/m8", "SETNE r/m8",
		"SETNG r/m8", "SETNGE r/m8", "SETNL r/m8", "SETNLE r/m8",
		"SETNO r/m8", "SETNP r/m8", "SETNS r/m8", "SETNZ r/m8",
		"SETO r/m8", "SETP r/m8", "SETPE r/m8", "SETPO r/m8",
		"SETS r/m8", "SETZ r/m8",
		"XCHG r8, r/m8", "XCHG r/m8, r8":
		if strings.HasPrefix(inst.Encoding, "REX.W ") || strings.HasPrefix(inst.Encoding, "REX ") {
			return true
		}
	}

	// This is undocumented and rare.
	if inst.Intel == "ICEBP" {
		return true
	}

	// This is inadvised and not supported
	// by all assemblers.
	if inst.Intel == "INT1" {
		return true
	}

	// These forms do not exist.
	if inst.Intel == "MOVSX r16, r/m16" || inst.Intel == "MOVZX r16, r/m16" {
		return true
	}

	// This form doesn't appear in
	// the Intel manual and clashes
	// with MOVQ mm1, mm2/m64, which
	// does.
	switch inst.Intel {
	case "MOVQ mm1, r/m64",
		"MOVQ r/m64, mm1":
		return true
	}

	// We skip these so we can use more
	// precise versions (added in
	// extras.go) instead. The problem
	// with these is that they become
	// ambiguous. Normally, if two
	// immediate sizes are available,
	// you can use an operand size
	// override prefix to switch between
	// them, with no other effect. With
	// these, the prefix also changes
	// the number of bytes pushed onto
	// the stack. As a result, we have
	// a version for each mode, which
	// can only be used in that mode,
	// plus explicit PUSHW imm16 and
	// PUSHD imm32 versions.
	switch inst.Intel {
	case "PUSH imm16", "PUSH imm32":
		return true
	}

	// These are instruction prefixes,
	// not instructions in their own
	// right.
	switch inst.Intel {
	case "LOCK", "XACQUIRE", "XRELEASE":
		return true
	}

	switch inst.Intel {
	case "ENTER imm16, 0", "ENTER imm16, 1":
		// These extra forms are redundant.
		return true
	case "MOVSXD r16, r/m32", "MOVSXD r32, r/m32":
		// These forms are valid (although
		// the first should have the source
		// operand r/m16), but are pointless
		// and are not supported by clang,
		// so we skip them.
		return true
	}

	// These extra forms exist for different
	// execution states, not different assembly.
	switch inst.Intel {
	case "CALL rel32",
		"JA rel32", "JAE rel32", "JB rel32", "JBE rel32",
		"JC rel32", "JE rel32", "JG rel32", "JGE rel32",
		"JL rel32", "JLE rel32", "JNA rel32", "JNAE rel32",
		"JNB rel32", "JNBE rel32", "JNE rel32", "JNG rel32",
		"JNGE rel32", "JNL rel32", "JNLE rel32", "JNO rel32",
		"JNP rel32", "JNS rel32", "JNZ rel32", "JO rel32",
		"JP rel32", "JS rel32", "JZ rel32", "JMP rel32",
		"LEAVE",
		"POP FS", "POP GS":
		if inst.Mode32 != "V" || inst.Mode64 != "V" {
			return true
		}
	}

	return false
}

// Fix makes any necessary changes to the
// instruction.
func Fix(mnemonic string, inst *x86csv.Inst) string {
	// Use a more idiomatic Ruse syntax for far
	// calls and jumps.
	if prefix, rest, ok := strings.Cut(inst.Intel, "_FAR"); ok {
		mnemonic = prefix + "-FAR"
		inst.Intel = prefix + "-FAR" + rest
	}

	// These do not have a 16-bit
	// operand version, so we don't
	// use the operand size override
	// prefix.
	switch inst.Intel {
	case "MOVNTI m32, r32":
		inst.DataSize = ""
	}

	switch inst.Intel {
	case "ENTER imm16, imm8b":
		// The args are wrong here.
		inst.Intel = "ENTER imm16u, imm5u" // The second arg cannot exceed 31.
	case "MOV Sreg, r32/m16":
		// Only accept the register form,
		// as the memory form can already
		// be chosen by MOV Sreg, r/m16.
		inst.Intel = "MOV Sreg, rmr32"
		inst.DataSize = "16"
	}

	// It's tricky to split the mode
	// compatibility here, so we drop
	// the 64-bit version here and add
	// it in extras.
	switch inst.Intel {
	case "ENQCMD r32/r64, m512":
		inst.Intel = "ENQCMD r32, m512"
	case "ENQCMDS r32/r64, m512":
		inst.Intel = "ENQCMDS r32, m512"
	case "MOVDIR64B r16/r32/r64, m512":
		inst.Intel = "MOVDIR64B r16/r32, m512"
	case "UMONITOR rmr16/rmr32/rmr64":
		inst.Intel = "UMONITOR rmr16/rmr32"
	}

	// This REX.W prefix should not be
	// mandatory.
	switch inst.Intel {
	case "CALL-FAR m16:64",
		"MOVQ mm1, r/m64",
		"MOVQ r/m64, mm1":
		inst.Encoding = strings.TrimPrefix(inst.Encoding, "REX.W ")
	}

	// Fixing the final operand.
	switch mnemonic {
	case "CMPPD", "VCMPPD",
		"CMPPS", "VCMPPS",
		"CMPSD", "VCMPSD",
		"CMPSS", "VCMPSS":
		if strings.HasSuffix(inst.Intel, "imm8") {
			inst.Intel = strings.TrimSuffix(inst.Intel, "imm8") + "imm5u" // The final immediate is in the range 0-31.
		}
	}

	// Thes become ambiguous when we later
	// split the operands
	switch inst.Intel {
	case "MOVQ xmm1, r/m64":
		inst.Intel = "MOVQ xmm1, rmr64" // This would clash with MOVQ xmm1, xmm2/m64.
	case "MOVQ r/m64, xmm1":
		inst.Intel = "MOVQ rmr64, xmm1" // This would clash with MOVQ xmm2/m64, xmm1.
	case "VMOVQ xmm1, r/m64":
		inst.Intel = "VMOVQ xmm1, rmr64" // This would clash with VMOVQ xmm1, m64.
	case "VMOVQ r/m64, xmm1":
		inst.Intel = "VMOVQ rmr64, xmm1" // This would clash with VMOVQ xmm2/m64, xmm1.
	}

	// Remove implied operands so we do not
	// expect to see them.
	switch inst.Intel {
	case "HRESET imm8, <EAX>":
		inst.Intel = strings.TrimSuffix(inst.Intel, ", <EAX>")
	case "LOADIWKEY xmm1, xmm2, <EAX>, <XMM0>":
		inst.Intel = strings.TrimSuffix(inst.Intel, ", <EAX>, <XMM0>")
	case "TPAUSE rmr32, <EDX>, <EAX>",
		"UMWAIT rmr32, <EDX>, <EAX>":
		inst.Intel = strings.TrimSuffix(inst.Intel, ", <EDX>, <EAX>")
	case "BLENDVPD xmm1, xmm2/m128, <XMM0>",
		"BLENDVPS xmm1, xmm2/m128, <XMM0>",
		"PBLENDVB xmm1, xmm2/m128, <XMM0>",
		"SHA256RNDS2 xmm1, xmm2/m128, <XMM0>":
		inst.Intel = strings.TrimSuffix(inst.Intel, ", <XMM0>")
	case "ENCODEKEY128 r32, rmr32, <XMM0>, <XMM1-2>, <XMM4-6>":
		inst.Intel = strings.TrimSuffix(inst.Intel, ", <XMM0>, <XMM1-2>, <XMM4-6>")
	case "ENCODEKEY256 r32, rmr32, <XMM0-1>, <XMM2-6>":
		inst.Intel = strings.TrimSuffix(inst.Intel, ", <XMM0-1>, <XMM2-6>")
	case "AESDECWIDE128KL m384, <XMM0-7>",
		"AESENCWIDE128KL m384, <XMM0-7>",
		"AESDECWIDE256KL m512, <XMM0-7>",
		"AESENCWIDE256KL m512, <XMM0-7>":
		inst.Intel = strings.TrimSuffix(inst.Intel, ", <XMM0-7>")
	}

	// Apply corrections to incorrect descriptions.
	switch inst.Intel {
	case "AAD imm8", "AAM imm8", "INT imm8":
		inst.Intel += "u" // The base must be unsigned.
	case "EXTRQ xmm1, imm8, imm8":
		inst.Intel = strings.ReplaceAll(inst.Intel, "xmm1", "xmm2")
	case "INT 3":
		// The 3 is not an operand.
		mnemonic = "int3"
		inst.Intel = "INT3"
	}

	// There are several instructions where the table combines
	// versions that take a register or memory argument but where
	// the data size matters. With these instructions, disassembling
	// is fine but assembling is ambiguous. To fix that, we split
	// the register versions (which have a data size equal to the
	// register size) and the memory version, which has no data
	// size. We then add a memory-only version in the extra data
	// added after this loop. This fixes the assembling.
	switch inst.Intel {
	case "NOP r/m16",
		"SLDT r/m16",
		"SMSW r/m16",
		"STR r/m16":
		inst.Intel = strings.ReplaceAll(inst.Intel, "r/m16", "rmr16")
		inst.DataSize = "16"
	case "NOP r/m32":
		inst.Intel = strings.ReplaceAll(inst.Intel, "r/m32", "rmr32")
		inst.DataSize = "32"
	case "SLDT r32/m16",
		"SMSW r32/m16",
		"STR r32/m16":
		inst.Intel = strings.ReplaceAll(inst.Intel, "r32/m16", "rmr32")
		inst.DataSize = "32"
	case "SLDT r64/m16",
		"SMSW r64/m16",
		"STR r64/m16":
		inst.Intel = strings.ReplaceAll(inst.Intel, "r64/m16", "rmr64")
		inst.DataSize = "64"
	}

	// Some instructions are missing data sizes.
	switch inst.Intel {
	case "CMPSB",
		"INSB",
		"JMP rel8",
		"LODSB",
		"MOVSB",
		"OUTSB",
		"PUSH imm8",
		"SCASB",
		"STOSB":
		inst.DataSize = "8"
		if !inst.HasTag("operand8") {
			inst.Tags += ",operand8"
		}
	case "CALL rel16",
		"CALL-FAR ptr16:16",
		"CBW",
		"CMPSW",
		"CWD",
		"INSW",
		"IRET",
		"JMP rel16",
		"JMP-FAR ptr16:16",
		"LODSW",
		"MOVSW",
		"OUTSW",
		"POPA",
		"POPF",
		"PUSHA",
		"PUSHF",
		"RDRAND rmr16",
		"RDSEED rmr16",
		"SCASW",
		"SLDT rmr16",
		"STOSW",
		"XBEGIN rel16":
		inst.DataSize = "16"
		if !inst.HasTag("operand16") {
			inst.Tags += ",operand16"
		}
	case "CALL rel32",
		"CALL-FAR ptr16:32",
		"CDQ",
		"CMPSD",
		"CWDE",
		"INSD",
		"IRETD",
		"JMP rel32",
		"JMP-FAR ptr16:32",
		"LODSD",
		"MOVSD",
		"OUTSD",
		"POPAD",
		"POPFD",
		"PUSHAD",
		"PUSHFD",
		"SCASD",
		"STOSD",
		"XBEGIN rel32":
		inst.DataSize = "32"
		if !inst.HasTag("operand32") {
			inst.Tags += ",operand32"
		}
	case "CDQE",
		"INSQ",
		"IRETQ",
		"LODSQ",
		"MOVSQ",
		"OUTSQ",
		"POPFQ",
		"PUSHFQ",
		"SCASQ",
		"STOSQ":
		inst.DataSize = "64"
	}
	if m := cmovRegex.FindStringSubmatch(inst.Intel); len(m) > 0 {
		inst.DataSize = m[2]
	}
	if m := jxxRegex.FindStringSubmatch(inst.Intel); len(m) > 0 {
		inst.DataSize = m[2]
		if !inst.HasTag("operand" + m[2]) {
			inst.Tags += ",operand" + m[2]
		}
	}

	// Some instructions are missing
	// data sizes but don't have any
	// operand size ambiguity.
	switch mnemonic {
	case "VPEXTRB",
		"VPINSRB":
		inst.DataSize = "8"
	case "VPEXTRW",
		"VPINSRW":
		inst.DataSize = "16"
	case "VPEXTRD",
		"VADDSS",
		"VCMPSS",
		"VCOMISS",
		"VCVTSS2SD",
		"VDIVSS",
		"VINSERTPS",
		"VMAXSS",
		"VMINSS",
		"VMOVSS",
		"VMULSS",
		"VSQRTSS",
		"VSUBSS",
		"VUCOMISS":
		inst.DataSize = "32"
	case "VPEXTRQ",
		"VADDSD",
		"VCMPSD",
		"VCOMISD",
		"VCVTSD2SS",
		"VDIVSD",
		"VMAXSD",
		"VMINSD",
		"VMOVHPD",
		"VMOVLPD",
		"VMOVQ",
		"VMOVSD",
		"VMULSD",
		"VSQRTSD",
		"VSUBSD",
		"VUCOMISD":
		inst.DataSize = "64"
	}
	switch inst.Intel {
	case "VCVTSI2SD xmm1, xmmV, r/m32{er}",
		"VCVTSI2SS xmm1, xmmV, r/m32{er}":
		inst.DataSize = "32"
	case "VCVTSI2SD xmm1, xmmV, r/m64{er}",
		"VCVTSI2SS xmm1, xmmV, r/m64{er}":
		inst.DataSize = "64"
	}

	// Some instructions have operand tags that are
	// unhelpful.
	switch inst.Intel {
	case "ADOX r32, r/m32",
		"CRC32 r32, r/m8",
		"CRC32 r64, r/m8",
		"CVTSD2SI r32, xmm2/m64",
		"CVTSI2SD xmm1, r/m32",
		"CVTSI2SS xmm1, r/m32",
		"CVTSS2SI r32, xmm2/m32",
		"CVTTSD2SI r32, xmm2/m64",
		"CVTTSS2SI r32, xmm2/m32",
		"MOV Sreg, r/m16",
		"MOV Sreg, rmr32",
		"MOV m16, Sreg",
		"MOV rmr16, Sreg",
		"MOV rmr32, Sreg",
		"POP r64op", "POP r/m64",
		"PUSH r64op", "PUSH r/m64",
		"PTWRITE r/m32":
		inst.Tags = ""
	}

	// The 'cm' encoding marker appears to be
	// spurious, so we remove it here.
	inst.Encoding = strings.TrimSuffix(inst.Encoding, " cm")

	return mnemonic
}
