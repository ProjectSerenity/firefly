// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package compiler

import (
	"fmt"
	"go/constant"
	"math"
	"strconv"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/internal/x86"
	"firefly-os.dev/tools/ruse/ssafir"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

func lowerX86(fset *token.FileSet, arch *sys.Arch, sizes types.Sizes, fun *ssafir.Function) (err error) {
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

	l := &x86Lowerer{
		fset:     fset,
		arch:     arch,
		sizes:    sizes,
		block:    block,
		function: fun,
	}

	ctx := &x86Context{
		Func:   fun,
		FSet:   fset,
		Labels: make(map[string]*x86Label),
	}

	defer func() {
		v := recover()
		if v == nil {
			return
		}

		if e, ok := v.(error); ok {
			err = e
			return
		}

		panic(v)
	}()

	for _, anno := range fun.Code.Annotations {
		if len(anno.X.Elements) == 0 {
			return ctx.Errorf(anno.X.ParenClose, "invalid annotation: no keyword")
		}

		ident, ok := anno.X.Elements[0].(*ast.Identifier)
		if !ok {
			return ctx.Errorf(anno.X.Elements[0].Pos(), "invalid annotation: bad keyword: %s %s", anno.X.Elements[0].String(), anno.X.Elements[0].Print())
		}

		switch ident.Name {
		case "mode":
			mode, ok := anno.X.Elements[1].(*ast.Literal)
			if !ok || mode.Kind != token.Integer {
				continue
			}

			num, err := strconv.Atoi(mode.Value)
			if err != nil {
				return fmt.Errorf("invalid mode %q: %v", mode.Value, err)
			}

			if ctx.Mode.Int != 0 {
				return ctx.Errorf(anno.X.Elements[0].Pos(), "invalid annotation: cannot specify mode more than once")
			}

			switch num {
			case 16:
				ctx.Mode = x86.Mode16
			case 32:
				ctx.Mode = x86.Mode32
			case 64:
				ctx.Mode = x86.Mode64
			default:
				return fmt.Errorf("invalid mode %q: %v", mode.Value, err)
			}
		default:
			// We can safely ignore unrecognised annotations.
			continue
		}
	}

	// Default to 64-bit mode for x86-64.
	if ctx.Mode.Int == 0 {
		ctx.Mode = x86.Mode64
	}

	fun.Extra = ctx.Mode

	var lastResult *ssafir.Value
	for i, v := range fun.Entry.Values {
		switch v.Op {
		case ssafir.OpConstantInt8, ssafir.OpConstantInt16, ssafir.OpConstantInt32, ssafir.OpConstantInt64, ssafir.OpConstantUntypedInt:
			l.MoveNumber(v)
		case ssafir.OpConstantUint8, ssafir.OpConstantUint16, ssafir.OpConstantUint32, ssafir.OpConstantUint64:
			l.MoveNumber(v)
		case ssafir.OpConstantString:
			l.MoveString(v)
		case ssafir.OpDrop:
			// Nothing to do here, this is just debugging
			// information for the register allocator.
		case ssafir.OpCopy:
			l.MoveNumber(v)
		case ssafir.OpParameter:
			// Nothing to do here, the caller has already
			// put the value in the relevant register.
		case ssafir.OpMakeResult:
			l.MoveNumber(v)
			lastResult = fun.Entry.Values[i]
		case ssafir.OpFunctionCall:
			l.Call(v)
		default:
			return fmt.Errorf("failed to lower value %s: unexpected op %s", v, v.Op)
		}
	}

	if lastResult == nil {
		lastResult = &ssafir.Value{
			Pos: block.End - 1,
			End: block.End,
		}
	}

	// Finally, complete any link references.
	var offset int
	for _, value := range l.insts {
		data, ok := value.Extra.(*x86InstructionData)
		if !ok {
			continue
		}

		for i, arg := range data.Args {
			if arg == nil {
				break
			}

			link, ok := arg.(*tempLink)
			if !ok {
				continue
			}

			// Replace the instruction index with
			// the offset into the function, plus
			// the offset into the instruction.
			link.Link.Offset = offset + link.InnerOffset
			link.Link.Address = uintptr(offset) + link.InnerAddress

			// Store the final link.
			data.Args[i] = link.Link
			fun.Links = append(fun.Links, link.Link)
		}

		offset += int(data.Length)
	}

	l.Return(lastResult)

	l.block.Values = l.insts
	l.function.Entry = l.block
	l.function.Blocks = []*ssafir.Block{l.block}

	return err
}

// x86Lowerer maintains state while lowering SSAFIR
// with register allocations to x86 instructions.
type x86Lowerer struct {
	fset     *token.FileSet
	arch     *sys.Arch
	sizes    types.Sizes
	code     x86.Code
	block    *ssafir.Block
	insts    []*ssafir.Value
	function *ssafir.Function
}

// addInst creates a copy of the given SSAFIR
// value, setting its `Op` and `Extra` to the given
// `op` and `data`, adding the result to
// `l.insts`.
func (l *x86Lowerer) addInst(v *ssafir.Value, op ssafir.Op, data *x86InstructionData) {
	// Calculate the instruction length.
	err := x86EncodeInstruction(&l.code, x86.Mode64, op, data)
	if err != nil {
		panic(fmt.Errorf("%s: failed to encode instruction: %v", l.fset.Position(v.Pos), err))
	}

	// Handle any link by storing the
	// offset into the instruction of
	// any immediate value (as that is
	// where the link would be inserted).
	for i, arg := range data.Args {
		if arg == nil {
			break
		}

		link, ok := arg.(*ssafir.Link)
		if !ok {
			continue
		}

		if l.code.CodeOffsetLen == 0 && l.code.ImmediateLen == 0 && l.code.DisplacementLen == 0 {
			panic(fmt.Errorf("%s: internal error: instruction specified a link to %s, but no code offset, immediate, or displacement was produced", l.fset.Position(v.Pos), link.Name))
		}

		// Update the link's offsets. The
		// inner offset is the offset within
		// this instruction. The outer offset
		// is the instruction's offset into
		// the function. For now, the latter
		// is just the instruction index, but
		// we replace it with the full offset
		// later.
		link2 := &tempLink{
			InnerOffset:  l.code.Len() - (l.code.CodeOffsetLen + l.code.ImmediateLen + l.code.DisplacementLen),
			InnerAddress: uintptr(l.code.Len()), // The instruction is relative to the next instruction.
			Link:         link,
		}

		link.Offset = len(l.insts)
		data.Args[i] = link2
	}

	data.Length = uint8(l.code.Len())

	v2 := &ssafir.Value{
		ID:    v.ID,
		Op:    op,
		Type:  v.Type,
		Extra: data,
		Args:  v.Args,
		Block: l.block,
		Pos:   v.Pos,
		End:   v.End,
		Uses:  v.Uses,
	}

	l.insts = append(l.insts, v2)
}

// location turns any abstract locations to
// concrete equivalents. Registers are returned
// unchanged. Stack locations are translated
// into an `x86.Memory`.
func (l *x86Lowerer) location(v *ssafir.Value, loc sys.Location) any {
	switch loc := loc.(type) {
	case *x86.Register:
		return loc
	case sys.Stack:
		return &x86.Memory{
			Base:         loc.Pointer.(*x86.Register),
			Displacement: int64(loc.Offset),
		}
	default:
		panic(fmt.Errorf("%s: value %s (op %s) has unexpected location %#v, want register or stack location", l.fset.Position(v.Pos), v, v.Op, loc))
	}
}

// Call inserts a relative CALL instruction.
func (l *x86Lowerer) Call(v *ssafir.Value) {
	fun, ok := v.Extra.(*types.Function)
	if !ok {
		panic(fmt.Errorf("%s: value %v (op %s) has unexpected data %#v, want *types.Function", l.fset.Position(v.Pos), v, v.Op, v.Extra))
	}

	// First, build the link to the
	// destination function.
	link := &ssafir.Link{
		Pos:  v.Pos,
		Name: fun.Package().Path + "." + fun.Name(),
		Type: ssafir.LinkRelativeAddress,
		Size: 32, // We always use a 32-bit relative address in case the other function is far away.
	}

	op := ssafir.OpX86CALL_Rel32
	data := &x86InstructionData{
		Args: [4]any{link},
	}

	l.addInst(v, op, data)
}

// MoveNumber inserts a MOV instruction for a
// numerical value in either a register
// or a numerical constant.
func (l *x86Lowerer) MoveNumber(v *ssafir.Value) {
	alloc, ok := v.Extra.(*Alloc)
	if !ok {
		panic(fmt.Errorf("%s: value %v (op %s) has unexpected data %#v, want *Alloc", l.fset.Position(v.Pos), v, v.Op, v.Extra))
	}

	// First, work out whether we're moving
	// data or a register/stack value.
	if alloc.Src != nil {
		// Register/stack move.

		// We can ignore moves with the same
		// source and destination.
		if alloc.Dst == alloc.Src {
			return
		}

		op := ssafir.OpX86MOV_R64_Rmr64_REX
		data := &x86InstructionData{
			Args: [4]any{
				l.location(v, alloc.Dst),
				l.location(v, alloc.Src),
			},
		}

		// We default to MOV r64, r/m64, but
		// swap if we're moving to a stack
		// location.
		if _, ok := data.Args[0].(*x86.Memory); ok {
			op = ssafir.OpX86MOV_M64_R64_REX
		} else if _, ok := data.Args[1].(*x86.Memory); ok {
			op = ssafir.OpX86MOV_R64_M64_REX
		}

		l.addInst(v, op, data)

		return
	}

	// Constant store.

	// Prefer 32-bit immediates, as they use
	// less space in the instruction stream
	// than 64-bit immediates. However, 16-bit
	// and smaller moves don't clear the upper
	// bits and thus corrupt the data.
	op := ssafir.OpX86MOV_R64op_Imm64_REX
	data := &x86InstructionData{
		Args: [4]any{
			l.location(v, alloc.Dst),
		},
	}

	switch imm := alloc.Data.(type) {
	case int64:
		data.Args[1] = uint64(imm)
		if math.MinInt32 <= imm && imm <= math.MaxInt32 {
			// We can use a 32-bit move.
			op = ssafir.OpX86MOV_R32op_Imm32
			if reg, ok := data.Args[0].(*x86.Register); ok {
				smaller, ok := reg.ToSize(32)
				if !ok {
					panic(fmt.Errorf("%s: value %v (op %s) has unexpected destination register %s with no 32-bit form", l.fset.Position(v.Pos), v, v.Op, reg))
				}

				// Use the 32-bit reference to the register.
				data.Args[0] = smaller
			}
		}
	case uint64:
		data.Args[1] = imm
		if math.MaxUint32 < imm {
			op = ssafir.OpX86MOV_R64op_Imm64_REX
		}
		if imm <= math.MaxUint32 {
			// We can use a 32-bit move.
			op = ssafir.OpX86MOV_R32op_Imm32
			if reg, ok := data.Args[0].(*x86.Register); ok {
				smaller, ok := reg.ToSize(32)
				if !ok {
					panic(fmt.Errorf("%s: value %v (op %s) has unexpected destination register %s with no 32-bit form", l.fset.Position(v.Pos), v, v.Op, reg))
				}

				// Use the 32-bit reference to the register.
				data.Args[0] = smaller
			}
		}
	case constant.Value:
		val, ok := constant.Int64Val(imm)
		if !ok {
			panic(fmt.Errorf("%s: value %v (op %s) has constant value %s which overflows int64", l.fset.Position(v.Pos), v, v.Op, imm))
		}

		data.Args[1] = uint64(val)
		if math.MinInt32 <= val && val <= math.MaxInt32 {
			// We can use a 32-bit move.
			op = ssafir.OpX86MOV_R32op_Imm32
			if reg, ok := data.Args[0].(*x86.Register); ok {
				smaller, ok := reg.ToSize(32)
				if !ok {
					panic(fmt.Errorf("%s: value %v (op %s) has unexpected destination register %s with no 32-bit form", l.fset.Position(v.Pos), v, v.Op, reg))
				}

				// Use the 32-bit reference to the register.
				data.Args[0] = smaller
			}
		}
	default:
		panic(fmt.Errorf("%s: value %v (op %s) has unexpected constant data %#v (%T)", l.fset.Position(v.Pos), v, v.Op, alloc.Data, alloc.Data))
	}

	l.addInst(v, op, data)
}

// MoveString inserts MOV instructions for a
// string value in either a pair of registers
// or a string constant.
func (l *x86Lowerer) MoveString(v *ssafir.Value) {
	alloc, ok := v.Extra.(*Alloc)
	if !ok {
		panic(fmt.Errorf("%s: value %v (op %s) has unexpected data %#v, want *Alloc", l.fset.Position(v.Pos), v, v.Op, v.Extra))
	}

	// First, work out whether we're moving
	// data or a register/stack value.
	if alloc.Src != nil {
		// Register/stack move.

		// We can ignore moves with the same
		// source and destination.
		if alloc.Dst == alloc.Src {
			return
		}

		op := ssafir.OpX86MOV_R64_Rmr64_REX
		data := &x86InstructionData{
			Args: [4]any{
				l.location(v, alloc.Dst),
				l.location(v, alloc.Src),
			},
		}

		// We default to MOV r64, r/m64, but
		// swap if we're moving to a stack
		// location.
		if _, ok := data.Args[0].(*x86.Memory); ok {
			op = ssafir.OpX86MOV_M64_R64_REX
		} else if _, ok := data.Args[1].(*x86.Memory); ok {
			op = ssafir.OpX86MOV_R64_M64_REX
		}

		l.addInst(v, op, data)

		return
	}

	// Constant store.

	// Prefer 32-bit immediates, as they use
	// less space in the instruction stream
	// than 64-bit immediates. However, 16-bit
	// and smaller moves don't clear the upper
	// bits and thus corrupt the data.
	op := ssafir.OpX86MOV_R64op_Imm64_REX
	data := &x86InstructionData{
		Args: [4]any{
			l.location(v, alloc.Dst),
		},
	}

	switch imm := alloc.Data.(type) {
	case string:
		// Unnamed string literal.
		link := &ssafir.Link{
			Pos:  v.Pos,
			Name: "." + imm,
			Type: ssafir.LinkFullAddress,
			Size: 64,
		}
		data.Args[1] = link
	case constant.Value:
		val := constant.StringVal(imm)
		data.Args[1] = uint64(len(val))
	default:
		panic(fmt.Errorf("%s: value %v (op %s) has unexpected constant data %#v (%T)", l.fset.Position(v.Pos), v, v.Op, alloc.Data, alloc.Data))
	}

	l.addInst(v, op, data)
}

// Return emits a return instruction.
func (l *x86Lowerer) Return(v *ssafir.Value) {
	op := ssafir.OpX86RET
	data := &x86InstructionData{}

	l.addInst(v, op, data)
}
