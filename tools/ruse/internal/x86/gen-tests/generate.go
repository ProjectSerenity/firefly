// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Generate an extensive set of instructions
// from a set of register definitions.

package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"firefly-os.dev/tools/ruse/internal/x86"
)

func GenerateTestEntries(insts []*x86.Instruction) ([]*TestEntry, error) {
	var entries []*TestEntry
	var ruseOptionsSet, intelOptionsSet [][]string
	for _, inst := range insts {
		mnemonic := inst.Mnemonic
		params := inst.Parameters

		// Skip vextractps, as clang always emits
		// an EVEX encoding, even when the VEX
		// form is valid and shorter.
		if mnemonic == "vextractps" {
			continue
		}

		// Clang always uses the EVEX form, even
		// if it's longer.
		switch mnemonic {
		case "vpdpbusd", "vpdpbusds",
			"vpdpwssd", "vpdpwssds":
			if inst.Encoding.VEX {
				continue
			}
		}

		// Skip XBEGIN for now, as its behaviour
		// is odd.
		if mnemonic == "xbegin" {
			continue
		}

		// These instructions are not yet widely
		// supported.
		switch inst.Mnemonic {
		case "aesdec128kl", "aesdec256kl",
			"aesdecwide128kl", "aesdecwide256kl",
			"aesenc128kl", "aesenc256kl",
			"aesencwide128kl", "aesencwide256kl",
			"clrssbsy",
			"clui",
			"encodekey128", "encodekey256",
			"hreset",
			"loadiwkey",
			"rstorssp",
			"senduipi",
			"stui",
			"testui",
			"ud0",
			"uiret",
			"umonitor":
			continue
		}

		// These instructions are a pain to
		// make test vectors for, because the
		// register size is ignored.
		switch inst.Mnemonic {
		case "enqcmd", "enqcmds",
			"movdir64b":
			continue
		}

		intelMnemonic := mnemonic
		switch intelMnemonic {
		case "call-far":
			intelMnemonic = "lcall"
		case "jmp-far":
			intelMnemonic = "ljmp"
		case "ret-far":
			intelMnemonic = "retf"
		case "cmpsb", "cmpsw", "cmpsd", "cmpsq":
			if len(params) != 0 && !strings.Contains(inst.Syntax, "xmm") { // Don't mess up the unrelated XMM instruction CMPSD.
				intelMnemonic = "cmps"
			}
		case "insb", "insw", "insd", "insq":
			if len(params) != 0 {
				intelMnemonic = "ins"
			}
		case "lodsb", "lodsw", "lodsd", "lodsq":
			if len(params) != 0 {
				intelMnemonic = "lods"
			}
		case "movsb", "movsw", "movsd", "movsq":
			if len(params) != 0 && !strings.Contains(inst.Syntax, "xmm") { // Don't mess up the unrelated XMM instruction MOVSD.
				intelMnemonic = "movs"
			}
		case "outsb", "outsw", "outsd", "outsq":
			if len(params) != 0 {
				intelMnemonic = "outs"
			}
		case "scasb", "scasw", "scasd", "scasq":
			if len(params) != 0 {
				intelMnemonic = "scas"
			}
		case "stosb", "stosw", "stosd", "stosq":
			if len(params) != 0 {
				intelMnemonic = "stos"
			}
		case "pushw", "pushd":
			if len(params) == 1 && (params[0].Syntax == "imm16" || params[0].Syntax == "imm32") {
				intelMnemonic = "push"
			}
		case "vmovapd", "vmovaps",
			"vmovd", "vmovq",
			"vmovddup",
			"vmovdqa", "vmovdqa32", "vmovdqa64",
			"vmovdqu", "vmovdqu8", "vmovdqu16", "vmovdqu32", "vmovdqu64",
			"vmovsd", "vmovss",
			"vmovupd", "vmovups",
			"vpextrw":
			// If we have no memory address,
			// this is ambiguous with the other
			// form.
			mem := false
			regs := 0
			for _, param := range params {
				if param.Type == x86.TypeMemory {
					mem = true
				}
				if param.Type == x86.TypeRegister && param.Encoding != x86.EncodingNone {
					regs++
				}
			}

			if !mem && regs >= 2 {
				continue
			}
		}

		// Broadcast memory is a pain to handle,
		// as the Ruse syntax is very different
		// (an annotation on the instruction as
		// a whole, rather than a suffix on the
		// memory address), so we skip them for
		// now.
		broadcast := false
		for _, param := range params {
			if param == x86.ParamM16bcst || param == x86.ParamM32bcst || param == x86.ParamM64bcst {
				broadcast = true
				break
			}
		}

		if broadcast {
			continue
		}

		// Clang doesn't seem to encode mask
		// registers correctly, so we skip them
		// for now.
		masks := false
		for _, param := range params {
			if param == x86.ParamK1 {
				masks = true
				break
			}
		}

		if masks {
			continue
		}

		// Clang expects a 32-bit source register,
		// but an 8/16-bit register makes more
		// sense, so we skip these variants.
		switch inst.Mnemonic {
		case "vpbroadcastb", "vpbroadcastw":
			continue
		}

		// Clang insists on a different approach
		// to size hints than the Intel manual,
		// so we just skip it for now.
		switch inst.Mnemonic {
		case "vpmovqd", "vpmovsqd", "vpmovusqd":
			if inst.Parameters[0].Type == x86.TypeMemory {
				continue
			}
		}

		// We don't support SIB yet.
		if inst.Encoding.SIB {
			continue
		}

		// Clang doesn't seem to support
		// AVX512-FP16 instructions yet.
		if inst.HasCPUID("AVX512-FP16") || inst.HasCPUID("AVX512-FP16 AVX512VL") {
			continue
		}

		for len(ruseOptionsSet) < len(params) {
			ruseOptionsSet = append(ruseOptionsSet, make([]string, 0, 10))
			intelOptionsSet = append(intelOptionsSet, make([]string, 0, 10))
		}

		ruseOptionsSet = ruseOptionsSet[:len(params)]
		intelOptionsSet = intelOptionsSet[:len(params)]
		for _, mode := range x86.Modes {
			switch mode.Int {
			case 16:
				if !inst.Mode16 {
					continue
				}
			case 32:
				if !inst.Mode32 {
					continue
				}
			case 64:
				if !inst.Mode64 {
					continue
				}

				switch inst {
				// These forms are not recommended, as
				// MOV would be simpler.
				case x86.MOVSXD_R16_Rmr16, x86.MOVSXD_R16_M16:
					continue
				}
			}

			// We don't generate EVEX instructions
			// in 32-bit mode, as a good assembler
			// will always use VEX alternatives, as
			// they're shorter.
			if mode.Int == 32 && inst.Encoding.EVEX {
				continue
			}

			for i, param := range params {
				var err error
				ruseOptionsSet[i], intelOptionsSet[i], err = syntaxToOptions(inst, ruseOptionsSet[i][:0], intelOptionsSet[i][:0], mode.Int, param.Syntax)
				if err != nil {
					return nil, fmt.Errorf("failed to generate tests for %s (%s): %v", inst.Syntax, inst.UID, err)
				}

				if len(ruseOptionsSet[i]) == 0 {
					// This happens sometimes for 16-bit mode,
					// with SSE instructions, which are valid
					// for 32-bit mode but not 16-bit mode.
					return nil, fmt.Errorf("mode %d: got no options for %q in %s / %s (%s)", mode.Int, param.Syntax, inst.Syntax, inst.Encoding.Syntax, inst.UID)
				}
			}

			// Multiply the form across all of the arg
			// combinations.
			ruse := []string{mnemonic}
			intel := []string{intelMnemonic}
			ruseOptions := ruseOptionsSet // Make a copy so the originals keep their offset.
			intelOptions := intelOptionsSet
			ruseJoin := " "
			intelJoin := " "
			for len(ruseOptions) > 0 {
				rSet := ruseOptions[0]
				iSet := intelOptions[0]
				ruseOptions = ruseOptions[1:]
				intelOptions = intelOptions[1:]
				nextRuse := make([]string, len(ruse)*len(rSet))
				nextIntel := make([]string, len(intel)*len(rSet))

				for i := range rSet {
					for j := range intel {
						nextRuse[i*len(ruse)+j] = ruse[j] + ruseJoin + rSet[i]
						nextIntel[i*len(intel)+j] = intel[j] + intelJoin + iSet[i]
					}
				}

				ruse = nextRuse
				intel = nextIntel
				intelJoin = ", "
			}

			for i := range ruse {
				if mode.Int == 64 {
					// We can't use high legacy registers
					// and new registers together.
					var newRegs, highOldRegs, evexRegs bool
					for _, arg := range strings.Fields(intel[i]) {
						arg = noComma(arg)
						if strings.HasPrefix(arg, "r") && x86.RegisterSizes[arg] == 64 {
							newRegs = true
							continue
						}

						switch arg {
						case "spl", "bpl", "sil", "dil",
							"r8b", "r9b", "r10b", "r11b", "r12b", "r13b", "r14b", "r15b",
							"r8w", "r9w", "r10w", "r11w", "r12w", "r13w", "r14w", "r15w",
							"r8d", "r9d", "r10d", "r11d", "r12d", "r13d", "r14d", "r15d":
							newRegs = true
						case "ah", "ch", "dh", "bh":
							highOldRegs = true
						case "xmm16", "xmm17", "xmm18", "xmm19", "xmm20", "xmm21", "xmm22", "xmm23",
							"xmm24", "xmm25", "xmm26", "xmm27", "xmm28", "xmm29", "xmm30", "xmm31",
							"ymm16", "ymm17", "ymm18", "ymm19", "ymm20", "ymm21", "ymm22", "ymm23",
							"ymm24", "ymm25", "ymm26", "ymm27", "ymm28", "ymm29", "ymm30", "ymm31",
							"zmm0", "zmm1", "zmm2", "zmm3", "zmm4", "zmm5", "zmm6", "zmm7",
							"zmm8", "zmm9", "zmm10", "zmm11", "zmm12", "zmm13", "zmm14", "zmm15",
							"zmm16", "zmm17", "zmm18", "zmm19", "zmm20", "zmm21", "zmm22", "zmm23",
							"zmm24", "zmm25", "zmm26", "zmm27", "zmm28", "zmm29", "zmm30", "zmm31":
							evexRegs = true
						}
					}

					if highOldRegs && (newRegs || inst.Encoding.REX) {
						continue
					}

					if inst.Encoding.EVEX && !evexRegs {
						// If we aren't using any extended
						// registers, Clang will use a VEX
						// encoding instead, as it's shorter.
						continue
					}
				}

				var code string
				switch mnemonic {
				// This is not widely supported and its
				// encoding is trivial.
				case "clui":
					code = "f30f01ee"
				// String operations must have
				// matching parameter sizes.
				case "cmps", "cmpsb", "cmpsw", "cmpsd", "cmpsq",
					"movs", "movsb", "movsw", "movsd", "movsq":
					if strings.Contains(intel[i], "[edi]") && !strings.Contains(intel[i], "[esi]") ||
						strings.Contains(intel[i], "[di]") && !strings.Contains(intel[i], "[si]") {
						// Skip this one.
						continue
					}
				case "fwait", "wait":
					// Objdump seems to merge this into
					// any subsequent fxam instruction,
					// which is next alphabetically.
					// Since its encoding is trivial,
					// we just hard-code it here.
					code = "9b"
				case "call", "jmp":
					// We are at risk of linker errors with
					// memory references consisting only
					// of a displacement, if that displacement
					// falls outside the bounds of the binary's
					// address space.
					j := strings.LastIndexByte(intel[i], ' ')
					if i < 0 || len(inst.Parameters) == 1 && inst.Parameters[0].Type == x86.TypeRelativeAddress {
						break
					}

					maybeNum := strings.TrimSuffix(strings.TrimPrefix(intel[i][j+1:], "["), "]")
					_, errInt := strconv.ParseInt(maybeNum, 0, 64)
					_, errUint := strconv.ParseUint(maybeNum, 0, 64)
					if errInt == nil || errUint == nil {
						// Just a displacement, so skip for now.
						continue
					}
				case "call-far", "jmp-far":
					// The small address literals we generate
					// for 16-bit far jumps will never be
					// encoded as 16-bit jumps by Clang,
					// so we skip them here.
					if mode.Int != 16 && (inst == x86.CALL_FAR_Ptr16v16 || inst == x86.JMP_FAR_Ptr16v16) {
						continue
					}

					// Similarly, Clang doesn't seem to like
					// 32-bit jumps (but not calls?) in 16-bit
					// mode.
					if mode.Int == 16 && inst == x86.JMP_FAR_Ptr16v32 {
						continue
					}

					// Clang doesn't like size hints for
					// indirect absolute addresses. Rather
					// than try and work around it, we just
					// skip non-native address sizes for
					// now.
					if inst.Parameters[0].Type == x86.TypeMemory && inst.Parameters[0].Bits-16 != int(mode.Int) {
						continue
					}
				case "pop", "popw", "popd", "popq":
					// Clang and objdump both have slightly
					// odd behaviour with popping a segment
					// register, so we just hard-code them
					// here.
					if (mnemonic == "popw" && mode.Int != 16) ||
						(mnemonic == "popd" && mode.Int == 16) {
						code = "66" // Operand size override prefix.
					} else if mnemonic == "popq" {
						code = "48" // REX.W
					}

					if inst.Mnemonic == "pop" && len(inst.Parameters) == 1 && inst.Parameters[0].Type == x86.TypeRegister && inst.Parameters[0].Encoding == x86.EncodingModRMrm {
						// This is ambiguous with the older
						// POP reg form.
						continue
					}

					switch _, operand, _ := strings.Cut(intel[i], " "); operand {
					case "es":
						code += "07"
					case "ss":
						code += "17"
					case "ds":
						code += "1f"
					case "fs":
						code += "0fa1"
					case "gs":
						code += "0fa9"
					default:
						code = "" // Fall back to test cases.
					}
				case "popa", "popf", "pusha", "pushf",
					"popad", "popfd", "pushad", "pushfd",
					"popfq", "pushfq":
					// Objdump always gives the short name
					// for the word and double word versions
					// of these instructions. Since their
					// encodings are so simple, we just
					// hard-code them here.
					if strings.HasSuffix(mnemonic, "q") {
						code = "48" // REX.W
					} else if (mode.Int == 16 && strings.HasSuffix(mnemonic, "d")) ||
						(mode.Int == 32 && !strings.HasSuffix(mnemonic, "d")) ||
						(mode.Int == 64 && strings.HasSuffix(mnemonic, "f")) {
						code = "66" // Operand size override.
					}

					code += strings.ToLower(inst.Encoding.Syntax)
				case "push", "pushw", "pushd", "pushq":
					// Clang and objdump both have slightly
					// odd behaviour with pushing a segment
					// register, so we just hard-code them
					// here.
					if mnemonic == "pushq" {
						code = "48" // REX.W
					} else if (mnemonic == "pushw" && mode.Int != 16) ||
						(mnemonic == "pushd" && mode.Int == 16) {
						code = "66" // Operand size override prefix.
					}

					if inst.Mnemonic == "push" && len(inst.Parameters) == 1 && inst.Parameters[0].Type == x86.TypeRegister && inst.Parameters[0].Encoding == x86.EncodingModRMrm {
						// This is ambiguous with the older
						// PUSH reg form.
						continue
					}

					switch _, operand, _ := strings.Cut(intel[i], " "); operand {
					case "es":
						code += "06"
					case "cs":
						code += "0e"
					case "ss":
						code += "16"
					case "ds":
						code += "1e"
					case "fs":
						code += "0fa0"
					case "gs":
						code += "0fa8"
					default:
						code = "" // Fall back to test cases.
					}
				// This is not widely supported and its
				// encoding is trivial.
				case "stui":
					code = "f30f01ef"
				case "sysexit":
					if inst.Encoding.REX_W {
						code = "480f35" // This is easier than forcing the REX.W prefix through Clang.
					}
				case "sysret":
					if inst.Encoding.REX_W {
						code = "480f07" // This is easier than forcing the REX.W prefix through Clang.
					}
				// This is not widely supported and its
				// encoding is trivial.
				case "testui":
					code = "f30f01ed"
				// This is not widely supported and its
				// encoding is trivial.
				case "uiret":
					code = "f30f01ec"
				case "xlatb":
					if inst.Encoding.REX_W {
						code = "48d7" // This is easier than forcing the REX.W prefix through Clang.
					}
				}

				var err error
				addOperandOverrideIfMode := func(wantMode uint8) {
					if mode.Int == wantMode || mode.Int == 64 && wantMode == 32 {
						code += "66" // Operand size override prefix.
					}
				}

				addSignedImmediate := func(field, bits int, subtract int64) {
					parts := strings.Fields(intel[i])
					s := noComma(parts[field])
					s = strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
					v, e := strconv.ParseInt(s, 0, bits)
					if e != nil {
						err = fmt.Errorf("failed to parse %q operand %q: %v", inst.Syntax, s, e)
						return
					}

					v -= subtract

					switch bits {
					case 8:
						code += hex.EncodeToString([]byte{uint8(int8(v))})
					case 16:
						code += hex.EncodeToString(binary.LittleEndian.AppendUint16(nil, uint16(int16(v))))
					case 32:
						code += hex.EncodeToString(binary.LittleEndian.AppendUint32(nil, uint32(int32(v))))
					case 64:
						code += hex.EncodeToString(binary.LittleEndian.AppendUint64(nil, uint64(v)))
					default:
						panic(bits)
					}
				}

				addUnsignedImmediate := func(field, bits int) {
					parts := strings.Fields(intel[i])
					s := noComma(parts[field])
					s = strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
					v, e := strconv.ParseUint(s, 0, bits)
					if e != nil {
						err = fmt.Errorf("failed to parse %q operand %q: %v", inst.Syntax, s, e)
						return
					}

					switch bits {
					case 8:
						code += hex.EncodeToString([]byte{uint8(v)})
					case 16:
						code += hex.EncodeToString(binary.LittleEndian.AppendUint16(nil, uint16(v)))
					case 32:
						code += hex.EncodeToString(binary.LittleEndian.AppendUint32(nil, uint32(v)))
					case 64:
						code += hex.EncodeToString(binary.LittleEndian.AppendUint64(nil, v))
					default:
						panic(bits)
					}
				}

				// For some reason, Clang always inserts an
				// operand size override prefix to RET in
				// 16-bit mode. I can't see why this would
				// be correct, so we hard-code here (and in
				// simpleUnsignedInstructions below).
				switch inst.Syntax {
				case "RET":
					code = "c3"
				case "RET-FAR":
					code = "cb"
				}

				// These instruction forms are ambiguous
				// with an older version that only supports
				// registers.
				switch inst.Syntax {
				case "DEC r/m8", "DEC r/m16", "DEC r/m32", "DEC r/m64",
					"INC r/m8", "INC r/m16", "INC r/m32", "INC r/m64":
					if !strings.Contains(intel[i], "[") && mode.Int != 64 {
						continue
					}
				// These versions are the same but their
				// counterparts are still supported in
				// 64-bit mode.
				case "MOV r/m8, imm8u", "MOV r/m16, imm16", "MOV r/m32, imm32":
					if !strings.Contains(intel[i], "[") {
						continue
					}
				}

				// Thse aren't ambiguous, but fixing VEX
				// prefixes is too tedious for now.
				switch mnemonic {
				case "vmovapd", "vmovaps",
					"vmovdqa", "vmovdqu",
					"vmovq",
					"vmovsd", "vmovss",
					"vmovupd", "vmovups",
					"vpextrw":
					if inst.Encoding.VEX && !strings.Contains(intel[i], "[") {
						continue
					}
				}

				// Some forms overlap with others and
				// are not chosen by clang, meaning that
				// the machine code we get uses the other
				// form. For example, "adc ax, 0x12"
				// matches both of the following syntaxes,
				// with clang choosing the latter:
				//
				// 	ADC AX, imm16
				// 	ADC r/m16, imm8
				//
				// To ensure that the forms match, we
				// hard-code the expected code here.
				// We also skip some ambiguous forms.

				if simple, ok := simpleSignedInstructions[inst.Syntax]; ok {
					if inst.Encoding.REX_W {
						code += "48"
					}
					addOperandOverrideIfMode(simple.OverrideMode)
					code += simple.Opcode
					if len(inst.Parameters) != 1 || inst.Parameters[0].Type != x86.TypeRelativeAddress {
						addSignedImmediate(simple.ArgIndex, simple.ArgSize, 0)
					} else {
						addSignedImmediate(simple.ArgIndex, simple.ArgSize, int64(len(code)/2+simple.ArgSize/8))
					}
				}

				if simple, ok := simpleUnsignedInstructions[inst.Syntax]; ok {
					if inst.Encoding.REX_W {
						code += "48"
					}
					addOperandOverrideIfMode(simple.OverrideMode)
					code += simple.Opcode
					size := simple.ArgSize
					if size == 0 {
						size = int(mode.Int)
					}
					addUnsignedImmediate(simple.ArgIndex, size)
				}

				if ambiguous, ok := ambiguousInstructions[inst.Syntax]; ok {
					// Check whether there is a corresponding form just
					// for the accumulator register. If so, we skip this
					// form, as it would not be chosen by the assembler.
					if ambiguous.Prefix != "" && strings.HasPrefix(intel[i], ambiguous.Prefix) {
						continue
					}

					// Check whether the form overlaps with another
					// form that takes a smaller immediate argument.
					if ambiguous.OtherBits != 0 {
						parts := strings.Fields(intel[i])
						v, err := strconv.ParseInt(parts[len(parts)-1], 0, 64)
						if err != nil {
							return nil, fmt.Errorf("failed to parse immediate argument in %q (for %q): %v", intel[i], inst.Syntax, err)
						}

						min := -1 << (ambiguous.OtherBits - 1)
						max := 1<<(ambiguous.OtherBits-1) - 1
						if int(v) < min || max < int(v) {
							// This value is within the range of the
							// other instruction form, so we skip it.
							continue
						}
					}
				}

				if inst.Syntax == "CALL-FAR ptr16:32" {
					// Clang doesn't like making 6-byte
					// aboslute far calls in 16-bit mode
					// so we hard-code them here.
					addOperandOverrideIfMode(16)
					code += "9a"
					addUnsignedImmediate(2, 32)
					addUnsignedImmediate(1, 16)
				}

				if err != nil {
					return nil, err
				}

				entries = append(entries, &TestEntry{
					Inst:  inst,
					Mode:  mode,
					Ruse:  "(" + ruse[i] + ")",
					Intel: intel[i],
					Code:  code, // Generally, this will be the empty string and will be populated later.
				})
			}
		}
	}

	return entries, nil
}

// simpleInstruction describes
// an instruction with a one-byte
// opcode and a single immediate
// argument.
//
// We calculate their machine
// code ourselves, as they are
// ambiguous with other forms
// and not chosen by clang.
type simpleInstruction struct {
	OverrideMode uint8  // The CPU mode (if any) where an operand size override prefix is necessary.
	Opcode       string // The instruction's opcode.
	ArgIndex     int    // The index into the assembly where the immediate argument appears.
	ArgSize      int    // The size of the immediate argument in bits.
}

var simpleSignedInstructions = map[string]simpleInstruction{
	"ADC AL, imm8":  {0, "14", 2, 8},
	"ADC AX, imm16": {32, "15", 2, 16},
	"ADD AX, imm16": {32, "05", 2, 16},
	"AND AX, imm16": {32, "25", 2, 16},
	"CMP AX, imm16": {32, "3d", 2, 16},
	"CALL rel16":    {32, "e8", 1, 16},
	"CALL rel32":    {16, "e8", 1, 32},
	"JO rel8":       {0, "70", 1, 8},
	"JO rel16":      {32, "0f80", 1, 16},
	"JO rel32":      {16, "0f80", 1, 32},
	"JNO rel8":      {0, "71", 1, 8},
	"JNO rel16":     {32, "0f81", 1, 16},
	"JNO rel32":     {16, "0f81", 1, 32},
	"JB rel8":       {0, "72", 1, 8},
	"JB rel16":      {32, "0f82", 1, 16},
	"JB rel32":      {16, "0f82", 1, 32},
	"JC rel8":       {0, "72", 1, 8},
	"JC rel16":      {32, "0f82", 1, 16},
	"JC rel32":      {16, "0f82", 1, 32},
	"JNAE rel8":     {0, "72", 1, 8},
	"JNAE rel16":    {32, "0f82", 1, 16},
	"JNAE rel32":    {16, "0f82", 1, 32},
	"JAE rel8":      {0, "73", 1, 8},
	"JAE rel16":     {32, "0f83", 1, 16},
	"JAE rel32":     {16, "0f83", 1, 32},
	"JNB rel8":      {0, "73", 1, 8},
	"JNB rel16":     {32, "0f83", 1, 16},
	"JNB rel32":     {16, "0f83", 1, 32},
	"JNC rel8":      {0, "73", 1, 8},
	"JNC rel16":     {32, "0f83", 1, 16},
	"JNC rel32":     {16, "0f83", 1, 32},
	"JE rel8":       {0, "74", 1, 8},
	"JE rel16":      {32, "0f84", 1, 16},
	"JE rel32":      {16, "0f84", 1, 32},
	"JZ rel8":       {0, "74", 1, 8},
	"JZ rel16":      {32, "0f84", 1, 16},
	"JZ rel32":      {16, "0f84", 1, 32},
	"JNE rel8":      {0, "75", 1, 8},
	"JNE rel16":     {32, "0f85", 1, 16},
	"JNE rel32":     {16, "0f85", 1, 32},
	"JNZ rel8":      {0, "75", 1, 8},
	"JNZ rel16":     {32, "0f85", 1, 16},
	"JNZ rel32":     {16, "0f85", 1, 32},
	"JBE rel8":      {0, "76", 1, 8},
	"JBE rel16":     {32, "0f86", 1, 16},
	"JBE rel32":     {16, "0f86", 1, 32},
	"JNA rel8":      {0, "76", 1, 8},
	"JNA rel16":     {32, "0f86", 1, 16},
	"JNA rel32":     {16, "0f86", 1, 32},
	"JA rel8":       {0, "77", 1, 8},
	"JA rel16":      {32, "0f87", 1, 16},
	"JA rel32":      {16, "0f87", 1, 32},
	"JNBE rel8":     {0, "77", 1, 8},
	"JNBE rel16":    {32, "0f87", 1, 16},
	"JNBE rel32":    {16, "0f87", 1, 32},
	"JS rel8":       {0, "78", 1, 8},
	"JS rel16":      {32, "0f88", 1, 16},
	"JS rel32":      {16, "0f88", 1, 32},
	"JNS rel8":      {0, "79", 1, 8},
	"JNS rel16":     {32, "0f89", 1, 16},
	"JNS rel32":     {16, "0f89", 1, 32},
	"JP rel8":       {0, "7a", 1, 8},
	"JP rel16":      {32, "0f8a", 1, 16},
	"JP rel32":      {16, "0f8a", 1, 32},
	"JPE rel8":      {0, "7a", 1, 8},
	"JPE rel16":     {32, "0f8a", 1, 16},
	"JPE rel32":     {16, "0f8a", 1, 32},
	"JNP rel8":      {0, "7b", 1, 8},
	"JNP rel16":     {32, "0f8b", 1, 16},
	"JNP rel32":     {16, "0f8b", 1, 32},
	"JPO rel8":      {0, "7b", 1, 8},
	"JPO rel16":     {32, "0f8b", 1, 16},
	"JPO rel32":     {16, "0f8b", 1, 32},
	"JL rel8":       {0, "7c", 1, 8},
	"JL rel16":      {32, "0f8c", 1, 16},
	"JL rel32":      {16, "0f8c", 1, 32},
	"JNGE rel8":     {0, "7c", 1, 8},
	"JNGE rel16":    {32, "0f8c", 1, 16},
	"JNGE rel32":    {16, "0f8c", 1, 32},
	"JGE rel8":      {0, "7d", 1, 8},
	"JGE rel16":     {32, "0f8d", 1, 16},
	"JGE rel32":     {16, "0f8d", 1, 32},
	"JNL rel8":      {0, "7d", 1, 8},
	"JNL rel16":     {32, "0f8d", 1, 16},
	"JNL rel32":     {16, "0f8d", 1, 32},
	"JLE rel8":      {0, "7e", 1, 8},
	"JLE rel16":     {32, "0f8e", 1, 16},
	"JLE rel32":     {16, "0f8e", 1, 32},
	"JNG rel8":      {0, "7e", 1, 8},
	"JNG rel16":     {32, "0f8e", 1, 16},
	"JNG rel32":     {16, "0f8e", 1, 32},
	"JG rel8":       {0, "7f", 1, 8},
	"JG rel16":      {32, "0f8f", 1, 16},
	"JG rel32":      {16, "0f8f", 1, 32},
	"JNLE rel8":     {0, "7f", 1, 8},
	"JNLE rel16":    {32, "0f8f", 1, 16},
	"JNLE rel32":    {16, "0f8f", 1, 32},
	"JCXZ rel8":     {0, "e3", 1, 8},
	"JECXZ rel8":    {0, "e3", 1, 8},
	"JRCXZ rel8":    {0, "e3", 1, 8},
	"JMP rel8":      {0, "eb", 1, 8},
	"JMP rel16":     {32, "e9", 1, 16},
	"JMP rel32":     {16, "e9", 1, 32},
	"LOOP rel8":     {0, "e2", 1, 8},
	"LOOPE rel8":    {0, "e1", 1, 8},
	"LOOPNE rel8":   {0, "e0", 1, 8},
	"OR AX, imm16":  {32, "0d", 2, 16},
	"PUSH imm16":    {0, "68", 1, 16},
	"PUSH imm32":    {0, "68", 1, 32},
	"PUSHW imm16":   {32, "68", 1, 16},
	"PUSHD imm32":   {16, "68", 1, 32},
	"SBB AX, imm16": {32, "1d", 2, 16},
	"SUB AX, imm16": {32, "2d", 2, 16},
	"XBEGIN rel16":  {32, "c7f8", 1, 16},
	"XBEGIN rel32":  {16, "c7f8", 1, 32},
	"XOR AX, imm16": {32, "35", 2, 16},
}

var simpleUnsignedInstructions = map[string]simpleInstruction{
	"MOV AL, moffs8":   {0, "a0", 4, 0},
	"MOV AX, moffs16":  {32, "a1", 4, 0},
	"MOV EAX, moffs32": {16, "a1", 4, 0},
	"MOV RAX, moffs64": {0, "a1", 4, 0},
	"MOV moffs8, AL":   {0, "a2", 3, 0},
	"MOV moffs16, AX":  {32, "a3", 3, 0},
	"MOV moffs32, EAX": {16, "a3", 3, 0},
	"MOV moffs64, RAX": {0, "a3", 3, 0},
	"RET imm16u":       {0, "c2", 1, 16},
	"RET-FAR imm16u":   {0, "ca", 1, 16},
}

// ambiguousInstruction describes
// an instruction that is ambiguous
// with another instruction.
//
// For example, the pair:
//
//   - ADC AX, imm16
//   - ADC r/m16, imm8
//
// is ambiguous, as either could be
// chosen for the assembly code
// `adc ax, 2`. This structure is
// used for the likes of `ADC r/m16, imm8`,
// which is a more general instruction
// that overlaps with a more specific
// one.
//
// It also covers cases where
// there is a third form that is
// identical, except for the size
// of the immediate argument. For
// example, the pair:
//
//   - ADC r/m16, imm8
//   - ADC r/m16, imm16
type ambiguousInstruction struct {
	Prefix    string // The prefix of an instruction instance that would be ambiguous.
	OtherBits int    // The size in bits of the immediate argument of the smaller variant of this instruction (or 0).
}

var ambiguousInstructions = map[string]ambiguousInstruction{
	"ADC r/m16, imm16":       {"adc ax,", 8},
	"ADC r/m32, imm32":       {"adc eax,", 8},
	"ADC r/m64, imm32":       {"adc rax,", 8},
	"ADD r/m16, imm16":       {"add ax,", 8},
	"ADD r/m32, imm32":       {"add eax,", 8},
	"ADD r/m64, imm32":       {"add rax,", 8},
	"AND r/m16, imm16":       {"and ax,", 8},
	"AND r/m32, imm32":       {"and eax,", 8},
	"AND r/m64, imm32":       {"and rax,", 8},
	"CMP r/m16, imm16":       {"cmp ax,", 8},
	"CMP r/m32, imm32":       {"cmp eax,", 8},
	"CMP r/m64, imm32":       {"cmp rax,", 8},
	"IMUL r16, r/m16, imm16": {"", 8},
	"IMUL r32, r/m32, imm32": {"", 8},
	"IMUL r64, r/m64, imm32": {"", 8},
	"OR r/m16, imm16":        {"or ax,", 8},
	"OR r/m32, imm32":        {"or eax,", 8},
	"OR r/m64, imm32":        {"or rax,", 8},
	"SBB r/m16, imm16":       {"sbb ax,", 8},
	"SBB r/m32, imm32":       {"sbb eax,", 8},
	"SBB r/m64, imm32":       {"sbb rax,", 8},
	"SUB r/m16, imm16":       {"sub ax,", 8},
	"SUB r/m32, imm32":       {"sub eax,", 8},
	"SUB r/m64, imm32":       {"sub rax,", 8},
	"XOR r/m16, imm16":       {"xor ax,", 8},
	"XOR r/m32, imm32":       {"xor eax,", 8},
	"XOR r/m64, imm32":       {"xor rax,", 8},
}

// syntaxToOptions turns an
// x86 instruction syntax into
// a sequence of one or more
// exploratory instructions,
// intended to give good coverage
// of the assembler.
//
// For example, the syntax
// `"ADD AL, imm8"` might give
// the options:
//
//	[]string{
//		"add al, -0x80",
//		"add al, -0x7",
//		"add al, 0x0",
//		"add al, 0x8",
//		"add al, 0x7f",
//	}
func syntaxToOptions(inst *x86.Instruction, ruse, intel []string, mode uint8, operand string) (ruseOut, intelOut []string, err error) {
	both := func(s ...string) {
		for _, s := range s {
			ruse = append(ruse, s)
			intel = append(intel, s)
		}
	}

	pairs := func(pairs ...string) {
		if len(pairs)%2 != 0 {
			panic(fmt.Sprintf("pairs: got %d entries", len(pairs)))
		}

		for i := 0; i < len(pairs); i += 2 {
			ruse = append(ruse, pairs[i+0])
			intel = append(intel, pairs[i+1])
		}
	}

	memory16 := func(ruseSize, intelSize string) {
		if ruseSize != "" {
			ruseSize = "'(*" + ruseSize + ")"
		}
		if intelSize != "" {
			intelSize = intelSize + " ptr "
		}
		pairs(
			// Mod 00
			ruseSize+"(+ bx si)", intelSize+"[bx+si]",
			ruseSize+"(+ bx di)", intelSize+"[bx+di]",
			ruseSize+"(+ bp si)", intelSize+"[bp+si]",
			ruseSize+"(+ bp di)", intelSize+"[bp+di]",
			ruseSize+"(si)", intelSize+"[si]",
			ruseSize+"(di)", intelSize+"[di]",
			ruseSize+"(0xa)", intelSize+"[0xa]",
			ruseSize+"(0x10)", intelSize+"[0x10]",
			ruseSize+"(0xff)", intelSize+"[0xff]",
			ruseSize+"(0x7fff)", intelSize+"[0x7fff]",
			// 16-bit solo displacements can't be negative, as they're relative to the segment base.
			ruseSize+"(bx)", intelSize+"[bx]",
			// Mod 01
			ruseSize+"(+ bx si 0x1)", intelSize+"[bx+si+0x1]",
			ruseSize+"(+ ss bx di 0x7f)", intelSize+"ss:[bx+di+0x7f]",
			ruseSize+"(+ bp si -0x80)", intelSize+"[bp+si-0x80]",
			ruseSize+"(+ bp di -0x1)", intelSize+"[bp+di-0x1]",
			ruseSize+"(+ si 0x12)", intelSize+"[si+0x12]",
			ruseSize+"(+ di -0x34)", intelSize+"[di-0x34]",
			ruseSize+"(+ bp 0x0)", intelSize+"[bp+0x0]",
			ruseSize+"(+ ds bp 0x7f)", intelSize+"ds:[bp+0x7f]",
			ruseSize+"(+ bp -0x1)", intelSize+"[bp-0x1]",
			ruseSize+"(+ bp -0x80)", intelSize+"[bp-0x80]",
			ruseSize+"(+ bx 0x7)", intelSize+"[bx+0x7]",
			// Mod 10
			ruseSize+"(+ es bp 0x0)", intelSize+"es:[bp+0x0]",
			ruseSize+"(+ bp 0xff)", intelSize+"[bp+0xff]",
			ruseSize+"(+ bp 0x7fff)", intelSize+"[bp+0x7fff]",
			ruseSize+"(+ bp -0xfe)", intelSize+"[bp-0xfe]",
			ruseSize+"(+ bp -0x8000)", intelSize+"[bp-0x8000]",
		)
	}

	memory32 := func(ruseSize, intelSize string, fullSize bool) {
		if ruseSize != "" {
			ruseSize = "'(*" + ruseSize + ")"
		}
		if intelSize != "" {
			intelSize = intelSize + " ptr "
		}
		pairs(
			// Mod 00
			ruseSize+"(eax)", intelSize+"[eax]",
			ruseSize+"(ecx)", intelSize+"[ecx]",
			ruseSize+"(edx)", intelSize+"[edx]",
			ruseSize+"(ebx)", intelSize+"[ebx]",
			// SIB
			ruseSize+"(+ esp (* eax 1))", intelSize+"[esp+eax*1]",
			ruseSize+"(+ ebp ecx)", intelSize+"[ebp+ecx]",
			ruseSize+"(+ eax (* edx 1))", intelSize+"[eax+edx*1]",
			ruseSize+"(esp)", intelSize+"[esp]",
			ruseSize+"(ebp)", intelSize+"[ebp]",
			ruseSize+"(+ esp (* eax 2))", intelSize+"[esp+eax*2]",
			ruseSize+"(+ ebp (* ecx 2) 0x7)", intelSize+"[ebp+ecx*2+0x7]",
			ruseSize+"(+ ecx (* edx 2))", intelSize+"[ecx+edx*2]",
			ruseSize+"(+ esp (* eax 4) -0x12)", intelSize+"[esp+eax*4-0x12]",
			ruseSize+"(+ ebp (* ecx 4))", intelSize+"[ebp+ecx*4]",
			ruseSize+"(+ edi edx 7)", intelSize+"[edi+edx+0x7]",
			ruseSize+"(+ esp (* eax 8))", intelSize+"[esp+eax*8]",
			ruseSize+"(0xb)", intelSize+"[0xb]",
			ruseSize+"(0x11)", intelSize+"[0x11]",
			ruseSize+"(0xfe)", intelSize+"[0xfe]",
			//  32-bit solo displacements can't be negative, as they're relative to the segment base.
			ruseSize+"(esi)", intelSize+"[esi]",
			ruseSize+"(edi)", intelSize+"[edi]",
			// Mod 01
			ruseSize+"(+ eax 0x7)", intelSize+"[eax+0x7]",
			ruseSize+"(+ ecx 0x7f)", intelSize+"[ecx+0x7f]",
			ruseSize+"(+ edx -0x12)", intelSize+"[edx-0x12]",
			ruseSize+"(+ ebx -0x80)", intelSize+"[ebx-0x80]",
			ruseSize+"(+ edi -0x10)", intelSize+"[edi-0x10]",
			// Mod 10
			ruseSize+"(+ eax 0xff)", intelSize+"[eax+0xff]",
			ruseSize+"(+ edx -0x112)", intelSize+"[edx-0x112]",
		)

		// 32-bit addresses.
		if fullSize {
			pairs(
				ruseSize+"(+ (* eax 4) 7)", intelSize+"[eax*4+0x7]",
				ruseSize+"(* eax 8)", intelSize+"[eax*8+0x0]",
				ruseSize+"(+ esp 7)", intelSize+"[esp+0x7]",
				ruseSize+"(+ ebp (* ecx 8) 0x7fffffff)", intelSize+"[ebp+ecx*8+0x7fffffff]",
				ruseSize+"(+ esp (* eax 8) -0x80000000)", intelSize+"[esp+eax*8-0x80000000]",
				ruseSize+"(0x7fffffff)", intelSize+"[0x7fffffff]",
				// ruseSize+"(-0x80000000)", intelSize+"[-0x80000000]", // 32-bit solo displacements wont' be negative, as they're relative to the segment base.
				ruseSize+"(+ ecx 0x7fffffff)", intelSize+"[ecx+0x7fffffff]",
				ruseSize+"(+ ebx -0x80000000)", intelSize+"[ebx-0x80000000]",
			)
		}
	}

	memory64 := func(ruseSize, intelSize string) {
		if ruseSize != "" {
			ruseSize = "'(*" + ruseSize + ")"
		}
		if intelSize != "" {
			intelSize = intelSize + " ptr "
		}
		pairs(
			// Mod 00
			ruseSize+"(rax)", intelSize+"[rax]",
			ruseSize+"(rcx)", intelSize+"[rcx]",
			ruseSize+"(rdx)", intelSize+"[rdx]",
			ruseSize+"(rbx)", intelSize+"[rbx]",
			// SIB
			ruseSize+"(+ rsp rax)", intelSize+"[rsp+rax]",
			ruseSize+"(+ rsp (* rax 1))", intelSize+"[rsp+rax*1]",
			ruseSize+"(+ rbp rcx)", intelSize+"[rbp+rcx]",
			ruseSize+"(+ rsp rax)", intelSize+"[rsp+rax]",
			ruseSize+"(rsp)", intelSize+"[rsp]",
			ruseSize+"(rbp)", intelSize+"[rbp]",
			ruseSize+"(+ rsp (* rax 2))", intelSize+"[rsp+rax*2]",
			ruseSize+"(+ rbp (* rcx 2) 0x7)", intelSize+"[rbp+rcx*2+0x7]",
			ruseSize+"(+ rsp (* rax 2))", intelSize+"[rsp+rax*2]",
			ruseSize+"(+ rsp (* rax 4) -0x12)", intelSize+"[rsp+rax*4-0x12]",
			ruseSize+"(+ rbp (* rcx 4))", intelSize+"[rbp+rcx*4]",
			ruseSize+"(+ rsp (* rax 4))", intelSize+"[rsp+rax*4]",
			ruseSize+"(+ rsp (* rax 8))", intelSize+"[rsp+rax*8]",
			ruseSize+"(+ rbp (* rcx 8) 0x7fffffff)", intelSize+"[rbp+rcx*8+0x7fffffff]",
			ruseSize+"(+ rsp (* rax 8) -0x80000000)", intelSize+"[rsp+rax*8-0x80000000]",
			ruseSize+"(0xa)", intelSize+"[0xa]",
			ruseSize+"(0x10)", intelSize+"[0x10]",
			ruseSize+"(0xff)", intelSize+"[0xff]",
			ruseSize+"(0x7fffffff)", intelSize+"[0x7fffffff]",
			ruseSize+"(-0xa)", intelSize+"[-0xa]",
			ruseSize+"(-0xff)", intelSize+"[-0xff]",
			ruseSize+"(-0x80000000)", intelSize+"[-0x80000000]",
			ruseSize+"(rsi)", intelSize+"[rsi]",
			ruseSize+"(rdi)", intelSize+"[rdi]",
			// Mod 01
			ruseSize+"(+ rax 0x7)", intelSize+"[rax+0x7]",
			ruseSize+"(+ rcx 0x7f)", intelSize+"[rcx+0x7f]",
			ruseSize+"(+ rdx -0x12)", intelSize+"[rdx-0x12]",
			ruseSize+"(+ rbx -0x80)", intelSize+"[rbx-0x80]",
			ruseSize+"(+ rdi 0x40)", intelSize+"[rdi+0x40]",
			// Mod 10
			ruseSize+"(+ rax 0xff)", intelSize+"[rax+0xff]",
			ruseSize+"(+ rcx 0x7fffffff)", intelSize+"[rcx+0x7fffffff]",
			ruseSize+"(+ rdx -0x112)", intelSize+"[rdx-0x112]",
			ruseSize+"(+ rbx -0x80000000)", intelSize+"[rbx-0x80000000]",
		)
	}

	memoryOffset := func(size string, mode uint8) {
		var ruseSize, intelSize string
		if size != "" {
			ruseSize = "'(*" + size + ")"
			intelSize = size + " ptr "
		}
		if mode == 16 || mode == 32 {
			// 16-bit offsets.
			pairs(
				ruseSize+"(0x0)", intelSize+"[0x0]",
				ruseSize+"(0xff)", intelSize+"[0xff]",
				ruseSize+"(0xfefd)", intelSize+"[0xfefd]",
			)
		}
		if mode == 32 {
			// 32-bit offsets.
			pairs(
				ruseSize+"(0xfffefd)", intelSize+"[0xfffefd]",
				ruseSize+"(0xfffffffe)", intelSize+"[0xfffffffe]",
				ruseSize+"(0xfefdfcfb)", intelSize+"[0xfefdfcfb]",
			)
		}
		if mode == 64 {
			// 64-bit offsets.
			pairs(
				ruseSize+"(0x8877665544332211)", intelSize+"[0x8877665544332211]",
				ruseSize+"(0x7ff8f9fafbfcfdfe)", intelSize+"[0x7ff8f9fafbfcfdfe]",
			)
		}
	}

	stringDst16 := func(size string) {
		pairs(
			"'(*"+size+")(di)", size+" ptr [di]",
			"'(*"+size+")(es di)", size+" ptr es:[di]",
		)
	}

	stringDst32 := func(size string) {
		pairs(
			"'(*"+size+")(edi)", size+" ptr [edi]",
			"'(*"+size+")(es edi)", size+" ptr es:[edi]",
		)
	}

	stringDst64 := func(size string) {
		pairs(
			"'(*"+size+")(rdi)", size+" ptr [rdi]",
		)
	}

	stringSrc16 := func(size string) {
		pairs(
			"'(*"+size+")(si)", size+" ptr [si]",
			"'(*"+size+")(ds si)", size+" ptr ds:[si]",
		)
	}

	stringSrc32 := func(size string) {
		pairs(
			"'(*"+size+")(esi)", size+" ptr [esi]",
			"'(*"+size+")(ds esi)", size+" ptr ds:[esi]",
		)
	}

	stringSrc64 := func(size string) {
		pairs(
			"'(*"+size+")(rsi)", size+" ptr [rsi]",
		)
	}

	switch operand {
	case "imm8":
		both("-0x80", "-7", "0", "7", "0x7f")
	case "imm16":
		both("-0x8000", "-256", "256", "0x7fff")
	case "imm32":
		both("-0x80000000", "-65536", "65536", "0x7fffffff")
	case "imm64":
		both("-0x8000000000000000", "-4294967296", "4294967296", "0x7fffffffffffffff")
	case "imm5u":
		both("0", "11", "31")
	case "imm8u":
		both("0", "12", "0x7f", "0xff")
	case "imm16u":
		both("0x7fff", "0xffff")
	case "imm32u":
		both("0x7fffffff", "0xffffffff")
	case "imm64u":
		both("0x7fffffffffffffff", "0xffffffffffffff")
	case "rel8":
		both("-0x11", "0", "0x11")
	case "rel16":
		both("-0x1122", "0", "0x1122")
	case "rel32":
		both("-0x112233", "0", "0x112233")
	case "ST":
		both("st")
	case "AL", "CL", "AX", "DX", "ES", "CS", "SS", "DS", "FS", "GS",
		"EAX", "ECX",
		"RAX",
		"XMM0",
		"CR8",
		"0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		both(strings.Map(alphanumRunesOnlyAndLower, operand))
	case "r8", "rmr8", "r8V", "r8op", "rmr8op":
		// Don't include al, as there are too
		// many instructions that specialise
		// for the accumulator, resulting in
		// ambiguous assembly.
		both("cl", "bl", "ah", "ch", "bh")
		if mode == 64 {
			pairs(
				"dil", "dil",
				"spl", "spl",
				"r8l", "r8b",
				"r10l", "r10b",
				"r15l", "r15b",
			)
		}
	case "r16", "rmr16", "r16V", "r16op", "rmr16op":
		// Don't include ax, as there are too
		// many instructions that specialise
		// for the accumulator, resulting in
		// ambiguous assembly.
		both("cx", "bp", "sp", "di")
		if mode == 64 {
			both("r8w", "r10w", "r15w")
		}
	case "r32", "rmr32", "r32V", "r32op", "rmr32op":
		// Don't include eax, as there are too
		// many instructions that specialise
		// for the accumulator, resulting in
		// ambiguous assembly.
		both("ecx", "ebp", "esp", "edi")
		if mode == 64 {
			both("r8d", "r10d", "r15d")
		}
	case "r64", "rmr64", "r64V", "r64op", "rmr64op":
		// Don't include rax, as there are too
		// many instructions that specialise
		// for the accumulator, resulting in
		// ambiguous assembly.
		if mode == 16 || mode == 32 {
			return nil, nil, fmt.Errorf("no options for syntax %q in %d-bit mode", operand, mode)
		} else {
			both("rcx", "rbp", "rsp", "rdi", "r8", "r10", "r15")
		}
	case "k1", "k2", "kV":
		both("k1", "k4", "k7")
	case "Sreg":
		both("es", "cs", "ss", "ds", "fs", "gs")
	case "ST(i)":
		pairs(
			"st1", "st(1)",
			"st3", "st(3)",
			"st7", "st(7)",
		)
	case "CR0-CR7":
		both("cr0", "cr1", "cr5", "cr7")
	case "DR0-DR7":
		both("dr0", "dr1", "dr5", "dr7")
	case "mm1", "mm2":
		both("mm0", "mm1", "mm7")
	case "xmm1", "xmm2":
		if inst.Encoding.EVEX {
			if mode == 64 {
				both("xmm5", "xmm19", "xmm31")
			}
		} else {
			both("xmm0", "xmm1", "xmm7")
			if mode == 64 {
				both("xmm8", "xmm15")
			}
		}
	case "xmmV", "xmmIH":
		if inst.Encoding.EVEX {
			if mode == 64 {
				both("xmm5", "xmm19", "xmm31")
			}
		} else if mode == 16 {
			// Not supported in 16-bit mode.
		} else {
			both("xmm0", "xmm1", "xmm7")
			if mode == 64 {
				both("xmm8", "xmm15")
			}
		}
	case "ymm1", "ymm2":
		if inst.Encoding.EVEX {
			if mode == 64 {
				both("ymm5", "ymm19", "ymm31")
			}
		} else {
			both("ymm0", "ymm1", "ymm7")
			if mode == 64 {
				both("ymm8", "ymm15")
			}
		}
	case "ymmV", "ymmIH":
		if inst.Encoding.EVEX {
			if mode == 64 {
				both("ymm5", "ymm19", "ymm31")
			}
		} else if mode == 16 {
			// Not supported in 16-bit mode.
		} else {
			both("ymm0", "ymm1", "ymm7")
			if mode == 64 {
				both("ymm8", "ymm15")
			}
		}
	case "zmm1", "zmm2", "zmmV", "zmmIH":
		if mode == 64 && inst.Encoding.EVEX {
			both("zmm0", "zmm1", "zmm7", "zmm8", "zmm15", "zmm19", "zmm31")
		}
	case "[es:edi:8]":
		if mode == 16 {
			stringDst16("byte")
		}
		stringDst32("byte")
	case "[es:edi:16]":
		if mode == 16 {
			stringDst16("word")
		}
		stringDst32("word")
	case "[es:edi:32]":
		if mode == 16 {
			stringDst16("dword")
		}
		stringDst32("dword")
	case "[rdi:64]":
		if mode != 64 {
			return nil, nil, fmt.Errorf("no options for syntax %q in %d-bit mode", operand, mode)
		} else {
			stringDst64("qword")
		}
	case "[ds:esi:8]":
		if mode == 16 {
			stringSrc16("byte")
		}
		stringSrc32("byte")
	case "[ds:esi:16]":
		if mode == 16 {
			stringSrc16("word")
		}
		stringSrc32("word")
	case "[ds:esi:32]":
		if mode == 16 {
			stringSrc16("dword")
		}
		stringSrc32("dword")
	case "[rsi:64]":
		if mode != 64 {
			return nil, nil, fmt.Errorf("no options for syntax %q in %d-bit mode", operand, mode)
		} else {
			stringSrc64("qword")
		}
	case "m", "m14/28byte", "m94/108byte", "m384", "m512byte":
		if (mode == 16 || mode == 32) && inst.Mode16 {
			memory16("", "")
		}
		if mode == 32 || !inst.Encoding.EVEX {
			memory32("", "", mode > 16)
		}
		if mode == 64 {
			memory64("", "")
		}
	case "m8":
		if (mode == 16 || mode == 32) && inst.Mode16 {
			memory16("byte", "byte")
		}
		if mode == 32 || !inst.Encoding.EVEX {
			memory32("byte", "byte", mode > 16)
		}
		if mode == 64 {
			memory64("byte", "byte")
		}
	case "m16", "m16int", "m16op", "m16bcst", "m2byte":
		if (mode == 16 || mode == 32) && inst.Mode16 {
			memory16("word", "word")
		}
		if mode == 32 || !inst.Encoding.EVEX {
			memory32("word", "word", mode > 16)
		}
		if mode == 64 {
			memory64("word", "word")
		}
	case "m16:16":
		if (mode == 16 || mode == 32) && inst.Mode16 {
			memory16("dword", "word")
		}
		if mode == 32 || !inst.Encoding.EVEX {
			memory32("dword", "word", mode > 16)
		}
		if mode == 64 {
			memory64("dword", "word")
		}
	case "m16:32":
		if (mode == 16 || mode == 32) && inst.Mode16 {
			memory16("tword", "word")
		}
		if mode == 32 || !inst.Encoding.EVEX {
			memory32("tword", "word", mode > 16)
		}
		if mode == 64 {
			memory64("tword", "word")
		}
	case "m32", "m32fp", "m32int", "m32op", "m32bcst", "m16&16":
		if (mode == 16 || mode == 32) && inst.Mode16 {
			memory16("dword", "dword")
		}
		if mode == 32 || !inst.Encoding.EVEX {
			memory32("dword", "dword", mode > 16)
		}
		if mode == 64 {
			memory64("dword", "dword")
		}
	case "m16&32":
		if (mode == 16 || mode == 32) && inst.Mode16 {
			memory16("tword", "")
		}
		if mode == 32 || !inst.Encoding.EVEX {
			memory32("tword", "", mode > 16)
		}
		if mode == 64 {
			memory64("tword", "")
		}
	case "m64", "m64fp", "m64int", "m64op", "m64bcst", "m32&32":
		if (mode == 16 || mode == 32) && inst.Mode16 {
			memory16("qword", "qword")
		}
		if mode == 32 || !inst.Encoding.EVEX {
			memory32("qword", "qword", mode > 16)
		}
		if mode == 64 {
			memory64("qword", "qword")
		}
	case "m16:64", "m16&64":
		if (mode == 16 || mode == 32) && inst.Mode16 {
			memory16("tbyte", "qword")
		}
		if mode == 32 || !inst.Encoding.EVEX {
			memory32("tbyte", "qword", mode > 16)
		}
		if mode == 64 {
			memory64("tbyte", "qword")
		}
	case "m80bcd", "m80dec", "m80fp":
		if mode == 32 || !inst.Encoding.EVEX {
			memory32("tbyte", "tbyte", mode > 16)
		}
		if mode == 64 {
			memory64("tbyte", "tbyte")
		}
	case "m128":
		if (mode == 16 || mode == 32) && inst.Mode16 {
			memory16("xmmword", "xmmword")
		}
		if mode == 32 || !inst.Encoding.EVEX {
			memory32("xmmword", "xmmword", mode > 16)
		}
		if mode == 64 {
			memory64("xmmword", "xmmword")
		}
	case "m256":
		if (mode == 16 || mode == 32) && inst.Mode16 {
			memory16("ymmword", "ymmword")
		}
		if mode == 32 || !inst.Encoding.EVEX {
			memory32("ymmword", "ymmword", mode > 16)
		}
		if mode == 64 {
			memory64("ymmword", "ymmword")
		}
	case "m512":
		if mode == 32 || !inst.Encoding.EVEX {
			memory32("zmmword", "zmmword", mode > 16)
		}
		if mode == 64 {
			memory64("zmmword", "zmmword")
		}
	case "moffs8":
		memoryOffset("byte", mode)
	case "moffs16":
		memoryOffset("word", mode)
	case "moffs32":
		memoryOffset("dword", mode)
	case "moffs64":
		memoryOffset("qword", mode)
	case "ptr16:16":
		pairs(
			"(0x0 0x0)", "0x0, 0x0",
			"(0xfd 0xfe)", "0xfd, 0xfe",
			"(0xfefd 0xf2fe)", "0xfefd, 0xf2fe",
		)
	case "ptr16:32":
		pairs(
			"(0x0 0x10000)", "0x0, 0x10000",
			"(0xfd 0xfefcfd)", "0xfd, 0xfefcfd",
			"(0xfefd 0xfffff2fe)", "0xfefd, 0xfffff2fe",
		)
	default:
		return nil, nil, fmt.Errorf("no known options for syntax %q", operand)
	}

	return ruse, intel, nil
}
