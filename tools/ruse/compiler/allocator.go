// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package compiler

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"

	"firefly-os.dev/tools/ruse/constant"
	"firefly-os.dev/tools/ruse/internal/x86"
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

func (a *Alloc) String() string {
	if a.Dst == nil {
		// This should only happen in a drop.
		return a.Src.String()
	}

	if a.Src != nil {
		return fmt.Sprintf("move %s to %s", a.Src, a.Dst)
	}

	switch a.Data.(type) {
	case string:
		return fmt.Sprintf("move %q to %s", a.Data, a.Dst)
	default:
		return fmt.Sprintf("move %v to %s", a.Data, a.Dst)
	}
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
	// Process the function, block by block.
	a.Debugf("%s: starting at entry point for %s", a.function.Entry, a.function.Name)
	err := a.doBlock(make(map[*ssafir.Block]bool), a.function.Entry, nil)
	if err != nil {
		return err
	}

	// Identify whether we need to save any
	// registers, based on our ABI.
	used := make([]sys.Location, 0, len(a.abi.UnusedRegisters))
	for _, loc := range a.abi.UnusedRegisters {
		if _, ok := a.allocated[loc]; ok {
			used = append(used, loc)
		}
	}

	// Prepend any saves and then append any
	// restores in reverse order.
	if len(used) > 0 {
		pos := a.function.Code.Elements[0].Pos() // The 'func' keyword.
		end := a.function.Code.Elements[1].End() // The end of the signature.
		values := make([]*ssafir.Value, 0, len(a.function.Blocks[0].Values)+len(used))

		// Add the saves in forwards order to
		// the first block, which is the function
		// entry point.
		for i := 0; i < len(used); i++ {
			loc := used[i]
			a.Debugf("preserving callee-saved register %s", loc)
			values = append(values, &ssafir.Value{
				ID:    0, // This is special.
				Op:    ssafir.OpSaveRegister,
				Block: a.function.Entry,
				Pos:   pos,
				End:   end,
				Extra: loc,
			})
		}

		a.Debugf("prepending register preservation to block %s", a.function.Blocks[0])
		a.function.Blocks[0].Values = append(values, a.function.Blocks[0].Values...)

		// Add the restores in reverse order to
		// every return block.
		suffix := make([]*ssafir.Value, 0, len(used))
		for i := len(used); i > 0; i-- {
			loc := used[i-1]
			a.Debugf("restoring callee-saved register %s", loc)
			suffix = append(suffix, &ssafir.Value{
				ID:    0, // This is special.
				Op:    ssafir.OpRestoreRegister,
				Block: a.function.Entry,
				Pos:   pos,
				End:   end,
				Extra: loc,
			})
		}

		for _, block := range a.function.Blocks {
			if block.Kind == ssafir.BlockReturn {
				a.Debugf("appending register restoration to block %s", block)
				block.Values = append(block.Values, suffix...)
			}
		}
	}

	return nil
}

// doBlock performs register allocation for the
// given block. The resulting values are used to
// overwrite b.Values.
func (a *allocator) doBlock(done map[*ssafir.Block]bool, block, stopAt *ssafir.Block) error {
	if done[block] {
		a.Debugf("%s: skipping block, which has already been lowered", block)
		return nil
	}

	if block == stopAt {
		a.Debugf("%s: stopping early, as requested", block)
		return nil
	}

	done[block] = true

	a.block = block
	values := block.Values
	block.Values = nil

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
					a.Debugf("decrementing %s, as %s is unused", arg, v)
					arg.Uses--
					ignoreIdempotent(arg)
				}
			}
		}
	}

	for _, v := range values {
		ignoreIdempotent(v)
	}

	calleeIsScratch := make(map[sys.Location]bool, len(a.registers))
	for _, v := range values {
		switch v.Op {
		case ssafir.OpMakeMemoryState:
			// We can ignore these.
		case ssafir.OpMakeResult:
			a.Debugf("%s: preparing a result", v)
			a.PrepareResult(v)
		case ssafir.OpReturn:
			a.Debugf("%s: preparing a result", v)
			a.PrepareResult(v)

			// Finally, add a return with
			// no allocation to signal
			// where the return instruction
			// goes.
			a.Debugf("%s: adding a signalling value with no allocation", v)
			a.addAlloc(v, nil)
		case ssafir.OpParameter:
			a.Debugf("%s: noting a parameter", v)
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
			if len(v.Args) == 1 && v.Args[0].Uses == 1 {
				a.Debugf("%s: moving %s to %s", v, v.Args[0], v)
				a.MoveValue(v, v.Args[0])
			} else {
				a.Debugf("%s: copying %s to %s", v, v.Args[0], v)
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
			for _, reg := range calleeABI.ParamRegisters {
				calleeIsScratch[reg] = true
			}
			for _, reg := range calleeABI.ScratchRegisters {
				calleeIsScratch[reg] = true
			}
			for _, reg := range calleeABI.ResultRegisters {
				calleeIsScratch[reg] = true
			}
			for _, reg := range calleeABI.ScratchRegisters {
				a.Debugf("%s: %s: checking whether we need to save caller-saved register %s", v, fun.Name(), reg)
				a.SaveValue(reg, calleeIsScratch)
			}
			for _, reg := range calleeABI.ResultRegisters {
				a.Debugf("%s: %s: checking whether we need to save result register %s", v, fun.Name(), reg)
				a.SaveValue(reg, calleeIsScratch)
			}

			// Copy the parameters to the parameter
			// registers, according to the callee's
			// ABI.
			locs := a.arch.Parameters(calleeABI, args)
			for i, v := range v.Args {
				a.Debugf("%s: %s: preparing parameter %s to %v", v, fun.Name(), v, locs[i])
				a.PrepareParameter(fun, sig, locs[i], v, calleeIsScratch)
			}

			// Perform the function call itself.
			a.allocs = append(a.allocs, v)
		case ssafir.OpFunctionResult:
			// Save the result.
			fun := v.Args[0].Extra.(*types.Function)
			sig := fun.Type().(*types.Signature)

			// Preserve any values currently in
			// the function's scratch registers.
			calleeABI := fun.ABI()
			if calleeABI == nil {
				calleeABI = &a.arch.DefaultABI
			}

			if v.Uses != 0 {
				a.Debugf("%s: %s: saving result from %s", v, fun.Name(), v.Args[0])
				a.SaveResult(fun, sig, calleeABI, v)
			}
		default:
			// Search by group next.
			switch v.Op.Info().Group {
			// Straightforward arithmetic operations.
			case ssafir.OpAdd,
				ssafir.OpSubtract,
				ssafir.OpBitwiseOr,
				ssafir.OpBitwiseAnd,
				ssafir.OpBitwiseXor,
				ssafir.OpLogicalOr,
				ssafir.OpLogicalAnd,
				ssafir.OpEqual,
				ssafir.OpNotEqual,
				ssafir.OpLessThan,
				ssafir.OpLessThanOrEqual,
				ssafir.OpGreaterThan,
				ssafir.OpGreaterThanOrEqual:
				// We just add this for now and resolve
				// it when we lower the code.

				// If we're continuing a bigger op, we
				// carry on where we left off.
				dst := a.locations[v.Args[0]][0] // The first operand.
				skipAlloc := false
				if v.Uses == 1 && v == block.Control {
					skipAlloc = true
					dst = nil
					a.Debugf("%s: skipping allocation, as value is just used as block control", v)
				}

				if v.ID != v.Args[0].ID && !skipAlloc {
					dst = a.GetLocation() // Use a new destination.
				}

				src := a.locations[v.Args[0]][0] // The first operand.
				arg := a.locations[v.Args[1]][0] // The other operand.
				if !skipAlloc {
					a.locations[v] = []sys.Location{dst}
					a.allocated[dst] = v
				}
				alloc := &Alloc{Dst: dst, Src: src, Data: arg}
				a.Debugf("%s: %s: inputs %s (%s) and %s (%s) => %s", v, v.Op, v.Args[0], src, v.Args[1], arg, dst)
				a.addAlloc(v, alloc)

			// Arithmetic operations with only one operand.
			case ssafir.OpNegate:
				// We just add this for now and resolve
				// it when we lower the code.
				dst := a.GetLocation()
				src := a.locations[v.Args[0]][0] // The first operand.
				a.locations[v] = []sys.Location{dst}
				a.allocated[dst] = v
				alloc := &Alloc{Dst: dst, Src: src, Data: src}
				a.Debugf("%s: %s: inputs %s (%s) => %s", v, v.Op, v.Args[0], src, dst)
				a.addAlloc(v, alloc)

			// Arithmetic operations with a fixed first operand register.
			case ssafir.OpMultiply,
				ssafir.OpDivide:
				// We just add this for now and resolve
				// it when we lower the code.

				// The destination is fixed to RDX:RAX,
				// so we need to preserve any current
				// occupants.
				// TODO: make multiply/divide destination allocation architecture-agnostic.
				clear(calleeIsScratch)
				calleeIsScratch[x86.RDX] = true
				calleeIsScratch[x86.RAX] = true
				for _, v := range v.Args {
					calleeIsScratch[a.locations[v][0]] = true
				}

				a.SaveValue(x86.RDX, calleeIsScratch)
				a.SaveValue(x86.RAX, calleeIsScratch)

				dst := x86.RAX
				src := a.locations[v.Args[0]][0] // The first operand.
				arg := a.locations[v.Args[1]][0] // The other operand.
				a.locations[v] = []sys.Location{dst}
				a.allocated[dst] = v
				alloc := &Alloc{Dst: dst, Src: src, Data: arg}
				a.Debugf("%s: %s: inputs %s (%s) and %s (%s) => %s", v, v.Op, v.Args[0], src, v.Args[1], arg, dst)
				a.addAlloc(v, alloc)

			// Arithmetic operations with a fixed second operand register.
			case ssafir.OpShiftLeft,
				ssafir.OpShiftRight:
				// We just add this for now and resolve
				// it when we lower the code.

				clear(calleeIsScratch)
				for _, arg := range v.Args {
					// Add the value if it's not been
					// stored yet.
					if a.locations[arg] == nil {
						a.Debugf("%s: adding %s for use as input to %s", v, arg, v)
						a.AddValue(arg)
					}

					calleeIsScratch[a.locations[arg][0]] = true
				}

				// We have a special case for when the
				// shift is a constant.
				var arg any
				if con, ok := v.Extra.(constant.Value); ok {
					arg = con // The other operand.
				} else {
					// Otherwise, the shift is fixed at CL,
					// so we need to preserve any current
					// occupants.
					// TODO: make shift left/right destination allocation architecture-agnostic.
					calleeIsScratch[x86.RCX] = true
					a.SaveValue(x86.RCX, calleeIsScratch)
					a.allocated[x86.RCX] = v.Args[1] // Make sure we don't pick RCX for our destination.

					arg = a.locations[v.Args[1]][0] // The other operand.
				}

				dst := a.GetLocation()
				src := a.locations[v.Args[0]][0] // The first operand.
				a.Debugf("%s: %s: inputs %s (%s) and %s => %s", v, v.Op, v.Args[0], src, arg, dst)
				alloc := &Alloc{Dst: dst, Src: src, Data: arg}

				a.locations[v] = []sys.Location{dst}
				a.allocated[dst] = v
				a.addAlloc(v, alloc)
			default:
				return fmt.Errorf("failed to allocate value %s: unexpected op %s", v, v.Op)
			}
		}

		// Drop any unused values.
		for i, arg := range v.Args {
			if arg.Uses == 0 {
				return fmt.Errorf("internal error: %s.Args[%d] (%s) already had zero uses", v, i, arg)
			}

			arg.Uses--
			if arg.Uses == 0 {
				a.Debugf("%s: dropping %s after %s", v, arg, v)
				a.DropValue(arg)
			}
		}
	}

	block.Values = a.allocs
	a.allocs = nil

	switch block.Kind {
	case ssafir.BlockNormal:
		for _, next := range block.Successors {
			a.Debugf("%s: continuing to next block %s, stopping at %s", block, next.Block(), stopAt)
			err := a.doBlock(done, next.Block(), stopAt)
			if err != nil {
				return err
			}
		}
	case ssafir.BlockIf:
		// Work out whether we have an else
		// block.
		//
		// If we do, our second successor
		// will be the else block and thus
		// have only one predecessor.
		//
		// If not, it will be the next block,
		// which is linked to by both us and
		// the if block.
		haveElse := len(block.Successors[1].Block().Predecessors) == 1
		ifBlock := block.Successors[0].Block()
		nextBlock := block.Successors[1].Block()
		finalBlock := block.Successors[2].Block() // The block after the if/else blocks.
		var elseBlock *ssafir.Block
		if haveElse {
			elseBlock = nextBlock
			nextBlock = finalBlock
		}

		// Append the if block.
		a.Debugf("%s: continuing into if block %s, stopping at %s", block, ifBlock, finalBlock)
		err := a.doBlock(done, ifBlock, finalBlock)
		if err != nil {
			return err
		}

		// If we have an else block, then
		// we need to add an unconditional
		// jump to the next block and then
		// the contents of the else block.
		if haveElse {
			// Append the else block.
			a.Debugf("%s: continuing into else block %s, stopping at %s", block, elseBlock, finalBlock)
			err = a.doBlock(done, elseBlock, finalBlock)
			if err != nil {
				return err
			}
		}

		// Finally, add the next block.
		a.Debugf("%s: continuing to next block %s, stopping at %s", block, nextBlock, stopAt)
		return a.doBlock(done, nextBlock, stopAt)
	case ssafir.BlockReturn:
		// We stop here, as there's no point
		// in adding more blocks after a
		// return.
	}

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
	debug     bool
}

func (a *allocator) Errorf(pos token.Pos, format string, v ...any) error {
	position := a.fset.Position(pos)
	return errors.New(fmt.Sprintf("%s: ", position) + fmt.Sprintf(format, v...))
}

func (a *allocator) Debugf(format string, v ...any) {
	if !a.debug {
		return
	}

	msg := fmt.Sprintf(format, v...)
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "???"
		line = 0
	}

	fmt.Fprintf(os.Stderr, "%s:%d: %s\n", file, line, msg)
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

	// We can also use unused parameter
	// registers as scratch space.
	lastParam := len(fun.Type.Params()) - 1
	for i, reg := range abi.ParamRegisters {
		if i <= lastParam {
			continue
		}

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

	a := &allocator{
		fset:  fset,
		arch:  arch,
		pkg:   pkg,
		sizes: sizes,
		// We don't ever modify registers, so
		// it's fine to use a shallow copy.
		registers: registers,
		abi:       abi,
		function:  fun,
		allocated: make(map[sys.Location]*ssafir.Value, len(arch.ABIRegisters)),
		locations: make(map[*ssafir.Value][]sys.Location, len(arch.ABIRegisters)),
		debug:     os.Getenv("RUSE_DEBUG_ALLOC") == fun.Name,
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

		a.Debugf("%s: noting parameter %d in %s (%s) with op %s", v, v.ExtraInt, v, loc, v.Op)
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
// available.
func (a *allocator) AddValue(v *ssafir.Value) {
	if a.locations[v] != nil {
		panic(fmt.Sprintf("AddValue(%s): value %s is already recorded", v, v))
	}

	// Handle constants.
	if len(v.Args) == 0 {
		dst := a.GetLocation()
		a.allocated[dst] = v
		a.locations[v] = append(a.locations[v], dst)
		a.Debugf("%s: adding %s at %s", v, v, dst)
		a.addOpAlloc(v, ssafir.OpCopy, &Alloc{Dst: dst, Data: v.Extra})

		return
	}

	orig := v.Args[0]
	for _, loc := range a.locations[orig] {
		dst := a.GetLocation()
		a.allocated[dst] = v
		a.locations[v] = append(a.locations[v], dst)
		a.Debugf("%s: adding %s at %s", v, v, dst)
		a.addOpAlloc(v, ssafir.OpCopy, &Alloc{Dst: dst, Src: loc, Data: v.Extra})
	}
}

// SaveValue moves any value in the given
// register to another register (if possible)
// or the stack. The given list of scratch
// registers will be avoided.
func (a *allocator) SaveValue(reg sys.Location, avoid map[sys.Location]bool) {
	v := a.allocated[reg]
	if v == nil {
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
		a.allocated[candidate] = v
		a.allocated[reg] = nil
		a.locations[v] = append(a.locations[v], candidate)
		a.Debugf("%s: saving %s to %s", v, v, candidate)
		a.addOpAlloc(v, ssafir.OpCopy, &Alloc{Dst: candidate, Src: reg})

		locs := a.locations[v]
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
	panic("failed to find spare location")
}

// GetLocation finds a spare location we can
// use for a temporary or result value.
func (a *allocator) GetLocation() sys.Location {
	for _, candidate := range a.registers {
		if a.allocated[candidate] != nil {
			// This register is occupied.
			continue
		}

		// We can save to candidate.
		return candidate
	}

	// No registers are available,
	// so we spill to the stack.
	// TODO: implement spilling to the stack.
	panic("failed to find spare location")
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
		a.Debugf("%s: moving %s with op %s, as neither %s nor %s is used", new, new, new.Op, old, new)
		a.allocs = append(a.allocs, new)
	}

	for _, loc := range a.locations[old] {
		a.Debugf("%s: moving %s (%s) to %s (%s)", new, old, loc, new, loc)
		a.addOpAlloc(new, ssafir.OpCopy, &Alloc{Dst: loc, Src: loc})
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
		a.addOpAlloc(v, ssafir.OpDrop, &Alloc{Src: loc})
		if a.allocated[loc] == v {
			a.allocated[loc] = nil // We don't delete, so we know the location has been used.
		} else {
			a.Debugf("%s: dropping %s, but %s is already occupied by %s", v, v, loc, a.allocated[loc])
		}
	}

	delete(a.locations, v)
}

// PrepareResult ensures that the function's
// result values (if any) are in the appropriate
// memory location(s).
func (a *allocator) PrepareResult(v *ssafir.Value) {
	if a.function.Type.Result() == nil {
		// No result, nothing to do.
		return
	}

	// Add move actions.
	for i, result := range a.function.Result {
		oldLocs := a.locations[v.Args[i]]
		for j, loc := range result {
			a.allocated[loc] = v

			// Check old values.
			if len(oldLocs) == 0 {
				// Store the value.
				a.Debugf("%s: storing result %s %v to %s", v, v, v.Args[1].Extra, loc)
				a.addAlloc(v, &Alloc{Dst: loc, Data: v.Args[i].Extra})
			} else {
				// Handle any existing values.
				oldLoc := oldLocs[j]

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

				a.Debugf("%s: storing result %s in %s to %s", v, v, oldLoc, loc)
				a.addAlloc(v, &Alloc{Dst: loc, Src: oldLoc})
			}
		}

		a.locations[v] = append(a.locations[v][:0], result...)
	}
}

// PrepareParameter ensures that the given
// function parameter is in the appropriate
// memory location(s).
func (a *allocator) PrepareParameter(fun *types.Function, sig *types.Signature, locs []sys.Location, v *ssafir.Value, avoid map[sys.Location]bool) {
	for _, loc := range locs {
		avoid[loc] = true
	}

	// Add move actions.
	for i, loc := range locs {
		// If we already have a value in the
		// destination location, we need to
		// save it before overwriting.
		if a.allocated[loc] != nil {
			a.Debugf("%s: saving value at %s to make space for parameter %s", v, loc, v)
			a.SaveValue(loc, avoid)
		}

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
					con := types.NewConstant(nil, v.Pos, v.End, nil, "", v.Type, constant.MakeString(s), 1)
					a.pkg.Literals = append(a.pkg.Literals, con)
					a.Debugf("%s: storing literal string pointer %s in %s", v, v, loc)
					a.addOpAlloc(v, ssafir.OpConstantString, &Alloc{Dst: loc, Data: s})
				case 1:
					a.Debugf("%s: storing literal string length %s in %s", v, v, loc)
					a.addOpAlloc(v, ssafir.OpConstantUntypedInt, &Alloc{Dst: loc, Data: int64(len(s))})
				}

				continue
			}

			if con, ok := v.Extra.(*types.Constant); ok && con.Value().Kind() == constant.String {
				val := con.Value()
				switch i {
				case 0:
					a.Debugf("%s: storing constant string pointer %s in %s", v, v, loc)
					a.addOpAlloc(v, ssafir.OpConstantString, &Alloc{Dst: loc, Data: con})
				case 1:
					s := constant.StringVal(val)
					a.Debugf("%s: storing constant string length %s in %s", v, v, loc)
					a.addOpAlloc(v, ssafir.OpConstantUntypedInt, &Alloc{Dst: loc, Data: int64(len(s))})
				}

				continue
			}

			a.Debugf("%s: storing constant %s in %s with op %s", v, v, loc, v.Op)
			a.addAlloc(v, &Alloc{Dst: loc, Data: v.Extra})

			continue
		}

		// If we're copying a constant,
		// we float as above.
		if v.Op == ssafir.OpCopy && len(v.Args) == 1 && isConstant(v.Args[0]) {
			a.Debugf("%s: storing constant %s to %s in %s", v, v.Args[0], v, loc)
			a.addAlloc(v, &Alloc{Dst: loc, Data: v.Args[0].Extra})

			continue
		}

		if i < len(a.locations[v]) {
			src := a.locations[v][i]
			a.allocated[src] = nil
			a.Debugf("%s: storing value %s from %s in %s", v, v, src, loc)
			a.addOpAlloc(v, ssafir.OpCopy, &Alloc{Dst: loc, Src: src})
		}
	}

	a.locations[v] = append(a.locations[v][:0], locs...)
}

// SaveResult takes note of the fact that the
// given result already exists in a location
// determined by the function's calling
// convention.
func (a *allocator) SaveResult(fun *types.Function, sig *types.Signature, abi *sys.ABI, v *ssafir.Value) {
	results := v.Extra.(*FunctionResult)
	sizes := make([]int, len(results.Result))
	for i, result := range results.Result {
		sizes[i] = a.sizes.SizeOf(result.Type)
	}

	locs := a.arch.Result(abi, sizes)
	for i, locs := range locs {
		// We only process this value's
		// locations here.
		if results.Result[i] != v {
			continue
		}

		for _, loc := range locs {
			a.allocated[loc] = v
			a.Debugf("%s: saving result %d in %s with op %s", v, i+1, loc, v.Op)
			a.addAlloc(v, &Alloc{Dst: loc, Src: loc})
		}

		a.locations[v] = append(a.locations[v][:0], locs...)
	}
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
