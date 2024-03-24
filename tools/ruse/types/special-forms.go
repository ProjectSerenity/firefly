// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"fmt"
	"strings"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/constant"
	"firefly-os.dev/tools/ruse/token"
)

// SpecialForm represents a special form in
// the Ruse language, which has no intrinsic
// type. However, at each call site where a
// special form is used, a function type is
// instantiated and recorded.
type SpecialForm struct {
	object
	id SpecialFormID
}

var _ Object = (*SpecialForm)(nil)

func (f *SpecialForm) String() string {
	return "special form " + f.id.String()
}

func (f *SpecialForm) ID() SpecialFormID { return f.id }

type SpecialFormID int

const (
	// Syntactic forms.
	SpecialFormAsmFunc SpecialFormID = iota
	SpecialFormAt
	SpecialFormABI
	SpecialFormFunc
	SpecialFormLen
	SpecialFormLet
	SpecialFormSection
	SpecialFormSizeOf

	// Arithmetic forms.
	SpecialFormAdd
	SpecialFormSubtract
	SpecialFormMultiply
	SpecialFormDivide
)

func (id SpecialFormID) String() string {
	switch id {
	case SpecialFormAsmFunc:
		return "asm-func"
	case SpecialFormAt:
		return "at"
	case SpecialFormABI:
		return "abi"
	case SpecialFormFunc:
		return "func"
	case SpecialFormLen:
		return "len"
	case SpecialFormLet:
		return "let"
	case SpecialFormSection:
		return "section"
	case SpecialFormSizeOf:
		return "size-of"
	case SpecialFormAdd:
		return "+"
	case SpecialFormSubtract:
		return "-"
	case SpecialFormMultiply:
		return "*"
	case SpecialFormDivide:
		return "/"
	}

	return fmt.Sprintf("specialFormId(%d)", id)
}

var specialForms = [...]*SpecialForm{
	SpecialFormAsmFunc: {},
	SpecialFormAt:      {},
	SpecialFormABI:     {},
	SpecialFormFunc:    {},
	SpecialFormLen:     {},
	SpecialFormLet:     {},
	SpecialFormSection: {},
	SpecialFormSizeOf:  {},

	// Arithmetic forms.
	SpecialFormAdd:      {},
	SpecialFormSubtract: {},
	SpecialFormMultiply: {},
	SpecialFormDivide:   {},
}

var specialFormTypes [len(specialForms)]func(c *checker, scope *Scope, fun *ast.List) (sig *Signature, typ Type, err error)

func defPredeclaredSpecialForms() {
	var numericTypes = []Type{
		Int,
		Int8,
		Int16,
		Int32,
		Int64,
		Uint,
		Uint8,
		Uint16,
		Uint32,
		Uint64,
		Uintptr,
	}

	// Syntactic forms.
	specialFormTypes[SpecialFormAsmFunc] = func(c *checker, scope *Scope, fun *ast.List) (sig *Signature, typ Type, err error) {
		// TODO: implement (asm-func)
		return nil, nil, fmt.Errorf("(asm-func) not supported")
	}

	specialFormTypes[SpecialFormAt] = func(c *checker, scope *Scope, fun *ast.List) (sig *Signature, typ Type, err error) {
		if len(fun.Elements[1:]) < 2 {
			return nil, nil, c.errorf(fun.ParenOpen, "too few arguments in call to at: expected %d, found %d", 2, len(fun.Elements[1:]))
		} else if len(fun.Elements[1:]) > 2 {
			return nil, nil, c.errorf(fun.ParenOpen, "too many arguments in call to at: expected %d, found %d", 2, len(fun.Elements[1:]))
		}

		arg0 := fun.Elements[1]
		arg1 := fun.Elements[2]
		obj, arrayType, err := c.ResolveExpression(scope, arg0)
		if err != nil {
			return nil, nil, err
		}

		// TODO: Add support for more types to special form at.
		array, ok := arrayType.(*Array)
		if !ok {
			return nil, nil, c.errorf(arg0.Pos(), "invalid argument: %s (%s) for at: want array", arg0.Print(), arrayType)
		}

		_, indexType, err := c.ResolveExpression(scope, arg1)
		if err != nil {
			return nil, nil, err
		}

		index, ok := constant.Val(c.consts[arg1]).(int64)
		if !ok {
			return nil, nil, c.errorf(arg1.Pos(), "invalid argument: %s (%s) for at: want integer", arg1.Print(), indexType)
		}

		if index < 0 || uint(index) > array.length {
			return nil, nil, c.errorf(fun.ParenOpen, "invalid argument: index %s (%d) overflows array with length %d", arg1.Print(), index, array.length)
		}

		sig = &Signature{
			name: "at",
			params: []*Variable{
				NewParameter(nil, arg0.Pos(), arg0.End(), nil, "v", arrayType),
				NewParameter(nil, arg1.Pos(), arg1.End(), nil, "i", indexType),
			},
			result: array.Element(),
		}

		// Make the length of a constant string also
		// a constant.
		var value constant.Value
		if con, ok := obj.(*Constant); ok {
			value = constant.ArrayVal(con.value)[index]
		}

		c.record(fun, sig.result, value)
		c.record(fun.Elements[0], sig, nil)

		return sig, sig, nil
	}

	specialFormTypes[SpecialFormABI] = func(c *checker, scope *Scope, fun *ast.List) (sig *Signature, typ Type, err error) {
		// Build an ABI object and return
		// only the result. We don't include
		// a function signature, as ABIs are
		// resolved immediately.
		var invertedStack *ast.Identifier
		var params, result, scratch, unused []*ast.Identifier
		for _, elt := range fun.Elements[1:] {
			list, ok := elt.(*ast.List)
			if !ok {
				return nil, nil, c.errorf(elt.Pos(), "invalid abi field %s: got %s, want list", elt.Print(), elt)
			}

			kind, rest, err := c.interpretIdentifiersDefinition(list, "abi spec field")
			if err != nil {
				return nil, nil, c.error(err)
			}

			switch kind.Name {
			case "inverted-stack":
				if invertedStack != nil {
					return nil, nil, c.errorf(kind.NamePos, "duplicate abi field %s", kind.Name)
				}

				if len(rest) != 1 {
					return nil, nil, c.errorf(kind.NamePos, "invalid abi field %s: got %d values, want 1 bool", kind.Name, len(rest))
				}

				invertedStack = rest[0]
			case "params":
				if params != nil {
					return nil, nil, c.errorf(kind.NamePos, "duplicate abi field %s", kind.Name)
				}

				params = rest
			case "result":
				if result != nil {
					return nil, nil, c.errorf(kind.NamePos, "duplicate abi field %s", kind.Name)
				}

				result = rest
			case "scratch":
				if scratch != nil {
					return nil, nil, c.errorf(kind.NamePos, "duplicate abi field %s", kind.Name)
				}

				scratch = rest
			case "unused":
				if unused != nil {
					return nil, nil, c.errorf(kind.NamePos, "duplicate abi field %s", kind.Name)
				}

				unused = rest
			default:
				return nil, nil, c.errorf(kind.NamePos, "unrecognised abi field %s", kind.Name)
			}
		}

		abi, err := NewRawABI(c.arch, invertedStack, params, result, scratch, unused)
		if err != nil {
			return nil, nil, c.errorf(fun.ParenOpen, "%v", err)
		}

		return nil, abi, nil
	}

	specialFormTypes[SpecialFormFunc] = func(c *checker, scope *Scope, fun *ast.List) (sig *Signature, typ Type, err error) {
		// TODO: implement (func)
		return nil, nil, fmt.Errorf("(func) not supported")
	}

	specialFormTypes[SpecialFormLen] = func(c *checker, scope *Scope, fun *ast.List) (sig *Signature, typ Type, err error) {
		if len(fun.Elements[1:]) != 1 {
			return nil, nil, c.errorf(fun.ParenOpen, "too many arguments in call to len: expected %d, found %d", 1, len(fun.Elements[1:]))
		}

		arg := fun.Elements[1]
		obj, typ, err := c.ResolveExpression(scope, arg)
		if err != nil {
			return nil, nil, err
		}

		// TODO: Add support for more types to special form len.
		array, isArray := typ.(*Array)
		if !isArray && !AssignableTo(String, typ) {
			return nil, nil, c.errorf(arg.Pos(), "invalid argument: %s (%s) for len", arg.Print(), typ)
		}

		sig = &Signature{
			name: "len",
			params: []*Variable{
				NewParameter(nil, arg.Pos(), arg.End(), nil, "v", UntypedString),
			},
			result: Int,
		}

		// Make the length of a constant string also
		// a constant.
		var value constant.Value
		if con, ok := obj.(*Constant); ok {
			if isArray {
				value = constant.MakeInt64(int64(array.Length()))
			} else {
				val := constant.StringVal(con.value)
				value = constant.MakeInt64(int64(len(val)))
			}
		}

		c.record(fun, sig.result, value)
		c.record(fun.Elements[0], sig, nil)

		return sig, sig, nil
	}

	specialFormTypes[SpecialFormLet] = func(c *checker, scope *Scope, fun *ast.List) (sig *Signature, typ Type, err error) {
		typ, err = c.ResolveLet(scope, fun)
		if err != nil {
			return nil, nil, err
		}

		sig = &Signature{
			name: "let",
			params: []*Variable{
				NewParameter(nil, token.NoPos, token.NoPos, nil, "name", typ),
				NewParameter(nil, token.NoPos, token.NoPos, nil, "value", typ),
			},
			result: typ,
		}

		return sig, sig, nil
	}

	specialFormTypes[SpecialFormSection] = func(c *checker, scope *Scope, fun *ast.List) (sig *Signature, typ Type, err error) {
		// Build a section object and return
		// only the result. We don't include
		// a function signature, as sections
		// are resolved immediately.
		var name *ast.Literal
		var fixedAddr uintptr
		var permissions *ast.Identifier
		for _, elt := range fun.Elements[1:] {
			list, ok := elt.(*ast.List)
			if !ok {
				return nil, nil, c.errorf(elt.Pos(), "invalid section field %s: got %s, want list", elt.Print(), elt)
			}

			kind, rest, err := c.interpretDefinition(list, "section spec field")
			if err != nil {
				return nil, nil, c.error(err)
			}

			switch kind.Name {
			case "name":
				if name != nil {
					return nil, nil, c.errorf(kind.NamePos, "duplicate section field %s", kind.Name)
				}

				if len(rest) != 1 {
					return nil, nil, c.errorf(kind.NamePos, "invalid section field %s: got %d values, want 1 string", kind.Name, len(rest))
				}

				name, ok = rest[0].(*ast.Literal)
				if !ok {
					return nil, nil, c.errorf(kind.NamePos, "invalid section field %s: got %s, want string", kind.Name, rest[0])
				}
			case "fixed-address":
				if fixedAddr != 0 {
					return nil, nil, c.errorf(kind.NamePos, "duplicate section field %s", kind.Name)
				}

				if len(rest) != 1 {
					return nil, nil, c.errorf(kind.NamePos, "invalid section field %s: got %d values, want 1 integer", kind.Name, len(rest))
				}

				obj, typ, err := c.ResolveExpression(scope, rest[0])
				if err != nil {
					return nil, nil, c.errorf(rest[0].Pos(), "invalid section field %s: %v", kind.Name, err)
				}

				if !AssignableTo(Uintptr, typ) {
					return nil, nil, c.errorf(rest[0].Pos(), "invalid section field %s: invalid type %s", kind.Name, typ)
				}

				// Extract the value.
				con, ok := obj.(*Constant)
				if !ok {
					return nil, nil, c.errorf(rest[0].Pos(), "invalid section field %s: value must be a positive integer constant, got %s", kind.Name, obj)
				}

				value := constant.Val(con.Value())
				val, ok := value.(int64)
				if !ok {
					return nil, nil, c.errorf(rest[0].Pos(), "invalid section field %s: value must be a positive integer constant, got %s", kind.Name, obj)
				}

				if val <= 0 {
					return nil, nil, c.errorf(rest[0].Pos(), "invalid section field %s: value must be a positive integer constant, got %s", kind.Name, val)
				}

				fixedAddr = uintptr(val)
			case "permissions":
				if permissions != nil {
					return nil, nil, c.errorf(kind.NamePos, "duplicate section field %s", kind.Name)
				}

				if len(rest) != 1 {
					return nil, nil, c.errorf(kind.NamePos, "invalid section field %s: got %d values, want 1 identifier", kind.Name, len(rest))
				}

				permissions, ok = rest[0].(*ast.Identifier)
				if !ok {
					return nil, nil, c.errorf(kind.NamePos, "invalid section field %s: got %s, want identifier", kind.Name, rest[0])
				}
			default:
				return nil, nil, c.errorf(kind.NamePos, "unrecognised section field %s", kind.Name)
			}
		}

		section, err := NewRawSection(name, fixedAddr, permissions)
		if err != nil {
			return nil, nil, c.errorf(fun.ParenOpen, "%v", err)
		}

		return nil, section, nil
	}

	specialFormTypes[SpecialFormSizeOf] = func(c *checker, scope *Scope, fun *ast.List) (sig *Signature, typ Type, err error) {
		// Return the size of a specified type
		// as an unsigned integer.
		if len(fun.Elements[1:]) != 1 {
			return nil, nil, c.errorf(fun.ParenClose, "too many arguments in call to size-of: expected %d, found %d", 1, len(fun.Elements[1:]))
		}

		arg := fun.Elements[1]
		typeName, ok := arg.(*ast.Identifier)
		if !ok {
			return nil, nil, c.errorf(arg.Pos(), "invalid type: want an identifier, found %s", arg)
		}

		// Try array types first, as they're a little different.
		if strings.HasPrefix(typeName.Name, "array/") {
			_, typ, err = c.checkArrayType(scope, typeName, -1)
			if err != nil {
				return nil, nil, c.errorf(typeName.NamePos, "%v", err)
			}
		}

		if typ == nil {
			_, obj := scope.LookupParent(typeName.Name, token.NoPos)
			if obj == nil {
				return nil, nil, c.errorf(typeName.NamePos, "undefined type: %s", typeName.Name)
			}

			c.use(typeName, obj)
			typ = obj.Type()
		}

		if typ == nil {
			return nil, nil, c.errorf(typeName.NamePos, "undefined type: %s", typeName.Name)
		}

		c.record(typeName, typ, nil)

		sizes := SizesFor(c.arch)
		size := sizes.SizeOf(typ)

		sig = &Signature{
			name: "len",
			params: []*Variable{
				NewParameter(nil, arg.Pos(), arg.End(), nil, "t", typ),
			},
			result: UntypedInt,
		}

		value := constant.MakeInt64(int64(size))

		c.record(fun, sig.result, value)
		c.record(fun.Elements[0], sig, nil)

		return sig, sig, nil
	}

	// Arithmetic forms.
	specialFormTypes[SpecialFormAdd] = (&arithmeticOp{
		Name:       "+",
		UnaryTypes: numericTypes, // No unary positive op on strings.
		BinaryTypes: []Type{
			String,
			Int,
			Int8,
			Int16,
			Int32,
			Int64,
			Uint,
			Uint8,
			Uint16,
			Uint32,
			Uint64,
			Uintptr,
		},
		Op: constant.OpAdd,
	}).signature

	specialFormTypes[SpecialFormSubtract] = (&arithmeticOp{
		Name: "-",
		UnaryTypes: []Type{
			Int,
			Int8,
			Int16,
			Int32,
			Int64,
		},
		BinaryTypes: numericTypes,
		Op:          constant.OpSubtract,
	}).signature

	specialFormTypes[SpecialFormMultiply] = (&arithmeticOp{
		Name:        "*",
		BinaryTypes: numericTypes,
		Op:          constant.OpMultiply,
	}).signature

	specialFormTypes[SpecialFormDivide] = (&arithmeticOp{
		Name:        "/",
		BinaryTypes: numericTypes,
		Op:          constant.OpDivide,
	}).signature

	for id, form := range specialForms {
		form.id = SpecialFormID(id)
		form.object = object{name: SpecialFormID(id).String()}
		def(form)
	}
}

type arithmeticOp struct {
	Name        string
	UnaryTypes  []Type
	BinaryTypes []Type
	MaxOperands int
	Op          constant.Op
}

func (op *arithmeticOp) signature(c *checker, scope *Scope, fun *ast.List) (sig *Signature, typ Type, err error) {
	numOperands := len(fun.Elements[1:])
	minOperands := 2
	if op.UnaryTypes != nil {
		minOperands = 1
	}

	if minOperands > numOperands {
		return nil, nil, c.errorf(fun.ParenClose, "expected at least %d parameters, found %d", minOperands, numOperands)
	}

	if op.MaxOperands != 0 && numOperands > op.MaxOperands {
		return nil, nil, c.errorf(fun.ParenClose, "%s supports no more than %d operands, found %d", op.Name, op.MaxOperands, numOperands)
	}

	// Start by resolving the types of the arguments,
	// which can all be evaluated.
	allConst := true
	argTypes := make([]Type, numOperands)
	constants := make([]constant.Value, numOperands)
	for i, expr := range fun.Elements[1:] {
		var obj Object
		obj, argTypes[i], err = c.ResolveExpression(scope, expr)
		if err != nil {
			return nil, nil, err
		}

		value := c.consts[expr]
		if value != nil {
			constants[i] = value
		} else if con, ok := obj.(*Constant); ok {
			constants[i] = con.value
		} else {
			allConst = false
		}
	}

	sig = &Signature{
		name:   op.Name,
		params: make([]*Variable, len(argTypes)),
	}

	for i, arg := range argTypes {
		// TODO: work out how to handle the case where
		// the first argument is an untyped constant.
		if i == 0 {
			ok := false
			for _, allow := range op.BinaryTypes {
				if AssignableTo(allow, arg) {
					ok = true
					break
				}
			}

			if !ok {
				return nil, nil, c.errorf(fun.Elements[i+1].Pos(), "invalid operation: %s not defined for %s", op.Name, arg)
			}

			sig.result = arg
			sig.params[i] = NewParameter(nil, token.NoPos, token.NoPos, nil, fmt.Sprintf("arg%d", i), arg)
			continue
		}

		if !AssignableTo(sig.result, arg) {
			return nil, nil, c.errorf(fun.Elements[i+1].Pos(), "expected %s parameter, found %s", sig.result, arg)
		}
	}

	// If all values are constant, we
	// may get a constant result.
	var value constant.Value
	if allConst {
		value = constant.Operation(op.Op, constants...)
	}

	c.record(fun, sig.result, value)
	c.record(fun.Elements[0], sig, nil)
	for i, arg := range fun.Elements[1:] {
		c.record(arg, sig.result, constants[i])
	}

	return sig, sig, nil
}
