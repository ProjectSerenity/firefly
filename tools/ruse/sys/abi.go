// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package sys

import (
	"fmt"

	"firefly-os.dev/tools/ruse/internal/x86"
)

// Location represents a single location in memory.
// This is used to describe aspects of a function's
// calling convention. A location will typically be
// either a CPU register or an offset into the call
// stack.
type Location interface {
	IsRegister() bool
	String() string
}

var (
	_ Location = (*x86.Register)(nil)
)

// Stack represents a location on the stack, which
// is a common Location.
type Stack struct {
	Pointer Location // The stack pointer.
	Offset  int      // An offset from the stack pointer.
}

var _ Location = Stack{}

func (s Stack) IsRegister() bool { return false }
func (s Stack) String() string   { return fmt.Sprintf("%s%+d", s.Pointer, s.Offset) }

// An ABI is used to determine the calling convention
// for a function. This consists of the memory locations
// where its parameters and results are stored, plus the
// memory locations that are available as scratch space
// to the function.
//
// Each architecture will have a default ABI, plus
// additional ABIs can be created in Ruse code.
type ABI struct {
	// Whether parameters and results passed on
	// the stack are pushed in order. If true,
	// earlier values will be further from the
	// stack pointer.
	InvertedStack bool

	// The sequence of registers available to be
	// used to carry parameters. If the ABI passes
	// all parameters on the stack, ParameterRegisters
	// will be empty.
	ParamRegisters []Location

	// The sequence of registers available to be
	// used to carry results. If the ABI passes
	// all results on the stack, ResultRegisters
	// will be empty.
	ResultRegisters []Location

	// The set of registers that a function may
	// overwrite at will and thus must be preserved
	// by the caller if needed after the function
	// call.
	ScratchRegisters []Location

	// The set of registers that a function must
	// preserve or leave unused.
	UnusedRegisters []Location
}

// roundUpLocationSize returns the given memory
// location size, rounded up to the next location
// size if not already an exact location size.
func (a *Arch) roundUpLocationSize(size int) int {
	rem := size % a.LocationSize
	if rem == 0 {
		return size
	}

	return size + (a.LocationSize - rem)
}

// allocator is used to allocate memory locations.
type allocator struct {
	Arch      *Arch
	ABI       *ABI
	Registers []Location
	TotalSize int

	nextRegister    int
	nextStackOffset int
	sizeSeen        int
}

// Allocate is a helper function used to allocate
// memory locations. It is called by Parameters
// and Result.
func (a *allocator) Allocate(size int) []Location {
	// Determine how many locations we
	// need.
	var locations int
	for size > 0 {
		locations++
		size -= a.Arch.LocationSize
	}

	// Allocate the locations.
	loc := make([]Location, locations)
	for j := range loc {
		a.sizeSeen += a.Arch.LocationSize

		// Take a register if possible.
		if a.nextRegister < len(a.Registers) {
			loc[j] = a.Registers[a.nextRegister]
			a.nextRegister++
			continue
		}

		if a.ABI.InvertedStack {
			a.nextStackOffset = a.TotalSize - a.sizeSeen
		}

		offset := a.nextStackOffset
		if !a.Arch.StackGrowsDown {
			offset = -offset
		}

		loc[j] = Stack{
			Pointer: a.Arch.StackPointer,
			Offset:  offset,
		}

		a.nextStackOffset += a.Arch.LocationSize
	}

	return loc
}

// Parameters allocates memory locations to the
// parameters to a function. The caller passes
// a slice representing the parameters. Each
// element describes the size of the parameter
// in bytes. The result is the set of memory
// locations. Large parameters (such as strings)
// may require multiple memory locations.
func (arch *Arch) Parameters(abi *ABI, sizes []int) [][]Location {
	// Use the default ABI if necessary.
	if abi == nil {
		abi = &arch.DefaultABI
	}

	// First, we sum the sizes which can be
	// necessary for determining stack
	// locations if they are not pushed in
	// reverse order.
	sizeSum := 0
	for _, size := range sizes {
		sizeSum += arch.roundUpLocationSize(size)
	}

	alloc := allocator{
		Arch:      arch,
		ABI:       abi,
		Registers: abi.ParamRegisters,
		TotalSize: sizeSum,
	}

	out := make([][]Location, len(sizes))
	for i, size := range sizes {
		out[i] = alloc.Allocate(size)
	}

	return out
}

// Result allocates memory locations to the
// result of a function. The caller passes
// the size of the result in bytes. The result
// is the set of memory locations. Large
// results (such as strings) may require
// multiple memory locations.
func (arch *Arch) Result(abi *ABI, size int) []Location {
	// Use the default ABI if necessary.
	if abi == nil {
		abi = &arch.DefaultABI
	}

	alloc := allocator{
		Arch:      arch,
		ABI:       abi,
		Registers: abi.ResultRegisters,
		TotalSize: arch.roundUpLocationSize(size),
	}

	return alloc.Allocate(size)
}

// Validate checks that the ABI is
// internally consistent for the given
// architecture.
func (arch *Arch) Validate(abi *ABI) error {
	// Check that all of the registers in
	// the ABI are in the list of ABI
	// registers in the architecture.
	//
	// We also allow the stack pointer
	// in the set of unused registers.
	seen := make(map[Location]bool)
	for _, reg := range arch.ABIRegisters {
		if !reg.IsRegister() {
			return fmt.Errorf("invalid ABI register %s: not a register", reg)
		}

		seen[reg] = false
	}

	for _, reg := range abi.ParamRegisters {
		repeat, ok := seen[reg]
		if !ok {
			return fmt.Errorf("invalid parameter register %s: not an ABI register for %s", reg, arch.Name)
		}

		if repeat {
			return fmt.Errorf("invalid parameter register %s: repeated in parameter registers", reg)
		}

		seen[reg] = true
	}

	// Reset the map.
	for reg := range seen {
		seen[reg] = false
	}

	for _, reg := range abi.ResultRegisters {
		repeat, ok := seen[reg]
		if !ok {
			return fmt.Errorf("invalid result register %s: not an ABI register for %s", reg, arch.Name)
		}

		if repeat {
			return fmt.Errorf("invalid result register %s: repeated in result registers", reg)
		}

		seen[reg] = true
	}

	// Reset the map.
	for reg := range seen {
		seen[reg] = false
	}

	for _, reg := range abi.ScratchRegisters {
		repeat, ok := seen[reg]
		if !ok {
			return fmt.Errorf("invalid scratch register %s: not an ABI register for %s", reg, arch.Name)
		}

		if repeat {
			return fmt.Errorf("invalid scratch register %s: repeated in scratch registers", reg)
		}

		seen[reg] = true
	}

	// Reset the map.
	for reg := range seen {
		seen[reg] = false
	}

	seenStackPointer := false
	for _, reg := range abi.UnusedRegisters {
		if reg == arch.StackPointer {
			if seenStackPointer {
				return fmt.Errorf("invalid unused register %s: repeated in unused registers", reg)
			}

			seenStackPointer = true
			continue
		}

		repeat, ok := seen[reg]
		if !ok {
			return fmt.Errorf("invalid unused register %s: not an ABI register for %s", reg, arch.Name)
		}

		if repeat {
			return fmt.Errorf("invalid unused register %s: repeated in unused registers", reg)
		}

		seen[reg] = true
	}

	// Check that unused registers really
	// are unused.
	usage := make(map[Location][]string)
	for _, reg := range abi.ParamRegisters {
		usage[reg] = append(usage[reg], "parameter")
	}
	for _, reg := range abi.ResultRegisters {
		usage[reg] = append(usage[reg], "result")
	}
	for _, reg := range abi.ScratchRegisters {
		usage[reg] = append(usage[reg], "scratch")
	}

	for _, reg := range abi.UnusedRegisters {
		if uses := usage[reg]; len(uses) != 0 {
			var text string
			switch len(uses) {
			case 1:
				text = uses[0]
			case 2:
				text = fmt.Sprintf("%s and %s", uses[0], uses[1])
			case 3:
				text = fmt.Sprintf("%s, %s, and %s", uses[0], uses[1], uses[2])
			default:
				panic(len(uses))
			}

			return fmt.Errorf("invalid unused register %s: also listed as %s register", reg, text)
		}
	}

	return nil
}
