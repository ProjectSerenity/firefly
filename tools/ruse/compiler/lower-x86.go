// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package compiler

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"

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
	l := &x86Lowerer{
		fset:     fset,
		arch:     arch,
		sizes:    sizes,
		function: fun,
		debug:    os.Getenv("RUSE_DEBUG_LOWER") == fun.Name,

		blockOffsets: make(map[*ssafir.Block]int),
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
	l.block = l.function.Entry
	l.addInst(&ssafir.Value{
		ID:    0, // This is special.
		Block: l.block,
		Pos:   l.function.Code.Elements[0].Pos(), // The 'func' keyword.
		End:   l.function.Code.Elements[1].End(), // The end of the signature.
	}, ssafir.OpX86_ENDBR64, &x86InstructionData{})

	// Lower the remaining instructions.
	l.Debugf("%s: starting at entry point for %s", l.function.Entry, l.function.Name)
	err = l.doBlock(make(map[*ssafir.Block]bool), l.function.Entry, nil)
	if err != nil {
		return err
	}

	// Make a synthetic block to represent
	// the function as a whole.
	l.block = &ssafir.Block{
		ID:       1,
		Kind:     ssafir.BlockReturn,
		Function: l.function,
		Pos:      l.function.Func.Pos(),
		End:      l.function.Func.End(),
		Values:   l.insts,
	}

	l.function.Entry = l.block
	l.function.Blocks = []*ssafir.Block{l.block}

	// Finalise any jumps within the function,
	// such as if statements.
	for _, jump := range l.blockJumps {
		v := l.insts[jump.index]
		data := v.Extra.(*x86InstructionData)
		target, ok := l.blockOffsets[jump.target]
		if !ok {
			return fmt.Errorf("%s: internal error: jump to block %s with unknown offset", fset.Position(jump.pos), jump.target)
		}

		jumpLength := calculateJumpDistance(l.insts, target, jump.index)
		l.Debugf("%s: calculated jump distance to %s as %d", l.fset.Position(v.Pos), jump.target, jumpLength)
		data.Args[0] = uint64(jumpLength + int64(data.Length)) // Offset the subtraction done in the encoding process.
	}

	// Next, optimise jumps to smaller
	// jump instructions if possible.
	var code x86.Code
	for _, jump := range l.blockJumps {
		v := l.insts[jump.index]
		data := v.Extra.(*x86InstructionData)

		inst := x86OpToInstruction(v.Op)
		jumpLength := int64(data.Args[0].(uint64))

		// Check whether we can encode the jump in an
		// 8-bit or 16-bit version of the same jump.

		newUID8 := strings.Replace(inst.UID, "32", "8", 1)
		inst8, ok := x86.InstructionsByUID[newUID8]
		if ok && inst8.Supports(ctx.Mode) && math.MinInt8 <= jumpLength && jumpLength <= math.MaxInt8 {
			v.Op = x86Opcodes[newUID8]
			err := x86EncodeInstruction(&code, ctx.Mode, v.Op, data)
			if err != nil {
				return err
			}

			l.Debugf("%s: shortened jump to %s to 8 bits", l.fset.Position(v.Pos), jump.target)
			data.Length = uint8(code.Len())
			continue
		}

		newUID16 := strings.Replace(inst.UID, "32", "16", 1)
		inst16, ok := x86.InstructionsByUID[newUID16]
		if ok && inst16.Supports(ctx.Mode) && math.MinInt16 <= jumpLength && jumpLength <= math.MaxInt16 {
			v.Op = x86Opcodes[newUID16]
			err := x86EncodeInstruction(&code, ctx.Mode, v.Op, data)
			if err != nil {
				return err
			}

			l.Debugf("%s: shortened jump to %s to 16 bits", l.fset.Position(v.Pos), jump.target)
			data.Length = uint8(code.Len())
			continue
		}
	}

	// Next, re-calculate jump distances,
	// after any optimisations.
	for _, jump := range l.blockJumps {
		v := l.insts[jump.index]
		data := v.Extra.(*x86InstructionData)
		oldLength := int64(data.Args[0].(uint64))
		target := l.blockOffsets[jump.target] // We've checked this already above.
		jumpLength := calculateJumpDistance(l.insts, target, jump.index)
		l.Debugf("%s: recalculated jump distance to %s from %d to %d", l.fset.Position(v.Pos), jump.target, oldLength, jumpLength)
		data.Args[0] = uint64(jumpLength + int64(data.Length)) // Offset the subtraction done in the encoding process.
	}

	// Finally, complete any link references.
	var offset int
	for _, v := range l.insts {
		data := v.Extra.(*x86InstructionData)
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

	return nil
}

// doBlock lowers the logical instructions in block to
// x86-64 machine instructions.
func (l *x86Lowerer) doBlock(done map[*ssafir.Block]bool, block, stopAt *ssafir.Block) error {
	if done[block] {
		l.Debugf("%s: skipping block, which has already been lowered", block)
		return nil
	}

	if block == stopAt {
		l.Debugf("%s: stopping early, as requested", block)
		return nil
	}

	done[block] = true

	boolConstToUint64 := func(v constant.Value) uint64 {
		if constant.BoolVal(v) {
			return 1
		}

		return 0
	}

	l.block = block
	l.blockOffsets[block] = len(l.insts)
	l.Debugf("%s: starting block at offset %06x", block, len(l.insts))

	for _, v := range block.Values {
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
		case ssafir.OpCopy, ssafir.OpFunctionResult:
			l.MoveNumber(v)
		case ssafir.OpParameter:
			// Nothing to do here, the caller has already
			// put the value in the relevant register.
		case ssafir.OpMakeResult:
			// If the last operation
			// is a return statement,
			// we don't need to do
			// anything.
			if v.Args[0].Op == ssafir.OpReturn {
				continue
			}

			l.MoveNumber(v)
		case ssafir.OpReturn:
			// These may contain a move.
			// We add the return instruction
			// at the block leve.
			if alloc, ok := v.Extra.(*Alloc); ok && alloc != nil {
				l.MoveNumber(v)
			}
		case ssafir.OpSaveRegister:
			l.addInst(v, ssafir.OpX86_PUSH_R64op, &x86InstructionData{Args: [4]any{v.Extra}})
		case ssafir.OpRestoreRegister:
			l.addInst(v, ssafir.OpX86_POP_R64op, &x86InstructionData{Args: [4]any{v.Extra}})
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
			alloc := v.Extra.(*Alloc)
			con, constArg := alloc.Data.(constant.Value)
			data := &x86InstructionData{
				Args: [4]any{
					x86RegisterTo8(alloc.Src),
				},
			}

			var op ssafir.Op
			switch v.Op {
			case ssafir.OpLogicalOr:
				if constArg {
					op = ssafir.OpX86_OR_Rmr8_Imm8
					data.Args[1] = boolConstToUint64(con)
				} else {
					op = ssafir.OpX86_OR_R8_Rmr8
					data.Args[1] = x86RegisterTo8(alloc.Data.(sys.Location))
				}
			case ssafir.OpLogicalAnd:
				if constArg {
					op = ssafir.OpX86_AND_Rmr8_Imm8
					data.Args[1] = boolConstToUint64(con)
				} else {
					op = ssafir.OpX86_AND_R8_Rmr8
					data.Args[1] = x86RegisterTo8(alloc.Data.(sys.Location))
				}
			}

			l.addInst(v, op, data)

			// Then, we store the result.
			op = ssafir.OpX86_SETNZ_Rmr8
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

	switch block.Kind {
	case ssafir.BlockNormal:
		for _, next := range block.Successors {
			l.Debugf("%s: continuing to next block %s, stopping at %s", block, next.Block(), stopAt)
			err := l.doBlock(done, next.Block(), stopAt)
			if err != nil {
				return err
			}
		}
	case ssafir.BlockIf:
		// This block ends with an if statement
		// so we need to insert one or more
		// jumps.
		//
		// If we have just an if block, then we
		// place the if block immediately after
		// this block and skip over it if the
		// condition evaluates to false. In this
		// approach, the if path won't jump.
		//
		// If we have both if and else blocks,
		// then we place the if block after
		// this block and the else block after
		// that. We jump to the else block if
		// the condition is false. We add an
		// unconditional jump after the if
		// block. This means that each path
		// will have a single jump.
		//
		// To summarise, we always place the
		// if block after this block, followed
		// by any else block. Then the next
		// block comes. We always have an
		// inverted jump to after the if
		// block and if there is an else
		// block, we have an unconditional
		// jump after the if block to jump
		// over the else block.
		var elseJump ssafir.Op // Note that this is always the opposite of the condition.
		switch block.Control.Op.Info().Group {
		case ssafir.OpEqual:
			elseJump = ssafir.OpX86_JNE_Rel32
		case ssafir.OpNotEqual:
			elseJump = ssafir.OpX86_JE_Rel32
		case ssafir.OpLessThan:
			elseJump = ssafir.OpX86_JNL_Rel32
		case ssafir.OpLessThanOrEqual:
			elseJump = ssafir.OpX86_JNLE_Rel32
		case ssafir.OpGreaterThan:
			elseJump = ssafir.OpX86_JNG_Rel32
		case ssafir.OpGreaterThanOrEqual:
			elseJump = ssafir.OpX86_JNGE_Rel32
		default:
			return fmt.Errorf("internal error: unexpected control operation %s", block.Control.Op)
		}

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
		afterIfBlock := nextBlock
		finalBlock := block.Successors[2].Block() // The block after the if/else blocks.
		var elseBlock *ssafir.Block
		if haveElse {
			elseBlock = nextBlock
			nextBlock = finalBlock
		}

		// Add the jump over the if block.
		jump := &blockJump{pos: block.Control.Pos, index: len(l.insts), target: afterIfBlock} // Jump over the if block to the else/next block.
		l.blockJumps = append(l.blockJumps, jump)
		l.addInst(block.Control, elseJump, &x86InstructionData{Args: [4]any{uint64(0)}})

		// Append the if block.
		l.Debugf("%s: continuing into if block %s, stopping at %s", block, ifBlock, finalBlock)
		err := l.doBlock(done, ifBlock, finalBlock)
		if err != nil {
			return err
		}

		// If we have an else block, then
		// we need to add an unconditional
		// jump to the next block and then
		// the contents of the else block.
		if haveElse {
			jump := &blockJump{pos: block.Control.Pos, index: len(l.insts), target: nextBlock}
			l.blockJumps = append(l.blockJumps, jump)
			l.addInst(block.Control, ssafir.OpX86_JMP_Rel32, &x86InstructionData{Args: [4]any{uint64(0)}})

			// Append the else block.
			l.Debugf("%s: continuing into else block %s, stopping at %s", block, elseBlock, finalBlock)
			err = l.doBlock(done, elseBlock, finalBlock)
			if err != nil {
				return err
			}
		}

		// Finally, add the next block.
		l.Debugf("%s: continuing to next block %s, stopping at %s", block, nextBlock, stopAt)
		return l.doBlock(done, nextBlock, stopAt)
	case ssafir.BlockReturn:
		// This block ends with a return, which
		// may be implicit. Here we add the RET
		// instruction.
		l.addInst(block.Control, ssafir.OpX86_RET, new(x86InstructionData))

		// We stop here, as there's no point
		// in adding more blocks after a
		// return.
	}

	return nil
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
	debug    bool

	blockJumps   []*blockJump
	blockOffsets map[*ssafir.Block]int
}

func (l *x86Lowerer) Debugf(format string, v ...any) {
	if !l.debug {
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

	op := ssafir.OpX86_CALL_Rel32
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

		op := ssafir.OpX86_MOV_R64_Rmr64_REX
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
			op = ssafir.OpX86_MOV_M64_R64_REX
		} else if _, ok := data.Args[1].(*x86.Memory); ok {
			op = ssafir.OpX86_MOV_R64_M64_REX
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
	op := ssafir.OpX86_MOV_R64op_Imm64_REX
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
			op = ssafir.OpX86_MOV_R32op_Imm32
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
			op = ssafir.OpX86_MOV_R64op_Imm64_REX
		}
		if extra <= math.MaxUint32 {
			// We can use a 32-bit move.
			op = ssafir.OpX86_MOV_R32op_Imm32
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
			op = ssafir.OpX86_MOV_R32op_Imm32
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

		op := ssafir.OpX86_MOV_R64_Rmr64_REX
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
			op = ssafir.OpX86_MOV_M64_R64_REX
		} else if _, ok := data.Args[1].(*x86.Memory); ok {
			op = ssafir.OpX86_MOV_R64_M64_REX
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
	op := ssafir.OpX86_LEA_R64_M_REX
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
	var second sys.Location
	if loc, ok := alloc.Data.(sys.Location); ok {
		second = loc
	} else {
		second = alloc.Src
	}

	data := &x86InstructionData{
		Args: [4]any{
			alloc.Dst,
			second,
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
	comparisons := [4][2]ssafir.Op{
		{ssafir.OpX86_CMP_R8_Rmr8, ssafir.OpX86_CMP_Rmr8_Imm8},
		{ssafir.OpX86_CMP_R16_Rmr16, ssafir.OpX86_CMP_Rmr16_Imm16},
		{ssafir.OpX86_CMP_R32_Rmr32, ssafir.OpX86_CMP_Rmr32_Imm32},
		{ssafir.OpX86_CMP_R64_Rmr64_REX, ssafir.OpX86_CMP_Rmr64_Imm32_REX},
	}

	constArg := 0
	con, ok := alloc.Data.(constant.Value)
	var imm uint64
	if ok {
		constArg = 1
		imm, _ = constant.Uint64Val(con)
		data.Args[1] = imm
	}

	op := map[ssafir.Op][4][2]ssafir.Op{
		ssafir.OpAdd: {
			{ssafir.OpX86_ADD_R8_Rmr8, ssafir.OpX86_ADD_Rmr8_Imm8},
			{ssafir.OpX86_ADD_R16_Rmr16, ssafir.OpX86_ADD_Rmr16_Imm16},
			{ssafir.OpX86_ADD_R32_Rmr32, ssafir.OpX86_ADD_Rmr32_Imm32},
			{ssafir.OpX86_ADD_R64_Rmr64_REX, ssafir.OpX86_ADD_Rmr64_Imm32_REX},
		},
		ssafir.OpSubtract: {
			{ssafir.OpX86_SUB_R8_Rmr8, ssafir.OpX86_SUB_Rmr8_Imm8},
			{ssafir.OpX86_SUB_R16_Rmr16, ssafir.OpX86_SUB_Rmr16_Imm16},
			{ssafir.OpX86_SUB_R32_Rmr32, ssafir.OpX86_SUB_Rmr32_Imm32},
			{ssafir.OpX86_SUB_R64_Rmr64_REX, ssafir.OpX86_SUB_Rmr64_Imm32_REX},
		},
		ssafir.OpMultiply: {
			{ssafir.OpX86_MUL_Rmr8, 0},
			{ssafir.OpX86_MUL_Rmr16, 0},
			{ssafir.OpX86_MUL_Rmr32, 0},
			{ssafir.OpX86_MUL_Rmr64_REX, 0},
		},
		ssafir.OpDivide: {
			{ssafir.OpX86_DIV_Rmr8, 0},
			{ssafir.OpX86_DIV_Rmr16, 0},
			{ssafir.OpX86_DIV_Rmr32, 0},
			{ssafir.OpX86_DIV_Rmr64_REX, 0},
		},
		ssafir.OpNegate: {
			{ssafir.OpX86_NEG_Rmr8, 0},
			{ssafir.OpX86_NEG_Rmr16, 0},
			{ssafir.OpX86_NEG_Rmr32, 0},
			{ssafir.OpX86_NEG_Rmr64_REX, 0},
		},
		ssafir.OpBitwiseOr: {
			{ssafir.OpX86_OR_R8_Rmr8, ssafir.OpX86_OR_Rmr8_Imm8},
			{ssafir.OpX86_OR_R16_Rmr16, ssafir.OpX86_OR_Rmr16_Imm16},
			{ssafir.OpX86_OR_R32_Rmr32, ssafir.OpX86_OR_Rmr32_Imm32},
			{ssafir.OpX86_OR_R64_Rmr64_REX, ssafir.OpX86_OR_Rmr64_Imm32_REX},
		},
		ssafir.OpBitwiseAnd: {
			{ssafir.OpX86_AND_R8_Rmr8, ssafir.OpX86_AND_Rmr8_Imm8},
			{ssafir.OpX86_AND_R16_Rmr16, ssafir.OpX86_AND_Rmr16_Imm16},
			{ssafir.OpX86_AND_R32_Rmr32, ssafir.OpX86_AND_Rmr32_Imm32},
			{ssafir.OpX86_AND_R64_Rmr64_REX, ssafir.OpX86_AND_Rmr64_Imm32_REX},
		},
		ssafir.OpBitwiseXor: {
			{ssafir.OpX86_XOR_R8_Rmr8, ssafir.OpX86_XOR_Rmr8_Imm8},
			{ssafir.OpX86_XOR_R16_Rmr16, ssafir.OpX86_XOR_Rmr16_Imm16},
			{ssafir.OpX86_XOR_R32_Rmr32, ssafir.OpX86_XOR_Rmr32_Imm32},
			{ssafir.OpX86_XOR_R64_Rmr64_REX, ssafir.OpX86_XOR_Rmr64_Imm32_REX},
		},
		ssafir.OpShiftLeft: {
			{ssafir.OpX86_SAL_Rmr8_CL, ssafir.OpX86_SAL_Rmr8_Imm8u},
			{ssafir.OpX86_SAL_Rmr16_CL, ssafir.OpX86_SAL_Rmr16_Imm8u},
			{ssafir.OpX86_SAL_Rmr32_CL, ssafir.OpX86_SAL_Rmr32_Imm8u},
			{ssafir.OpX86_SAL_Rmr64_CL_REX, ssafir.OpX86_SAL_Rmr64_Imm8u_REX},
		},
		ssafir.OpShiftRight: {
			{ssafir.OpX86_SAR_Rmr8_CL, ssafir.OpX86_SAR_Rmr8_Imm8u},
			{ssafir.OpX86_SAR_Rmr16_CL, ssafir.OpX86_SAR_Rmr16_Imm8u},
			{ssafir.OpX86_SAR_Rmr32_CL, ssafir.OpX86_SAR_Rmr32_Imm8u},
			{ssafir.OpX86_SAR_Rmr64_CL_REX, ssafir.OpX86_SAR_Rmr64_Imm8u_REX},
		},
		ssafir.OpEqual:              comparisons,
		ssafir.OpNotEqual:           comparisons,
		ssafir.OpLessThan:           comparisons,
		ssafir.OpLessThanOrEqual:    comparisons,
		ssafir.OpGreaterThan:        comparisons,
		ssafir.OpGreaterThanOrEqual: comparisons,
	}[info.Group][size][constArg]

	if op == 0 {
		panic(fmt.Errorf("%s: unexpected op %s", l.fset.Position(v.Pos), v.Op))
	}

	// Do any unusual tweaks.
	switch info.Group {
	case ssafir.OpMultiply:
		data.Args[0], data.Args[1] = data.Args[1], nil // The destination is implied.
	case ssafir.OpDivide:
		// Clear out RDX, as it forms the
		// top 64 bits of the divident.
		l.addInst(v, ssafir.OpX86_XOR_R64_Rmr64_REX, &x86InstructionData{
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
		// If our data is a constant, then
		// we use one of the immediate forms,
		// depending on the specific constant.
		if constArg == 0 {
			// Otherwise, move the shift (second arg) into CL.
			v.Extra = &Alloc{Dst: x86.RCX, Src: alloc.Data.(sys.Location)}
			l.MoveNumber(v)

			data.Args[1] = x86.CL // The shift is now in place.
			break
		}

		// Optimise for a shift of 1.
		if imm == 1 {
			data.Args[1] = nil // The value is implied.
			op = map[ssafir.Op][4]ssafir.Op{
				ssafir.OpShiftLeft: {
					ssafir.OpX86_SAL_Rmr8_1,
					ssafir.OpX86_SAL_Rmr16_1,
					ssafir.OpX86_SAL_Rmr32_1,
					ssafir.OpX86_SAL_Rmr64_1_REX,
				},
				ssafir.OpShiftRight: {
					ssafir.OpX86_SAR_Rmr8_1,
					ssafir.OpX86_SAR_Rmr16_1,
					ssafir.OpX86_SAR_Rmr32_1,
					ssafir.OpX86_SAR_Rmr64_1_REX,
				},
			}[info.Group][size]
		}

	case ssafir.OpEqual,
		ssafir.OpNotEqual,
		ssafir.OpLessThan,
		ssafir.OpLessThanOrEqual,
		ssafir.OpGreaterThan,
		ssafir.OpGreaterThanOrEqual:
		// First, we do the comparison.
		//
		// If our data is a constant, then
		// we use an immediate forms.
		if constArg == 0 {
			l.addInst(v, op, &x86InstructionData{
				Args: [4]any{
					alloc.Src,
					alloc.Data.(sys.Location),
				},
			})
		} else {
			imm, _ := constant.Int64Val(con)
			l.addInst(v, op, &x86InstructionData{
				Args: [4]any{
					alloc.Src,
					uint64(imm),
				},
			})
		}

		// If we're just used as block control,
		// we don't need to store the value, as
		// we use the flags directly instead.
		if alloc.Dst == nil {
			return
		}

		// Then we store the result.
		data.Args[0] = x86RegisterTo8(alloc.Dst)
		data.Args[1] = nil // There is no second arg.
		switch info.Group {
		case ssafir.OpEqual:
			op = ssafir.OpX86_SETE_Rmr8
		case ssafir.OpNotEqual:
			op = ssafir.OpX86_SETNE_Rmr8
		case ssafir.OpLessThan:
			op = ssafir.OpX86_SETL_Rmr8
		case ssafir.OpLessThanOrEqual:
			op = ssafir.OpX86_SETLE_Rmr8
		case ssafir.OpGreaterThan:
			op = ssafir.OpX86_SETG_Rmr8
		case ssafir.OpGreaterThanOrEqual:
			op = ssafir.OpX86_SETGE_Rmr8
		default:
			panic(fmt.Sprintf("internal error: op %s (group %s) not covered in comparisons", v.Op, info.Group))
		}
	}

	if op == 0 {
		panic(fmt.Errorf("%s: unexpected op %s", l.fset.Position(v.Pos), v.Op))
	}

	l.addInst(v, op, data)
}
