// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"regexp"
	"strings"
	"unicode"

	"firefly-os.dev/tools/ruse/internal/x86"
)

func (m *Mnemonic) fix(stats *Stats) error {
	// Some instructions have such long
	// opcodes that they wrap, leading
	// to a bad decoding by us.
	switch m.Instruction {
	case "/r ib ROUNDPS xmm1, xmm2/m128, imm8":
		m.Opcode += " /r ib"
		m.Instruction = "ROUNDPS xmm1, xmm2/m128, imm8"
	case "ibVPERMILPS zmm1 {k1}{z}, zmm2/m512/m32bcst, imm8":
		m.Opcode += " ib"
		m.Instruction = "VPERMILPS zmm1 {k1}{z}, zmm2/m512/m32bcst, imm8"
	case "/ib VREDUCESS xmm1 {k1}{z}, xmm2, xmm3/m32{sae}, imm8":
		m.Opcode += " ib"
		m.Instruction = "VREDUCESS xmm1 {k1}{z}, xmm2, xmm3/m32{sae}, imm8"
	}

	// And these instructions have odd
	// implicit operands, which we drop.
	switch m.Instruction {
	case "AESDECWIDE128KL m384, <XMM0-7>", "AESDECWIDE256KL m512, <XMM0-7>",
		"AESENCWIDE128KL m384, <XMM0-7>", "AESENCWIDE256KL m512, <XMM0-7>":
		m.Instruction = strings.TrimSuffix(m.Instruction, ", <XMM0-7>")
	case "ENCODEKEY128 r32, r32, <XMM0-2>, <XMM4-6>":
		m.Instruction = "ENCODEKEY128 r32, r32"
	case "ENCODEKEY256 r32, r32 <XMM0-6>":
		m.Instruction = "ENCODEKEY256 r32, r32"
	}

	// And these are missing CPUID
	// feature info.
	switch m.Instruction {
	case "CMPXCHG r/m8, r8", "CMPXCHG r/m16, r16", "CMPXCHG r/m32, r32",
		"CPUID",
		"INVD", "INVLPG m",
		"WBINVD":
		// We have extra CPUID info here.
		if m.CPUID == "" {
			m.CPUID = "486"
		}
	case "CMPXCHG8B m64",
		"RDMSR", "WRMSR":
		// We have extra CPUID info here.
		if m.CPUID == "" {
			m.CPUID = "Pentium"
		}
	case "SYSENTER", "SYSEXIT":
		// We have extra CPUID info here.
		if m.CPUID == "" {
			m.CPUID = "PentiumII"
		}
	}

	// Whereas these are errors.
	switch m.Instruction {
	case "VREDUCESD xmm1 {k1}{z}, xmm2, xmm3/m64{sae}, imm8/r":
		stats.InstructionError("p.%d: Malformed instruction mnemonic %q", m.Page, m.Instruction)
		m.Opcode += " /r"
		m.Instruction = "VREDUCESD xmm1 {k1}{z}, xmm2, xmm3/m64{sae}, imm8"
	case "VCVTTPD2UDQ xmm1 {k1}{z}, ymm2/m256/m64bcst":
		if m.Opcode == "EVEX.256.0F.W1 78 02 /r" {
			stats.InstructionError("p.%d: Spurious %q in opcode value", m.Page, "02")
			m.Opcode = "EVEX.256.0F.W1 78 /r"
		}
	}

	if err := m.fixOpcode(stats); err != nil {
		return err
	}

	if err := m.fixInstruction(stats); err != nil {
		return err
	}

	if err := m.fixOperandEncoding(stats); err != nil {
		return err
	}

	// Error in CMOVG r64, r/m64.
	if m.Mode64 == "V/N.E." && m.Mode32 == "N/A" {
		stats.InstructionError("p.%d: Invalid compatibility values %q and %q", m.Page, m.Mode64, m.Mode32)
		m.Mode64 = "V"
		m.Mode32 = "N.E."
	}

	if err := m.fixMode64(stats); err != nil {
		return err
	}

	if err := m.fixMode32(stats); err != nil {
		return err
	}

	if err := m.fixMode16(stats); err != nil {
		return err
	}

	if err := m.fixCPUID(stats); err != nil {
		return err
	}

	if err := m.fixDescription(stats); err != nil {
		return err
	}

	return nil
}

func (m *Mnemonic) fixOpcode(stats *Stats) error {
	if m.Opcode == "" {
		return Errorf(m.Page, "no instruction opcode")
	}

	// Check the encoding is well-formed.
	// e.g.: `EVEX.128.66.0F38.W1 8A /r`
	//
	// First, we make changes from one
	// correct form to another. We give
	// the benefit of the doubt to spacing
	// errors, which may just be an
	// artefact of the PDF encoding
	// process.
	m.Opcode = respace(m.Opcode)
	fix := func(old, new string) {
		m.Opcode = respace(strings.ReplaceAll(m.Opcode, old, new))
	}

	fix(" imm8", " ib")
	fix("REX.w", "REX.W")
	fix("REX.W+", "REX.W +")
	fix("0F 38.WIG", "0F38.WIG")
	fix("0F .WIG", "0F.WIG")
	fix("0F38 .WIG", "0F38.WIG")
	fix("NDS .LZ", "NDS.LZ")
	fix("58+ r", "58+r")
	fix("B0+ ", "B0+")
	fix("B8+ ", "B8+")
	fix("40+ ", "40+")
	fix("*", "")
	fix(",", " ")
	fix("/", " /")
	fix("REX.W +", "REX.W")
	fix("REX +", "REX")
	fix("REX 0F BE", "REX.W 0F BE")
	fix("REX 0F B2", "REX.W 0F B2")
	fix("REX 0F B4", "REX.W 0F B4")
	fix("REX 0F B5", "REX.W 0F B5")
	fix("/05", "/5")
	fix("/ib", "ib")
	fix(" (mod=11)", "")
	fix(" (mod!=11 /5 memory only)", "")
	fix(" (mod!=11 /5 RM=010)", "")

	// Next, we make changes that correct
	// an error.
	fix = func(old, new string) {
		for {
			prefix, suffix, found := strings.Cut(m.Opcode, old)
			if !found {
				return
			}

			stats.InstructionError("p.%d: Invalid opcode component %q, should be %q", m.Page, old, new)
			m.Opcode = prefix + new + suffix
		}
	}

	fix(" 0f ", " 0F ")
	fix(". 0F38", ".0F38")
	fix("0F38 30.WIG", "0F38.WIG 30")
	fix("0F38.0", "0F38.W0")
	fix(".660F.", ".66.0F.")
	fix("VEX128", "VEX.128")
	fix("0F3A.W0.1D", "0F3A.W0 1D")
	fix("66 0F38 ", "66 0F 38 ")
	fix("66 0F3A ", "66 0F 3A ")
	switch m.Opcode {
	case "VEX.128.66.0F.W0 6E /":
		stats.InstructionError("p.%d: Invalid opcode component %q missing %q", m.Page, m.Opcode, "r")
		m.Opcode = "VEX.128.66.0F.W0 6E /r"
	}

	// Finally, check the encoding.
	_, err := x86.ParseEncoding(m.Opcode)
	if err != nil {
		return Errorf(m.Page, "invalid opcode %q: %v", m.Opcode, err)
	}

	return nil
}

var (
	regOrMemRegex   = regexp.MustCompile(`^r/m(\d+)$`)
	unsplitOperands = map[string]bool{
		"m14/28byte":  true,
		"m94/108byte": true,
	}
)

func (m *Mnemonic) fixInstruction(stats *Stats) error {
	// Check the instruction mnemonic is
	// well-formed.
	switch m.Instruction {
	// This is just a consequence of
	// the challenges of parsing a
	// PDF.
	case "VCMPSDxmm1,xmm2,xmm3/m64, imm8":
		m.Instruction = "VCMPSD xmm1, xmm2, xmm3/m64, imm8" // Space out the parameters.
	case "V4FMADDPSzmm1{k1}{z},zmm2+3, m128":
		m.Instruction = "V4FMADDPS zmm1 {k1}{z}, zmm2, m128" // Space out the parameters and drop the +3.
	case "VP4DPWSSDzmm1{k1}{z},zmm2+3, m128":
		m.Instruction = "VP4DPWSSD zmm1 {k1}{z}, zmm2, m128" // Space out the parameters and drop the +3.

	// These are cases where we choose
	// a different syntax, which is
	// less ambiguous.
	case "EXTRACTPS reg/m32, xmm1, imm8":
		m.Instruction = "EXTRACTPS r/m32, xmm1, imm8" // Resolve `reg`.
	case "VEXTRACTPS reg/m32, xmm1, imm8":
		m.Instruction = "VEXTRACTPS r/m32, xmm1, imm8" // Resolve `reg`.
	case "LAHF", "SAHF":
		m.Mode64 = "Valid" // The manual says invalid, but this isn't always true.
	case "LAR reg, r32/m161":
		m.Instruction = "LAR r32, r32/m16" // Resolve `reg` and drop the superscript 1.
	case "MOV r16/r32/m16, Sreg":
		m.Instruction = "MOV r32, Sreg" // This makes much more sense.
	case "MOVMSKPD reg, xmm", "VMOVMSKPD reg, xmm2", "VMOVMSKPD reg, ymm2",
		"MOVMSKPS reg, xmm", "VMOVMSKPS reg, xmm2", "VMOVMSKPS reg, ymm2":
		m.Instruction = strings.Replace(m.Instruction, "reg", "r32", 1) // Resolve `reg`.
	case "PEXTRB reg/m8, xmm2, imm8", "VPEXTRB reg/m8, xmm2, imm8":
		m.Instruction = strings.Replace(m.Instruction, "reg", "r32", 1) // Resolve `reg`.
	case "PEXTRW reg, mm, imm8", "PEXTRW reg, xmm, imm8", "PEXTRW reg/m16, xmm, imm8",
		"VPEXTRW reg, xmm1, imm8", "VPEXTRW reg/m16, xmm2, imm8":
		m.Instruction = strings.Replace(m.Instruction, "reg", "r32", 1) // Resolve `reg`.
	case "PMOVMSKB reg, mm":
		m.Instruction = "PMOVMSKB r32, mm" // Resolve `reg`.
	case "PMOVMSKB reg, xmm", "VPMOVMSKB reg, xmm1":
		m.Instruction = strings.Replace(m.Instruction, "reg", "r32", 1) // Resolve `reg`.
	case "VPMOVMSKB reg, ymm1":
		m.Instruction = "VPMOVMSKB r32, ymm1" // Resolve `reg`.
	case "SENDUIPI reg":
		m.Instruction = "SENDUIPI r64" // Resolve `reg`.
	case "SLDT r/m16":
		m.Instruction = "SLDT r16/r32/m16" // Add a 32-bit register option.
	case "STR r/m16":
		m.Instruction = "STR r16/r32/m16" // Add a 32-bit register option.
	case "TILELOADD tmm1, sibmem", "TILELOADDT1 tmm1, sibmem":
		m.Instruction = strings.Replace(m.Instruction, "sibmem", "mib", 1) // Canonicalise `sibmem` as `mib`.
	case "TILESTORED sibmem, tmm1":
		m.Instruction = "TILESTORED mib, tmm1" // Canonicalise `sibmem` as `mib`.
	case "TPAUSE r32, <edx>, <eax>":
		m.Instruction = "TPAUSE r32, <EDX>, <EAX>" // Capitalise implicit registers.
	case "UMWAIT r32, <edx>, <eax>":
		m.Instruction = "UMWAIT r32, <EDX>, <EAX>" // Capitalise implicit registers.
	case "VMOVW xmm1, reg/m16", "VMOVW reg/m16, xmm1":
		m.Instruction = strings.Replace(m.Instruction, "reg", "r16", 1) // Resolve `reg`.
	case "VP2INTERSECTD k1+1, xmm2, xmm3/m128/m32bcst", "VP2INTERSECTD k1+1, ymm2, ymm3/m256/m32bcst", "VP2INTERSECTD k1+1, zmm2, zmm3/m512/m32bcst",
		"VP2INTERSECTQ k1+1, xmm2, xmm3/m128/m64bcst", "VP2INTERSECTQ k1+1, ymm2, ymm3/m256/m64bcst", "VP2INTERSECTQ k1+1, zmm2, zmm3/m512/m64bcst":
		m.Instruction = strings.Replace(m.Instruction, "k1+1", "k1", 1) // Trim the `+1` array indicator.
	case "VPBROADCASTB xmm1 {k1}{z}, reg", "VPBROADCASTB ymm1 {k1}{z}, reg", "VPBROADCASTB zmm1 {k1}{z}, reg":
		m.Instruction = strings.Replace(m.Instruction, "reg", "r8", 1) // Resolve `reg`.
	case "VPBROADCASTW xmm1 {k1}{z}, reg", "VPBROADCASTW ymm1 {k1}{z}, reg", "VPBROADCASTW zmm1 {k1}{z}, reg":
		m.Instruction = strings.Replace(m.Instruction, "reg", "r16", 1) // Resolve `reg`.
	case "V4FNMADDPS zmm1{k1}{z}, zmm2+3, m128":
		m.Instruction = "V4FNMADDPS zmm1 {k1}{z}, zmm2, m128" // Trim the `+3` array indicator.
	case "V4FMADDSS xmm1{k1}{z}, xmm2+3, m128", "V4FNMADDSS xmm1{k1}{z}, xmm2+3, m128":
		m.Instruction = strings.Replace(m.Instruction, "xmm2+3", "xmm2", 1) // Trim the `+3` array indicator.
	case "VP4DPWSSDS zmm1{k1}{z}, zmm2+3, m128":
		m.Instruction = "VP4DPWSSDS zmm1 {k1}{z}, zmm2, m128" // Trim the `+3` array indicator.
	case "VGATHERDPD xmm1, vm32x, xmm2", "VGATHERQPD xmm1, vm64x, xmm2" /*VGATHERDPD ymm1, vm32x, ymm2*/, "VGATHERQPD ymm1, vm64y, ymm2",
		"VGATHERDPS xmm1, vm32x, xmm2", "VGATHERQPS xmm1, vm64x, xmm2", "VGATHERDPS ymm1, vm32y, ymm2", "VGATHERQPS xmm1, vm64y, xmm2",
		"VPGATHERDD xmm1, vm32x, xmm2", "VPGATHERQD xmm1, vm64x, xmm2", "VPGATHERDD ymm1, vm32y, ymm2", /*VPGATHERQD xmm1, vm64y, xmm2*/
		"VPGATHERDQ xmm1, vm32x, xmm2", "VPGATHERQQ xmm1, vm64x, xmm2", "VPGATHERDQ ymm1, vm32x, ymm2", "VPGATHERQQ ymm1, vm64y, ymm2":
		if !strings.Contains(m.Opcode, "/vsib") && !strings.Contains(m.Opcode, "/sib") {
			m.Opcode += " /vsib"
		}

	// These are inconsequential stylistic
	// choices.
	case "VEXTRACTF32x4 xmm1/m128 {k1}{z}, zmm2, imm8", "VEXTRACTF64x4 ymm1/m256 {k1}{z}, zmm2, imm8",
		"VEXTRACTI32x4 xmm1/m128 {k1}{z}, zmm2, imm8", "VEXTRACTI64x4 ymm1/m256 {k1}{z}, zmm2, imm8":
		m.Instruction = strings.Replace(m.Instruction, "x4", "X4", 1) // Capitalise the X in the mnemonic.
	case "VFNMADD132PD xmm0 {k1}{z}, xmm1, xmm2/m128/m64bcst":
		m.Instruction = "VFNMADD132PD xmm1 {k1}{z}, xmm2, xmm3/m128/m64bcst" // Shift the register names up to avoid xmm0.
	case "VBROADCASTI32x2 xmm1 {k1}{z}, xmm2/m64", "VBROADCASTI32x2 ymm1 {k1}{z}, xmm2/m64", "VBROADCASTI32x2 zmm1 {k1}{z}, xmm2/m64":
		m.Instruction = strings.Replace(m.Instruction, "x2", "X2", 1) // Capitalise the X in the mnemonic.
	case "VSHUFF32x4 zmm1{k1}{z}, zmm2, zmm3/m512/m32bcst, imm8", "VSHUFI32x4 zmm1{k1}{z}, zmm2, zmm3/m512/m32bcst, imm8":
		m.Instruction = strings.Replace(m.Instruction, "x4", "X4", 1) // Capitalise the X in the mnemonic.
	case "VSHUFF64x2 zmm1{k1}{z}, zmm2, zmm3/m512/m64bcst, imm8", "VSHUFI64x2 zmm1{k1}{z}, zmm2, zmm3/m512/m64bcst, imm8":
		m.Instruction = strings.Replace(m.Instruction, "x2", "X2", 1) // Capitalise the X in the mnemonic.

	// Whereas these are errors.
	case "ENCODEKEY256 r32, r32 <XMM0-6>":
		stats.InstructionError("p.%d: Missing comma after operand 2 in mnemonic %q", m.Page, m.Instruction)
		m.Instruction = "ENCODEKEY256 r32, r32, <XMM0-6>" // Added a comma after operand 2.
	case "KSHIFTLB k1, k2, imm8", "KSHIFTLW k1, k2, imm8", "KSHIFTLD k1, k2, imm8", "KSHIFTLQ k1, k2, imm8",
		"KSHIFTRB k1, k2, imm8", "KSHIFTRW k1, k2, imm8", "KSHIFTRD k1, k2, imm8", "KSHIFTRQ k1, k2, imm8",
		"VFIXUPIMMPS xmm1 {k1}{z}, xmm2, xmm3/m128/m32bcst, imm8",
		"VFIXUPIMMPS ymm1 {k1}{z}, ymm2, ymm3/m256/m32bcst, imm8",
		"VFPCLASSSS k2 {k1}, xmm2/m32, imm8",
		"VRANGESD xmm1 {k1}{z}, xmm2, xmm3/m64{sae}, imm8", "VRANGESS xmm1 {k1}{z}, xmm2, xmm3/m32{sae}, imm8",
		"VREDUCESD xmm1 {k1}{z}, xmm2, xmm3/m64{sae}, imm8":
		if !strings.Contains(m.Opcode, " ib") {
			stats.InstructionError("p.%d: Opcode %q is missing a token to represent immediate operand", m.Page, m.Opcode)
			m.Opcode += " ib"
		}
	case "MOV r64, CR8", "MOV CR8, r64":
		if strings.HasSuffix(m.Opcode, "/0") {
			stats.InstructionError("p.%d: Inconsistent operand encoding for register CR8 in %q", m.Page, m.Opcode)
			m.Opcode = strings.TrimSuffix(m.Opcode, "/0") + "/r"
		}
	case "VGATHERDPD ymm1, vm32x, ymm2":
		stats.InstructionError("p.%d: Opcode %q is missing a token to represent VSIB operand", m.Page, m.Opcode)
		m.Instruction = "VGATHERDPD ymm1, vm32y, ymm2" // vm32x to vm32y.
		// Also, check the encoding.
		if !strings.Contains(m.Opcode, "/vsib") && !strings.Contains(m.Opcode, "/sib") {
			m.Opcode += " /vsib"
		}
	case "VMOVDQU32 xmm1 {k1}{z}, xmm2/mm128":
		stats.InstructionError("p.%d: Spurious operand token in mnemonic %q", m.Page, m.Instruction)
		m.Instruction = "VMOVDQU32 xmm1 {k1}{z}, xmm2/m128" // Trim the second `m` in `mm128`.
	case "VPAND ymm1, ymm2, ymm3/.m256":
		stats.InstructionError("p.%d: Spurious period in mnemonic %q", m.Page, m.Instruction)
		m.Instruction = "VPAND ymm1, ymm2, ymm3/m256" // Trim the `.` in `.m256`.
	case "VPGATHERQD xmm1, vm64y, xmm2":
		stats.InstructionError("p.%d: Spurious 128-bit operands in 256-bit instruction %q", m.Page, m.Instruction)
		m.Instruction = "VPGATHERQD ymm1, vm64y, ymm2" // xmm to ymm.
		// Also, check the encoding.
		if !strings.Contains(m.Opcode, "/vsib") && !strings.Contains(m.Opcode, "/sib") {
			m.Opcode += " /vsib"
		}
	case "VSCATTERPF0QPD vm64z {k1}":
		stats.InstructionError("p.%d: Spurious ZMM VSIB operand in YMM instruction %q", m.Page, m.Instruction)
		m.Instruction = "VSCATTERPF0QPD vm64y {k1}" // vm64z to vm64y.
	case "VSCATTERPF1QPD vm64z {k1}":
		stats.InstructionError("p.%d: Spurious ZMM VSIB operand in YMM instruction %q", m.Page, m.Instruction)
		m.Instruction = "VSCATTERPF1QPD vm64y {k1}" // vm64z to vm64y.
	case "XBEGIN rel16":
		if !strings.Contains(m.Opcode, "cw") {
			stats.InstructionError("p.%d: Opcode %q is missing a token to represent code offset operand", m.Page, m.Opcode)
			m.Opcode += " cw"
		}
	case "XBEGIN rel32":
		if !strings.Contains(m.Opcode, "cd") {
			stats.InstructionError("p.%d: Opcode %q is missing a token to represent code offset operand", m.Page, m.Opcode)
			m.Opcode += " cd"
		}
	}

	name, rest, found := strings.Cut(m.Instruction, " ")
	if name != strings.ToUpper(name) {
		return Errorf(m.Page, "invalid name %q: not all upper case (%q)", name, m.Instruction)
	}

	if !found {
		m.Instruction = name
		return nil
	}

	args := strings.Split(rest, ",")
	for i, arg := range args {
		args[i] = strings.TrimSpace(arg)
	}

	if len(args) > 4 {
		return Errorf(m.Page, "instruction has %d operands, want at most 4", len(args))
	}

	// We should only have at most one
	// operand that has multiple forms.
	multiform := false
	for i, arg := range args {
		// Trim any suffixes.
		arg = strings.TrimRight(arg, "*") // Asterisk(s).
		arg, zero := strings.CutSuffix(strings.TrimSpace(arg), "{z}")
		arg, mask := strings.CutSuffix(strings.TrimSpace(arg), "{k1}")
		arg, msk2 := strings.CutSuffix(strings.TrimSpace(arg), "{k2}")
		arg, ernd := strings.CutSuffix(strings.TrimSpace(arg), "{er}")
		arg, supr := strings.CutSuffix(strings.TrimSpace(arg), "{sae}")
		if !supr {
			arg, supr = strings.CutSuffix(strings.TrimSpace(arg), "{ sae}")
		}
		arg = strings.TrimSpace(regOrMemRegex.ReplaceAllString(strings.TrimSpace(arg), "r${1}/m${1}"))
		if unsplitOperands[arg] {
			// We don't split these, as they're
			// not a representation of different
			// operand types, just an acknowledgement
			// that the size of the data in memory
			// will depend on the current CPU mode.
			switch arg {
			case "m14/28byte", "m94/108byte":
			default:
				return Errorf(m.Page, "invalid parameter %d (%q)", i+1, args[i])
			}

			if supr {
				arg += " {sae}"
			}
			if ernd {
				arg += " {er}"
			}
			if mask && zero {
				arg += " {k1}{z}"
			} else if mask {
				arg += " {k1}"
			} else if msk2 {
				arg += " {k2}"
			} else if zero {
				arg += " {z}"
			}

			args[i] = arg

			continue
		}

		parts := strings.Split(arg, "/")
		if len(parts) > 1 {
			if multiform {
				return Errorf(m.Page, "invalid parameter %d (%q): already seen another split operand", i+1, args[i])
			}

			multiform = true
		}

		for j, part := range parts {
			part = strings.TrimSpace(part)
			parts[j] = part
			switch part {
			case "AL", "CL", "AX", "DX", "EAX", "ECX", "RAX", "CR8", "XMM0":
			case "ES", "CS", "SS", "DS", "FS", "GS":
			case "ST":
			case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			case "r8", "r16", "r32", "r64":
			case "r32a", "r32b":
				parts[j] = "r32"
			case "r64a", "r64b":
				parts[j] = "r64"
			case "Sreg":
			case "CR0-CR7", "CR0–CR7":
				parts[j] = "CR0-CR7"
			case "DR0-DR7", "DR0–DR7":
				parts[j] = "DR0-DR7"
			case "bnd", "bnd1", "bnd2", "bnd3":
			case "k1", "k2", "k3":
			case "ST(0)", "ST(i)":
			case "mm", "mm1", "mm2":
			case "tmm", "tmm1", "tmm2", "tmm3":
			case "xmm", "xmm1", "xmm2", "xmm3", "xmm4":
			case "ymm", "ymm1", "ymm2", "ymm3", "ymm4":
			case "zmm", "zmm1", "zmm2", "zmm3", "zmm4":
			case "rel8", "rel16", "rel32":
			case "ptr16:16", "ptr16:32":
			case "mib":
				if !strings.Contains(m.Opcode, "/sib") {
					stats.InstructionError("p.%d: Opcode %q is missing token to represent MIB operand", m.Page, m.Opcode)
					m.Opcode += " /sib"
				}
			case "mem":
				parts[j] = "m"
			case "m", "m8", "m16", "m32", "m64", "m128", "m256", "m384", "m512":
			case "m2byte", "m512byte":
			case "m16&16", "m16&32", "m32&32", "m16&64":
			case "m16:16", "m16:32", "m16:64":
			case "m80bcd", "m80dec":
			case "m16fp", "m32fp", "m64fp", "m80fp":
			case "m16int", "m32int", "m64int":
			case "m16bcst", "m32bcst", "m64bcst":
			case "moffs8", "moffs16", "moffs32", "moffs64":
			case "vm32x", "vm32y", "vm32z", "vm64x", "vm64y", "vm64z":
			case "imm8", "imm16", "imm32", "imm64":
			// Implicit parameters.
			case "<EAX>", "<ECX>", "<EDX>", "<XMM0>":
			default:
				return Errorf(m.Page, "invalid parameter %d (%q): invalid component %q", i+1, args[i], part)
			}
		}

		arg = strings.Join(parts, "/")
		if ernd {
			arg += " {er}"
		}
		if supr {
			arg += " {sae}"
		}
		if mask && zero {
			arg += " {k1}{z}"
		} else if mask {
			arg += " {k1}"
		} else if msk2 {
			arg += " {k2}"
		} else if zero {
			arg += " {z}"
		}

		args[i] = arg
	}

	m.Instruction = name + " " + strings.Join(args, ", ")

	return nil
}

func (m *Mnemonic) fixOperandEncoding(stats *Stats) error {
	// PREFETCHW has a mismatch
	// between the two tables.
	if m.OperandEncoding == "A" && m.Instruction == "PREFETCHW m8" {
		stats.ListingError("p.%d: Spurious operand encoding %q", m.Page, m.OperandEncoding)
		m.OperandEncoding = "M"
	}

	// VMASKMOV has a very narrow
	// Op/En column, so the values
	// wrap.
	if (m.OperandEncoding == "RV\nM" || m.OperandEncoding == "MV\nR") && strings.HasPrefix(m.Instruction, "VMASKMOV") {
		m.OperandEncoding = dropSpaces(m.OperandEncoding)
	}

	// VPERM2F128 has a very narrow
	// Op/En column, so the value
	// wraps.
	if m.OperandEncoding == "RV\nMI" && m.Instruction == "VPERM2F128 ymm1, ymm2, ymm3/m256, imm8" {
		m.OperandEncoding = "RVMI"
	}

	// XCHG erroneously uses the same encoding
	// for two different instruction forms.
	if m.OperandEncoding == "O" {
		switch m.Instruction {
		case "XCHG AX, r16",
			"XCHG EAX, r32",
			"XCHG RAX, r64":
			stats.ListingError("p.%d: Spurious operand encoding %q", m.Page, m.OperandEncoding)
			m.OperandEncoding = "AO"
		case "XCHG r16, AX",
			"XCHG r32, EAX",
			"XCHG r64, RAX":
			stats.ListingError("p.%d: Spurious operand encoding %q", m.Page, m.OperandEncoding)
			m.OperandEncoding = "OA"
		}
	}

	// Check the operand encoding is
	// well-formed.
	if i := strings.IndexFunc(m.OperandEncoding, unicode.IsSpace); i >= 0 {
		return Errorf(m.Page, "invalid operand encoding %q", m.OperandEncoding)
	}

	return nil
}

func (m *Mnemonic) fixMode64(stats *Stats) error {
	// Check the 64-bit mode compatibility
	// is well-formed.
	switch dropSpaces(m.Mode64) {
	case "Valid*":
		fallthrough // An asterisk is fine.
	case "V", "Valid":
		m.Mode64 = "Valid"
	case "NE", "N.E", "Inv.":
		stats.InstructionError("p.%d: Malformed 64-bit mode indicator %q", m.Page, dropSpaces(m.Mode64))
		fallthrough
	case "I", "Invalid", "Invalid*", "N.E.", "N.P.", "N.I.", "N.S.":
		m.Mode64 = "Invalid"
	default:
		return Errorf(m.Page, "invalid 64-bit mode compatibility %q", m.Mode64)
	}

	return nil
}

func (m *Mnemonic) fixMode32(stats *Stats) error {
	// Check the 32-bit mode compatibility
	// is well-formed.
	switch dropSpaces(m.Mode32) {
	case "Valid*":
		fallthrough // An asterisk is fine.
	case "V", "Valid":
		m.Mode32 = "Valid"
	case "NE", "N.E", "Inv.":
		stats.InstructionError("p.%d: Malformed 32-bit mode indicator %q", m.Page, dropSpaces(m.Mode32))
		fallthrough
	case "I", "Invalid", "Invalid*", "N.E.":
		m.Mode32 = "Invalid"
	default:
		return Errorf(m.Page, "invalid 32-bit mode compatibility %q", m.Mode32)
	}

	return nil
}

func (m *Mnemonic) fixMode16(stats *Stats) error {
	// Determine the 16-bit mode compatibility.
	m.Mode16 = m.Mode32
	if strings.Contains(m.Opcode, "VEX.") {
		m.Mode16 = "Invalid"
	}

	mnemonic, _, _ := strings.Cut(m.Instruction, " ")
	switch mnemonic {
	case "CLRSSBSY",
		"INVPCID",
		"JECXZ",
		"LAR",
		"LSL",
		"RSTORSSP":
		m.Mode16 = "Invalid"
	}

	return nil
}

func (m *Mnemonic) fixCPUID(stats *Stats) error {
	// Check the CPUID feature flags are
	// well-formed.
	m.CPUID = respace(m.CPUID)
	switch m.CPUID {
	case "":
	case "486":
	case "ADX":
	case "AES":
	case "AES AVX":
	case "AESKLE":
	case "AESKLE WIDE_KL":
	case "AESKLEWIDE_KL":
		stats.InstructionError("p.%d: Malformed CPUID feature flag %q", m.Page, m.CPUID)
		m.CPUID = "AESKLE WIDE_KL"
	case "AMX-BF16":
	case "AMX-INT8":
	case "AMX-TILE":
	case "AVX":
	case "AVX2":
	case "AVX512_4FMAPS":
	case "AVX512_4VNNIW":
	case "AVX512_BITALG":
	case "AVX512_BITALG AVX512VL":
	case "AVX512BW":
	case "AVX512CD":
	case "AVX512D Q":
		stats.InstructionError("p.%d: Malformed CPUID feature flag %q", m.Page, m.CPUID)
		m.CPUID = "AVX512DQ"
	case "AVX512DQ":
	case "AVX512ER":
	case "AVX512F":
	case "AVX512F AVX512_BF16":
	case "AVX512F AVX512BW":
	case "AVX512F AVX512_VP2INTERSECT":
	case "AVX512F GFNI":
	case "AVX512-FP16":
	case "AVX512-FP16 AVX512VL":
	case "AVX512_IFMA":
	case "AVX512_IFMA AVX512VL":
	case "AVX512PF":
	case "AVX512_VBMI":
	case "AVX512_VBMI2":
	case "AVX512_VBMI2 AVX512VL":
	case "AVX512_VBMI AVX512VL":
	case "AVX512VL":
	case "AVX512VL AVX512_BF16":
	case "AVX512VL AVX512BW":
	case "AVX512VL AVX512CD":
	case "AVX512VL AVX512DQ":
	case "AVX512VLA VX512DQ":
		stats.InstructionError("p.%d: Malformed CPUID feature flag %q", m.Page, m.CPUID)
		m.CPUID = "AVX512VL AVX512DQ"
	case "AVX512VL AVX512F":
	case "AVX512VL AVX512_VBMI":
	case "AVX512VL AVX512_VP2INTERSECT":
	case "AVX512VL GFNI":
	case "AVX512_VNNI":
	case "AVX512_VNNI AVX512VL":
	case "AVX512_VPOPCNTDQ":
	case "AVX512_VPOPCNTDQ AVX512VL":
	case "AVX GFNI":
	case "AVX-VNNI":
	case "BMI1":
	case "BMI2":
	case "Both AES and AVX flags":
		m.CPUID = "AES AVX"
	case "CET_IBT":
	case "CET_SS":
	case "CLDEMOTE":
	case "CLWB":
	case "ENQCMD":
	case "F16C":
	case "FMA":
	case "FSGSBASE":
	case "GFNI":
	case "HLE":
	case "HLE or RTM":
		m.CPUID = "HLE RTM"
	case "HRESET":
	case "INVPCID":
	case "KL":
	case "LZCNT":
	case "MMX":
	case "MOVBE":
	case "MOVDIR64B":
	case "MOVDIRI":
	case "MPX":
	case "OSPKE":
	case "PCLMULQDQ":
	case "PCLMULQDQ AVX":
	case "PCONFIG":
	case "Pentium":
	case "PentiumII":
	case "PREFETCHW":
	case "PREFETCHWT1":
	case "PTWRITE":
	case "RDPID":
	case "RDRAND":
	case "RDSEED":
	case "RTM":
	case "SERIALIZE":
	case "SHA":
	case "SMAP":
	case "SSE":
	case "SSE2":
	case "SSE3":
	case "SSE4_1":
	case "SSE4_2":
	case "SSSE3":
	case "TSXLDTRK":
	case "UINTR":
	case "VAES":
	case "VAES AVX512F":
	case "VAES AVX512VL":
	case "VAES AVX512VL the Equivalent Inverse Cipher, using one 128-bit data":
		// Parsing bug.
		m.CPUID = "VAES AVX512VL"
	case "VAES AVX512VL the Equivalent Inverse Cipher, using two 128-bit data":
		// Parsing bug.
		m.CPUID = "VAES AVX512VL"
	case "VPCLMULQDQ":
	case "VPCLMULQDQ AVX512F":
	case "VPCLMULQDQ AVX512VL":
	case "WAITPKG":
	case "WBNOINVD":
	case "XSAVE":
	case "XSAVEC":
	case "XSAVEOPT Save state components specified by EDX:EAX":
		// Parsing bug.
		m.CPUID = "XSAVEOPT"
	case "XSS":
	default:
		return Errorf(m.Page, "invalid CPUID feature flag %q", m.CPUID)
	}

	return nil
}

func (m *Mnemonic) fixDescription(stats *Stats) error {
	// Check the description is well-formed.
	m.Description = respace(m.Description)

	return nil
}

func (e *OperandEncoding) fix(stats *Stats) error {
	if err := e.fixEncoding(stats); err != nil {
		return err
	}

	if err := e.fixTupleType(stats); err != nil {
		return err
	}

	if err := e.fixOperands(stats); err != nil {
		return err
	}

	return nil
}

func (e *OperandEncoding) fixEncoding(stats *Stats) error {
	// XCHG erroneously uses the same encoding
	// for two different instruction forms.
	if e.Encoding == "O" && e.Operands == ([4]string{"AX/EAX/RAX (r, w)", "opcode + rd (r, w)", "N/A", "N/A"}) {
		stats.ListingError("p.%d: Ambiguous operand encoding %q", e.Page, e.Encoding)
		e.Encoding = "AO"
	} else if e.Encoding == "O" && e.Operands == ([4]string{"opcode + rd (r, w)", "AX/EAX/RAX (r, w)", "N/A", "N/A"}) {
		stats.ListingError("p.%d: Ambiguous operand encoding %q", e.Page, e.Encoding)
		e.Encoding = "OA"
	}

	// One of the VMOVLPD forms has an invalid
	// encoding.
	if e.Operands == ([4]string{"ModRM:r/m (r)", "VEX.vvvv (r)", "ModRM:r/m (r)", "N/A"}) {
		stats.ListingError("p.%d: Incorrect operand encoding %q", e.Page, e.Operands)
		e.Operands[0] = "ModRM:reg (w)"
	}

	// Check the encoding is well-formed.
	if i := strings.IndexFunc(e.Encoding, unicode.IsSpace); i >= 0 {
		return Errorf(e.Page, "invalid operand encoding %q", e.Encoding)
	}

	return nil
}

func (e *OperandEncoding) fixTupleType(stats *Stats) error {
	// Check the tuple type is well-formed.
	switch e.TupleType {
	case "NA":
		stats.ListingError("p.%d: Malformed tuple type %q", e.Page, e.TupleType)
		e.TupleType = ""
	case "", "N/A":
		e.TupleType = ""
	case "Full":
	case "Half":
	case "Full Mem":
	case "Scalar":
		stats.ListingError("p.%d: Malformed tuple type %q", e.Page, e.TupleType)
		e.TupleType = "Tuple1 Scalar"
	case "Tuple1 Scalar":
	case "Tuple1 Fixed":
	case "Tuple2":
	case "Tuple1_4X":
		stats.ListingError("p.%d: Malformed tuple type %q", e.Page, e.TupleType)
		e.TupleType = "Tuple4"
	case "Tuple4":
	case "Tuple8":
	case "Half Mem":
	case "Quarter":
		stats.ListingError("p.%d: Malformed tuple type %q", e.Page, e.TupleType)
		e.TupleType = "Quarter Mem"
	case "Quarter Mem":
	case "Eighth Mem":
	case "Mem128":
	case "MOVDDUP":
	default:
		return Errorf(e.Page, "invalid tuple type %q", e.TupleType)
	}

	return nil
}

func (e *OperandEncoding) fixOperands(stats *Stats) error {
	// Check the operands are well-formed.
	for i, operand := range e.Operands {
		// Trim any suffixes.
		operand = strings.TrimSpace(strings.TrimSuffix(operand, "(r)"))
		operand = strings.TrimSpace(strings.TrimSuffix(operand, "(w)"))
		operand = strings.TrimSpace(strings.TrimSuffix(operand, "(r,w)"))
		operand = strings.TrimSpace(strings.TrimSuffix(operand, "(r, w)"))
		if strings.HasSuffix(operand, " (R)") {
			stats.ListingError("p.%d: Malformed operand %q", e.Page, e.Operands[i])
			operand = strings.TrimSpace(strings.TrimSuffix(operand, "(R)"))
		}

		switch operand {
		// No operand.
		case "N/A":
		case "NA":
			stats.ListingError("p.%d: Malformed operand %q", e.Page, e.Operands[i])
			operand = "N/A"

		// Operands that aren't encoded.
		case "None":
		case "1",
			"AL/AX/EAX/RAX", "AX/EAX/RAX", "CL":
			operand = "None"

		// Implied operands that aren't encoded.
		case "<XMM0>":
			stats.InstructionError("p.%d: Malformed operand %q", e.Page, e.Operands[i])
			fallthrough
		case "Implicit EAX", "Implicit EAX/RAX", "implicit XMM0", "Implicit XMM0", "Implicit XMM0-2", "Implicit XMM0-6", "Implicit XMM0-7", "Implicit XMM4-6":
			operand = "Implicit"

		// VEX/EVEX prefixes.
		case "VEX.vvvv":
		case "VEX.1vvv":
			operand = "VEX.vvvv"
		case "EVEX.vvvv":

		// Register modifier.
		case "opcode + rd":
			operand = "Opcode"

		// Code offset.
		case "Offset":
		case "Segment + Absolute Address":
			operand = "Offset"

		// ModRM byte.
		case "ModRM:reg":
		case "ModRM:r/m":

		// (V)SIB.
		case "SIB.index":
			operand = "SIB"
		case "BaseReg (R): VSIB:base, VectorReg(R): VSIB:index",
			"BaseReg (R): VSIB:base, VectorReg(R):VSIB:index",
			"VectorReg(R): VSIB:index":
			operand = "VSIB"

		// Displacement.
		case "Moffs":
			operand = "Displacement"

		// Immediate.
		case "Imm8", "imm8", "imm8[3:0]", "imm16",
			"imm8/16/32", "imm8/16/32/64",
			"iw":
			operand = "Immediate"

		// VEX /is4
		case "imm8[7:4]":
			operand = "VEX /is4"

		default:
			return Errorf(e.Page, "invalid operand %d %q", i+1, operand)
		}

		e.Operands[i] = operand
	}

	return nil
}
