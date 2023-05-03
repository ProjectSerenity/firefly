// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sort"

	"firefly-os.dev/tools/ruse/internal/x86"
)

// SortInstructions sorts a set of
// x86 instructions by machine code
// sequence length and register size.
func SortInstructions(insts []*x86.Instruction) {
	// Sort the instructions.
	//
	// It's important that we get this right
	// so that we always produce the ideal
	// instruction variant when multiple
	// variants match the input.
	//
	// When sorting, we prefer shorter machine
	// code sequences over longer ones and
	// smaller registers over larger.
	sort.Slice(insts, func(i, j int) bool {
		inst1 := insts[i]
		inst2 := insts[j]

		// First, we sort by mnemonic.
		mnemonic1 := inst1.Mnemonic
		mnemonic2 := inst2.Mnemonic
		if mnemonic1 != mnemonic2 {
			return mnemonic1 < mnemonic2
		}

		// Then, number of operands. This shouldn't affect
		// assembling, but it simplifies things.
		params1 := inst1.Parameters
		params2 := inst2.Parameters
		if len(params1) != len(params2) {
			return len(params1) < len(params2)
		}

		// Now, we prioritise the params.
		for i := range params1 {
			operand1 := params1[i]
			operand2 := params2[i]
			if operand1 != operand2 {
				op1 := operandPriority[operand1.Syntax]
				op2 := operandPriority[operand2.Syntax]
				if op1 == 0 || op2 == 0 {
					panic("failed to sort " + mnemonic1 + ": bad syntax:\n" + inst1.Syntax + "\n" + inst2.Syntax)
				}

				return op1 < op2
			}
		}

		rex1 := inst1.Encoding.REX
		rex2 := inst2.Encoding.REX
		if rex1 != rex2 {
			return rex2 // Prefer non-REX over REX.
		}

		rexW1 := inst1.Encoding.REX_W
		rexW2 := inst2.Encoding.REX_W
		if rexW1 != rexW2 {
			return rexW2 // Prefer non-REX over REX.
		}

		evex1 := inst1.Encoding.EVEX
		evex2 := inst2.Encoding.EVEX
		if evex1 != evex2 {
			return evex2 // Prefer non-EVEX over EVEX.
		}

		vex1 := inst1.Encoding.VEX
		vex2 := inst2.Encoding.VEX
		if vex1 != vex2 {
			return vex2 // Prefer non-VEX over VEX.
		}

		vector1 := inst1.Encoding.VectorSize()
		vector2 := inst2.Encoding.VectorSize()
		if vector1 != vector2 {
			return vector1 < vector2 // Prefer smaller vector sizes.
		}

		// This shouldn't happen.
		panic(fmt.Sprintf("bad instruction: %s: no difference:\n%s    %s (%v)\n%s    %s (%v)", mnemonic1,
			inst1.Syntax, inst1.Encoding.Syntax, inst1.Parameters,
			inst2.Syntax, inst2.Encoding.Syntax, inst2.Parameters))
	})
}

var operandPriority map[string]int

func init() {
	operandPriority = make(map[string]int, len(operandOrder))
	for i, operand := range operandOrder {
		operandPriority[operand.Syntax] = i + 1
	}
}

// This is used when sorting instructions
// to prioritise smaller machine code.
var operandOrder = [...]*x86.Parameter{
	x86.Param0,
	x86.Param1,
	x86.Param2,
	x86.Param3,
	x86.Param4,
	x86.Param5,
	x86.Param6,
	x86.Param7,
	x86.Param8,
	x86.Param9,

	x86.ParamAL,
	x86.ParamCL,
	x86.ParamAX,
	x86.ParamDX,
	x86.ParamEAX,
	x86.ParamECX,
	x86.ParamRAX,

	x86.ParamES,
	x86.ParamCS,
	x86.ParamSS,
	x86.ParamDS,
	x86.ParamFS,
	x86.ParamGS,

	x86.ParamCR8,
	x86.ParamST,
	x86.ParamXMM0,

	x86.ParamStrDst8,
	x86.ParamStrDst16,
	x86.ParamStrDst32,
	x86.ParamStrDst64,
	x86.ParamStrSrc8,
	x86.ParamStrSrc16,
	x86.ParamStrSrc32,
	x86.ParamStrSrc64,

	x86.ParamImm8,
	x86.ParamImm16,
	x86.ParamImm32,
	x86.ParamImm64,
	x86.ParamImm5u,
	x86.ParamImm8u,
	x86.ParamImm16u,
	x86.ParamImm32u,
	x86.ParamImm64u,

	x86.ParamRel8,
	x86.ParamRel16,
	x86.ParamRel32,
	x86.ParamPtr16v16,
	x86.ParamPtr16v32,

	x86.ParamSTi,

	x86.ParamR8,
	x86.ParamR16,
	x86.ParamR32,
	x86.ParamR64,
	x86.ParamK1,
	x86.ParamSreg,
	x86.ParamCR0toCR7,
	x86.ParamDR0toDR7,
	x86.ParamMM1,
	x86.ParamXMM1,
	x86.ParamYMM1,
	x86.ParamZMM1,

	x86.ParamRmr8,
	x86.ParamRmr16,
	x86.ParamRmr32,
	x86.ParamRmr64,
	x86.ParamK2,
	x86.ParamMM2,
	x86.ParamXMM2,
	x86.ParamYMM2,
	x86.ParamZMM2,

	x86.ParamR8op,
	x86.ParamR16op,
	x86.ParamR32op,
	x86.ParamR64op,

	x86.ParamR8V,
	x86.ParamR16V,
	x86.ParamR32V,
	x86.ParamR64V,
	x86.ParamKV,
	x86.ParamXMMV,
	x86.ParamYMMV,
	x86.ParamZMMV,

	x86.ParamXMMIH,
	x86.ParamYMMIH,
	x86.ParamZMMIH,

	x86.ParamM,
	x86.ParamM8,
	x86.ParamM16,
	x86.ParamM32,
	x86.ParamM64,
	x86.ParamM80bcd,
	x86.ParamM80dec,
	x86.ParamM128,
	x86.ParamM256,
	x86.ParamM384,
	x86.ParamM512,
	x86.ParamM512byte,
	x86.ParamM32fp,
	x86.ParamM32bcst,
	x86.ParamM64fp,
	x86.ParamM64bcst,
	x86.ParamM80fp,
	x86.ParamM16int,
	x86.ParamM32int,
	x86.ParamM64int,
	x86.ParamM16v16,
	x86.ParamM16v32,
	x86.ParamM16v64,
	x86.ParamM16x16,
	x86.ParamM16x32,
	x86.ParamM16x64,
	x86.ParamM32x32,
	x86.ParamM2byte,
	x86.ParamM14l28byte,
	x86.ParamM94l108byte,

	x86.ParamMoffs8,
	x86.ParamMoffs16,
	x86.ParamMoffs32,
	x86.ParamMoffs64,
}
