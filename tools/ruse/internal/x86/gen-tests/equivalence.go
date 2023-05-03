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
	if a.Inst.Mnemonic == "xchg" && b.Inst.Mnemonic == "xchg" &&
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
		case "fadd", "fdiv", "fsub":
			if len(argsA) == 2 && len(argsB) == 2 &&
				argsA[0] == argsB[1] &&
				argsA[1] == argsB[0] {
				return true
			}
		case "faddp", "fdivp", "fdivrp", "fmulp", "fsubp", "fsubrp":
			if (len(argsA) == 0 || (len(argsA) == 2 && argsA[0] == "st(1)" && argsA[1] == "st")) &&
				(len(argsB) == 0 || (len(argsB) == 2 && argsB[0] == "st(1)" && argsB[1] == "st")) {
				return true
			}
		case "fcom", "fcomp", "fucom", "fucomp", "fxch":
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
	if a.Inst.Mnemonic == "mov" && b.Inst.Mnemonic == "mov" &&
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
	if a.Inst.Mnemonic == "mov" && b.Inst.Mnemonic == "mov" &&
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
	family := strings.TrimRight(entry.Inst.Mnemonic, "bwdq")
	if family == "cmps" && strings.Contains(entry.Inst.Syntax, "xmm") {
		// We don't want to match on the XMM CMPS instruction.
		return "", false
	}

	switch family {
	case "cmps", "ins", "lods", "movs", "outs", "scas", "stos":
	default:
		return "", false
	}

	var acc, size, dst, src string
	switch {
	case strings.HasSuffix(entry.Inst.Mnemonic, "b"), strings.Contains(entry.Intel, " byte ptr"):
		acc = "al"
		size = "byte ptr"
	case strings.HasSuffix(entry.Inst.Mnemonic, "w"), strings.Contains(entry.Intel, " word ptr"):
		acc = "ax"
		size = "word ptr"
	case strings.HasSuffix(entry.Inst.Mnemonic, "d"), strings.Contains(entry.Intel, " dword ptr"):
		acc = "eax"
		size = "dword ptr"
	case strings.HasSuffix(entry.Inst.Mnemonic, "q"), strings.Contains(entry.Intel, " qword ptr"):
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
	case "cmps":
		canonical = fmt.Sprintf("cmps%s %s %s, %s %s", suffix, size, src, size, dst)
	case "ins":
		canonical = fmt.Sprintf("ins%s %s %s, dx", suffix, size, dst)
	case "lods":
		canonical = fmt.Sprintf("lods%s %s, %s %s", suffix, acc, size, src)
	case "movs":
		canonical = fmt.Sprintf("movs%s %s %s, %s %s", suffix, size, dst, size, src)
	case "outs":
		canonical = fmt.Sprintf("outs%s dx, %s %s", suffix, size, src)
	case "scas":
		canonical = fmt.Sprintf("scas%s %s, %s %s", suffix, acc, size, dst)
	case "stos":
		canonical = fmt.Sprintf("stos%s %s %s, %s", suffix, size, dst, acc)
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
	{"cmovb", "cmovc", "cmovnae"},
	{"cmovae", "cmovnb", "cmovnc"},
	{"cmove", "cmovz"},
	{"cmovne", "cmovnz"},
	{"cmovbe", "cmovna"},
	{"cmova", "cmovnbe"},
	{"cmovp", "cmovpe"},
	{"cmovnp", "cmovpo"},
	{"cmovl", "cmovnge"},
	{"cmovge", "cmovnl"},
	{"cmovle", "cmovng"},
	{"cmovg", "cmovnle"},
	{"jb", "jc", "jnae"},
	{"jae", "jnb", "jnc"},
	{"je", "jz"},
	{"jne", "jnz"},
	{"jbe", "jna"},
	{"ja", "jnbe"},
	{"jp", "jpe"},
	{"jnp", "jpo"},
	{"jl", "jnge"},
	{"jge", "jnl"},
	{"jle", "jng"},
	{"jg", "jnle"},
	{"jecxz", "jcxz", "jrcxz"},
	{"shl", "sal"},
	{"setb", "setnae", "setc"},
	{"setae", "setnb", "setnc"},
	{"sete", "setz"},
	{"setne", "setnz"},
	{"setbe", "setna"},
	{"seta", "setnbe"},
	{"setp", "setpe"},
	{"setnp", "setpo"},
	{"setl", "setnge"},
	{"setge", "setnl"},
	{"setle", "setng"},
	{"setg", "setnle"},
	{"wait", "fwait"},
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
		{"pop", "popw"},
		{"push", "pushw"},
	},
	32: {
		{"pop", "popd"},
		{"push", "pushd"},
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
// For example, `cmpeqpd` is a
// specialisation of `cmppd`,
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
	"cmppd": {
		{Mnemonic: "cmpeqpd", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "cmpltpd", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "cmplepd", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "cmpunordpd", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "cmpfalsepd", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "cmpneqpd", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "cmpnltpd", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "cmpnlepd", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "cmpordpd", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "cmptrue_uspd", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"cmpps": {
		{Mnemonic: "cmpeqps", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "cmpltps", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "cmpleps", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "cmpunordps", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "cmpfalseps", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "cmpneqps", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "cmpnltps", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "cmpnleps", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "cmpordps", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "cmptrue_usps", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"cmpsd": {
		{Mnemonic: "cmpeqsd", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "cmpltsd", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "cmplesd", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "cmpunordsd", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "cmpfalsesd", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "cmpneqsd", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "cmpnltsd", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "cmpnlesd", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "cmpordsd", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "cmptrue_ussd", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"cmpss": {
		{Mnemonic: "cmpeqss", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "cmpltss", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "cmpless", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "cmpunordss", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "cmpfalsess", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "cmpneqss", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "cmpnltss", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "cmpnless", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "cmpordss", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "cmptrue_usss", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"pclmulqdq": {
		{Mnemonic: "pclmullqlqdq", Suffixes: []string{"0"}},
		{Mnemonic: "pclmulhqlqdq", Suffixes: []string{"1"}},
		{Mnemonic: "pclmullqhqdq", Suffixes: []string{"16"}},
		{Mnemonic: "pclmulhqhqdq", Suffixes: []string{"17"}},
	},
	"vcmppd": {
		{Mnemonic: "vcmpeqpd", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "vcmpltpd", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "vcmplepd", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "vcmpunordpd", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "vcmpfalsepd", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "vcmpneqpd", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "vcmpnltpd", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "vcmpnlepd", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "vcmpordpd", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "vcmptrue_uspd", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"vcmpps": {
		{Mnemonic: "vcmpeqps", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "vcmpltps", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "vcmpleps", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "vcmpunordps", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "vcmpfalseps", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "vcmpneqps", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "vcmpnltps", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "vcmpnleps", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "vcmpordps", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "vcmptrue_usps", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"vcmpsd": {
		{Mnemonic: "vcmpeqsd", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "vcmpltsd", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "vcmplesd", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "vcmpunordsd", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "vcmpfalsesd", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "vcmpneqsd", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "vcmpnltsd", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "vcmpnlesd", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "vcmpordsd", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "vcmptrue_ussd", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"vcmpss": {
		{Mnemonic: "vcmpeqss", Suffixes: []string{"0", "8", "16", "24"}},
		{Mnemonic: "vcmpltss", Suffixes: []string{"1", "9", "17", "25"}},
		{Mnemonic: "vcmpless", Suffixes: []string{"2", "10", "18", "26"}},
		{Mnemonic: "vcmpunordss", Suffixes: []string{"3", "11", "19", "27"}},
		{Mnemonic: "vcmpfalsess", Suffixes: []string{"3", "11", "19", "27"}}, // Objdump alternative.
		{Mnemonic: "vcmpneqss", Suffixes: []string{"4", "12", "20", "28"}},
		{Mnemonic: "vcmpnltss", Suffixes: []string{"5", "13", "21", "29"}},
		{Mnemonic: "vcmpnless", Suffixes: []string{"6", "14", "22", "30"}},
		{Mnemonic: "vcmpordss", Suffixes: []string{"7", "15", "23", "31"}},
		{Mnemonic: "vcmptrue_usss", Suffixes: []string{"7", "15", "23", "31"}}, // Objdump alternative.
	},
	"vpclmulqdq": {
		{Mnemonic: "vpclmullqlqdq", Suffixes: []string{"0"}},
		{Mnemonic: "vpclmulhqlqdq", Suffixes: []string{"1"}},
		{Mnemonic: "vpclmullqhqdq", Suffixes: []string{"16"}},
		{Mnemonic: "vpclmulhqhqdq", Suffixes: []string{"17"}},
	},
}
