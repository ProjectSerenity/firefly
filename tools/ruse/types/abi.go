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

func NewABI(abi *sys.ABI) ABI {
	return ABI{abi: abi}
}

func NewRawABI(arch *sys.Arch, invertedStack *ast.Identifier, params, result, scratch, unused []*ast.Identifier) (ABI, error) {
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
		reg, ok := arch.RegisterNames[param.Name]
		if !ok {
			return ABI{}, fmt.Errorf("invalid ABI: bad parameter register %s: register not recognised for %s", param.Name, arch.Name)
		}

		abi.ParamRegisters = append(abi.ParamRegisters, reg)
	}

	for _, result := range result {
		reg, ok := arch.RegisterNames[result.Name]
		if !ok {
			return ABI{}, fmt.Errorf("invalid ABI: bad result register %s: register not recognised for %s", result.Name, arch.Name)
		}

		abi.ResultRegisters = append(abi.ResultRegisters, reg)
	}

	for _, scratch := range scratch {
		reg, ok := arch.RegisterNames[scratch.Name]
		if !ok {
			return ABI{}, fmt.Errorf("invalid ABI: bad scratch register %s: register not recognised for %s", scratch.Name, arch.Name)
		}

		abi.ScratchRegisters = append(abi.ScratchRegisters, reg)
	}

	if unused != nil {
		for _, unused := range unused {
			reg, ok := arch.RegisterNames[unused.Name]
			if !ok {
				return ABI{}, fmt.Errorf("invalid ABI: bad unused register %s: register not recognised for %s", unused.Name, arch.Name)
			}

			abi.UnusedRegisters = append(abi.UnusedRegisters, reg)
		}
	} else {
		// Derive the unused registers from
		// the other registers.
		//
		// We do this by deleting from the
		// registers map each register in
		// the used set.
		registers := make(map[string]sys.Location)
		for _, reg := range arch.ABIRegisters {
			registers[reg.String()] = reg
		}

		for _, reg := range params {
			delete(registers, reg.Name)
		}
		for _, reg := range result {
			delete(registers, reg.Name)
		}
		for _, reg := range scratch {
			delete(registers, reg.Name)
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

func (a ABI) ABI() *sys.ABI    { return a.abi }
func (a ABI) Underlying() Type { return a }
func (a ABI) String() string   { return "abi" }
