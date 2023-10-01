// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package compiler

import (
	"fmt"
	"go/constant"
	"slices"
	"strings"

	"firefly-os.dev/tools/ruse/ssafir"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

// # Register allocation
//
// We take a fairly straightforward approach to register
// allocation, in part to accommodate our flexible support
// for ABIs. This could definitely be optimised further.
//
// We start by determining the set of general-purpose
// registers available to us, prioritising those that are
// caller-preserved, as using them does not require us to
// save or restore their previous values.
//
// We then iterate chronologically through the function,
// assigning values to registers as needed. When a value
// is no longer needed, we mark the register as available.
//
// When calling a function, we start by saving any caller-preserved
// registers, either by 'dodging' into another register
// or by 'spilling' onto the stack. We then copy the
// function parameters into position, according to the
// called function's ABI.
//
// Note that we only move values lazily when needed.
// We don't do any prior analysis of the ABIs of any
// functions called by the function. This means that
// we may do an unnecessary number of copies by dodging
// values between conflicting caller-saved registers,
// but it keeps the implementation simple and fast.

// Alloc contains the information added to a SSAFIR
// instruction that moves data to an allocated register.
//
// Exactly one of `Src` or `Data` will be non-nil.
type Alloc struct {
	Dst  sys.Location // The destination register or stack location.
	Src  sys.Location // The source register or stack location (or nil).
	Data any          // The source data for constants (or nil).
}

// Allocate passes through the set of values in the
// function, allocating each used value to a memory
// location and tracking these through the life of
// the function. This produces an implementation
// that is ready to be lowered to assembly.
func Allocate(fset *token.FileSet, arch *sys.Arch, sizes types.Sizes, pkg *Package, fun *ssafir.Function) error {
	return newAllocator(fset, arch, sizes, pkg, fun).run()
}

// run is the main loop for register allocation.
func (a *allocator) run() error {
	// First, we iterate through the values
	// to detect those that are never used
	// and have no side effects.
	var ignoreIdempotent func(v *ssafir.Value)
	ignoreIdempotent = func(v *ssafir.Value) {
		switch v.Op {
		case ssafir.OpParameter,
			ssafir.OpCopy:
			if v.Uses == 0 {
				for _, arg := range v.Args {
					arg.Uses--
					ignoreIdempotent(arg)
				}
			}
		}
	}

	for _, v := range a.function.Entry.Values {
		ignoreIdempotent(v)
	}

	// Next, we iterate through the values
	// so that we can detect the point at
	// which each value is referenced for
	// the last time. This means that we
	// can drop values as soon as they are
	// no longer needed. This allows us to
	// avoid unnecessary copies by avoiding
	// tracking values we will not use any
	// more.
	//
	// We do this by keeping track of the
	// value index whenever each value is
	// consumed, then inverting it to the
	// set of values dropped at each index.
	lastUseIndex := make(map[*ssafir.Value]int)
	for i, v := range a.function.Entry.Values {
		for _, arg := range v.Args {
			lastUseIndex[arg] = i
		}
	}

	droppedValues := make([][]*ssafir.Value, len(a.function.Entry.Values))
	for v, i := range lastUseIndex {
		droppedValues[i] = append(droppedValues[i], v)
	}

	calleeIsScratch := make(map[sys.Location]bool, len(a.registers))
	for i, v := range a.function.Entry.Values {
		dropped := droppedValues[i]
		switch v.Op {
		case ssafir.OpMakeMemoryState:
			// We can ignore these.
		case ssafir.OpMakeResult:
			a.PrepareResult(v)

			// Drop the input if it's not
			// used again.
			for _, v := range dropped {
				if v != a.function.Entry.Control {
					a.DropValue(v)
				}
			}
		case ssafir.OpParameter:
			a.NoteParameter(v)
		case ssafir.OpConstantInt64,
			ssafir.OpConstantUint64,
			ssafir.OpConstantString,
			ssafir.OpConstantUntypedInt:
			// No need to do anything here,
			// we'll pull the value when it's
			// used.
		case ssafir.OpCopy:
			// If we're dropping the input, then
			// it's a move. If not, it's a full
			// copy.
			if len(dropped) == 1 && len(v.Args) == 1 && dropped[0] == v.Args[0] {
				a.MoveValue(v, v.Args[0])
			} else {
				a.AddValue(v)
			}
		case ssafir.OpFunctionCall:
			fun := v.Extra.(*types.Function)
			sig := fun.Type().(*types.Signature)
			params := sig.Params()
			args := make([]int, len(params))
			for i, arg := range v.Args {
				args[i] = a.sizes.SizeOf(arg.Type)
			}

			// Preserve any values currently in
			// the function's scratch registers.
			calleeABI := fun.ABI()
			if calleeABI == nil {
				calleeABI = &a.arch.DefaultABI
			}

			clear(calleeIsScratch)
			for _, reg := range calleeABI.ScratchRegisters {
				calleeIsScratch[reg] = true
			}
			for _, reg := range calleeABI.ScratchRegisters {
				a.SaveValue(reg, calleeIsScratch)
			}

			// Copy the parameters to the parameter
			// registers, according to the callee's
			// ABI.
			locs := a.arch.Parameters(calleeABI, args)
			for i, v := range v.Args {
				a.PrepareParameter(fun, sig, locs[i], v)
			}

			// Perform the function call itself.
			a.allocs = append(a.allocs, v)

			// Note any results.
			if v.Uses != 0 && sig.Result() != nil {
				a.NoteResult(fun, sig, v)
			}
		default:
			return fmt.Errorf("failed to allocate value %s: unexpected op %s", v, v.Op)
		}
	}

	a.block.Values = a.allocs
	a.function.Entry = a.block
	a.function.Blocks = []*ssafir.Block{a.block}

	return nil
}

// allocator implements a register allocator for
// use in the compiler. An allocator is initialised
// with a set of available registers. These are
// then populated by any parameters to the function,
// according to its calling convention. At this
// point, the allocator is now ready.
//
// The compiler then proceeds through the function,
// updating the allocator with requirements, such
// as new values being created and values being
// assigned to registers in preparation for function
// calls. At the point of a function call, various
// movements may be performed to avoid scratch
// registers and function results.
type allocator struct {
	fset      *token.FileSet
	arch      *sys.Arch
	pkg       *Package
	sizes     types.Sizes
	block     *ssafir.Block
	allocs    []*ssafir.Value
	registers []sys.Location
	abi       *sys.ABI
	function  *ssafir.Function
	allocated map[sys.Location]*ssafir.Value
	locations map[*ssafir.Value][]sys.Location
	stack     []*ssafir.Value
}

// newAllocator prepares a register allocator for
// the given function.
func newAllocator(fset *token.FileSet, arch *sys.Arch, sizes types.Sizes, pkg *Package, fun *ssafir.Function) *allocator {
	// Take a copy of the registers, placing
	// scratch registers at the start so that
	// we avoid using callee-preserved as much
	// as possible.
	abi := fun.Func.ABI()
	if abi == nil {
		abi = &arch.DefaultABI // We don't modify the ABI, so a shallow copy is fine.
	}

	registers := slices.Clone(arch.ABIRegisters)
	isScratch := make(map[sys.Location]bool, len(abi.ScratchRegisters))
	for _, reg := range abi.ScratchRegisters {
		isScratch[reg] = true
	}

	slices.SortStableFunc(registers, func(a, b sys.Location) int {
		// Prioritise scratch registers over
		// callee-preserved, then fall back
		// to alphabetical comparison so that
		// we are consistent.
		aS := isScratch[a]
		bS := isScratch[b]
		if aS && !bS {
			return -1
		}

		if !aS && bS {
			return 1
		}

		return 0
	})

	block := &ssafir.Block{
		ID:           fun.Entry.ID,
		Kind:         fun.Entry.Kind,
		Likely:       fun.Entry.Likely,
		Successors:   fun.Entry.Successors,
		Predecessors: fun.Entry.Predecessors,
		Control:      fun.Entry.Control,
		Function:     fun.Entry.Function,
		Pos:          fun.Entry.Pos,
		End:          fun.Entry.End,
	}

	a := &allocator{
		fset:  fset,
		arch:  arch,
		pkg:   pkg,
		sizes: sizes,
		// We don't ever modify registers, so
		// it's fine to use a shallow copy.
		registers: registers,
		abi:       abi,
		block:     block,
		function:  fun,
		allocated: make(map[sys.Location]*ssafir.Value, len(arch.ABIRegisters)),
		locations: make(map[*ssafir.Value][]sys.Location, len(arch.ABIRegisters)),
	}

	return a
}

// addAlloc creates a copy of the given SSAFIR
// value, setting its Extra to the given alloc,
// adding the result to `a.allocs`.
func (a *allocator) addAlloc(v *ssafir.Value, alloc *Alloc) {
	a.addOpAlloc(v, v.Op, alloc)
}

// addOpAlloc creates a copy of the given SSAFIR
// value, setting its `Op` and `Extra` to the given
// `op` and `alloc`, adding the result to
// `a.allocs`.
func (a *allocator) addOpAlloc(v *ssafir.Value, op ssafir.Op, alloc *Alloc) {
	v2 := &ssafir.Value{
		ID:    v.ID,
		Op:    op,
		Type:  v.Type,
		Extra: alloc,
		Args:  v.Args,
		Block: a.block,
		Pos:   v.Pos,
		End:   v.End,
		Uses:  v.Uses,
	}

	a.allocs = append(a.allocs, v2)
}

// Debug prints a verbose representation of the
// allocator's current state.
func (a *allocator) Debug() string {
	var b strings.Builder
	fmt.Fprintf(&b, "allocator for %s %s\n", a.function.Name, a.function.Type)

	// Start with the registers.
	for _, reg := range a.registers {
		v := a.allocated[reg]
		if v == nil {
			fmt.Fprintf(&b, "  %-5s [free]\n", reg.String()+":")
		} else {
			fmt.Fprintf(&b, "  %-5s %s\n", reg.String()+":", v)
		}
	}

	// Next, do the stack.
	// The earlier indices hold the
	// values furthest from the stack
	// pointer, as we append/truncate.
	stackMul := -1
	if a.arch.StackGrowsDown {
		stackMul = +1
	}

	reg := a.arch.StackPointer
	for i, v := range a.stack {
		offset := i * stackMul * a.arch.LocationSize
		if v == nil {
			fmt.Fprintf(&b, "  %s%+d: [empty]\n", reg, offset)
		} else {
			fmt.Fprintf(&b, "  %s%+d: %s\n", reg, offset, v)
		}
	}

	return b.String()
}

// NoteParameter takes note of the fact that the
// given parameter already exists in a memory
// location determined by the function's calling
// convention.
//
// NoteParameter must be called at the beginning
// of the function, before other allocations, or
// it may panic.
func (a *allocator) NoteParameter(v *ssafir.Value) {
	locs := a.function.Params[v.ExtraInt]
	for _, loc := range locs {
		if other := a.allocated[loc]; other != nil {
			panic(fmt.Sprintf("NoteParameter(%d, %s): location %s is already occupied by %s", v.ExtraInt, v, loc, other))
		}

		a.addAlloc(v, &Alloc{Dst: loc, Data: v.ExtraInt})

		if v.Uses != 0 {
			a.allocated[loc] = v
		}
	}

	if v.Uses == 0 {
		return
	}

	if a.locations[v] != nil {
		panic(fmt.Sprintf("NoteParameter(%d, %s): value %s is already allocated", v.ExtraInt, v, v))
	}

	a.locations[v] = append(make([]sys.Location, 0, len(locs)), locs...) // Make a deep copy, so we can reuse it over time.
}

// AddValue records the given value as being
// available. This will only be allocated to
// a memory location lazily, when necessary.
//
// This ensures that unused values are dropped.
func (a *allocator) AddValue(v *ssafir.Value) {
	if a.locations[v] != nil {
		panic(fmt.Sprintf("AddValue(%s): value %s is already recorded", v, v))
	}

	a.addAlloc(v, &Alloc{Data: v.Extra}) // This probably needs to be removed.

	if v.Uses != 0 {
		panic("unimplemented")
	}
}

// SaveValue moves any value in the given
// register to another register (if possible)
// or the stack. The given list of scratch
// registers will be avoided.
func (a *allocator) SaveValue(reg sys.Location, avoid map[sys.Location]bool) {
	value := a.allocated[reg]
	if value == nil {
		// Nothing to save.
		return
	}

	for _, candidate := range a.registers {
		if avoid[candidate] {
			// We cannot use this register.
			continue
		}

		if a.allocated[candidate] != nil {
			// This register is occupied.
			continue
		}

		// We can save to candidate.
		a.allocated[candidate] = value
		a.allocated[reg] = nil
		a.addAlloc(value, &Alloc{Dst: candidate, Src: reg})

		locs := a.locations[value]
		for i, loc := range locs {
			if loc == reg {
				locs[i] = candidate
				break
			}
		}

		return
	}

	// No registers are available,
	// so we spill to the stack.
	// TODO: implement spilling to the stack.
}

// MoveValue records the new value as taking
// the place of the old.
//
// This drops the old value.
func (a *allocator) MoveValue(new, old *ssafir.Value) {
	// Replace old with new in situ.
	if new.Uses != 0 {
		a.locations[new] = a.locations[old]
	}

	if new.Uses == 0 && old.Uses == 0 {
		a.allocs = append(a.allocs, new)
	}

	for _, loc := range a.locations[old] {
		a.addAlloc(new, &Alloc{Dst: loc, Src: loc})
		if new.Uses != 0 {
			a.allocated[loc] = new
		}
	}

	delete(a.locations, old)
}

// DropValue removes the given value and marks
// its memory location (if any) as free.
func (a *allocator) DropValue(v *ssafir.Value) {
	for _, loc := range a.locations[v] {
		a.allocated[loc] = nil // We don't delete, so we know the location has been used.
		a.addOpAlloc(v, ssafir.OpDrop, &Alloc{Src: loc})
	}

	delete(a.locations, v)
}

// PrepareResult ensures that the function's
// result value (if any) is in the appropriate
// memory location(s).
func (a *allocator) PrepareResult(v *ssafir.Value) {
	if a.function.Type.Result() == nil {
		// No result, nothing to do.
		return
	}

	// Add move actions.
	for i, loc := range a.function.Result {
		oldLoc := a.locations[v.Args[0]][i]

		// Drop the old value.
		if old := a.allocated[loc]; old != nil {
			var truncated []sys.Location
			for _, loc2 := range a.locations[old] {
				if loc2 != loc {
					truncated = append(truncated, loc2)
				}
			}

			a.locations[old] = append(a.locations[old][:0], truncated...)
		}

		a.allocated[loc] = v
		a.addAlloc(v, &Alloc{Dst: loc, Src: oldLoc})
	}

	a.locations[v] = append(a.locations[v][:0], a.function.Result...)
}

// PrepareParameter ensures that the given
// function parameter is in the appropriate
// memory location(s).
func (a *allocator) PrepareParameter(fun *types.Function, sig *types.Signature, locs []sys.Location, v *ssafir.Value) {
	// Add move actions.
	for i, loc := range locs {
		a.allocated[loc] = v

		// Constants are floating.
		isConstant := func(v *ssafir.Value) bool {
			switch v.Op {
			case ssafir.OpConstantInt64, ssafir.OpConstantUint64,
				ssafir.OpConstantUntypedInt, ssafir.OpConstantString:
				return true
			}

			return false
		}

		if isConstant(v) {
			// String constants are a little more complex,
			// as they come in two parts.
			if s, ok := v.Extra.(string); ok {
				switch i {
				case 0:
					a.addAlloc(v, &Alloc{Dst: loc, Data: s})
				case 1:
					a.addOpAlloc(v, ssafir.OpConstantUntypedInt, &Alloc{Dst: loc, Data: int64(len(s))})
				}

				continue
			}

			if val, ok := v.Extra.(constant.Value); ok && val.Kind() == constant.String {
				switch i {
				case 0:
					a.addAlloc(v, &Alloc{Dst: loc, Data: val})
				case 1:
					s := constant.StringVal(val)
					a.addOpAlloc(v, ssafir.OpConstantUntypedInt, &Alloc{Dst: loc, Data: int64(len(s))})
				}

				continue
			}

			a.addAlloc(v, &Alloc{Dst: loc, Data: v.Extra})

			continue
		}

		// If we're copying a constant,
		// we float as above.
		if v.Op == ssafir.OpCopy && len(v.Args) == 1 && isConstant(v.Args[0]) {
			a.addAlloc(v, &Alloc{Dst: loc, Data: v.Args[0].Extra})

			continue
		}

		if i < len(a.locations[v]) {
			a.addOpAlloc(v, ssafir.OpCopy, &Alloc{Dst: loc, Src: a.locations[v][i]})
		}
	}

	a.locations[v] = append(a.locations[v][:0], locs...)
}

// NoteResult takes note of the fact that the
// given result already exists in a location
// determined by the function's calling
// convention.
func (a *allocator) NoteResult(fun *types.Function, sig *types.Signature, v *ssafir.Value) {
	locs := a.arch.Result(a.abi, a.sizes.SizeOf(v.Type))
	for _, loc := range locs {
		a.allocated[loc] = v
		a.addOpAlloc(v, ssafir.OpMakeResult, &Alloc{Dst: loc, Src: loc})
	}

	a.locations[v] = append(a.locations[v][:0], locs...)
}

// CalculatePreservations determines which
// callee-preserved registers have been used.
// This ensures that we can preserve those
// registers before (and restore them after)
// the function implementation.
//
// CalculatePreservations must be called
// immediately before the function's return,
// after calling [allocator.PrepareResult].
func (a *allocator) CalculatePreservations() (save, load []*ssafir.Value) {
	// TODO: implement CalculatePreservations.
	return nil, nil
}
