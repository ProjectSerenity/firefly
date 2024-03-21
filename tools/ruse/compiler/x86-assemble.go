// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// The bulk of the x86 assembler.

package compiler

import (
	"errors"
	"fmt"
	"math"
	"sort"
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

// x86Context is used to propagate the state
// while assembling a single x86 assembly
// function.
type x86Context struct {
	Comp   *compiler
	Func   *ssafir.Function
	Mode   x86.Mode // CPU mode.
	FSet   *token.FileSet
	Labels map[string]*x86Label
}

// x86Label contains information about a position
// label, as used in x86 assembly.
type x86Label struct {
	Label *ast.QuotedIdentifier // The label itself.
	Index int                   // The label's position in the instructino stream.
	Refs  []int                 // The indices of each (jump) instruction that references the label.
}

// Errorf is a helper function for including
// location information in error messages.
func (ctx *x86Context) Errorf(pos token.Pos, format string, v ...any) error {
	position := ctx.FSet.Position(pos)
	return errors.New(fmt.Sprintf("%s: ", position) + fmt.Sprintf(format, v...))
}

// x86InstructionData contains the information
// necessary to fully assemble an x86 instruction.
type x86InstructionData struct {
	Args [4]any // Unused args are untyped nil.

	Length    uint8         // Number of bytes of machine code (max 15).
	REX_W     bool          // Whether to force a REX prefix, with REX.W set.
	Mask      uint8         // Any EVEX mask register.
	Zero      bool          // Any EVEX zeroing.
	Broadcast bool          // Any EVEX memory broadcast.
	Prefixes  [5]x86.Prefix // Any optional legacy prefixes specified.
	PrefixLen uint8
}

func (d *x86InstructionData) String() string {
	if d.Args[0] == nil {
		return "(x86-instruction-data)"
	}

	ss := make([]string, 0, 4)
	for _, arg := range d.Args {
		if arg == nil {
			break
		}

		ss = append(ss, fmt.Sprintf("%v", arg))
	}

	return fmt.Sprintf("(x86-instruction-data %s)", strings.Join(ss, "  "))
}

// x86InstructionCandidate includes the
// information specified for each instruction
// in the generated instruction set data.
type x86InstructionCandidate struct {
	Op   ssafir.Op
	Inst *x86.Instruction
	Data *x86InstructionData
}

// x86OpToInstruction maps a `ssafir.Op` to
// an `*x86.Instruction`. If the op is not
// an x86 instruction, `nil` is returned.
func x86OpToInstruction(op ssafir.Op) *x86.Instruction {
	i := int(op - firstX86Op)
	if 0 <= i && i < len(x86.Instructions) {
		return x86.Instructions[i]
	}

	return nil
}

// assembleX86 assembles a single Ruse assembly
// function for x86.
func assembleX86(fset *token.FileSet, arch *sys.Arch, pkg *types.Package, assembly *ast.List, info *types.Info, sizes types.Sizes) (*ssafir.Function, error) {
	// The asm-func keyword is the first identifier,
	// and the function name is the second. All the
	// subsequent expressions are assembly, either
	// in the form of a quoted identifier for a
	// label or a list containing one instruction.
	name := assembly.Elements[1].(*ast.List).Elements[0].(*ast.Identifier)
	function := info.Definitions[name].(*types.Function)
	signature := function.Type().(*types.Signature)
	fun := &ssafir.Function{
		Name: name.Name,
		Code: assembly,
		Func: function,
		Type: signature,

		NamedValues: make(map[*types.Variable][]*ssafir.Value),
	}

	// Compile the body.
	c := &compiler{
		fset:  fset,
		arch:  arch,
		pkg:   pkg,
		info:  info,
		fun:   fun,
		list:  assembly,
		sizes: sizes,

		vars: make(map[*types.Variable]*ssafir.Value),
	}

	ctx := &x86Context{
		Comp:   c,
		Func:   fun,
		FSet:   fset,
		Labels: make(map[string]*x86Label),
	}

	for _, anno := range assembly.Annotations {
		if len(anno.X.Elements) == 0 {
			return nil, ctx.Errorf(anno.X.ParenClose, "invalid annotation: no keyword")
		}

		ident, ok := anno.X.Elements[0].(*ast.Identifier)
		if !ok {
			return nil, ctx.Errorf(anno.X.Elements[0].Pos(), "invalid annotation: bad keyword: %s %s", anno.X.Elements[0].String(), anno.X.Elements[0].Print())
		}

		switch ident.Name {
		case "mode":
			mode, ok := anno.X.Elements[1].(*ast.Literal)
			if !ok || mode.Kind != token.Integer {
				continue
			}

			num, err := strconv.Atoi(mode.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid mode %q: %v", mode.Value, err)
			}

			if ctx.Mode.Int != 0 {
				return nil, ctx.Errorf(anno.X.Elements[0].Pos(), "invalid annotation: cannot specify mode more than once")
			}

			switch num {
			case 16:
				ctx.Mode = x86.Mode16
			case 32:
				ctx.Mode = x86.Mode32
			case 64:
				ctx.Mode = x86.Mode64
			default:
				return nil, fmt.Errorf("invalid mode %q: %v", mode.Value, err)
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

	// We handle position labels in five phases:
	//
	// 	1. We identify the set of labels and detect any duplicates.
	// 	2. We locate each label in the instruction stream (during the main assembly loop).
	// 	   We also link each label reference to the corresponding label and detect any
	// 	   references to labels that have not been declared.
	// 	3. Having assembled the other instructions, we calculate the distance from each
	// 	   reference to a label, to that label. This allows us to complete the instruction
	// 	   forms. We do so using 32-bit jumps to start, as these are the largest jump form,
	// 	   so any subsequent optimisations can only decrease jump distances.
	// 	4. We conduct an optimisation pass, switching to 16-bit or 8-bit jumps if possible.
	// 	5. We update all jump distances to account for the optimisations.
	//
	// We could conduct further optimisation passes,
	// but for now, we don't.
	var labelNames []string // Stored in the order in which they appear.
	for _, expr := range assembly.Elements[2:] {
		label, ok := expr.(*ast.QuotedIdentifier)
		if !ok {
			continue
		}

		if prev, ok := ctx.Labels[label.X.Name]; ok {
			return nil, ctx.Errorf(label.Quote, "invalid assembly label: label %q redefined. Original definition at %s", label.X.Name, ctx.FSet.Position(prev.Label.Quote))
		}

		ctx.Labels[label.X.Name] = &x86Label{Label: label}
		labelNames = append(labelNames, label.X.Name)
	}

	var code x86.Code
	var rexwOverride bool
	var prefixes []x86.Prefix
	c.AddCallingConvention()
	c.AddFunctionPrelude()
	options := make([]x86InstructionCandidate, 0, 10)
	for _, expr := range assembly.Elements[2:] {
		// Labels phase 2: store label location.
		if label, ok := expr.(*ast.QuotedIdentifier); ok {
			val := ctx.Labels[label.X.Name]
			val.Index = len(fun.Entry.Values) // The index of the next instruction.
			continue
		}

		list, ok := expr.(*ast.List)
		if !ok {
			return nil, ctx.Errorf(expr.Pos(), "invalid assembly directive: expected an expression in list form or a label, found %s %s", expr.String(), expr.Print())
		}

		// Handle innstruction prefixes (see Intel x86-64 manual, volume 2A, chapter 2, golang.org/x/arch/x86/x86asm#Prefix).
		elts := list.Elements
		var group1, group2, group3, group4 string // Prefix groups.
		type Prefix struct {
			Name   *string
			Prefix x86.Prefix
		}

		prefixMap := map[string]*Prefix{ // Supported prefixes.
			"lock":     {&group1, x86.PrefixLock},
			"repne":    {&group1, x86.PrefixRepeatNot},
			"repnz":    {&group1, x86.PrefixRepeatNot},
			"rep":      {&group1, x86.PrefixRepeat},
			"repe":     {&group1, x86.PrefixRepeat},
			"repz":     {&group1, x86.PrefixRepeat},
			"unlikely": {&group2, x86.PrefixUnlikely},
			"likely":   {&group2, x86.PrefixLikely},
			"data16":   {&group3, x86.PrefixOperandSize},
			"data32":   {&group3, x86.PrefixOperandSize},
			"addr16":   {&group4, x86.PrefixAddressSize},
			"addr32":   {&group4, x86.PrefixAddressSize},
		}

		rep := false
		prefixes = prefixes[:0]
		rexwOverride = false
		for len(elts) > 0 {
			// The REX.W prefix is odd syntactically.
			if qual, ok := elts[0].(*ast.Qualified); ok {
				if qual.X.Name == "rex" && qual.Y.Name == "w" {
					if rexwOverride {
						return nil, ctx.Errorf(qual.X.NamePos, "invalid assembly directive: rex.w prefix repeated")
					}

					rexwOverride = true
					elts = elts[1:]
					continue
				}
			}

			ident, ok := elts[0].(*ast.Identifier)
			if !ok {
				break
			}

			var found *Prefix
			for prefix, data := range prefixMap {
				if prefix == ident.Name {
					found = data
					switch ident.Name {
					case "rep", "repe", "repz", "repne", "repnz":
						rep = true
					}

					break
				}
			}

			if found == nil {
				break // Not a known prefix, should be a mnemonic.
			}

			if *found.Name == ident.Name {
				return nil, ctx.Errorf(ident.NamePos, "invalid assembly directive: %s prefix repeated", ident.Name)
			}

			if *found.Name != "" {
				return nil, ctx.Errorf(ident.NamePos, "invalid assembly directive: %s prefix cannot be used with %s prefix", ident.Name, *found.Name)
			}

			*found.Name = ident.Name
			prefixes = append(prefixes, found.Prefix)
			elts = elts[1:]
		}

		if len(elts) == 0 {
			return nil, ctx.Errorf(list.ParenClose, "invalid assembly directive: missing instruction mnemonic")
		}

		// Check any annotations for exact instruction match.
		var matchUID string
		for _, anno := range list.Annotations {
			ident, ok := anno.X.Elements[0].(*ast.Identifier)
			if !ok {
				continue
			}

			switch ident.Name {
			case "match":
				if len(anno.X.Elements) != 2 {
					return nil, ctx.Errorf(anno.Quote, "invalid instruction annotation: expected an instruction UID, found %d parameters", len(anno.X.Elements)-1)
				}

				x := anno.X.Elements[1]
				uid, ok := x.(*ast.Identifier)
				if !ok {
					return nil, ctx.Errorf(x.Pos(), "invalid instruction annotation: invalid instruction UID: %s %q", x, x.Print())
				}

				if matchUID != "" {
					return nil, ctx.Errorf(anno.Quote, "invalid instruction annotation: instruction UID specified multiple times")
				}

				matchUID = uid.Name

				// Do a quick sense check.
				if _, ok := x86.InstructionsByUID[matchUID]; !ok {
					return nil, ctx.Errorf(uid.NamePos, "invalid instruction annotation: unrecognised instruction UID %q", matchUID)
				}
			}
		}

		mnemonic, ok := elts[0].(*ast.Identifier)
		if !ok {
			return nil, ctx.Errorf(elts[0].Pos(), "invalid assembly directive: expected an instruction mnemonic, found %s", elts[0].Print())
		}

		candidates, ok := x86MnemonicToOps[mnemonic.Name]
		if !ok {
			return nil, ctx.Errorf(mnemonic.NamePos, "invalid assembly directive: mnemonic %q not recognised", mnemonic.Name)
		}

		params := elts[1:]
		options = options[:0]
		rightArity := false
		for _, op := range candidates {
			inst := x86OpToInstruction(op)
			if inst == nil {
				return nil, fmt.Errorf("internal error: found no instruction data for op %s", op)
			}

			if inst.Encoding.NoRepPrefixes && rep {
				return nil, ctx.Errorf(mnemonic.NamePos, "invalid assembly directive: mnemonic %q cannot be used with repeat prefixes", mnemonic.Name)
			}

			if matchUID != "" && matchUID != inst.UID {
				continue
			}

			if len(params) < inst.MinArgs || inst.MaxArgs < len(params) {
				if matchUID != "" {
					return nil, ctx.Errorf(mnemonic.NamePos, "invalid assembly directive: %s does not match instruction %s: got %d parameters, want %d-%d", list.Print(), matchUID, len(params), inst.MinArgs, inst.MaxArgs)
				}

				continue
			}

			rightArity = true
			data, err := ctx.Match(list, params, op, inst)
			if err != nil {
				return nil, err
			}

			if data == nil {
				if matchUID != "" {
					return nil, ctx.Errorf(mnemonic.NamePos, "invalid assembly directive: %s does not match instruction %s", list.Print(), matchUID)
				}

				continue
			}

			// Fill in the common fields and
			// encode the instruction.
			data.PrefixLen = uint8(copy(data.Prefixes[:], prefixes))
			data.REX_W = rexwOverride
			err = x86EncodeInstruction(&code, ctx.Mode, op, data)
			if err != nil {
				return nil, err
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

				if code.CodeOffsetLen == 0 && code.ImmediateLen == 0 && code.DisplacementLen == 0 {
					return nil, ctx.Errorf(link.Pos, "internal error: instruction specified a link to %s, but no code offset, immediate, or displacement was produced", link.Name)
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
					InnerOffset:  code.Len() - (code.CodeOffsetLen + code.ImmediateLen + code.DisplacementLen),
					InnerAddress: uintptr(code.Len()), // The instruction is relative to the next instruction.
					Link:         link,
				}

				link.Offset = len(ctx.Func.Entry.Values)
				data.Args[i] = link2
			}

			data.Length = uint8(code.Len())
			options = append(options, x86InstructionCandidate{Op: op, Inst: inst, Data: data})
		}

		if len(options) == 0 && matchUID != "" {
			return nil, ctx.Errorf(mnemonic.NamePos, "invalid assembly directive: %s does not match instruction %s", list.Print(), matchUID)
		}

		// Error handling for if we've got
		// completely the wrong number of
		// args. It's a bit verbose, but
		// it gives us better error messages.
		if len(options) == 0 && !rightArity {
			got := make([]string, len(params))
			for i, arg := range params {
				got[i] = arg.Print()
			}

			var want []int
			seenArity := make(map[int]bool)
			for _, op := range candidates {
				inst := x86OpToInstruction(op)
				for arity := inst.MinArgs; arity <= inst.MaxArgs; arity++ {
					if !seenArity[arity] {
						seenArity[arity] = true
						want = append(want, arity)
					}
				}
			}

			sort.Ints(want)

			var wantArities string
			switch len(want) {
			case 1:
				wantArities = strconv.Itoa(want[0])
			case 2:
				wantArities = fmt.Sprintf("%d or %d", want[0], want[1])
			default:
				text := make([]string, len(want))
				for i, arity := range want {
					text[i] = strconv.Itoa(arity)
					if i == len(want)-1 {
						text[i] = "or " + text[i]
					}
				}

				wantArities = strings.Join(text, ", ")
			}

			return nil, ctx.Errorf(mnemonic.NamePos, "invalid assembly directive: expected %s arguments, found %d: %s", wantArities, len(params), strings.Join(got, " "))
		}

		if len(options) == 0 {
			return nil, ctx.Errorf(mnemonic.NamePos, "invalid assembly directive: no matching instruction found for %s", list.Print())
		}

		// If we found more than one valid option,
		// we sort them by encoded length and pick
		// the shortest.
		if len(options) > 1 {
			sort.Slice(options, func(i, j int) bool {
				// First, prioritise shorter machine
				// code sequences.
				if options[i].Data.Length != options[j].Data.Length {
					return options[i].Data.Length < options[j].Data.Length
				}

				// Next, prefer options with smaller
				// data operations.
				if options[i].Inst.DataSize != 0 && options[j].Inst.DataSize != 0 &&
					options[i].Inst.DataSize != options[j].Inst.DataSize {
					return options[i].Inst.DataSize < options[j].Inst.DataSize
				}

				// If an EVEX encoding is not necessary
				// for an EVEX instruction, we fall back
				// to VEX encodings, which are smaller.
				// In that case, a VEX instruction may
				// also match. Prefer VEX over EVEX, as
				// it's more intuitive and doesn't have
				// any other effect.
				enc1 := options[i].Inst.Encoding
				enc2 := options[j].Inst.Encoding
				if enc1.VEX != enc2.VEX || enc1.EVEX != enc2.EVEX {
					return enc1.VEX
				}

				// Finally, resort to a comparison of
				// the opcode constant. It means little,
				// but it's consistent.
				return options[i].Op < options[j].Op
			})
		}

		option := options[0]

		c.currentBlock.NewValueExtra(list.ParenOpen, list.ParenClose+1, option.Op, nil, option.Data)
	}

	// Labels phase 3: calculate jump distances.
	for _, name := range labelNames {
		label := ctx.Labels[name]
		if len(label.Refs) == 0 {
			return nil, ctx.Errorf(label.Label.Quote, "label %q is not referenced by any instructions", label.Label.X.Name)
		}

		for _, ref := range label.Refs {
			v := fun.Entry.Values[ref]
			data := v.Extra.(*x86InstructionData)
			jumpLength := ctx.calculateJumpDistance(label, ref)

			// First, check we can store the jump in a 32-bit
			// relative (signed) address.
			if jumpLength < math.MinInt32 || math.MaxInt32 < jumpLength {
				return nil, ctx.Errorf(v.Pos, "jump to %q is too far to be encoded", name)
			}

			data.Args[0] = uint64(jumpLength)
		}
	}

	// Label phase 4: optimise to smaller
	// jump instructions if possible.
	for _, name := range labelNames {
		label := ctx.Labels[name]
		for _, ref := range label.Refs {
			v := fun.Entry.Values[ref]
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
					return nil, err
				}

				data.Length = uint8(code.Len())
				continue
			}

			newUID16 := strings.Replace(inst.UID, "32", "16", 1)
			inst16, ok := x86.InstructionsByUID[newUID16]
			if ok && inst16.Supports(ctx.Mode) && math.MinInt16 <= jumpLength && jumpLength <= math.MaxInt16 {
				v.Op = x86Opcodes[newUID16]
				err := x86EncodeInstruction(&code, ctx.Mode, v.Op, data)
				if err != nil {
					return nil, err
				}

				data.Length = uint8(code.Len())
				continue
			}
		}
	}

	// Labels phase 5: re-calculate jump distances, after any optimisations.
	for _, name := range labelNames {
		label := ctx.Labels[name]
		for _, ref := range label.Refs {
			data := fun.Entry.Values[ref].Extra.(*x86InstructionData)
			jumpLength := ctx.calculateJumpDistance(label, ref)
			data.Args[0] = uint64(jumpLength + int64(data.Length)) // Offset the subtraction done in the encoding process.
		}
	}

	// Finally, complete any link references.
	var offset int
	for _, value := range fun.Entry.Values {
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

	return fun, nil
}

// calculateJumpDistance calculates the
// number of machine code bytes between a
// jump instruction and the location label
// it jumps to.
func (ctx *x86Context) calculateJumpDistance(label *x86Label, ref int) int64 {
	// For each use, calculate the jump distance. For
	// backwards jumps (where the label is declared
	// before the jump), we iterate from the instruction
	// immediately after the label (at label.Index) to
	// the index of the jump instruction, including all
	// of the instruction lengths (including the jump).
	//
	// For forwards jumps (where the label is declared
	// after the jump), we iterate from the instruction
	// after the jump to the instruction before the label,
	// including all of the instruction lengths.
	//
	// Note that in this calculation, we measure from
	// the first machine code byte after the jump
	// instruction, which is how the jump distance is
	// encoded. However, the length of the jump instruction
	// is subtracted from this when it's stored into the
	// argument value, as this is then reversed during
	// the encoding process, so that we handle literal
	// relative addresses correctly.

	// We store the result in an int64 so we can identify
	// jumps that exceed 32 bits in length, rather than
	// wrapping silently.
	var jumpLength int64
	if label.Index <= ref {
		// Backwards jump.
		for i := label.Index; i <= ref; i++ {
			data := ctx.Func.Entry.Values[i].Extra.(*x86InstructionData)
			jumpLength -= int64(data.Length)
		}
	} else {
		// Forwards jump.
		for i := ref + 1; i < label.Index; i++ {
			data := ctx.Func.Entry.Values[i].Extra.(*x86InstructionData)
			jumpLength += int64(data.Length)
		}
	}

	return jumpLength
}

// handleInstructionAnnotations processes
// the annotations for a single instruction,
// updating `data` if necessary.
func (ctx *x86Context) handleInstructionAnnotations(data *x86InstructionData, list *ast.List, inst *x86.Instruction) (ok bool, err error) {
	var seenBroadcast, seenMask, seenZero bool
	for _, anno := range list.Annotations {
		ident, ok := anno.X.Elements[0].(*ast.Identifier)
		if !ok {
			continue
		}

		switch ident.Name {
		case "broadcast":
			if len(anno.X.Elements) != 2 {
				return false, ctx.Errorf(anno.Quote, "invalid EVEX annotation: expected a broadcast mode, found %d parameters", len(anno.X.Elements)-1)
			}

			x := anno.X.Elements[1]
			k, ok := x.(*ast.Identifier)
			if !ok {
				return false, ctx.Errorf(x.Pos(), "invalid EVEX annotation: invalid broadcast mode: %s %q", x, x.Print())
			}

			switch k.Name {
			case "true":
				data.Broadcast = true
			case "false":
				data.Broadcast = false
			default:
				return false, ctx.Errorf(k.NamePos, "invalid EVEX annotation: invalid broadcast mode: %s %q", k, k.Print())
			}

			// We now know the mode is valid,
			// but we can only use them with
			// EVEX instructions.
			if !inst.Encoding.EVEX {
				// Proceed without error but
				// skip this instruction form.
				return false, nil
			}

			if seenBroadcast {
				return false, ctx.Errorf(k.NamePos, "invalid EVEX annotation: broadcast mode specified twice")
			}

			seenBroadcast = true
		case "mask":
			if len(anno.X.Elements) != 2 {
				return false, ctx.Errorf(anno.Quote, "invalid EVEX annotation: expected a mask register, found %d parameters", len(anno.X.Elements)-1)
			}

			// We accept a mask register by name
			// (kX), in the range 1-7. Note that
			// k0 is not valid.
			switch k := anno.X.Elements[1].(type) {
			case *ast.Identifier:
				switch k.Name {
				case "k1", "k2", "k3", "k4", "k5", "k6", "k7":
					data.Mask = k.Name[1] - '0'
				default:
					return false, ctx.Errorf(k.NamePos, "invalid EVEX annotation: invalid mask register: %s %q", k, k.Print())
				}
			}

			// We now know the mask is valid,
			// but we can only use them with
			// EVEX instructions.
			if !inst.Encoding.EVEX {
				// Proceed without error but
				// skip this instruction form.
				return false, nil
			}

			if seenMask {
				return false, ctx.Errorf(anno.Quote, "invalid EVEX annotation: mask register specified twice")
			}

			seenMask = true
		case "zero":
			if len(anno.X.Elements) != 2 {
				return false, ctx.Errorf(anno.Quote, "invalid EVEX annotation: expected a zeroing mode, found %d parameters", len(anno.X.Elements)-1)
			}

			x := anno.X.Elements[1]
			k, ok := x.(*ast.Identifier)
			if !ok {
				return false, ctx.Errorf(x.Pos(), "invalid EVEX annotation: invalid zeroing mode: %s %q", x, x.Print())
			}

			switch k.Name {
			case "true", "false":
				data.Zero = k.Name == "true"
			default:
				return false, ctx.Errorf(k.NamePos, "invalid EVEX annotation: invalid zeroing mode: %s %q", k, k.Print())
			}

			// We now know the mode is valid,
			// but we can only use them with
			// EVEX instructions.
			if !inst.Encoding.EVEX {
				// Proceed without error but
				// skip this instruction form.
				return false, nil
			}

			if seenZero {
				return false, ctx.Errorf(k.NamePos, "invalid EVEX annotation: zeroing mode specified twice")
			}

			seenZero = true
		}
	}

	return true, nil
}

// Match matches an assembly instruction to
// an x86 instruction form. If there is no
// match, Match returns `nil, nil`.
func (ctx *x86Context) Match(list *ast.List, args []ast.Expression, op ssafir.Op, inst *x86.Instruction) (data *x86InstructionData, err error) {
	if len(args) < inst.MinArgs || inst.MaxArgs < len(args) || !inst.Supports(ctx.Mode) {
		return nil, nil
	}

	data = &x86InstructionData{}

	// Check any annotations for EVEX parameters.
	ok, err := ctx.handleInstructionAnnotations(data, list, inst)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, nil
	}

	defer func() {
		v := recover()
		if v == nil {
			return
		}

		e, ok := v.(error)
		if ok {
			err = e
			return
		}

		panic(v)
	}()

	for i, operand := range inst.Operands {
		if operand == nil || i >= len(args) {
			break
		}

		var arg any
		switch operand.Type {
		case x86.TypeSignedImmediate:
			arg = ctx.matchSignedImmediate(args[i], operand)
		case x86.TypeUnsignedImmediate:
			arg = ctx.matchUnsignedImmediate(args[i], operand)
		case x86.TypeRegister:
			arg = ctx.matchRegister(args[i], operand)
		case x86.TypeStackIndex:
			arg = ctx.matchStackIndex(args[i], operand)
		case x86.TypeRelativeAddress:
			arg = ctx.matchRelativeAddress(args[i], operand)
		case x86.TypeFarPointer:
			arg = ctx.matchFarPointer(args[i], operand)
		case x86.TypeMemory:
			arg = ctx.matchMemory(args[i], operand)
		case x86.TypeMemoryOffset:
			arg = ctx.matchMemoryOffset(args[i], operand)
		case x86.TypeStringDst:
			arg = ctx.matchStringDst(args[i], operand)
		case x86.TypeStringSrc:
			arg = ctx.matchStringSrc(args[i], operand)
		default:
			panic("unexpected parameter type: " + operand.Type.String())
		}

		if arg == nil {
			return nil, nil
		}

		data.Args[i] = arg
		reg, ok := arg.(*x86.Register)
		if ok && reg.EVEX && !inst.Encoding.EVEX {
			return nil, nil
		}
	}

	return data, nil
}

func (ctx *x86Context) isIdent(arg ast.Expression, options ...string) bool {
	ident, ok := arg.(*ast.Identifier)
	if !ok {
		return false
	}

	for _, option := range options {
		if option == ident.Name {
			return true
		}
	}

	return false
}

func (ctx *x86Context) rejectedBySizeHint(list *ast.List, bits int) bool {
	// The size of memory being copied can be
	// specified in an annotation.
	if len(list.Annotations) == 0 || bits == 0 {
		return false
	}

	foundMatch := false
	foundSizeHint := false
	for _, anno := range list.Annotations {
		if len(anno.X.Elements) != 2 {
			continue
		}

		got, ok := anno.X.Elements[0].(*ast.Identifier)
		if !ok {
			continue
		}

		numLit, ok := anno.X.Elements[1].(*ast.Literal)
		if !ok || numLit.Kind != token.Integer {
			continue
		}

		num, err := strconv.Atoi(numLit.Value)
		if err != nil {
			panic("invalid integer literal: " + err.Error())
		}

		switch got.Name {
		case "bits":
			foundSizeHint = true
			foundMatch = bits == num
		case "bytes":
			foundSizeHint = true
			foundMatch = bits == num*8
		}
	}

	// We only reject the argument if it
	// doesn't have the annotation we want
	// and does have another size annotation.
	if foundSizeHint && !foundMatch {
		return true
	}

	return false
}

func (ctx *x86Context) matchSpecialForm(list *ast.List, operand *x86.Operand) any {
	if len(list.Elements) != 2 {
		return nil
	}

	ident, ok := list.Elements[0].(*ast.Identifier)
	if !ok {
		return nil
	}

	ref := ident.Name
	switch ref {
	case "string-pointer", "@":
		// This must be an immediate with
		// enough space for a pointer, or
		// a relative address with plenty
		// of space in case the functions
		// end up far apart.
		var size uint8
		var linkType ssafir.LinkType
		switch operand.Encoding {
		case x86.EncodingImmediate:
			size = ctx.Mode.Int
			linkType = ssafir.LinkFullAddress
			if operand.Bits < int(ctx.Mode.Int) {
				return nil
			}
		case x86.EncodingCodeOffset:
			size = 32
			if ctx.Mode.Int == 16 {
				size = 16
			}

			linkType = ssafir.LinkRelativeAddress
			if operand.Bits != int(size) {
				return nil
			}
		default:
			return nil
		}

		ident, ok := list.Elements[1].(*ast.Identifier)
		if !ok {
			var qualified *ast.Qualified
			qualified, ok = list.Elements[1].(*ast.Qualified)
			if ok {
				ident = qualified.Y
			}
		}
		if !ok {
			panic("internal error: expected identifier, got " + list.Elements[1].Print())
		}

		obj := ctx.Comp.info.Uses[ident]
		if obj == nil {
			panic(ctx.Errorf(ident.NamePos, "undefined: %s", ident.Print()))
		}

		// Type-check the reference.
		switch ref {
		case "string-pointer":
			con, ok := obj.(*types.Constant)
			if !ok {
				panic(ctx.Errorf(ident.NamePos, "cannot use %s as string constant in symbol reference", obj))
			}

			typ := types.Underlying(con.Type())
			if typ != types.String && typ != types.UntypedString {
				panic(ctx.Errorf(ident.NamePos, "cannot use %s (%s) as string constant in symbol reference", con, con.Type()))
			}
		case "@":
			// First, handle function references.
			if fun, ok := obj.(*types.Function); ok {
				// The default ABI is not guaranteed
				// to be stable so if the function we're
				// calling uses it and has any params
				// or result, we return an error.
				if fun.ABI() == nil {
					sig := fun.Type().(*types.Signature)
					if len(sig.Params()) != 0 || sig.Result() != nil {
						panic(ctx.Errorf(ident.NamePos, "cannot call %s from assembly: function uses default ABI so its calling convention is unstable", ident.Name))
					}
				}

				// All good.
				break
			}

			// Otherwise, we expect a constant.
			con, ok := obj.(*types.Constant)
			if !ok {
				panic(ctx.Errorf(ident.NamePos, "cannot take the address of %s in symbol reference", obj))
			}

			val := con.Value()
			kind := val.Kind()
			switch kind {
			case constant.Array, constant.Integer, constant.String:
			default:
				panic(ctx.Errorf(ident.NamePos, "cannot take the address of %s in symbol reference", kind))
			}
		}

		link := &ssafir.Link{
			Pos:  ident.NamePos,
			Name: obj.Package().Path + "." + obj.Name(),
			Type: linkType,
			Size: size,
		}

		return link
	case "len":
		// This must be an immediate.
		if operand.Encoding != x86.EncodingImmediate {
			return nil
		}

		// Get the constant value from the type
		// info.
		typeAndValue := ctx.Comp.info.Types[list]
		if typeAndValue.Type != types.Int {
			panic("internal error: unexpected result type from len: " + typeAndValue.Type.String())
		}

		length, ok := constant.Int64Val(typeAndValue.Value)
		if !ok {
			panic("internal error: unexpected result value from len: " + typeAndValue.Value.String())
		}

		return uint64(length)
	case "int8", "int16", "int32", "int64",
		"uint8", "uint16", "uint32", "uint64":
		// This is where we reference a Ruse constant
		// and insert it into the assembly.
		typeAndValue := ctx.Comp.info.Types[list]
		size := ctx.Comp.sizes.SizeOf(typeAndValue.Type)
		if size*8 != operand.Bits {
			return nil
		}

		val, ok := constant.Uint64Val(typeAndValue.Value)
		if !ok {
			panic("internal error: unexpected result value from " + ref + ": " + typeAndValue.Value.String())
		}

		return val
	default:
		return nil
	}
}

func (ctx *x86Context) matchUint(arg ast.Expression, bits int) any {
	lit, ok := arg.(*ast.Literal)
	if !ok || lit.Kind != token.Integer {
		return nil
	}

	// ENTER's second argument must be in the range 0-31.
	if bits == 5 {
		v, err := strconv.ParseUint(lit.Value, 0, 8)
		if err != nil || v >= 32 {
			return nil
		}

		return v
	}

	v, err := strconv.ParseUint(lit.Value, 0, bits)
	if err != nil {
		return nil
	}

	return v
}

func (ctx *x86Context) matchSint(arg ast.Expression, bits int) any {
	lit, ok := arg.(*ast.Literal)
	if !ok || lit.Kind != token.Integer {
		return nil
	}

	v, err := strconv.ParseInt(lit.Value, 0, bits)
	if err != nil {
		// Signed integers are encoded as unsigned
		// integers, so we also accept unsigned
		// values.
		x, err := strconv.ParseUint(lit.Value, 0, bits)
		if err != nil {
			return nil
		}

		return x
	}

	return uint64(v)
}

func (ctx *x86Context) matchSpecificUint(arg ast.Expression, want ...uint8) any {
	lit, ok := arg.(*ast.Literal)
	if !ok || lit.Kind != token.Integer {
		return nil
	}

	got, err := strconv.ParseUint(lit.Value, 0, 8)
	if err != nil {
		return nil
	}

	for _, want := range want {
		if want == uint8(got) {
			return want
		}
	}

	return nil
}

func (ctx *x86Context) matchReg(arg ast.Expression, registers ...*x86.Register) *x86.Register {
	ident, ok := arg.(*ast.Identifier)
	if !ok {
		return nil
	}

	for _, reg := range registers {
		if reg.Name == ident.Name {
			if reg.MinMode != 0 && reg.MinMode > ctx.Mode.Int {
				panic(ctx.Errorf(arg.Pos(), "register %s cannot be used in %d-bit mode", ident.Name, ctx.Mode.Int))
			}

			return reg
		}

		for _, alias := range reg.Aliases {
			if alias == ident.Name {
				return reg
			}
		}
	}

	return nil
}

func (ctx *x86Context) matchRegPair(base, index *x86.Register) *x86.Register {
	switch {
	case base == x86.BX && index == x86.SI:
		return x86.BX_SI
	case base == x86.BX && index == x86.DI:
		return x86.BX_DI
	case base == x86.BP && index == x86.SI:
		return x86.BP_SI
	case base == x86.BP && index == x86.DI:
		return x86.BP_DI
	}

	return nil
}

func (ctx *x86Context) matchSignedImmediate(arg ast.Expression, operand *x86.Operand) any {
	if operand.Encoding == x86.EncodingNone {
		lit, ok := arg.(*ast.Literal)
		if !ok || lit.Kind != token.Integer {
			return nil
		}

		if lit.Value != operand.Syntax {
			return nil
		}

		// We store nothing.
		return struct{}{}
	}

	if list, ok := arg.(*ast.List); ok {
		return ctx.matchSpecialForm(list, operand)
	}

	return ctx.matchSint(arg, operand.Bits)
}

func (ctx *x86Context) matchUnsignedImmediate(arg ast.Expression, operand *x86.Operand) any {
	if operand.Encoding == x86.EncodingNone {
		lit, ok := arg.(*ast.Literal)
		if !ok || lit.Kind != token.Integer {
			return nil
		}

		if lit.Value != operand.Syntax {
			return nil
		}

		// We store nothing.
		return struct{}{}
	}

	if list, ok := arg.(*ast.List); ok {
		return ctx.matchSpecialForm(list, operand)
	}

	return ctx.matchUint(arg, operand.Bits)
}

func (ctx *x86Context) matchRegister(arg ast.Expression, operand *x86.Operand) any {
	reg := ctx.matchReg(arg, operand.Registers...)
	if reg == nil {
		return nil
	}

	return reg
}

func (ctx *x86Context) matchStackIndex(arg ast.Expression, operand *x86.Operand) any {
	ident, ok := arg.(*ast.Identifier)
	if !ok {
		return nil
	}

	// For ST, we store nothing.
	if operand.Encoding == x86.EncodingNone {
		if ident.Name != "st" {
			return nil
		}

		return struct{}{}
	}

	for _, reg := range operand.Registers {
		if reg.Name == ident.Name {
			return reg
		}

		for _, alias := range reg.Aliases {
			if alias == ident.Name {
				return reg
			}
		}
	}

	return nil
}

func (ctx *x86Context) matchRelativeAddress(arg ast.Expression, operand *x86.Operand) any {
	// Handle labels. We only accept labels for
	// 32-bit relative jumps so that optimisations
	// don't increase any jump distances.
	if label, ok := arg.(*ast.QuotedIdentifier); ok {
		if operand.Bits < 32 {
			return nil
		}

		// Register our use.
		node, ok := ctx.Labels[label.X.Name]
		if !ok {
			panic(ctx.Errorf(label.Quote, "label %q is not defined in function %q", label.X.Name, ctx.Func.Name))
		}

		node.Refs = append(node.Refs, len(ctx.Func.Entry.Values)) // The index this instruction will have.

		return uint64(0) // A placeholder address, we complete it later.
	}

	if list, ok := arg.(*ast.List); ok {
		return ctx.matchSpecialForm(list, operand)
	}

	// Relative addresses can't be unsigned.
	return ctx.matchSint(arg, operand.Bits)
}

func (ctx *x86Context) matchFarPointer(arg ast.Expression, operand *x86.Operand) any {
	pair, ok := arg.(*ast.List)
	if !ok || len(pair.Elements) != 2 {
		return nil
	}

	base := ctx.matchUint(pair.Elements[0], 16)
	index := ctx.matchUint(pair.Elements[1], operand.Bits-16)
	if base == nil || index == nil {
		return nil
	}

	// We concatenate the two pointers into a single 64-bit
	// integer. This is fine, as the largest size is a 16-bit
	// base and a 32-bit index. We encode the base in the high
	// bits, as it is enocded after the index.

	return (base.(uint64) << (operand.Bits - 16)) | index.(uint64)
}

func (ctx *x86Context) matchMemory(arg ast.Expression, operand *x86.Operand) any {
	list, ok := arg.(*ast.List)
	if !ok {
		return nil
	}

	// The size of memory being copied can be
	// specified in an annotation.
	if ctx.rejectedBySizeHint(list, operand.Bits) {
		return nil
	}

	displacementSize := int(ctx.Mode.Int)
	if displacementSize == 64 {
		// We can only use 64-bit displacements
		// in a memory offset, which is handled
		// separately.
		displacementSize = 32
	}

	// We allow an optional segment prefix, which we
	// strip from the arguments here to simplify the
	// remaining parsing.
	elements := list.Elements
	var segment *x86.Register
	if len(elements) > 1 && ctx.isIdent(elements[0], "+") {
		if sreg := ctx.matchReg(elements[1], x86.Registers16bitSegment...); sreg != nil {
			segment = sreg
			elements = append([]ast.Expression{elements[0]}, elements[2:]...)
		}
	} else if len(elements) > 0 {
		if sreg := ctx.matchReg(elements[0], x86.Registers16bitSegment...); sreg != nil {
			segment = sreg
			elements = elements[1:]
		}
	}

	// See https://blog.yossarian.net/2020/06/13/How-x86-addresses-memory
	// Options:
	//
	// 1. (+ base (* index scale) displacement)
	// 2. (+ (* index scale) displacement)
	// 3. (+ base (* index scale))
	// 4. (* index scale)
	// 5. (+ base index displacement)
	// 6. (+ base displacement)
	// 7. (+ base index)
	// 8. (base)
	// 9. (displacement)
	switch len(elements) {
	case 4: // 1, 5.
		// 5. (+ base index displacement)
		if ctx.isIdent(elements[0], "+") {
			base := ctx.matchReg(elements[1], x86.RegistersAddress...)
			index := ctx.matchReg(elements[2], x86.RegistersIndex...)
			displ, ok4 := ctx.matchSint(elements[3], displacementSize).(uint64)
			if base != nil && index != nil && ok4 {
				if pair := ctx.matchRegPair(base, index); pair != nil {
					// Legacy 16-bit addressing form.
					return &x86.Memory{Segment: segment, Base: pair, Displacement: int64(displ)}
				}

				return &x86.Memory{Segment: segment, Base: base, Index: index, Displacement: int64(displ)}
			}
		}

		// 1. (+ base (* index scale) displacement)
		mul, ok := elements[2].(*ast.List)
		if ctx.isIdent(elements[0], "+") && ok && ctx.isIdent(mul.Elements[0], "*") {
			base := ctx.matchReg(elements[1], x86.RegistersAddress...)
			index := ctx.matchReg(mul.Elements[1], x86.RegistersIndex...)
			scale, ok3 := ctx.matchSpecificUint(mul.Elements[2], 1, 2, 4, 8).(uint8)
			displ, ok4 := ctx.matchSint(elements[3], displacementSize).(uint64)
			if base != nil && index != nil && ok3 && ok4 {
				// We can't have a register pair here, as we have a scale, so we don't check for one.
				return &x86.Memory{Segment: segment, Base: base, Index: index, Scale: scale, Displacement: int64(displ)}
			}
		}
	case 3: // 2, 3, 4, 6, 7.
		// 6. (+ base displacement)
		if ctx.isIdent(elements[0], "+") {
			base := ctx.matchReg(elements[1], x86.RegistersAddress...)
			displ, ok4 := ctx.matchSint(elements[2], displacementSize).(uint64)
			if base != nil && ok4 {
				return &x86.Memory{Segment: segment, Base: base, Displacement: int64(displ)}
			}
		}

		// 7. (+ base index)
		if ctx.isIdent(elements[0], "+") {
			base := ctx.matchReg(elements[1], x86.RegistersAddress...)
			index := ctx.matchReg(elements[2], x86.RegistersIndex...)
			if base != nil && index != nil {
				if pair := ctx.matchRegPair(base, index); pair != nil {
					// Legacy 16-bit addressing form.
					return &x86.Memory{Segment: segment, Base: pair}
				}

				return &x86.Memory{Segment: segment, Base: base, Index: index}
			}
		}

		// 4. (* index scale)
		if ctx.isIdent(elements[0], "*") {
			index := ctx.matchReg(elements[1], x86.RegistersIndex...)
			scale, ok3 := ctx.matchSpecificUint(elements[2], 1, 2, 4, 8).(uint8)
			if index != nil && ok3 {
				return &x86.Memory{Segment: segment, Index: index, Scale: scale}
			}
		}

		// 2. (+ (* index scale) displacement)
		mul, ok := elements[1].(*ast.List)
		if ctx.isIdent(elements[0], "+") && ok && ctx.isIdent(mul.Elements[0], "*") {
			index := ctx.matchReg(mul.Elements[1], x86.RegistersIndex...)
			scale, ok3 := ctx.matchSpecificUint(mul.Elements[2], 1, 2, 4, 8).(uint8)
			displ, ok4 := ctx.matchSint(elements[2], displacementSize).(uint64)
			if index != nil && ok3 && ok4 {
				return &x86.Memory{Segment: segment, Index: index, Scale: scale, Displacement: int64(displ)}
			}
		}

		// 3. (+ base (* index scale))
		mul, ok = elements[2].(*ast.List)
		if ctx.isIdent(elements[0], "+") && ok && ctx.isIdent(mul.Elements[0], "*") {
			base := ctx.matchReg(elements[1], x86.RegistersAddress...)
			index := ctx.matchReg(mul.Elements[1], x86.RegistersIndex...)
			scale, ok := ctx.matchSpecificUint(mul.Elements[2], 1, 2, 4, 8).(uint8)
			if base != nil && index != nil && ok {
				// We can't have a register pair here, as we have a scale, so we don't check for one.

				return &x86.Memory{Segment: segment, Base: base, Index: index, Scale: scale}
			}
		}
	case 1: // 8, 9.
		// 8. (base)
		base := ctx.matchReg(elements[0], x86.RegistersAddress...)
		if base != nil {
			return &x86.Memory{Segment: segment, Base: base}
		}

		// 9. (displacement)
		displ, ok := ctx.matchSint(elements[0], displacementSize).(uint64)
		if ok {
			return &x86.Memory{Segment: segment, Displacement: int64(displ)}
		}
	}

	return nil
}

func (ctx *x86Context) matchMemoryOffset(arg ast.Expression, operand *x86.Operand) any {
	deref, ok := arg.(*ast.List)
	if !ok {
		return nil
	}

	// The size of memory being copied can be
	// specified in an annotation.
	if ctx.rejectedBySizeHint(deref, operand.Bits) {
		return nil
	}

	elements := deref.Elements
	switch len(elements) {
	case 2:
		segment := ctx.matchReg(elements[0], x86.Registers16bitSegment...)
		offset, ok2 := ctx.matchUint(elements[1], int(ctx.Mode.Int)).(uint64)
		if ctx.Mode.Int < 64 && segment != nil && ok2 {
			return &x86.Memory{Segment: segment, Displacement: int64(offset)}
		}
	case 1:
		offset, ok := ctx.matchUint(elements[0], int(ctx.Mode.Int)).(uint64)
		if ok {
			return &x86.Memory{Displacement: int64(offset)}
		}
	}

	return nil
}

func (ctx *x86Context) matchStringDst(arg ast.Expression, operand *x86.Operand) any {
	deref, ok := arg.(*ast.List)
	if !ok {
		return nil
	}

	// The size of memory being copied can be
	// specified in an annotation.
	if ctx.rejectedBySizeHint(deref, operand.Bits) {
		return nil
	}

	// We return the base register, in
	// case an address size override is
	// needed.
	elements := deref.Elements
	switch len(elements) {
	case 2:
		seg := ctx.matchReg(elements[0], x86.ES)
		reg := ctx.matchReg(elements[1], x86.DI, x86.EDI)
		if seg != nil && reg != nil {
			return reg
		}
	case 1:
		reg := ctx.matchReg(elements[0], x86.DI, x86.EDI, x86.RDI)
		if reg != nil {
			return reg
		}
	}

	return nil
}

func (ctx *x86Context) matchStringSrc(arg ast.Expression, operand *x86.Operand) any {
	deref, ok := arg.(*ast.List)
	if !ok {
		return nil
	}

	// The size of memory being copied can be
	// specified in an annotation.
	if ctx.rejectedBySizeHint(deref, operand.Bits) {
		return nil
	}

	// We return the base register, in
	// case an address size override is
	// needed.
	elements := deref.Elements
	switch len(elements) {
	case 2:
		seg := ctx.matchReg(elements[0], x86.DS)
		reg := ctx.matchReg(elements[1], x86.SI, x86.ESI)
		if seg != nil && reg != nil {
			return reg
		}
	case 1:
		reg := ctx.matchReg(elements[0], x86.SI, x86.ESI, x86.RSI)
		if reg != nil {
			return reg
		}
	}

	return nil
}
