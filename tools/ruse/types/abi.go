// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"fmt"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/sys"
)

// A ABI represents an application
// binary interface, defined at
// compile time.
type ABI struct {
	abi *sys.ABI
}

var _ Type = ABI{}

func NewABI(arch *sys.Arch, invertedStack *ast.Identifier, params, result, scratch, unused []*ast.Identifier) (ABI, error) {
	// Build up the set of registers we support.
	registers := make(map[string]sys.Location, len(arch.ABIRegisters))
	for _, reg := range arch.ABIRegisters {
		registers[reg.String()] = reg
	}

	abi := &sys.ABI{
		ParamRegisters:   make([]sys.Location, 0, len(params)),
		ResultRegisters:  make([]sys.Location, 0, len(result)),
		ScratchRegisters: make([]sys.Location, 0, len(scratch)),
		UnusedRegisters:  make([]sys.Location, 0, len(arch.ABIRegisters)-len(scratch)),
	}

	// InvertedStack defaults to false if
	// absent.
	if invertedStack != nil {
		switch invertedStack.Name {
		case "true":
			abi.InvertedStack = true
		case "false":
		default:
			return ABI{}, fmt.Errorf("invalid ABI: bad inverted-stack value %s: want bool", invertedStack.Name)
		}
	}

	for _, param := range params {
		reg, ok := registers[param.Name]
		if !ok {
			return ABI{}, fmt.Errorf("invalid ABI: bad parameter register %s: not an ABI register for %s", param.Name, arch.Name)
		}

		abi.ParamRegisters = append(abi.ParamRegisters, reg)
	}

	for _, result := range result {
		reg, ok := registers[result.Name]
		if !ok {
			return ABI{}, fmt.Errorf("invalid ABI: bad result register %s: not an ABI register for %s", result.Name, arch.Name)
		}

		abi.ResultRegisters = append(abi.ResultRegisters, reg)
	}

	for _, scratch := range scratch {
		reg, ok := registers[scratch.Name]
		if !ok {
			return ABI{}, fmt.Errorf("invalid ABI: bad scratch register %s: not an ABI register for %s", scratch.Name, arch.Name)
		}

		abi.ScratchRegisters = append(abi.ScratchRegisters, reg)
	}

	if unused != nil {
		for _, unused := range unused {
			reg, ok := registers[unused.Name]
			if !ok {
				return ABI{}, fmt.Errorf("invalid ABI: bad unused register %s: not an ABI register for %s", unused.Name, arch.Name)
			}

			abi.UnusedRegisters = append(abi.UnusedRegisters, reg)
		}
	} else {
		// Derive the unused registers from
		// the scratch registers.
		//
		// We do this by deleting from the
		// registers map each register in
		// the scratch set.
		for _, scratch := range scratch {
			delete(registers, scratch.Name)
		}

		for _, reg := range arch.ABIRegisters {
			if registers[reg.String()] != nil {
				abi.UnusedRegisters = append(abi.UnusedRegisters, reg)
			}
		}
	}

	if err := arch.Validate(abi); err != nil {
		return ABI{}, err
	}

	return ABI{abi: abi}, nil
}

func (a ABI) Underlying() Type { return a }
func (a ABI) String() string   { return "abi" }
