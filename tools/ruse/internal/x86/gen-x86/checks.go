// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Check that the produced instructions make sense.

package main

import (
	"fmt"
	"strings"

	"firefly-os.dev/tools/ruse/internal/x86"
)

// CheckInstruction performs various sanity checks
// on the instruction to identify logical errors.
func CheckInstruction(inst *x86.Instruction) error {
	// Categorise the parameters.
	var (
		registerInVEXvvvv  string
		registerModifier   string
		stackIndex         string
		codeOffset         string
		registerInModRMreg string
		registerInModRMrm  string
		vsib               string

		codeOffsets []int
		immediates  []int
		memory      string
	)

	for _, param := range inst.Parameters {
		switch param.Type {
		case x86.TypeSignedImmediate, x86.TypeUnsignedImmediate:
			if param.Encoding == x86.EncodingImmediate {
				bits := param.Bits
				if bits == 5 {
					bits = 8 // 5-bit values are stored in 8 bits.
				}

				immediates = append(immediates, bits)
			}
		case x86.TypeMemory:
			if memory != "" {
				return fmt.Errorf("found memory values %s and %s", memory, param.Syntax)
			}

			memory = param.Syntax
		}

		switch param.Encoding {
		case x86.EncodingVEXvvvv:
			if registerInVEXvvvv != "" {
				return fmt.Errorf("found registers %s and %s encoded in VEX.vvvv", registerInVEXvvvv, param.Syntax)
			}

			registerInVEXvvvv = param.Syntax
		case x86.EncodingRegisterModifier:
			if registerModifier != "" {
				return fmt.Errorf("found register modifier %s and %s encoded in the opcode", registerModifier, param.Syntax)
			}

			registerModifier = param.Syntax
		case x86.EncodingStackIndex:
			if stackIndex != "" {
				return fmt.Errorf("found x87 FPU stack indices %s and %s encoded in the opcode", stackIndex, param.Syntax)
			}

			stackIndex = param.Syntax
		case x86.EncodingCodeOffset:
			if codeOffset != "" {
				return fmt.Errorf("found relative code offsets %s and %s encoded after the opcode", codeOffset, param.Syntax)
			}

			codeOffset = param.Syntax
			codeOffsets = append(codeOffsets, param.Bits)
		case x86.EncodingModRMreg:
			if registerInModRMreg != "" {
				return fmt.Errorf("found registers %s and %s encoded in ModR/M.reg", registerInModRMreg, param.Syntax)
			}

			registerInModRMreg = param.Syntax
		case x86.EncodingModRMrm:
			if registerInModRMrm != "" {
				return fmt.Errorf("found registers %s and %s encoded in ModR/M.rm", registerInModRMrm, param.Syntax)
			}

			registerInModRMrm = param.Syntax
		case x86.EncodingVSIB:
			if vsib != "" {
				return fmt.Errorf("found memory %s and %s encoded in VSIB", vsib, param.Syntax)
			}

			vsib = param.Syntax
		}
	}

	// Get some extra details from the encoding
	// string, as there are some details we
	// don't extract generally.
	var (
		numParameters   int // This is mainly for the instructions in extras.go, where we include the Parameters by hand.
		codeOffsetSizes []int
		immediateSizes  []int
	)

	if i := strings.IndexByte(inst.Syntax, ' '); i > 0 {
		numParameters = 1 + strings.Count(inst.Syntax[i:], ",")
	}

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
		return fmt.Errorf("found register opcode modifier but no x87 FPU stack index or register parameter")
	}

	if vsib != "" && !inst.Encoding.VSIB {
		return fmt.Errorf("found parameter %s but instruction encoding is missing /vsib", vsib)
	}

	if inst.Tuple == x86.Tuple1Scalar && inst.DataSize == 0 {
		return fmt.Errorf("instruction has tuple type %s but no data operation size", inst.Tuple)
	}

	switch {
	case inst.Encoding.CodeOffset && codeOffset == "":
		return fmt.Errorf("a relative code offset is required but no relative offset parameters are included")
	case !inst.Encoding.CodeOffset && codeOffset != "":
		return fmt.Errorf("no relative code offset is expected but relative offset parameter %q is included", codeOffset)
	}

	if len(codeOffsets) != len(codeOffsetSizes) {
		return fmt.Errorf("found %d code offset parameters but expected %d from the encoding %s", len(codeOffsets), len(codeOffsetSizes), inst.Encoding.Syntax)
	}

	for i := range codeOffsets {
		if codeOffsets[i] != codeOffsetSizes[i] {
			return fmt.Errorf("code offset parameter %d of %d has size %d bits but expected %d bits from the encoding %s", i+1, len(codeOffsets), codeOffsets[i], codeOffsetSizes[i], inst.Encoding.Syntax)
		}
	}

	if inst.Encoding.ModRMreg != 0 && registerInModRMreg != "" {
		return fmt.Errorf("found register parameter %s and fixed value /%d encoded in ModR/M.reg", registerInModRMreg, inst.Encoding.ModRMreg-1)
	}

	if len(immediates) != len(immediateSizes) {
		return fmt.Errorf("found %d immediate parameters but expected %d from the encoding %s", len(immediates), len(immediateSizes), inst.Encoding.Syntax)
	}

	for i := range immediates {
		if immediates[i] != immediateSizes[i] {
			return fmt.Errorf("immediate parameter %d of %d has size %d bits but expected %d bits from the encoding %s", i+1, len(immediates), immediates[i], immediateSizes[i], inst.Encoding.Syntax)
		}
	}

	if numParameters != len(inst.Parameters) {
		return fmt.Errorf("expected %d parameters, but only found %d", numParameters, len(inst.Parameters))
	}

	return nil
}
