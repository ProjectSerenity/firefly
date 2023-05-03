// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"fmt"
	"go/constant"

	"firefly-os.dev/tools/ruse/token"
)

// The Universe scope contains language primitives,
// such as builtin functions.
var Universe *Scope

var (
	Bool    Type
	Int     Type
	Int8    Type
	Int16   Type
	Int32   Type
	Int64   Type
	Uint    Type
	Uint8   Type
	Byte    Type // Alias to uint8 with name byte.
	Uint16  Type
	Uint32  Type
	Uint64  Type
	Uintptr Type
	String  Type

	UntypedBool   Type
	UntypedInt    Type
	UntypedString Type

	builtins      = make(map[string]*Function)
	CastFunctions = make(map[string]*Function)
)

// BasicTypes contains the predeclared *Basic types indexed by
// their corresponding BasicKind.
//
// The *Basic type for BasicTypes[Byte] will have the name "uint8".
// Use Universe.Lookup("byte").Type() to obtain the specific
// alias basic type named "byte".
var BasicTypes = []*Basic{
	KindInvalid: {KindInvalid, 0, "invalid type"},

	KindBool:    {KindBool, IsBoolean, "bool"},
	KindInt:     {KindInt, IsInteger, "int"},
	KindInt8:    {KindInt8, IsInteger, "int8"},
	KindInt16:   {KindInt16, IsInteger, "int16"},
	KindInt32:   {KindInt32, IsInteger, "int32"},
	KindInt64:   {KindInt64, IsInteger, "int64"},
	KindUint:    {KindUint, IsInteger | IsUnsigned, "uint"},
	KindUint8:   {KindUint8, IsInteger | IsUnsigned, "uint8"},
	KindUint16:  {KindUint16, IsInteger | IsUnsigned, "uint16"},
	KindUint32:  {KindUint32, IsInteger | IsUnsigned, "uint32"},
	KindUint64:  {KindUint64, IsInteger | IsUnsigned, "uint64"},
	KindUintptr: {KindUintptr, IsInteger | IsUnsigned, "uintptr"},
	KindString:  {KindString, IsString, "string"},

	KindUntypedBool:   {KindUntypedBool, IsBoolean | IsUntyped, "untyped bool"},
	KindUntypedInt:    {KindUntypedInt, IsInteger | IsUntyped, "untyped integer"},
	KindUntypedString: {KindUntypedString, IsString | IsUntyped, "untyped string"},
}

var aliases = [...]*Basic{
	{KindByte, IsInteger | IsUnsigned, "byte"},
}

// Objects are inserted in the universe scope.
func def(obj Object) {
	if Universe.Insert(obj) != nil {
		panic("double declaration of predeclared identifier")
	}
}

func init() {
	Universe = NewScope(nil, token.NoPos, token.NoPos, "universe")

	defPredeclaredTypes()
	defPredeclaredConsts()

	Bool = Universe.Lookup("bool").Type()
	Int = Universe.Lookup("int").Type()
	Int8 = Universe.Lookup("int8").Type()
	Int16 = Universe.Lookup("int16").Type()
	Int32 = Universe.Lookup("int32").Type()
	Int64 = Universe.Lookup("int64").Type()
	Uint = Universe.Lookup("uint").Type()
	Uint8 = Universe.Lookup("uint8").Type()
	Byte = Universe.Lookup("byte").Type()
	Uint16 = Universe.Lookup("uint16").Type()
	Uint32 = Universe.Lookup("uint32").Type()
	Uint64 = Universe.Lookup("uint64").Type()
	Uintptr = Universe.Lookup("uintptr").Type()
	String = Universe.Lookup("string").Type()
	UntypedBool = Universe.Lookup("untyped bool").Type()
	UntypedInt = Universe.Lookup("untyped integer").Type()
	UntypedString = Universe.Lookup("untyped string").Type()

	defPredeclaredFuncs()
	defPredeclaredSpecialForms()
}

var predeclaredConsts = [...]struct {
	name string
	kind BasicKind
	val  constant.Value
}{
	{"true", KindUntypedBool, constant.MakeBool(true)},
	{"false", KindUntypedBool, constant.MakeBool(false)},
}

func defPredeclaredConsts() {
	for _, c := range predeclaredConsts {
		def(NewConstant(Universe, token.NoPos, token.NoPos, nil, c.name, BasicTypes[c.kind], c.val))
	}
}

func defPredeclaredTypes() {
	for _, t := range BasicTypes {
		def(NewTypeName(Universe, token.NoPos, token.NoPos, nil, t.name, t))
	}

	for _, t := range aliases {
		def(NewTypeName(Universe, token.NoPos, token.NoPos, nil, t.name, t))
	}
}

func defPredeclaredFuncs() {
	numberTypes := []struct {
		Name string
		Type Type
	}{
		{"int", Int},
		{"int8", Int8},
		{"int16", Int16},
		{"int32", Int32},
		{"int64", Int64},
		{"uint", Uint},
		{"uint8", Uint8},
		{"uint16", Uint16},
		{"uint32", Uint32},
		{"uint64", Uint64},
		{"uintptr", Uintptr},
	}

	for _, from := range numberTypes {
		for _, to := range numberTypes {
			if from.Name == to.Name {
				continue
			}

			name := fmt.Sprintf("%s->%s", from.Name, to.Name)
			arg := NewParameter(nil, token.NoPos, token.NoPos, nil, "arg", from.Type)
			sig := NewSignature("("+name+")", []*Variable{arg}, to.Type)
			fun := NewFunction(Universe, token.NoPos, token.NoPos, nil, name, sig)
			def(fun)
			builtins[name] = fun
			CastFunctions[name] = fun
		}
	}
}
