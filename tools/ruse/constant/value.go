// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package constant provides functionality to store
// and perform some operations on Rust constant
// values.
package constant

import (
	"fmt"
	"go/constant"
	gotoken "go/token"
	"strings"
	"unicode/utf8"

	"firefly-os.dev/tools/ruse/token"
)

// This package evolved from Go's go/constant, which
// was originally used in the Ruse type system. However,
// Ruse does not support Go's floating or complex number
// types and Go does not support Ruse's array constants.
//
// As a result, we defer to go/constant where possible
// and extend it where necessary.

// Kind indicates the constant type.
type Kind uint8

const (
	// An unknown constant type.
	Unknown Kind = iota

	Bool
	Integer
	String
	Array
)

func (k Kind) String() string {
	switch k {
	case Unknown:
		return "Unknown"
	case Bool:
		return "Bool"
	case Integer:
		return "Integer"
	case String:
		return "String"
	case Array:
		return "Array"
	default:
		return fmt.Sprintf("Kind(%d)", k)
	}
}

func fromGoKind(k constant.Kind) Kind {
	switch k {
	case constant.Bool:
		return Bool
	case constant.Int:
		return Integer
	case constant.String:
		return String
	default:
		return Unknown
	}
}

// Value contains a constant.
type Value interface {
	// Kind returns the constant's type.
	Kind() Kind

	// String returns a human-friendly string,
	// which may be truncated.
	String() string

	// ExactString returns the complete string
	// representation for the constant.
	ExactString() string

	// Prevent external implementations.
	isValue()
}

// Value implementations.

type (
	goVal    struct{ v constant.Value }
	arrayVal struct {
		typ string
		v   []Value
	}
)

var (
	_ Value = goVal{}
	_ Value = arrayVal{}
)

func (v goVal) Kind() Kind    { return fromGoKind(v.v.Kind()) }
func (v arrayVal) Kind() Kind { return Array }

func (v goVal) String() string { return v.v.String() }
func (v arrayVal) String() string {
	s := v.string()
	const maxLen = 72
	if utf8.RuneCountInString(s) > maxLen {
		// Drop the last 3 runes (not including
		// the closing parenthesis).
		i := 0
		for n := 0; n < maxLen-4; n++ {
			_, size := utf8.DecodeRuneInString(s[i:])
			i += size
		}

		last, _ := utf8.DecodeLastRuneInString(s)
		s = s[:i] + "..." + string(last)
	}

	return s
}

func (v goVal) ExactString() string    { return v.v.ExactString() }
func (v arrayVal) ExactString() string { return v.string() }

func (v goVal) isValue()    {}
func (v arrayVal) isValue() {}

func (v arrayVal) string() string {
	var b strings.Builder
	b.WriteByte('(')
	b.WriteString(v.typ)

	for _, v := range v.v {
		b.WriteByte(' ')
		b.WriteString(v.String())
	}

	b.WriteByte(')')

	return b.String()
}

// Makers.

// Make returns the value for x.
//
//	type of x       result Value
//	----------------------------
//	bool            Bool
//	int64           Integer
//	*big.Int        Integer
//	string          String
func Make(x any) Value { return goVal{v: constant.Make(x)} }

// MakeBool returns a Value representing
// the bool b.
func MakeBool(b bool) Value { return goVal{v: constant.MakeBool(b)} }

// MakeInt64 returns a Value representing
// the int64 x.
func MakeInt64(x int64) Value { return goVal{v: constant.MakeInt64(x)} }

// MakeUint64 returns a Value representing
// the uint64 x.
func MakeUint64(x uint64) Value { return goVal{v: constant.MakeUint64(x)} }

// MakeString returns a Value representing
// the string s.
func MakeString(s string) Value { return goVal{v: constant.MakeString(s)} }

// MakeArray returns a Value representing
// the array containing the given values.
func MakeArray(typeName string, v []Value) Value { return arrayVal{typ: typeName, v: v} }

// MakeFromLiteral returns a Value representing
// the given literal.
func MakeFromLiteral(lit string, tok token.Token) Value {
	var gotok gotoken.Token
	switch tok {
	case token.Integer:
		gotok = gotoken.INT
	case token.String:
		gotok = gotoken.STRING
	default:
		return goVal{v: constant.MakeUnknown()}
	}

	return goVal{v: constant.MakeFromLiteral(lit, gotok, 0)}
}

// Extractors.

// BoolVal returns the boolean value of v if v is a Bool,
// or a panic otherwise.
func BoolVal(v Value) bool {
	g, ok := v.(goVal)
	if !ok {
		panic(fmt.Sprintf("%v is not a Bool", v))
	}

	if g.v.Kind() != constant.Bool {
		panic(fmt.Sprintf("%v is not a Bool", v))
	}

	return constant.BoolVal(g.v)
}

// Int64Val returns the integer value of v if v is an Integer,
// or a panic otherwise.
func Int64Val(v Value) (i int64, exact bool) {
	g, ok := v.(goVal)
	if !ok {
		panic(fmt.Sprintf("%v is not an Integer", v))
	}

	if g.v.Kind() != constant.Int {
		panic(fmt.Sprintf("%v is not an Integer", v))
	}

	return constant.Int64Val(g.v)
}

// Uint64Val returns the integer value of v if v is an Integer,
// or a panic otherwise.
func Uint64Val(v Value) (i uint64, exact bool) {
	g, ok := v.(goVal)
	if !ok {
		panic(fmt.Sprintf("%v is not an Integer", v))
	}

	if g.v.Kind() != constant.Int {
		panic(fmt.Sprintf("%v is not an Integer", v))
	}

	return constant.Uint64Val(g.v)
}

// StringVal returns the string value of v if v is a String,
// or a panic otherwise.
func StringVal(v Value) string {
	g, ok := v.(goVal)
	if !ok {
		panic(fmt.Sprintf("%v is not a String", v))
	}

	if g.v.Kind() != constant.String {
		panic(fmt.Sprintf("%v is not a String", v))
	}

	return constant.StringVal(g.v)
}

// ArrayVal returns the values of v if v is an Array,
// or a panic otherwise.
func ArrayVal(v Value) []Value {
	a, ok := v.(arrayVal)
	if !ok {
		panic(fmt.Sprintf("%v is not an Array", v))
	}

	return a.v
}

// Val returns the underlying value of constant v.
//
//	v Kind        type of result
//	----------------------------
//	Bool          bool
//	Int           int64 or *big.Int
//	String        string
//	Array         []Value
//	other         nil
func Val(v Value) any {
	switch v := v.(type) {
	case goVal:
		return constant.Val(v.v)
	case arrayVal:
		return v.v
	default:
		return nil
	}
}

// Operations.

// Op represents an arithemetic operation.
type Op uint8

const (
	OpUnknown Op = iota
	OpAdd
	OpSubtract
	OpMultiply
	OpDivide
)

// Operation performs the given operation on at least
// one parameter.
func Operation(op Op, v ...Value) Value {
	var tok gotoken.Token
	switch op {
	case OpAdd:
		tok = gotoken.ADD
	case OpSubtract:
		tok = gotoken.SUB
	case OpMultiply:
		tok = gotoken.MUL
	case OpDivide:
		tok = gotoken.QUO_ASSIGN // Force integer division.
	default:
		panic(fmt.Sprintf("unrecognised operation Op(%d)", op))
	}

	var value constant.Value
	if len(v) == 1 {
		gv, ok := v[0].(goVal)
		if !ok {
			return goVal{v: constant.MakeUnknown()}
		}

		value = constant.UnaryOp(tok, gv.v, 0)
	} else {
		gv0, ok := v[0].(goVal)
		if !ok {
			return goVal{v: constant.MakeUnknown()}
		}

		gv1, ok := v[1].(goVal)
		if !ok {
			return goVal{v: constant.MakeUnknown()}
		}

		value = constant.BinaryOp(gv0.v, tok, gv1.v)
		for _, v := range v[2:] {
			gvn, ok := v.(goVal)
			if !ok {
				return goVal{v: constant.MakeUnknown()}
			}

			value = constant.BinaryOp(value, tok, gvn.v)
		}
	}

	return goVal{v: value}
}
