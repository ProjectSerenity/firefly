// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package compiler

import (
	"fmt"
	"math"
	"strconv"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/constant"
	"firefly-os.dev/tools/ruse/internal/x86"
	"firefly-os.dev/tools/ruse/ssafir"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

// x86RegisterTo8 returns the 8-bit version
// of the given register if one exists, or
// it returns the input.
func x86RegisterTo8(loc sys.Location) sys.Location {
	reg, ok := loc.(*x86.Register)
	if !ok {
		return loc
	}

	switch reg {
	case x86.RAX, x86.EAX, x86.AX:
		return x86.AL
	case x86.RCX, x86.ECX, x86.CX:
		return x86.CL
	case x86.RDX, x86.EDX, x86.DX:
		return x86.DL
	case x86.RBX, x86.EBX, x86.BX:
		return x86.BL
	case x86.RSI, x86.ESI, x86.SI:
		return x86.SIL
	case x86.RDI, x86.EDI, x86.DI:
		return x86.DIL
	case x86.R8, x86.R8D, x86.R8W:
		return x86.R8L
	case x86.R9, x86.R9D, x86.R9W:
		return x86.R9L
	case x86.R10, x86.R10D, x86.R10W:
		return x86.R10L
	case x86.R11, x86.R11D, x86.R11W:
		return x86.R11L
	case x86.R12, x86.R12D, x86.R12W:
		return x86.R12L
	case x86.R13, x86.R13D, x86.R13W:
		return x86.R13L
	case x86.R14, x86.R14D, x86.R14W:
		return x86.R14L
	case x86.R15, x86.R15D, x86.R15W:
		return x86.R15L
	}

	return loc
}

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

	// We start the function with ENDBR
	// so that it will support CET Indirect
	// Branch Tracking.
	l.addInst(&ssafir.Value{
		ID:    0, // This is special.
		Block: l.block,
		Pos:   fun.Code.Elements[0].Pos(), // The 'func' keyword.
		End:   fun.Code.Elements[1].End(), // The end of the signature.
	}, ssafir.OpX86ENDBR64, &x86InstructionData{})

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
		case ssafir.OpSaveRegister:
			l.addInst(v, ssafir.OpX86PUSH_R64op, &x86InstructionData{Args: [4]any{v.Extra}})
		case ssafir.OpRestoreRegister:
			l.addInst(v, ssafir.OpX86POP_R64op, &x86InstructionData{Args: [4]any{v.Extra}})
		case ssafir.OpFunctionCall:
			l.Call(v)
		case ssafir.OpLogicalOr,
			ssafir.OpLogicalAnd:
			// The logical operations are similar to arithmetic,
			// but they are different enough that it's easier to
			// do them separately.
			//
			// First, we do the operation to merge the
			// arguments, then we check the result.
			// TODO: have the logical (or) operation short-circuit.
			var op ssafir.Op
			switch v.Op {
			case ssafir.OpLogicalOr:
				op = ssafir.OpX86OR_R8_Rmr8
			case ssafir.OpLogicalAnd:
				op = ssafir.OpX86AND_R8_Rmr8
			}

			alloc := v.Extra.(*Alloc)
			data := &x86InstructionData{
				Args: [4]any{
					x86RegisterTo8(alloc.Src),
					x86RegisterTo8(alloc.Data.(sys.Location)),
				},
			}

			l.addInst(v, op, data)

			// Then, we store the result.
			op = ssafir.OpX86SETNZ_Rmr8
			data = &x86InstructionData{
				Args: [4]any{
					x86RegisterTo8(alloc.Dst),
				},
			}

			l.addInst(v, op, data)
		default:
			// Finally, handle arithmetic operations, as there
			// are a lot of them.
			switch v.Op.Info().Group {
			case ssafir.OpAdd,
				ssafir.OpSubtract,
				ssafir.OpMultiply,
				ssafir.OpDivide,
				ssafir.OpNegate,
				ssafir.OpBitwiseOr,
				ssafir.OpBitwiseAnd,
				ssafir.OpBitwiseXor,
				ssafir.OpShiftLeft,
				ssafir.OpShiftRight,
				ssafir.OpEqual,
				ssafir.OpNotEqual,
				ssafir.OpLessThan,
				ssafir.OpLessThanOrEqual,
				ssafir.OpGreaterThan,
				ssafir.OpGreaterThanOrEqual:
				l.DoArithmetic(v)
			default:
				return fmt.Errorf("failed to lower value %s: unexpected op %s", v, v.Op)
			}
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

	switch extra := alloc.Data.(type) {
	case int64:
		data.Args[1] = uint64(extra)
		if math.MinInt32 <= extra && extra <= math.MaxInt32 {
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
		data.Args[1] = extra
		if math.MaxUint32 < extra {
			op = ssafir.OpX86MOV_R64op_Imm64_REX
		}
		if extra <= math.MaxUint32 {
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
		val, ok := constant.Int64Val(extra)
		if !ok {
			panic(fmt.Errorf("%s: value %v (op %s) has constant value %s which overflows int64", l.fset.Position(v.Pos), v, v.Op, extra))
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

	// Use a RIP-relative address with
	// LEA, as it results in a relative
	// address and takes up less space
	// than using MOV with a 64-bit
	// immediate.
	op := ssafir.OpX86LEA_R64_M_REX
	data := &x86InstructionData{
		Args: [4]any{
			l.location(v, alloc.Dst),
		},
	}

	switch extra := alloc.Data.(type) {
	case string:
		// Unnamed string literal.
		link := &ssafir.Link{
			Pos:  v.Pos,
			Name: "." + extra,
			Type: ssafir.LinkRelativeAddress,
			Size: 32,
		}
		data.Args[1] = link
	case *types.Constant:
		// Named string constant.
		link := &ssafir.Link{
			Pos:  v.Pos,
			Name: extra.Package().Path + "." + extra.Name(),
			Type: ssafir.LinkRelativeAddress,
			Size: 32,
		}
		data.Args[1] = link
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

// DoArithmetic performs a binary arithmetic
// operation described by the given value.
func (l *x86Lowerer) DoArithmetic(v *ssafir.Value) {
	// Ideally, we would like to use the current
	// location for both arguments and skip straight
	// to the arithmetic instruction. However, we
	// will need to do a copy if:
	//
	// - The first parameter is reused (since x86
	//   arithmetic instructions overwrite it)
	// - Both parameters are stack addresses
	//
	// For now, we always create a new result
	// value to prioritise correctness over speed.

	// For most instructions, we need to copy the
	// first parameter to the destination.
	switch v.Op.Info().Group {
	case ssafir.OpEqual,
		ssafir.OpNotEqual,
		ssafir.OpLessThan,
		ssafir.OpLessThanOrEqual,
		ssafir.OpGreaterThan,
		ssafir.OpGreaterThanOrEqual:
	default:
		l.MoveNumber(v)
	}

	alloc := v.Extra.(*Alloc)
	data := &x86InstructionData{
		Args: [4]any{
			alloc.Dst,
			alloc.Data.(sys.Location),
		},
	}

	// Prepare our metadata.
	info := v.Op.Info()
	var size int
	switch info.Size {
	case 8:
		size = 0
	case 16:
		size = 1
	case 32:
		size = 2
	case 64:
		size = 3
	default:
		panic(fmt.Sprintf("internal error: unexpected size %d for op %s", info.Size, v.Op))
	}

	// Determine the first (and usually
	// only) instruction, based on the
	// size calculated above.
	//
	// Comparisons are the same for the
	// first step.
	comparisons := [4]ssafir.Op{
		ssafir.OpX86CMP_R8_Rmr8,
		ssafir.OpX86CMP_R16_Rmr16,
		ssafir.OpX86CMP_R32_Rmr32,
		ssafir.OpX86CMP_R64_Rmr64_REX,
	}

	op := map[ssafir.Op][4]ssafir.Op{
		ssafir.OpAdd: {
			ssafir.OpX86ADD_R8_Rmr8,
			ssafir.OpX86ADD_R16_Rmr16,
			ssafir.OpX86ADD_R32_Rmr32,
			ssafir.OpX86ADD_R64_Rmr64_REX,
		},
		ssafir.OpSubtract: {
			ssafir.OpX86SUB_R8_Rmr8,
			ssafir.OpX86SUB_R16_Rmr16,
			ssafir.OpX86SUB_R32_Rmr32,
			ssafir.OpX86SUB_R64_Rmr64_REX,
		},
		ssafir.OpMultiply: {
			ssafir.OpX86MUL_Rmr8,
			ssafir.OpX86MUL_Rmr16,
			ssafir.OpX86MUL_Rmr32,
			ssafir.OpX86MUL_Rmr64_REX,
		},
		ssafir.OpDivide: {
			ssafir.OpX86DIV_Rmr8,
			ssafir.OpX86DIV_Rmr16,
			ssafir.OpX86DIV_Rmr32,
			ssafir.OpX86DIV_Rmr64_REX,
		},
		ssafir.OpNegate: {
			ssafir.OpX86NEG_Rmr8,
			ssafir.OpX86NEG_Rmr16,
			ssafir.OpX86NEG_Rmr32,
			ssafir.OpX86NEG_Rmr64_REX,
		},
		ssafir.OpBitwiseOr: {
			ssafir.OpX86OR_R8_Rmr8,
			ssafir.OpX86OR_R16_Rmr16,
			ssafir.OpX86OR_R32_Rmr32,
			ssafir.OpX86OR_R64_Rmr64_REX,
		},
		ssafir.OpBitwiseAnd: {
			ssafir.OpX86AND_R8_Rmr8,
			ssafir.OpX86AND_R16_Rmr16,
			ssafir.OpX86AND_R32_Rmr32,
			ssafir.OpX86AND_R64_Rmr64_REX,
		},
		ssafir.OpBitwiseXor: {
			ssafir.OpX86XOR_R8_Rmr8,
			ssafir.OpX86XOR_R16_Rmr16,
			ssafir.OpX86XOR_R32_Rmr32,
			ssafir.OpX86XOR_R64_Rmr64_REX,
		},
		ssafir.OpShiftLeft: {
			ssafir.OpX86SAL_Rmr8_CL,
			ssafir.OpX86SAL_Rmr16_CL,
			ssafir.OpX86SAL_Rmr32_CL,
			ssafir.OpX86SAL_Rmr64_CL_REX,
		},
		ssafir.OpShiftRight: {
			ssafir.OpX86SAR_Rmr8_CL,
			ssafir.OpX86SAR_Rmr16_CL,
			ssafir.OpX86SAR_Rmr32_CL,
			ssafir.OpX86SAR_Rmr64_CL_REX,
		},
		ssafir.OpEqual:              comparisons,
		ssafir.OpNotEqual:           comparisons,
		ssafir.OpLessThan:           comparisons,
		ssafir.OpLessThanOrEqual:    comparisons,
		ssafir.OpGreaterThan:        comparisons,
		ssafir.OpGreaterThanOrEqual: comparisons,
	}[info.Group][size]

	if op == 0 {
		panic(fmt.Errorf("%s: unexpoected op %s", l.fset.Position(v.Pos), v.Op))
	}

	// Do any unusual tweaks.
	switch info.Group {
	case ssafir.OpMultiply:
		data.Args[0], data.Args[1] = data.Args[1], nil // The destination is implied.
	case ssafir.OpDivide:
		// Clear out RDX, as it forms the
		// top 64 bits of the divident.
		l.addInst(v, ssafir.OpX86XOR_R64_Rmr64_REX, &x86InstructionData{
			Args: [4]any{
				x86.RDX,
				x86.RDX,
			},
		})

		data.Args[0], data.Args[1] = data.Args[1], nil // The destination is implied.
	case ssafir.OpNegate:
		data.Args[1] = nil // There is no second argument.
	case ssafir.OpShiftLeft,
		ssafir.OpShiftRight:
		// Move the shift (second arg) into CL.
		v.Extra = &Alloc{Dst: x86.RCX, Src: alloc.Data.(sys.Location)}
		l.MoveNumber(v)

		data.Args[1] = x86.CL // The shift is now in place.
	case ssafir.OpEqual,
		ssafir.OpNotEqual,
		ssafir.OpLessThan,
		ssafir.OpLessThanOrEqual,
		ssafir.OpGreaterThan,
		ssafir.OpGreaterThanOrEqual:
		// First, we do the comparison.
		l.addInst(v, op, &x86InstructionData{
			Args: [4]any{
				alloc.Src,
				alloc.Data.(sys.Location),
			},
		})

		// Then we store the result.
		data.Args[0] = x86RegisterTo8(alloc.Dst)
		data.Args[1] = nil // There is no second arg.
		switch info.Group {
		case ssafir.OpEqual:
			op = ssafir.OpX86SETE_Rmr8
		case ssafir.OpNotEqual:
			op = ssafir.OpX86SETNE_Rmr8
		case ssafir.OpLessThan:
			op = ssafir.OpX86SETL_Rmr8
		case ssafir.OpLessThanOrEqual:
			op = ssafir.OpX86SETLE_Rmr8
		case ssafir.OpGreaterThan:
			op = ssafir.OpX86SETG_Rmr8
		case ssafir.OpGreaterThanOrEqual:
			op = ssafir.OpX86SETGE_Rmr8
		default:
			panic(fmt.Sprintf("internal error: op %s (group %s) not covered in comparisons", v.Op, info.Group))
		}
	}

	l.addInst(v, op, data)
}
