// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Tools for groups of instructions that are
// functionally equivalent, such as the pair:
//
// 	- `xchg r8, [rcx]`
// 	- `xchg [rcx], r8`
//
// And instructions with different mnemonics
// but the same meaning, such as the pair:
//
// 	- `movs byte ptr [rdi], byte ptr [rsi]`
// 	- `movsb`

package main

import (
	"fmt"
	"strings"
)

// AreEquivalent returns whether the
// two given instructions are equivalent,
// such as the pair:
//
//   - `xchg r8, [rcx]`
//   - `xchg [rcx], r8`
//
// And instructions with different mnemonics
// but the same meaning, such as the pair:
//
//   - `movs byte ptr [rdi], byte ptr [rsi]`
//   - `movsb`
func AreEquivalent(a, b *TestEntry) bool {
	// If the two are identical,
	// this is easy.
	if a.Mode == b.Mode &&
		a.Intel == b.Intel {
		return true
	}

	// NOP is an alternative name
	// for some other instruction
	// forms that have no effect.
	if a.Intel == "nop" && (b.Intel == "xchg ax, ax" || b.Intel == "xchg eax, eax" || b.Intel == "xchg rax, rax") {
		return true
	}

	// Next, we try instructions
	// with an implicit argument.
	if a.Inst.Encoding.Syntax == b.Inst.Encoding.Syntax &&
		(strings.HasPrefix(a.Intel, b.Intel) || strings.HasPrefix(b.Intel, a.Intel)) {
		return true
	}

	// Where either instruction
	// is a specialisation of
	// the other, that's an
	// equivalence.
	if IsSpecialisation(a, b) || IsSpecialisation(b, a) {
		return true
	}

	// At this point, we gather
	// their args for future
	// comparisons.
	argsA := a.IntelArgs()
	argsB := b.IntelArgs()

	// There is also XCHG, whose
	// arguments are reversible.
	if a.Inst.Mnemonic == "XCHG" && b.Inst.Mnemonic == "XCHG" &&
		len(argsA) == len(argsB) && len(argsA) == 2 {
		argsAFields0 := strings.Fields(argsA[0])
		argsAFields1 := strings.Fields(argsA[1])
		argsBFields0 := strings.Fields(argsB[0])
		argsBFields1 := strings.Fields(argsB[1])
		if equalStringSetsOrMemories(argsAFields0, argsBFields1) &&
			equalStringSetsOrMemories(argsBFields0, argsAFields1) {
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
	//
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
	//
	// Other x87 instructions take a single arg
	// and have an implicit form where the arg
	// is implied.
	//
	// FCOM ST(1) / FCOM.
	// FCOMP ST(1) / FCOMP.
	// FUCOM ST(1) / FUCOM.
	// FUCOMP ST(1) / FUCOMP.
	// FXCH ST(1) / FXCH.
	if a.Inst.Mnemonic == b.Inst.Mnemonic && len(argsA) <= 2 && len(argsB) <= 2 {
		switch a.Inst.Mnemonic {
		case "FADD", "FDIV", "FSUB":
			if len(argsA) == 2 && len(argsB) == 2 &&
				argsA[0] == argsB[1] &&
				argsA[1] == argsB[0] {
				return true
			}
		case "FADDP", "FDIVP", "FDIVRP", "FMULP", "FSUBP", "FSUBRP":
			if (len(argsA) == 0 || (len(argsA) == 2 && argsA[0] == "st(1)" && argsA[1] == "st")) &&
				(len(argsB) == 0 || (len(argsB) == 2 && argsB[0] == "st(1)" && argsB[1] == "st")) {
				return true
			}
		case "FCOM", "FCOMP", "FUCOM", "FUCOMP", "FXCH":
			if (len(argsA) == 0 || (len(argsA) == 1 && argsA[0] == "st(1)")) &&
				(len(argsB) == 0 || (len(argsB) == 1 && argsB[0] == "st(1)")) {
				return true
			}
		}
	}

	// In MOV reg/m16, Sreg, the
	// size of the destination
	// register does not change
	// the behaviour, so we accept
	// any size.
	if a.Inst.Mnemonic == "MOV" && b.Inst.Mnemonic == "MOV" &&
		strings.HasSuffix(a.Inst.Syntax, ", Sreg") && strings.HasSuffix(b.Inst.Syntax, ", Sreg") &&
		len(argsA) == 2 && len(argsB) == 2 {
		canonA, okA := CanonicaliseRegister(argsA[0])
		canonB, okB := CanonicaliseRegister(argsB[0])
		if okA && okB && canonA == canonB {
			return true
		}
	}

	// In MOV Sreg, reg/m16, the
	// size of the source
	// register does not change
	// the behaviour, so we accept
	// any size.
	if a.Inst.Mnemonic == "MOV" && b.Inst.Mnemonic == "MOV" &&
		strings.HasPrefix(a.Inst.Syntax, "MOV Sreg,") && strings.HasPrefix(b.Inst.Syntax, "MOV Sreg,") &&
		len(argsA) == 2 && len(argsB) == 2 {
		canonA, okA := CanonicaliseRegister(argsA[1])
		canonB, okB := CanonicaliseRegister(argsB[1])
		if okA && okB && canonA == canonB {
			return true
		}
	}

	// Then we check instructions
	// with different mnemonics
	// but the same underlying
	// meaning.
	if CanonicalMnemonic(a.Mode.Int, a.Inst.Mnemonic) == CanonicalMnemonic(b.Mode.Int, b.Inst.Mnemonic) &&
		a.Mode == b.Mode &&
		equalStringSets(argsA, argsB) {
		return true
	}

	// At this point, we gather
	// their args into individual
	// fields for more detailed
	// future comparisons.
	fieldsA := strings.Fields(a.Intel)
	fieldsB := strings.Fields(b.Intel)

	// Skip the mnemonic.
	fieldsA = fieldsA[1:]
	fieldsB = fieldsB[1:]

	// Some instructions have an implicit final argument,
	// which is not actually encoded.
	//
	// BLENDVPD xmm1, xmm2/m128, <XMM0>.
	// BLENDVPS xmm1, xmm2/m128, <XMM0>.
	// PBLENDVB xmm1, xmm2/m128, <XMM0>.
	if a.Inst.Mnemonic == b.Inst.Mnemonic &&
		(a.Inst.Mnemonic == "blendvpd" || a.Inst.Mnemonic == "blendvps" || a.Inst.Mnemonic == "pblendvb") {
		// Make a copy, in case we modify them.
		aFields := append([]string(nil), fieldsA...)
		bFields := append([]string(nil), fieldsB...)
		if len(argsA) == 3 {
			aFields = aFields[:len(aFields)-1]
			aFields[len(aFields)-1] = noComma(aFields[len(aFields)-1])
		}

		if len(argsB) == 3 {
			bFields = bFields[:len(bFields)-1]
			bFields[len(bFields)-1] = noComma(bFields[len(bFields)-1])
		}

		if equalStringSetsOrMemories(aFields, bFields) {
			return true
		}
	}

	// Normalise equivalent memory addresses,
	// such as those with an implicit
	// and explicit zero displacement.
	if len(fieldsA) == len(fieldsB) {
		for i := range fieldsA {
			fieldA := fieldsA[i]
			fieldB := fieldsB[i]
			if (!strings.HasSuffix(fieldA, "]") && !strings.HasSuffix(fieldA, "],")) ||
				(!strings.HasSuffix(fieldB, "]") && !strings.HasSuffix(fieldB, "],")) {
				continue
			}

			// Check the addresses are the
			// same and then that all other
			// fields are identical.
			if CanonicalIntelMemory(noComma(fieldA)) == CanonicalIntelMemory(noComma(fieldB)) &&
				equalStringSets(fieldsA[:i], fieldsB[:i]) &&
				equalStringSets(fieldsA[i+1:], fieldsB[i+1:]) {
				return true
			}

			break
		}
	}

	// Finally, we normalise string
	// operations, which often have
	// subtle variations.
	canonA, okA := CanonicaliseStringOperation(a, true)
	canonB, okB := CanonicaliseStringOperation(b, true)
	if okA && okB && canonA == canonB {
		return true
	}

	return false
}

// CanonicalIntelMemory takes an Intel
// memory reference and returns its
// canonical form, removing any zero
// displacement and any one scale.
func CanonicalIntelMemory(mem string) string {
	// Check it is actually an address.
	if i, j := strings.IndexByte(mem, '['), strings.IndexByte(mem, ']'); i < 0 || j < 0 || j < i {
		return mem
	}

	const zero = "+0"
	i := strings.Index(mem, zero)
	if i > 0 {
		// Check that this is not the start
		// of a longer number.
		before := mem[:i]
		after := mem[i+len(zero):]
		if after == "x0]" && before != "[" {
			after = "]"
		}
		if after == "]" && before != "[" {
			mem = before + after
		}
	}

	const one = "*1"
	i = strings.Index(mem, one)
	if i > 0 {
		// Check that this is not the start
		// of a longer number.
		before := mem[:i]
		after := mem[i+len(one):]
		if after == "]" || strings.HasPrefix(after, "+") || strings.HasPrefix(after, "-") {
			mem = before + after
		}
	}

	return mem
}

// CanonicaliseRegister takes the name
// of a general-purpose register and
// returns a canonical name, according
// to the following table:
//
// | Canonical | Constituents          |
// +-----------+-----------------------+
// | A         | al, ax, eax, rax      |
// | C         | cl, cx, ecx, rcx      |
// | D         | dl, dx, edx, rdx      |
// | B         | bl, bx, ebx, rbx      |
// | BP        | bpl, bp, ebp, rbp     |
// | SP        | spl, sp, esp, rsp     |
// | IP        | ip, eip, rip          |
// | DI        | dil, di, edi, rdi     |
// | SI        | sil, si, esi, rsi     |
// | R8        | r8l, r8w, r8d, r8     |
// | R9        | r9l, r9w, r9d, r9     |
// | R10       | r10l, r10w, r10d, r10 |
// | R11       | r11l, r11w, r11d, r11 |
// | R12       | r12l, r12w, r12d, r12 |
// | R13       | r13l, r13w, r13d, r13 |
// | R14       | r14l, r14w, r14d, r14 |
// | R15       | r15l, r15w, r15d, r15 |
//
// Note that the high 8-bit registers,
// such as "ah" are not included, as
// their behaviour is different from
// other general purpose register
// variants.
//
// If the given register is not listed
// above, CanonicaliseRegister returns
// `("", false)`.
func CanonicaliseRegister(reg string) (canonical string, ok bool) {
	ok = true
	switch strings.ToLower(noComma(reg)) {
	case "al", "ax", "eax", "rax":
		canonical = "A"
	case "cl", "cx", "ecx", "rcx":
		canonical = "C"
	case "dl", "dx", "edx", "rdx":
		canonical = "D"
	case "bl", "bx", "ebx", "rbx":
		canonical = "B"
	case "bpl", "bp", "ebp", "rbp":
		canonical = "BP"
	case "spl", "sp", "esp", "rsp":
		canonical = "SP"
	case "ip", "eip", "rip":
		canonical = "IP"
	case "dil", "di", "edi", "rdi":
		canonical = "DI"
	case "sil", "si", "esi", "rsi":
		canonical = "SI"
	case "r8l", "r8w", "r8d", "r8":
		canonical = "R8"
	case "r9l", "r9w", "r9d", "r9":
		canonical = "R9"
	case "r10l", "r10w", "r10d", "r10":
		canonical = "R10"
	case "r11l", "r11w", "r11d", "r11":
		canonical = "R11"
	case "r12l", "r12w", "r12d", "r12":
		canonical = "R12"
	case "r13l", "r13w", "r13d", "r13":
		canonical = "R13"
	case "r14l", "r14w", "r14d", "r14":
		canonical = "R14"
	case "r15l", "r15w", "r15d", "r15":
		canonical = "R15"
	default:
		ok = false
	}

	return canonical, ok
}

// CanonicaliseStringOperation takes
// a string operation instruction and
// returns it in the most precise form,
// allowing two instances of the same
// operation (with different levels of
// precision in their description) to
// be compared.
//
// If addSuffix is true, the returned
// mnemonic includes the (otherwise
// redundant) size suffix in the mnemonic.
//
// If entry is not a string operation,
// CanonicaliseStringOperation returns
// `("", false)`.
func CanonicaliseStringOperation(entry *TestEntry, addSuffix bool) (canonical string, ok bool) {
	family := strings.TrimRight(entry.Inst.Mnemonic, "BWDQ")
	if family == "CMPS" && strings.Contains(entry.Inst.Syntax, "xmm") {
		// We don't want to match on the XMM CMPS instruction.
		return "", false
	}

	switch family {
	case "CMPS", "INS", "LODS", "MOVS", "OUTS", "SCAS", "STOS":
	default:
		return "", false
	}

	var acc, size, dst, src string
	switch {
	case strings.HasSuffix(entry.Inst.Mnemonic, "B"), strings.Contains(entry.Intel, " byte ptr"):
		acc = "al"
		size = "byte ptr"
	case strings.HasSuffix(entry.Inst.Mnemonic, "W"), strings.Contains(entry.Intel, " word ptr"):
		acc = "ax"
		size = "word ptr"
	case strings.HasSuffix(entry.Inst.Mnemonic, "D"), strings.Contains(entry.Intel, " dword ptr"):
		acc = "eax"
		size = "dword ptr"
	case strings.HasSuffix(entry.Inst.Mnemonic, "Q"), strings.Contains(entry.Intel, " qword ptr"):
		acc = "rax"
		size = "qword ptr"
	default:
		panic("unexpected mnemonic: " + entry.Inst.Mnemonic)
	}

	// If we have a sized register
	// already, use that. Otherwise,
	// fall back to the mode size.
	if strings.Contains(entry.Intel, "si]") || strings.Contains(entry.Intel, "di]") {
		switch {
		case strings.Contains(entry.Intel, "[si]") || strings.Contains(entry.Intel, "[di]"):
			dst = "es:[di]"
			src = "ds:[si]"
		case strings.Contains(entry.Intel, "[esi]") || strings.Contains(entry.Intel, "[edi]"):
			dst = "es:[edi]"
			src = "ds:[esi]"
		case strings.Contains(entry.Intel, "[rsi]") || strings.Contains(entry.Intel, "[rdi]"):
			dst = "es:[rdi]" // This should not have a segment selector, but objdump prints them.
			src = "ds:[rsi]" // This should not have a segment selector, but objdump prints them.
		}
	} else {
		switch entry.Mode.Int {
		case 16:
			dst = "es:[di]"
			src = "ds:[si]"
		case 32:
			dst = "es:[edi]"
			src = "ds:[esi]"
		case 64:
			dst = "es:[rdi]" // This should not have a segment selector, but objdump prints them.
			src = "ds:[rsi]" // This should not have a segment selector, but objdump prints them.
		}
	}

	var suffix string
	if addSuffix {
		suffix = size[:1]
	}

	switch family {
	case "CMPS":
		canonical = fmt.Sprintf("%s%s %s %s, %s %s", strings.ToLower(family), suffix, size, src, size, dst)
	case "INS":
		canonical = fmt.Sprintf("%s%s %s %s, dx", strings.ToLower(family), suffix, size, dst)
	case "LODS":
		canonical = fmt.Sprintf("%s%s %s, %s %s", strings.ToLower(family), suffix, acc, size, src)
	case "MOVS":
		canonical = fmt.Sprintf("%s%s %s %s, %s %s", strings.ToLower(family), suffix, size, dst, size, src)
	case "OUTS":
		canonical = fmt.Sprintf("%s%s dx, %s %s", strings.ToLower(family), suffix, size, src)
	case "SCAS":
		canonical = fmt.Sprintf("%s%s %s, %s %s", strings.ToLower(family), suffix, acc, size, dst)
	case "STOS":
		canonical = fmt.Sprintf("%s%s %s %s, %s", strings.ToLower(family), suffix, size, dst, acc)
	default:
		panic("unexpected mnemonic: " + entry.Inst.Mnemonic)
	}

	return canonical, true
}

// CanonicalMnemonic returns the
// canonical mnemonic for the given
// mnemonic. For most instructions,
// this returns the input, but there
// are groups of instructions with
// identical meaning. For these
// latter instructions, the canonical
// mnemonic is returned (which may be
// the input).
func CanonicalMnemonic(mode uint8, mnemonic string) string {
	canonical, ok := EquivalentMnemonic[mnemonic]
	if ok {
		return canonical
	}

	group, ok := EquivalentMnemonicForMode[mode]
	if ok {
		canonical, ok = group[mnemonic]
	}

	if ok {
		return canonical
	}

	return mnemonic
}

// equalStringSets returns true if
// a and b are string slices with
// the same lengths and contents,
// in the same order.
func equalStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// equalStringSetsOrMemories returns
// true if a and b are string slices
// with the same lengths and contents,
// in the same order, or where the
// only difference is that both contain
// an Intel memory address at the same
// position but with different levels
// of pedantry, according to CanonicalIntelMemory.
func equalStringSetsOrMemories(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if strings.HasSuffix(a[i], ",") && strings.HasSuffix(b[i], ",") &&
			CanonicalIntelMemory(noComma(a[i])) == CanonicalIntelMemory(noComma(b[i])) {
			continue
		}

		if CanonicalIntelMemory(a[i]) != CanonicalIntelMemory(b[i]) {
			return false
		}
	}

	return true
}

// equivalentGroups are sets of x86
// instructions that share both
// encoding and meaning, such as
// jl and jnge. The first entry in
// each group is the canonical
// mnemonic.
var equivalentGroups = [][]string{
	{"CMOVB", "CMOVC", "CMOVNAE"},
	{"CMOVAE", "CMOVNB", "CMOVNC"},
	{"CMOVE", "CMOVZ"},
	{"CMOVNE", "CMOVNZ"},
	{"CMOVBE", "CMOVNA"},
	{"CMOVA", "CMOVNBE"},
	{"CMOVP", "CMOVPE"},
	{"CMOVNP", "CMOVPO"},
	{"CMOVL", "CMOVNGE"},
	{"CMOVGE", "CMOVNL"},
	{"CMOVLE", "CMOVNG"},
	{"CMOVG", "CMOVNLE"},
	{"JB", "JC", "JNAE"},
	{"JAE", "JNB", "JNC"},
	{"JE", "JZ"},
	{"JNE", "JNZ"},
	{"JBE", "JNA"},
	{"JA", "JNBE"},
	{"JP", "JPE"},
	{"JNP", "JPO"},
	{"JL", "JNGE"},
	{"JGE", "JNL"},
	{"JLE", "JNG"},
	{"JG", "JNLE"},
	{"JECXZ", "JCXZ", "JRCXZ"},
	{"SHL", "SAL"},
	{"SETB", "SETNAE", "SETC"},
	{"SETAE", "SETNB", "SETNC"},
	{"SETE", "SETZ"},
	{"SETNE", "SETNZ"},
	{"SETBE", "SETNA"},
	{"SETA", "SETNBE"},
	{"SETP", "SETPE"},
	{"SETNP", "SETPO"},
	{"SETL", "SETNGE"},
	{"SETGE", "SETNL"},
	{"SETLE", "SETNG"},
	{"SETG", "SETNLE"},
	{"WAIT", "FWAIT"},
}

// equivalentGroupsForMode are a set of
// x86 instructions that share an
// encoding and meaning in a particular
// CPU mode, such as pop and popw in
// 16-bit mode. The first entry in
// each group is the canonical
// mnemonic.
var equivalentGroupsForMode = map[uint8][][]string{
	16: {
		{"POP", "POPW"},
		{"PUSH", "PUSHW"},
	},
	32: {
		{"POP", "POPD"},
		{"PUSH", "PUSHD"},
	},
}

var (
	EquivalentMnemonic        = make(map[string]string)
	EquivalentMnemonicForMode = make(map[uint8]map[string]string)
)

func init() {
	for _, group := range equivalentGroups {
		for _, mnemonic := range group {
			EquivalentMnemonic[mnemonic] = group[0]
		}
	}

	for mode, groups := range equivalentGroupsForMode {
		inner := make(map[string]string)
		for _, group := range groups {
			for _, mnemonic := range group {
				inner[mnemonic] = group[0]
			}
		}

		EquivalentMnemonicForMode[mode] = inner
	}
}

// IsSpecialism returns whether
// `special` is a more specific
// form of `general`.
func IsSpecialisation(special, general *TestEntry) bool {
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
		finalArg := generalArgs[len(generalArgs)-1]
		for _, suffix := range option.Suffixes {
			if finalArg == suffix {
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
		specialArgs := special.IntelArgs()
		generalArgs = generalArgs[:len(generalArgs)-1]

		return equalStringSetsOrMemories(specialArgs, generalArgs)
	}

	// No matching specialisation found.
	return false
}

// specialisation represents a
// version of an x86 instruction
// that is a variant of a more
// general instruction.
//
// For example, `CMPEQPD` is a
// specialisation of `CMPPD`,
// where the final immediate
// argument is zero.
type specialisation struct {
	Mnemonic string
	Suffixes []string
}

// specialisations maps general
// instruction mnemonics to their
// more specialised forms.
var specialisations = map[string][]specialisation{
	"CMPPD": {
		{Mnemonic: "CMPEQPD", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "CMPLTPD", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "CMPLEPD", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "CMPUNORDPD", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "CMPFALSEPD", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "CMPNEQPD", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "CMPNLTPD", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "CMPNLEPD", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "CMPORDPD", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "CMPTRUE_USPD", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"CMPPS": {
		{Mnemonic: "CMPEQPS", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "CMPLTPS", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "CMPLEPS", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "CMPUNORDPS", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "CMPFALSEPS", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "CMPNEQPS", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "CMPNLTPS", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "CMPNLEPS", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "CMPORDPS", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "CMPTRUE_USPS", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"CMPSD": {
		{Mnemonic: "CMPEQSD", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "CMPLTSD", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "CMPLESD", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "CMPUNORDSD", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "CMPFALSESD", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "CMPNEQSD", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "CMPNLTSD", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "CMPNLESD", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "CMPORDSD", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "CMPTRUE_USSD", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"CMPSS": {
		{Mnemonic: "CMPEQSS", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "CMPLTSS", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "CMPLESS", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "CMPUNORDSS", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "CMPFALSESS", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "CMPNEQSS", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "CMPNLTSS", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "CMPNLESS", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "CMPORDSS", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "CMPTRUE_USSS", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"PCLMULQDQ": {
		{Mnemonic: "PCLMULLQLQDQ", Suffixes: []string{"0"}},
		{Mnemonic: "PCLMULHQLQDQ", Suffixes: []string{"1"}},
		{Mnemonic: "PCLMULLQHQDQ", Suffixes: []string{"16"}},
		{Mnemonic: "PCLMULHQHQDQ", Suffixes: []string{"17"}},
	},
	"VCMPPD": {
		{Mnemonic: "VCMPEQPD", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "VCMPLTPD", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "VCMPLEPD", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "VCMPUNORDPD", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "VCMPFALSEPD", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "VCMPNEQPD", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "VCMPNLTPD", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "VCMPNLEPD", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "VCMPORDPD", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "VCMPTRUE_USPD", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"VCMPPS": {
		{Mnemonic: "VCMPEQPS", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "VCMPLTPS", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "VCMPLEPS", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "VCMPUNORDPS", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "VCMPFALSEPS", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "VCMPNEQPS", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "VCMPNLTPS", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "VCMPNLEPS", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "VCMPORDPS", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "VCMPTRUE_USPS", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"VCMPSD": {
		{Mnemonic: "VCMPEQSD", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "VCMPLTSD", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "VCMPLESD", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "VCMPUNORDSD", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "VCMPFALSESD", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "VCMPNEQSD", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "VCMPNLTSD", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "VCMPNLESD", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "VCMPORDSD", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "VCMPTRUE_USSD", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"VCMPSS": {
		{Mnemonic: "VCMPEQSS", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "VCMPLTSS", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "VCMPLESS", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "VCMPUNORDSS", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "VCMPFALSESS", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "VCMPNEQSS", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "VCMPNLTSS", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "VCMPNLESS", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "VCMPORDSS", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "VCMPTRUE_USSS", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"VPCLMULQDQ": {
		{Mnemonic: "VPCLMULLQLQDQ", Suffixes: []string{"0"}},
		{Mnemonic: "VPCLMULHQLQDQ", Suffixes: []string{"1"}},
		{Mnemonic: "VPCLMULLQHQDQ", Suffixes: []string{"16"}},
		{Mnemonic: "VPCLMULHQHQDQ", Suffixes: []string{"17"}},
	},
}
