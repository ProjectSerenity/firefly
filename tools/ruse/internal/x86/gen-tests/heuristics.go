// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Helpers to check whether (slightly different)
// disassembly matches assembly.

package main

import (
	"regexp"
	"strconv"
	"strings"

	"firefly-os.dev/tools/ruse/internal/x86"
)

// IsDisassemblyMatch returns whether the given disassembly
// matches the test entry.
func IsDisassemblyMatch(entry *TestEntry, disasm, code string) bool {
	// We don't include REX.W prefixes in
	// the assembly, so strip them if they
	// are reasonable.
	if (entry.Inst.Encoding.REX_W && strings.HasPrefix(disasm, "rex.w ") && !strings.HasPrefix(entry.Intel, "rex.w ")) ||
		(entry.Inst.Mnemonic == "jmp-far" && entry.Mode.Int == 64 && strings.HasPrefix(disasm, "rex.w ")) {
		disasm = strings.TrimPrefix(disasm, "rex.w ")
	}

	got := strings.Fields(disasm)
	gotMnemonic, _, _ := strings.Cut(disasm, " ")
	gotEntry := &TestEntry{Inst: new(x86.Instruction), Mode: entry.Mode, Intel: disasm, Code: code}
	*gotEntry.Inst = *entry.Inst
	gotEntry.Inst.Mnemonic = gotMnemonic
	want := strings.Fields(entry.Intel)
	wantMnemonic := entry.Inst.Mnemonic

	if heuristicallySimilarArguments(got, want) {
		return true
	}

	// First, we check for instructions that take a
	// radix argument, with 10 as the default. In
	// these cases, objdump may print the radix,
	// even if the original radix was omitted.
	//
	// AAD.
	// AAM.
	if len(want) == 1 &&
		((wantMnemonic == "aad" && code == "d50a") ||
			(wantMnemonic == "aam" && code == "d40a")) {
		wantVerbose := []string{want[0], "0xa"}
		if heuristicallySimilarArguments(got, wantVerbose) {
			return true
		}
	}

	// Some instructions have an implicit final argument,
	// which is not actually encoded.
	//
	// BLENDVPD xmm1, xmm2/m128, <XMM0>.
	// BLENDVPS xmm1, xmm2/m128, <XMM0>.
	// PBLENDVB xmm1, xmm2/m128, <XMM0>.
	if got[len(got)-1] == "xmm0" && entry.Inst.Mnemonic == gotMnemonic &&
		(wantMnemonic == "blendvpd" ||
			wantMnemonic == "blendvps" ||
			wantMnemonic == "pblendvb") {
		// Strip the implicit xmm0 arg.
		gotExplicit := append([]string(nil), got[:len(got)-1]...)
		gotExplicit[len(gotExplicit)-1] = noComma(gotExplicit[len(gotExplicit)-1])
		if heuristicallySimilarArguments(gotExplicit, want) {
			return true
		}
	}

	// DS-relative CALL. A DS segment
	// override prefix on a CALL instruction
	// is ambiguous with the NOTRACK prefix
	// when CET is in use.
	if got[0] == "notrack" && !strings.HasPrefix(got[len(got)-1], "ds:") {
		gotSwitched := append([]string(nil), got[1:]...)
		gotSwitched[len(gotSwitched)-1] = "ds:" + gotSwitched[len(gotSwitched)-1]
		if heuristicallySimilarArguments(gotSwitched, want) {
			return true
		}
	}

	// Objdump uses a different syntax when
	// printing near CALLs. Here, we check
	// whether this is such a matching call.
	if wantMnemonic == "call" && gotMnemonic == "call" && len(got) >= 2 {
		// objdump: `call 7fff <_start-0x4010ec>`
		// clang:   `call dword ptr [0x7fff]`
		addr := want[len(want)-1]
		addr = strings.TrimPrefix(addr, "[0x")
		addr = strings.TrimSuffix(addr, "]")
		if addr == got[1] {
			return true
		}
	}

	// Objdump prints "call/jmpw" for the
	// 16-bit version of CALL/JMP.
	//
	// CALL.
	// JMP.
	if (wantMnemonic == "call" && gotMnemonic == "callw") ||
		(wantMnemonic == "jmp" && gotMnemonic == "jmpw") {
		wantRenamed := append([]string(nil), want...)
		wantRenamed[0] += "w"
		if heuristicallySimilarArguments(got, wantRenamed) {
			return true
		}
	}

	// Objdump uses a different syntax for
	// far calls/jumps than clang.
	//
	// CALL-FAR ptr.
	// JMP-FAR ptr.
	if strings.Contains(entry.Inst.Syntax, "ptr") &&
		(wantMnemonic == "call-far" ||
			wantMnemonic == "jmp-far") {
		// objdump: call 0x12:0xfcfdfe
		// clang:   lcall 0x12, 0xfcfdfe
		gotSplit := make([]string, len(got)-1, len(got)+1)
		copy(gotSplit, got)
		gotSplit = append(gotSplit, strings.Fields(strings.Replace(got[len(got)-1], ":", ", ", 1))...)

		wantCall := append([]string(nil), want...)
		wantCall[0] = strings.TrimPrefix(wantCall[0], "l") // call-far is lcall but jmp-far is just jmp.

		if heuristicallySimilarArguments(gotSplit, wantCall) {
			return true
		}
	}

	// Objdump and clang disagree on the
	// size hint for an indirect absolute
	// long jump. Clang expects the size
	// hint to match the instruction
	// pointer in the absolute address,
	// whereas objdump gives a hint for
	// the combined segment and instruction
	// pointer:
	//
	// 	| Syntax  | Clang  | Objdump |
	// 	+---------+--------+---------+
	// 	| m16:16  | word   | dword   |
	// 	| m16:32  | dword  | fword   |
	//
	// CALL-FAR m16:.
	// JMP-FAR m16:.
	if strings.Contains(entry.Inst.Syntax, "m16:") &&
		(wantMnemonic == "call-far" ||
			wantMnemonic == "jmp-far") {
		wantTweaked := append([]string(nil), want...)
		wantTweaked[0] = strings.TrimPrefix(wantTweaked[0], "l") // call-far is lcall but jmp-far is just jmp.
		switch entry.Mode.Int {
		case 16:
			wantTweaked[1] = "dword"
		case 32, 64:
			wantTweaked[1] = "fword"
		}

		if heuristicallySimilarArguments(got, wantTweaked) {
			return true
		}
	}

	// Several different mnemonics have
	// the same encoding, so we normalise
	// them here.
	//
	// CMOVcc.
	// SAL / SHL.
	// SETcc.
	if CanonicalMnemonic(entry.Mode.Int, gotMnemonic) == CanonicalMnemonic(entry.Mode.Int, wantMnemonic) && heuristicallySimilarArguments(got[1:], want[1:]) {
		return true
	}

	// There seems to be an objdump bug where
	// in 16-bit mode, it prints the wrong size
	// source register size in the CRC32 instruction.
	//
	// CRC32.
	if entry.Mode.Int == 16 && wantMnemonic == "crc32" && len(got) == 3 {
		gotFlipped := append([]string(nil), got...)
		gotFlipped[2] = flip16And32BitRegister(gotFlipped[2])
		if heuristicallySimilarArguments(gotFlipped, want) {
			return true
		}
	}

	// Handle specialisations (see equivalence.go).
	//
	// CMPPD / VCMPPD.
	// CMPPS / VCMPPS.
	// CMPSD / VCMPSD.
	// CMPSS / VCMPSS.
	// PCLMULQDQ / VPCLMULQDQ.
	if heuristicSpecialisation(entry, gotEntry) || heuristicSpecialisation(gotEntry, entry) {
		return true
	}

	// There is no difference between the
	// `MOVDIR64B r16/r32, m512` forms,
	// As a result, we accept either size
	// of the destination register.
	//
	// MOVDIR64B r16/r32, m512.
	if wantMnemonic == "movdir64b" {
		wantFlipped := append([]string(nil), want...)
		wantFlipped[1] = flip16And32BitRegister(wantFlipped[1])
		if heuristicallySimilarArguments(got, wantFlipped) {
			return true
		}

		// Missing size hints.
		if !strings.Contains(disasm, " ptr ") && strings.Contains(entry.Intel, " ptr ") {
			wantNoHint := strings.Fields(sizeHintRegex.ReplaceAllString(strings.Join(wantFlipped, " "), ""))
			if heuristicallySimilarArguments(got, wantNoHint) {
				return true
			}
		}
	}

	// Objdump seems to produce the wrong
	// size hint.
	//
	// MOVDIRI m32, r32.
	// MOVDIRI m64, r64.
	if wantMnemonic == "movdiri" && len(want) > 2 && len(got) > 2 {
		wantTweaked := append([]string(nil), want...)
		if want[1] == "dword" && got[1] == "word" {
			wantTweaked[1] = "word"
		}

		if heuristicallySimilarArguments(got, wantTweaked) {
			return true
		}
	}

	// Some x87 floating point instructions take
	// two floating point stack positions as their
	// arguments, which means that if both args
	// are st(0), the form can be ambiguous.
	//
	// FADD ST(0), ST(i) / FADD ST(i), ST(0).
	// FDIV ST(0), ST(i) / FDIV ST(i), ST(0).
	// FSUB ST(0), ST(i) / FSUB ST(i), ST(0).
	if len(want) == 3 &&
		((wantMnemonic == "fadd" && code == "d8c0") ||
			(wantMnemonic == "fdiv" && code == "d8f0") ||
			(wantMnemonic == "fsub" && code == "d8e0")) {
		wantReversed := []string{want[0], want[2] + ",", noComma(want[1])}
		if heuristicallySimilarArguments(got, wantReversed) {
			return true
		}
	}

	// Other x87 instructions take two arguments
	// and have an implicit form where both args
	// are implied.
	//
	// FADDP ST(1), ST / FADDP.
	// FDIVP ST(1), ST / FDIVP.
	// FDIVRP ST(1), ST / FDIVRP.
	// FMULP ST(1), ST / FMULP.
	// FSUBP ST(1), ST / FSUBP.
	// FSUBRP ST(1), ST / FSUBRP.
	if len(want) == 1 &&
		((wantMnemonic == "faddp" && code == "dec1") ||
			(wantMnemonic == "fdivp" && code == "def9") ||
			(wantMnemonic == "fdivrp" && code == "def1") ||
			(wantMnemonic == "fmulp" && code == "dec9") ||
			(wantMnemonic == "fsubp" && code == "dee9") ||
			(wantMnemonic == "fsubrp" && code == "dee1")) {
		wantExplicit := []string{want[0], "st(1),", "st"}
		if heuristicallySimilarArguments(got, wantExplicit) {
			return true
		}
	}

	// Other x87 instructions take a single arg
	// and have an implicit form where the arg
	// is implied.
	//
	// FCOM ST(1) / FCOM.
	// FCOMP ST(1) / FCOMP.
	// FUCOM ST(1) / FUCOM.
	// FUCOMP ST(1) / FUCOMP.
	// FXCH ST(1) / FXCH.
	if len(want) == 1 &&
		((wantMnemonic == "fcom" && code == "d8d1") ||
			(wantMnemonic == "fcomp" && code == "d8d9") ||
			(wantMnemonic == "fucom" && code == "dde1") ||
			(wantMnemonic == "fucomp" && code == "dde9") ||
			(wantMnemonic == "fxch" && code == "d9c9")) {
		wantExplicit := []string{want[0], "st(1)"}
		if heuristicallySimilarArguments(got, wantExplicit) {
			return true
		}
	}

	// Objdump gives a BYTE PTR size hint for the
	// INVLPG instruction, which is unnecessary.
	//
	// INVLPG.
	if wantMnemonic == "invlpg" && len(got) == 4 && got[1] == "byte" && got[2] == "ptr" {
		gotHintless := []string{got[0], got[3]}
		if heuristicallySimilarArguments(gotHintless, want) {
			return true
		}
	}

	// IRET has common alternative names for certain
	// CPU modes.
	//
	// IRET.
	// IRETW.
	// IRETD.
	if (wantMnemonic == "iret" && gotMnemonic == "iretw" && len(got) == 1 && entry.Mode.Int != 16 && strings.HasPrefix(code, "66")) ||
		(wantMnemonic == "iretd" && gotMnemonic == "iret" && len(got) == 1 && (entry.Mode.Int == 32 || entry.Mode.Int == 64) && !strings.HasPrefix(code, "66")) {
		return true
	}

	// There is no functional difference between
	// 16-bit and 32-bit source registers for these
	// instructions, as only 16 bits are read in
	// either case.
	//
	// LAR.
	// LSL.
	if len(want) == 3 &&
		(wantMnemonic == "lar" ||
			wantMnemonic == "lsl") {
		wantFlipped := append([]string(nil), want...)
		wantFlipped[2] = flip16And32BitRegister(wantFlipped[2])
		if heuristicallySimilarArguments(got, wantFlipped) {
			return true
		}
	}

	// Objdump and clang disagree on the
	// size hint for a load far pointer
	// operation. Clang expects the size
	// hint to match the instruction
	// pointer in the absolute address,
	// whereas objdump gives a hint for
	// the combined segment and instruction
	// pointer:
	//
	// 	| Syntax  | Clang  | Objdump |
	// 	+---------+--------+---------+
	// 	| m16:16  | word   | dword   |
	// 	| m16:32  | dword  | fword   |
	// 	| m16:64  | qword  | fword   |
	//
	// LES, LCS, LSS, LDS, LFS, LGS.
	if len(want) == 5 && len(got) == 5 &&
		(wantMnemonic == "les" ||
			wantMnemonic == "lcs" ||
			wantMnemonic == "lss" ||
			wantMnemonic == "lds" ||
			wantMnemonic == "lfs" ||
			wantMnemonic == "lgs") {
		wantTweaked := append([]string(nil), want...)
		size := x86.RegisterSizes[noComma(got[1])]
		switch size {
		case 16:
			wantTweaked[2] = "dword"
		case 32:
			wantTweaked[2] = "fword"
		case 64:
			wantTweaked[2] = "fword"
		}
		if heuristicallySimilarArguments(got, wantTweaked) {
			return true
		}
	}

	// Objdump uses a mnemonic "w" suffix in
	// 16-bit mode and "d" in 32-bit mode.
	//
	// LGDT.
	// LIDT.
	// SGDT.
	// SIDT.
	switch wantMnemonic {
	case "lgdt", "lidt", "sgdt", "sidt":
		gotRenamed := append([]string(nil), got...)
		switch entry.Mode.Int {
		case 16:
			gotRenamed[0] = strings.TrimSuffix(gotRenamed[0], "w")
		case 32:
			gotRenamed[0] = strings.TrimSuffix(gotRenamed[0], "d")
		}
		if heuristicallySimilarArguments(gotRenamed, want) {
			return true
		}
	}

	// There is no difference between the
	// `MOV r16, Sreg` and `MOV r32, Sreg`
	// forms, as only 16 bits of data are
	// copied in either case. As a result,
	// we accept either size of the destination
	// register.
	//
	// MOV r, Sreg.
	if wantMnemonic == "mov" && strings.HasSuffix(entry.Inst.Syntax, " Sreg") && len(want) == 3 {
		wantFlipped := append([]string(nil), want...)
		wantFlipped[1] = flip16And32BitRegister(wantFlipped[1])
		if heuristicallySimilarArguments(got, wantFlipped) {
			return true
		}
	}

	// MOVABS is just an alternative mnemonic
	// for the new versions of MOV unique to
	// 64-bit mode.
	//
	// MOVABS.
	if wantMnemonic == "mov" && len(entry.Inst.Parameters) == 2 &&
		(entry.Inst.Parameters[0].Type == x86.TypeMemoryOffset || entry.Inst.Parameters[1].Type == x86.TypeMemoryOffset || (!entry.Inst.Mode16 && !entry.Inst.Mode32)) {
		wantRenamed := append([]string(nil), want...)
		wantRenamed[0] = "movabs"
		if heuristicallySimilarArguments(got, wantRenamed) {
			return true
		}

		// Missing size hints.
		if !strings.Contains(disasm, " ptr ") && strings.Contains(entry.Intel, " ptr ") {
			wantNoHint := strings.Fields(sizeHintRegex.ReplaceAllString(entry.Intel, ""))
			wantNoHint[0] = "movabs"
			if heuristicallySimilarArguments(got, wantNoHint) {
				return true
			}
		}
	}

	// XCHG (R|E)AX, (R|E)AX is also
	// known as NOP. We're more permissive
	// and allow the NOP mnemonic for any
	// register register exchange where the
	// two registers are the same.
	//
	// NOP.
	if code == "90" && disasm == "nop" &&
		wantMnemonic == "xchg" && len(want) == 3 &&
		!strings.Contains(want[1], "]") &&
		noComma(want[1]) == want[2] {
		return true
	}

	// The EAX/rAX argument is implicit.
	//
	// SKINIT (EAX).
	// VMLOAD (rAX).
	// VMRUN (rAX).
	// VMSAVE (rAX).
	switch wantMnemonic {
	case "skinit", "vmload", "vmrun", "vmsave":
		wantRegister := "eax"
		if wantMnemonic != "skinit" && entry.Mode.Int == 64 {
			wantRegister = "rax"
		}

		if (len(got) == 1 && gotMnemonic == wantMnemonic) ||
			(len(got) == 2 && gotMnemonic == wantMnemonic && got[1] == wantRegister) {
			return true
		}
	}

	// The XMM0 third argument is implicit.
	//
	// SHA256RNDS2 xmm1, xmm2/m128, <XMM0>.
	switch wantMnemonic {
	case "sha256rnds2":
		gotTrimmed := append([]string(nil), got[:len(got)-1]...)
		gotTrimmed[len(gotTrimmed)-1] = noComma(gotTrimmed[len(gotTrimmed)-1])
		if heuristicallySimilarArguments(gotTrimmed, want) {
			return true
		}
	}

	// There is no difference between the
	// `UMONITOR rmr16/rmr32` forms. As a
	// result, we accept either size of the
	// destination register.
	//
	// UMONITOR rmr16/rmr32.
	if wantMnemonic == "umonitor" && len(want) == 2 {
		wantFlipped := append([]string(nil), want...)
		wantFlipped[1] = flip16And32BitRegister(wantFlipped[1])
		if heuristicallySimilarArguments(got, wantFlipped) {
			return true
		}
	}

	// XCHG's arguments are reversible,
	// but may involve memory addresses,
	// so we pivot on the comma.
	//
	// XCHG.
	if wantMnemonic == "xchg" {
		swapped := strings.Split(strings.TrimPrefix(entry.Intel, entry.Inst.Mnemonic), ",")
		var wantReversedText string
		if len(swapped) == 2 {
			wantReversedText = entry.Inst.Mnemonic + " " + swapped[1] + ", " + swapped[0]
		}
		wantReversedArgs := strings.Fields(wantReversedText)
		if heuristicallySimilarArguments(got, wantReversedArgs) {
			return true
		}
	}

	// Objdump prints "xbeginw" for the
	// 16-bit version of XBEGIN.
	//
	// XBEGIN.
	if wantMnemonic == "xbegin" && gotMnemonic == "xbeginw" {
		wantRenamed := append([]string(nil), want...)
		wantRenamed[0] = "xbeginw"
		if heuristicallySimilarArguments(got, wantRenamed) {
			return true
		}
	}

	// The XLATB instruction is a variant
	// of XLAT with an implicit address.
	//
	// XLATB.
	if wantMnemonic == "xlatb" {
		var wantExplicit []string
		switch entry.Mode.Int {
		case 16:
			wantExplicit = []string{"xlat", "byte", "ptr", "ds:[bx]"}
		case 32:
			wantExplicit = []string{"xlat", "byte", "ptr", "ds:[ebx]"}
		case 64:
			wantExplicit = []string{"xlat", "byte", "ptr", "ds:[rbx]"}
		}
		if heuristicallySimilarArguments(got, wantExplicit) {
			return true
		}
	}

	// Objdump prints debug registers as
	// dbX, rather than drX, so we adjust
	// here.
	//
	// Debug registers.
	if strings.Contains(entry.Inst.Syntax, "DR0-DR7") && len(got) == 3 {
		gotFixed := append([]string(nil), got...)
		if strings.HasPrefix(gotFixed[1], "db") {
			gotFixed[1] = "dr" + strings.TrimPrefix(gotFixed[1], "db")
		} else if strings.HasPrefix(gotFixed[2], "db") {
			gotFixed[2] = "dr" + strings.TrimPrefix(gotFixed[2], "db")
		}
		if heuristicallySimilarArguments(gotFixed, want) {
			return true
		}
	}

	// Objdump produces the clearer, more
	// verbose syntax, although we also
	// support the shorter versions.
	//
	// String operations.
	if canon, ok := CanonicaliseStringOperation(entry, false); ok {
		wantCanon := strings.Fields(canon)
		if heuristicallySimilarArguments(got, wantCanon) {
			return true
		}
	}

	// Sometimes objdump will print an address
	// without a size hint. In that case, we
	// remove it from the expected text and
	// check that everything else matches.
	//
	// Missing size hints.
	if !strings.Contains(disasm, " ptr ") && strings.Contains(entry.Intel, " ptr ") {
		wantNoHint := strings.Fields(sizeHintRegex.ReplaceAllString(entry.Intel, ""))
		if heuristicallySimilarArguments(got, wantNoHint) {
			return true
		}

		// Objdump uses a strange syntax
		// when the address consist only
		// of a displacement. This removes
		// the braces and the leading '0x'
		// of the displacement.
		maybeNum := strings.TrimSuffix(strings.TrimPrefix(want[len(want)-1], "["), "]")
		_, errInt := strconv.ParseInt(maybeNum, 0, 64)
		_, errUint := strconv.ParseUint(maybeNum, 0, 64)
		if wantMnemonic == "jmp" && (errInt == nil || errUint == nil) {
			wantNoHint[len(wantNoHint)-1] = strings.TrimPrefix(maybeNum, "0x")
			if heuristicallySimilarArguments(got, wantNoHint) {
				return true
			}
		}
	}

	return false
}

var sizeHintRegex = regexp.MustCompile(`[a-z]+ ptr `)

// flip16And32BitRegister is a helper function.
// It replaces 16-bit general purpose registers
// with their 32-bit form and vice versa.
func flip16And32BitRegister(s string) string {
	hasComma := strings.HasSuffix(s, ",")
	s = noComma(s)
	switch s {
	// Extend a 16-bit register to 32 bits.
	case "ax", "cx", "dx", "bx", "bp", "sp", "di", "si":
		s = "e" + s
	case "r8w", "r9w", "r10w", "r11w", "r12w", "r13w", "r14w", "r15w":
		s = strings.ReplaceAll(s, "w", "d")
	// Truncate a 32-bit register to 16 bits.
	case "eax", "ecx", "edx", "ebx", "ebp", "esp", "edi", "esi":
		s = strings.TrimPrefix(s, "e")
	case "r8d", "r9d", "r10d", "r11d", "r12d", "r13d", "r14d", "r15d":
		s = strings.ReplaceAll(s, "d", "w")
	}

	if hasComma {
		s += ","
	}

	return s
}

// heuristicSpecialisation is like IsSecialisation,
// but with a slightly more fuzzy check on the
// arguments once the specialisation has been
// accounted for.
func heuristicSpecialisation(special, general *TestEntry) bool {
	// Check that general has specialisations.
	// If not, we're done.
	options, ok := specialisations[general.Inst.Mnemonic]
	if !ok {
		return false
	}

	for _, option := range options {
		// Check that the specialisation
		// matches.
		if special.Inst.Mnemonic != option.Mnemonic {
			continue
		}

		// Check that the general form has
		// a suffix to match the specialisation
		// argument that will be absent from
		// the special form.
		generalArgs := general.IntelArgs()
		if len(generalArgs) == 0 {
			return false
		}

		// Find the special form.
		ok = false
		finalArg := generalArgs[len(generalArgs)-1:]
		for _, suffix := range option.Suffixes {
			if heuristicallySimilarArguments(finalArg, []string{suffix}) {
				ok = true
				break
			}
		}

		if !ok {
			return false
		}

		// Finally, check that the two
		// forms are identical, other than
		// the specialisation at the end
		// of the general form.
		//
		// First, we split by fields so that
		// we compare each component, as
		// expected by heuristicallySimilarArguments.
		specialArgs := strings.Fields(special.Intel)
		specialArgs[0] = general.Inst.Mnemonic
		generalArgs = strings.Fields(general.Intel)
		generalArgs = generalArgs[:len(generalArgs)-1]                             // Ignore the final arg.
		generalArgs[len(generalArgs)-1] = noComma(generalArgs[len(generalArgs)-1]) // Remove the trailing comma.

		return heuristicallySimilarArguments(specialArgs, generalArgs)
	}

	// No matching specialisation found.
	return false
}

// heuristicallySimilarArguments takes a fuzzy
// approach to determining whether gotSet and
// wantSet are the same. For example, if the
// values at a certain index are the same number
// in a different radix, heuristicallySimilarArguments
// would return true.
func heuristicallySimilarArguments(gotSet, wantSet []string) bool {
	if len(gotSet) != len(wantSet) {
		return false
	}

	equivalent := map[string]string{
		// Alternative size hints.
		"oword": "xmmword",
		"yword": "ymmword",
		"zword": "zmmword",
		// Alternative address syntaxes.
		"ds:0x0":                "[0x0]",
		"ds:0x1":                "[-0x7fffffffffffffff]",
		"ds:0xa":                "[0xa]",
		"ds:0xb":                "[0xb]",
		"ds:0x10":               "[0x10]",
		"ds:0x11":               "[0x11]",
		"ds:0xfe":               "[0xfe]",
		"ds:0xff":               "[0xff]",
		"ds:0x7fff":             "[0x7fff]",
		"ds:0xfefd":             "[0xfefd]",
		"ds:0xfffefd":           "[0xfffefd]",
		"ds:0xfefdfcfb":         "[0xfefdfcfb]",
		"ds:0xfffffffe":         "[0xfffffffe]",
		"ds:0x7fffffff":         "[0x7fffffff]",
		"ds:0x7ff8f9fafbfcfdfe": "[0x7ff8f9fafbfcfdfe]",
		"ds:0x8877665544332211": "[0x8877665544332211]",
		"ds:0xffffffff80000000": "[-0x80000000]",
		"ds:0xffffffffeffffffe": "[0xffffffffeffffffe]",
		"ds:0xfffffffffbfcfdfe": "[0xf7f8f9fafbfcfdfe]",
		"ds:0xfffffffffefdfcfb": "[0xfefdfcfb]",
		"ds:0xffffffffffffff01": "[-0xff]",
		"ds:0xffffffffffffffef": "[0x7fffffffffffffef]",
		"ds:0xfffffffffffffff6": "[-0xa]",
		"ds:0xfffffffffffffffe": "[0xfffffffe]",
		"ds:[si]":               "[si]",
		"es:[di]":               "[di]",
		"ds:[esi]":              "[esi]",
		"es:[edi]":              "[edi]",
		"ffffffffffffff01":      "[-0xff]",
		"fffffffffffffff6":      "[-0xa]",
		"[rbx+0x1]":             "[rbx-0x7fffffffffffffff]",
		"[rcx-0x11]":            "[rcx+0x7fffffffffffffef]",
		"[rbp+rcx*8-0x1]":       "[rbp+rcx*8+0x7fffffffffffffff]",
		"[rsp+rax*8+0x1]":       "[rsp+rax*8-0x7fffffffffffffff]",
	}

	for i := range gotSet {
		// Start by checking whether they're
		// the same. If so, we keep going.
		got := gotSet[i]
		want := wantSet[i]
		if got == want {
			continue
		}

		// Remove any trailing comma, provided
		// it's shared.
		if strings.HasSuffix(got, ",") && strings.HasSuffix(want, ",") {
			got = noComma(got)
			want = noComma(want)
		}

		// Next, check for identical memory
		// addreses.
		if CanonicalIntelMemory(got) == CanonicalIntelMemory(want) {
			continue
		}

		// One common difference is integers
		// in a different base, so we try to
		// account for that first.
		uint1, err1 := strconv.ParseUint(got, 0, 64)
		uint2, err2 := strconv.ParseUint(want, 0, 64)
		if err1 == nil && err2 == nil && uint1 == uint2 {
			continue
		}
		int3, err3 := strconv.ParseInt(got, 0, 64)
		int4, err4 := strconv.ParseInt(want, 0, 64)
		if err3 == nil && err4 == nil && int3 == int4 {
			continue
		}

		// We also try to account for where a
		// negative number has been encoded
		// correctly but assumed positive by
		// objdump.
		//
		// We compare signed to unsigned values
		// for 60-bit negatives (which exceed an
		// int64 in their unsigned form), then we
		// subtract register sizes to cover small
		// negatives.
		if (err1 == nil && err4 == nil && int64(uint1) == int4) ||
			(err2 == nil && err3 == nil && int64(uint2) == int3) {
			continue
		}
		if err3 == nil && err4 == nil {
			if int3-0x100 == int4 ||
				int3-0x10000 == int4 ||
				int3-0x10000_0000 == int4 {
				continue
			}
		}

		// Exceptions:
		if fixed, ok := equivalent[got]; ok && fixed == want {
			continue
		}

		// We haven't been able to
		// reconcile the difference,
		// so we give up.
		return false
	}

	return true
}
