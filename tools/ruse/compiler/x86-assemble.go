// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// The bulk of the x86 assembler.

package compiler

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/internal/x86"
	"firefly-os.dev/tools/ruse/ssafir"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

// x86Context is used to propagate the state
// while assembling a single x86 assembly
// function.
type x86Context struct {
	Mode x86.Mode // CPU mode.
	FSet *token.FileSet
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
	Op   ssafir.Op
	Inst *x86.Instruction
	Args [4]any // Unused args are untyped nil.

	Length    uint8          // Number of bytes of machine code (max 15).
	REX_W     bool           // Whether to force a REX prefix, with REX.W set.
	Mask      uint8          // Any EVEX mask register.
	Zero      bool           // Any EVEX zeroing.
	Broadcast bool           // Any EVEX memory broadcast.
	Prefixes  [14]x86.Prefix // Any optional legacy prefixes specified.
	PrefixLen uint8
}

// x86InstructionCandidate includes the
// information specified for each instruction
// in the generated instruction set data.
type x86InstructionCandidate struct {
	Op   ssafir.Op
	Inst *x86.Instruction
}

// assembleX86 assembles a single Ruse assembly
// function for x86.
func assembleX86(fset *token.FileSet, pkg *types.Package, assembly *ast.List, info *types.Info, sizes types.Sizes) (*ssafir.Function, error) {
	// The asm-func keyword is the first identifier,
	// and the function name is the second. All the
	// subsequent expressions are assembly, either
	// in the form of a quoted identifier for a
	// label or a list containing one instruction.
	name := assembly.Elements[1].(*ast.Identifier)
	function := info.Definitions[name].(*types.Function)
	signature := function.Type().(*types.Signature)
	fun := &ssafir.Function{
		Name: name.Name,
		Type: signature,

		NamedValues: make(map[*types.Variable][]*ssafir.Value),
	}

	// Compile the body.
	c := &compiler{
		fset:  fset,
		pkg:   pkg,
		info:  info,
		fun:   fun,
		list:  assembly,
		sizes: sizes,

		vars: make(map[*types.Variable]*ssafir.Value),
	}

	ctx := &x86Context{
		Mode: x86.Mode64, // Default to 64-bit mode for x86-64.
		FSet: fset,
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

	fun.Extra = ctx.Mode

	var rexwOverride bool
	var prefixes []x86.Prefix
	c.AddFunctionPrelude()
	options := make([]*x86InstructionData, 0, 10)
	for _, expr := range assembly.Elements[2:] {
		// TODO: handle labels in the generated assembler.
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

		candidates, ok := x86MnemonicToCandidates[mnemonic.Name]
		if !ok {
			return nil, ctx.Errorf(mnemonic.NamePos, "invalid assembly directive: mnemonic %q not recognised", mnemonic.Name)
		}

		var code x86.Code
		params := elts[1:]
		options = options[:0]
		rightArity := false
		for _, candidate := range candidates {
			if candidate.Inst.Encoding.NoRepPrefixes && rep {
				return nil, ctx.Errorf(mnemonic.NamePos, "invalid assembly directive: mnemonic %q cannot be used with repeat prefixes", mnemonic.Name)
			}

			if matchUID != "" && matchUID != candidate.Inst.UID {
				continue
			}

			if len(params) != len(candidate.Inst.Parameters) {
				if matchUID != "" {
					return nil, ctx.Errorf(mnemonic.NamePos, "invalid assembly directive: %s does not match instruction %s: got %d parameters, want %d", list.Print(), matchUID, len(params), len(candidate.Inst.Parameters))
				}

				continue
			}

			rightArity = true
			data, err := ctx.Match(list, params, candidate)
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
			err = data.Encode(&code, ctx.Mode)
			if err != nil {
				return nil, err
			}

			data.Length = uint8(code.Len())
			options = append(options, data)
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
			for _, candidate := range candidates {
				arity := len(candidate.Inst.Parameters)
				if !seenArity[arity] {
					seenArity[arity] = true
					want = append(want, arity)
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
				if options[i].Length != options[j].Length {
					return options[i].Length < options[j].Length
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
		c.currentBlock.NewValueExtra(list.ParenOpen, list.ParenClose, option.Op, nil, option)
	}

	return fun, nil
}

// splitPrefixes takes x86 machine code in
// hexadecimal format and splits it into
// the set of legacy x86 prefixes and the
// remaining machine code.
//
// If the input is not valid hexadecimal,
// splitPrefixes will panic.
func splitPrefixes(s string) (prefixOpcodes, prefixes []byte, rest string) {
	code, err := hex.DecodeString(s)
	if err != nil {
		panic("invalid hex '" + s + "' passed to SplitPrefixes: " + err.Error())
	}

	for i, b := range code {
		switch b {
		case 0x9b:
			prefixOpcodes = append(prefixOpcodes, b)
		case 0xf0, 0xf2, 0xf3, // Group 1.
			0x2e, 0x36, 0x3e, 0x26, 0x64, 0x65, // Group 2.
			0x66, // Group 3.
			0x67: // Group 4.
			prefixes = append(prefixes, b)
		default:
			// Machine code.
			rest = s[i*2:]
			return prefixOpcodes, prefixes, rest
		}
	}

	return prefixOpcodes, prefixes, rest
}

// sortPrefixes takes x86 machine code in
// hexadecimal format and returns it with
// the x86 prefixes sorted into numerical
// order.
//
// If the input is not valid hexadecimal,
// sortPrefixes will panic.
func sortPrefixes(s string) string {
	prefixOpcodes, prefixes, rest := splitPrefixes(s)
	if len(prefixes) == 0 && len(prefixOpcodes) == 0 {
		return rest
	}

	if len(prefixes) == 0 {
		return hex.EncodeToString(prefixOpcodes) + rest
	}

	sort.Slice(prefixes, func(i, j int) bool { return prefixes[i] < prefixes[j] })

	return hex.EncodeToString(prefixOpcodes) + hex.EncodeToString(prefixes) + rest
}

// handleInstructionAnnotations processes
// the annotations for a single instruction,
// updating `data` if necessary.
func (ctx *x86Context) handleInstructionAnnotations(data *x86InstructionData, list *ast.List, inst x86InstructionCandidate) (ok bool, err error) {
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
			case "true", "false":
				data.Broadcast = k.Name == "true"
			default:
				return false, ctx.Errorf(k.NamePos, "invalid EVEX annotation: invalid broadcast mode: %s %q", k, k.Print())
			}

			// We now know the mode is valid,
			// but we can only use them with
			// EVEX instructions.
			if !inst.Inst.Encoding.EVEX {
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

			// We accept either a mask register
			// by name (kX) or by number (X),
			// in the range 1-7. Note that k0
			// is not valid.
			switch k := anno.X.Elements[1].(type) {
			case *ast.Identifier:
				switch k.Name {
				case "k1", "k2", "k3", "k4", "k5", "k6", "k7":
					data.Mask = k.Name[1] - '0'
				default:
					return false, ctx.Errorf(k.NamePos, "invalid EVEX annotation: invalid mask register: %s %q", k, k.Print())
				}
			case *ast.Literal:
				if k.Kind != token.Integer {
					return false, ctx.Errorf(k.ValuePos, "invalid EVEX annotation: invalid mask register: %s %q", k, k.Print())
				}

				switch k.Value {
				case "1", "2", "3", "4", "5", "6", "7":
					data.Mask = k.Value[0] - '0'
				default:
					return false, ctx.Errorf(k.ValuePos, "invalid EVEX annotation: invalid mask register: %s %q", k, k.Print())
				}
			}

			// We now know the mask is valid,
			// but we can only use them with
			// EVEX instructions.
			if !inst.Inst.Encoding.EVEX {
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
			if !inst.Inst.Encoding.EVEX {
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
func (ctx *x86Context) Match(list *ast.List, args []ast.Expression, inst x86InstructionCandidate) (data *x86InstructionData, err error) {
	if len(args) != len(inst.Inst.Parameters) ||
		ctx.Mode.Int == 16 && !inst.Inst.Mode16 ||
		ctx.Mode.Int == 32 && !inst.Inst.Mode32 ||
		ctx.Mode.Int == 64 && !inst.Inst.Mode64 {
		return nil, nil
	}

	data = &x86InstructionData{
		Op:   inst.Op,
		Inst: inst.Inst,
	}

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

	for i, param := range inst.Inst.Parameters {
		var arg any
		switch param.Type {
		case x86.TypeSignedImmediate:
			arg = ctx.matchSignedImmediate(args[i], param)
		case x86.TypeUnsignedImmediate:
			arg = ctx.matchUnsignedImmediate(args[i], param)
		case x86.TypeRegister:
			arg = ctx.matchRegister(args[i], param)
		case x86.TypeStackIndex:
			arg = ctx.matchStackIndex(args[i], param)
		case x86.TypeRelativeAddress:
			arg = ctx.matchRelativeAddress(args[i], param)
		case x86.TypeFarPointer:
			arg = ctx.matchFarPointer(args[i], param)
		case x86.TypeMemory:
			arg = ctx.matchMemory(args[i], param)
		case x86.TypeMemoryOffset:
			arg = ctx.matchMemoryOffset(args[i], param)
		case x86.TypeStringDst:
			arg = ctx.matchStringDst(args[i], param)
		case x86.TypeStringSrc:
			arg = ctx.matchStringSrc(args[i], param)
		default:
			panic("unexpected parameter type: " + param.Type.String())
		}

		if arg == nil {
			return nil, nil
		}

		data.Args[i] = arg
		reg, ok := arg.(*x86.Register)
		if ok && reg.EVEX && !inst.Inst.Encoding.EVEX {
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

	var wantSize string
	switch bits {
	case 8:
		wantSize = "*byte"
	case 16:
		wantSize = "*word"
	case 32:
		wantSize = "*dword"
	case 48:
		wantSize = "*tword"
	case 64:
		wantSize = "*qword"
	case 80:
		wantSize = "*tbyte"
	case 128:
		wantSize = "*xmmword"
	case 256:
		wantSize = "*ymmword"
	case 512:
		wantSize = "*zmmword"
	default:
		panic(fmt.Sprintf("unexpected memory transaction size: %d bits", bits))
	}

	foundMatch := false
	foundSizeHint := false
	for _, anno := range list.Annotations {
		if len(anno.X.Elements) != 1 {
			continue
		}

		got, ok := anno.X.Elements[0].(*ast.Identifier)
		if !ok || !strings.HasPrefix(got.Name, "*") {
			continue
		}

		foundSizeHint = true
		if got.Name == wantSize {
			foundMatch = true
			break
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
		return nil
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

func (ctx *x86Context) matchSignedImmediate(arg ast.Expression, param *x86.Parameter) any {
	if param.Encoding == x86.EncodingNone {
		lit, ok := arg.(*ast.Literal)
		if !ok || lit.Kind != token.Integer {
			return nil
		}

		if lit.Value != param.Syntax {
			return nil
		}

		// We store nothing.
		return struct{}{}
	}

	return ctx.matchSint(arg, param.Bits)
}

func (ctx *x86Context) matchUnsignedImmediate(arg ast.Expression, param *x86.Parameter) any {
	return ctx.matchUint(arg, param.Bits)
}

func (ctx *x86Context) matchRegister(arg ast.Expression, param *x86.Parameter) any {
	reg := ctx.matchReg(arg, param.Registers...)
	if reg == nil {
		return nil
	}

	return reg
}

func (ctx *x86Context) matchStackIndex(arg ast.Expression, param *x86.Parameter) any {
	ident, ok := arg.(*ast.Identifier)
	if !ok {
		return nil
	}

	// For ST, we store nothing.
	if param.Encoding == x86.EncodingNone {
		if ident.Name != "st" {
			return nil
		}

		return struct{}{}
	}

	for i, reg := range param.Registers {
		if reg.Name == ident.Name {
			return uint8(i)
		}

		for _, alias := range reg.Aliases {
			if alias == ident.Name {
				return uint8(i)
			}
		}
	}

	return nil
}

func (ctx *x86Context) matchRelativeAddress(arg ast.Expression, param *x86.Parameter) any {
	// Relative addresses can't be unsigned.
	return ctx.matchSint(arg, param.Bits)
}

func (ctx *x86Context) matchFarPointer(arg ast.Expression, param *x86.Parameter) any {
	pair, ok := arg.(*ast.List)
	if !ok || len(pair.Elements) != 2 {
		return nil
	}

	base := ctx.matchUint(pair.Elements[0], 16)
	index := ctx.matchUint(pair.Elements[1], param.Bits-16)
	if base == nil || index == nil {
		return nil
	}

	// We concatenate the two pointers into a single 64-bit
	// integer. This is fine, as the largest size is a 16-bit
	// base and a 32-bit index. We encode the base in the high
	// bits, as it is enocded after the index.

	return (base.(uint64) << (param.Bits - 16)) | index.(uint64)
}

func (ctx *x86Context) matchMemory(arg ast.Expression, param *x86.Parameter) any {
	list, ok := arg.(*ast.List)
	if !ok {
		return nil
	}

	// The size of memory being copied can be
	// specified in an annotation.
	if ctx.rejectedBySizeHint(list, param.Bits) {
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

func (ctx *x86Context) matchMemoryOffset(arg ast.Expression, param *x86.Parameter) any {
	deref, ok := arg.(*ast.List)
	if !ok {
		return nil
	}

	// The size of memory being copied can be
	// specified in an annotation.
	if ctx.rejectedBySizeHint(deref, param.Bits) {
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

func (ctx *x86Context) matchStringDst(arg ast.Expression, param *x86.Parameter) any {
	deref, ok := arg.(*ast.List)
	if !ok {
		return nil
	}

	// The size of memory being copied can be
	// specified in an annotation.
	if ctx.rejectedBySizeHint(deref, param.Bits) {
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

func (ctx *x86Context) matchStringSrc(arg ast.Expression, param *x86.Parameter) any {
	deref, ok := arg.(*ast.List)
	if !ok {
		return nil
	}

	// The size of memory being copied can be
	// specified in an annotation.
	if ctx.rejectedBySizeHint(deref, param.Bits) {
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
