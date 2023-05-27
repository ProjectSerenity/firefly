// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"firefly-os.dev/tools/ruse/internal/x86"
)

// ParameterCombinations takes a set of x86
// instruction arguments in Intel syntax and
// produces the set of parameter combinations.
//
// For simple argument sets, there will be a
// single combination, matching the arguments.
// For more complex arguments, these may be
// split up into multiple versions of each
// argument, producing a larger number of
// combinations.
//
// The result is either an error or an arbitrary
// number of combinations, where each combination
// is a sequence of parameters equal in length to
// the number of arguments, and in the same order.
func ParameterCombinations(args []string) (combinations [][]*x86.Parameter, err error) {
	if len(args) == 0 {
		return nil, nil
	}

	optionSets := make([][]*x86.Parameter, len(args))
	for i, arg := range args {
		// Normalise the arg.
		switch arg {
		case "mem":
			arg = "m"
		case "mm", "xmm", "ymm":
			arg += "1"
		case "ST(0)":
			arg = "ST"
		}

		// Select the parameter value.
		var options []*x86.Parameter
		if got, ok := commonExpansions[arg]; ok {
			options = got
		} else {
			param, ok := x86.Parameters[arg]
			if !ok {
				return nil, fmt.Errorf("could not find parameter definition for %q", arg)
			}

			options = []*x86.Parameter{param}
		}

		optionSets[i] = options
	}

	// Expand the combinations.
	numOptions := 1
	for _, set := range optionSets {
		numOptions *= len(set)
	}

	combinations = make([][]*x86.Parameter, numOptions)
	for i := range combinations {
		combinations[i] = make([]*x86.Parameter, len(args))
	}

	// Iterate through the sets of options,
	// distributing them evenly across the
	// combinations.
	//
	// We do this by iterating through each
	// of the groups of options, starting
	// with the first of each (indices is
	// initialised to a set of 0s). After
	// each combination, we iterate the
	// last index. If that exceeds the
	// number of options in the last set,
	// we reset that and iterate the next
	// index, until we reach the last set.
	indices := make([]int, len(optionSets))
	for i := range combinations {
		for j, k := range indices {
			combinations[i][j] = optionSets[j][k]
		}

		for n := len(indices) - 1; n >= 0; n-- {
			indices[n]++
			if indices[n] < len(optionSets[n]) {
				break
			}

			indices[n] = 0
		}
	}

	return combinations, nil
}

var commonExpansions = map[string][]*x86.Parameter{
	"k2/m16":                 {x86.ParamK2, x86.ParamM16},
	"k2/m32":                 {x86.ParamK2, x86.ParamM32},
	"k2/m64":                 {x86.ParamK2, x86.ParamM64},
	"k2/m8":                  {x86.ParamK2, x86.ParamM8},
	"mm2/m16":                {x86.ParamMM2, x86.ParamM16},
	"mm2/m32":                {x86.ParamMM2, x86.ParamM32},
	"mm2/m64":                {x86.ParamMM2, x86.ParamM64},
	"r16/r32":                {x86.ParamR16, x86.ParamR32},
	"r16/r32/m16":            {x86.ParamRmr16, x86.ParamRmr32, x86.ParamM16},
	"r16/r32/r64":            {x86.ParamR16, x86.ParamR32, x86.ParamR64},
	"r32/m16":                {x86.ParamRmr32, x86.ParamM16},
	"r32/m8":                 {x86.ParamRmr32, x86.ParamM8},
	"r32/r64":                {x86.ParamR32, x86.ParamR64},
	"r64/m16":                {x86.ParamRmr64, x86.ParamM16},
	"reg":                    {x86.ParamR8, x86.ParamR16, x86.ParamR32, x86.ParamR64},
	"rmr8/rmr16/rmr32/rmr64": {x86.ParamRmr8, x86.ParamRmr16, x86.ParamRmr32, x86.ParamRmr64},
	"rmr16/rmr32":            {x86.ParamRmr16, x86.ParamRmr32},
	"rmr16/rmr32/rmr64":      {x86.ParamRmr16, x86.ParamRmr32, x86.ParamRmr64},
	"r/m16":                  {x86.ParamRmr16, x86.ParamM16},
	"r/m32":                  {x86.ParamRmr32, x86.ParamM32},
	"r/m64":                  {x86.ParamRmr64, x86.ParamM64},
	"r/m8":                   {x86.ParamRmr8, x86.ParamM8},
	"xmm2/m32/m16bcst":       {x86.ParamXMM2, x86.ParamM32, x86.ParamM16bcst},
	"xmm2/m64/m16bcst":       {x86.ParamXMM2, x86.ParamM64, x86.ParamM16bcst},
	"xmm2/m128/m16bcst":      {x86.ParamXMM2, x86.ParamM128, x86.ParamM16bcst},
	"xmm2/m128/m32bcst":      {x86.ParamXMM2, x86.ParamM128, x86.ParamM32bcst},
	"xmm2/m128/m64bcst":      {x86.ParamXMM2, x86.ParamM128, x86.ParamM64bcst},
	"xmm2/m128":              {x86.ParamXMM2, x86.ParamM128},
	"xmm2/m16":               {x86.ParamXMM2, x86.ParamM16},
	"xmm2/m32":               {x86.ParamXMM2, x86.ParamM32},
	"xmm2/m64/m32bcst":       {x86.ParamXMM2, x86.ParamM64, x86.ParamM32bcst},
	"xmm2/m64":               {x86.ParamXMM2, x86.ParamM64},
	"xmm2/m8":                {x86.ParamXMM2, x86.ParamM8},
	"ymm2/m256/m16bcst":      {x86.ParamYMM2, x86.ParamM256, x86.ParamM16bcst},
	"ymm2/m256/m32bcst":      {x86.ParamYMM2, x86.ParamM256, x86.ParamM32bcst},
	"ymm2/m256/m64bcst":      {x86.ParamYMM2, x86.ParamM256, x86.ParamM64bcst},
	"ymm2/m256":              {x86.ParamYMM2, x86.ParamM256},
	"zmm2/m512/m16bcst":      {x86.ParamZMM2, x86.ParamM512, x86.ParamM16bcst},
	"zmm2/m512/m32bcst":      {x86.ParamZMM2, x86.ParamM512, x86.ParamM32bcst},
	"zmm2/m512/m64bcst":      {x86.ParamZMM2, x86.ParamM512, x86.ParamM64bcst},
	"zmm2/m512":              {x86.ParamZMM2, x86.ParamM512},
}
