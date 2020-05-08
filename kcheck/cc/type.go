// Copyright 2013 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cc

import (
	"fmt"
	"strings"
)

var printf = fmt.Printf

type Type struct {
	SyntaxInfo
	Kind     TypeKind
	Qual     TypeQual
	Base     *Type
	Tag      string
	Decls    []*Decl
	Width    *Expr
	Name     string
	TypeDecl *Decl
}

type TypeKind int

const (
	_ TypeKind = iota
	Void
	Int8
	Uint8
	Int16
	Uint16
	Int
	Uint
	Int32
	Uint32
	Int64
	Uint64
	Uintptr
	Float32
	Float64
	Enum
	Ptr
	Struct
	Union
	Array
	Func
	TypedefType
)

var typeKindString = []string{
	Void:        "void",
	Int8:        "int8",
	Uint8:       "uint8",
	Int16:       "int16",
	Uint16:      "uint16",
	Int:         "int",
	Uint:        "uint",
	Int32:       "int32",
	Uint32:      "uint32",
	Int64:       "int64",
	Uint64:      "uint64",
	Uintptr:     "uintptr",
	Float32:     "float32",
	Float64:     "float64",
	Ptr:         "pointer",
	Struct:      "struct",
	Union:       "union",
	Enum:        "enum",
	Array:       "array",
	Func:        "func",
	TypedefType: "<typedef>",
}

func (k TypeKind) String() string {
	if 0 <= int(k) && int(k) <= len(typeKindString) && typeKindString[k] != "" {
		return typeKindString[k]
	}
	return fmt.Sprintf("TypeKind(%d)", k)
}

func (k TypeKind) GoString() string {
	return k.String()
}

type TypeQual int

const (
	Const TypeQual = 1 << iota
	Volatile
)

func (q TypeQual) String() string {
	s := ""
	if q&Const != 0 {
		s += "const "
	}
	if q&Volatile != 0 {
		s += "volatile "
	}
	if s == "" {
		return ""
	}
	return s[:len(s)-1]
}

type Storage int

const (
	Auto Storage = 1 << iota
	Static
	Extern
	Typedef
	Register
	Inline
)

func (c Storage) String() string {
	s := ""
	if c&Auto != 0 {
		s += "auto "
	}
	if c&Static != 0 {
		s += "static "
	}
	if c&Extern != 0 {
		s += "extern "
	}
	if c&Typedef != 0 {
		s += "typedef "
	}
	if c&Register != 0 {
		s += "register "
	}
	if c&Inline != 0 {
		s += "inline "
	}
	if s == "" {
		return ""
	}
	return s[:len(s)-1]
}

var (
	CharType      = newType(Int8)
	UcharType     = newType(Uint8)
	ShortType     = newType(Int16)
	UshortType    = newType(Uint16)
	IntType       = newType(Int32)
	UintType      = newType(Uint32)
	LongType      = newType(Int64)
	UlongType     = newType(Uint64)
	UlonglongType = newType(Uintptr)
	FloatType     = newType(Float32)
	DoubleType    = newType(Float64)
	VoidType      = newType(Void)
	BoolType      = &Type{Kind: TypedefType, Name: "bool", Base: IntType}
)

type typeOp int

const (
	tChar typeOp = 1 << iota
	tShort
	tInt
	tLong
	tSigned
	tUnsigned
	tFloat
	tDouble
	tVoid
	tLonglong
)

var builtinTypes = map[typeOp]*Type{
	tChar:                        CharType,
	tChar | tSigned:              CharType,
	tChar | tUnsigned:            UcharType,
	tShort:                       ShortType,
	tShort | tSigned:             ShortType,
	tShort | tUnsigned:           UshortType,
	tShort | tInt:                ShortType,
	tShort | tSigned | tInt:      ShortType,
	tShort | tUnsigned | tInt:    UshortType,
	tInt:                         IntType,
	tInt | tSigned:               IntType,
	tInt | tUnsigned:             UintType,
	tLong:                        LongType,
	tLong | tSigned:              LongType,
	tLong | tUnsigned:            UlongType,
	tLong | tInt:                 LongType,
	tLong | tSigned | tInt:       LongType,
	tLong | tUnsigned | tInt:     UlongType,
	tLonglong | tUnsigned:        UlonglongType,
	tLonglong | tUnsigned | tInt: UlonglongType,
	tFloat:                       FloatType,
	tDouble:                      DoubleType,
	tVoid:                        VoidType,
}

func splitTypeWords(ws []string) (c Storage, q TypeQual, ty *Type) {
	// Could check for doubled words in general,
	// like const const, but no one cares.
	var t typeOp
	var ts []string
	for _, w := range ws {
		switch w {
		case "const":
			q |= Const
		case "volatile":
			q |= Volatile
		case "auto":
			c |= Auto
		case "static":
			c |= Static
		case "extern":
			c |= Extern
		case "typedef":
			c |= Typedef
		case "register":
			c |= Register
		case "inline":
			c |= Inline
		case "char":
			t |= tChar
			ts = append(ts, w)
		case "short":
			t |= tShort
			ts = append(ts, w)
		case "int":
			t |= tInt
			ts = append(ts, w)
		case "long":
			if t&tLong != 0 {
				t ^= tLonglong | tLong
			} else {
				t |= tLong
			}
			ts = append(ts, w)
		case "signed":
			t |= tSigned
			ts = append(ts, w)
		case "unsigned":
			t |= tUnsigned
			ts = append(ts, w)
		case "float":
			t |= tFloat
			ts = append(ts, w)
		case "double":
			t |= tDouble
			ts = append(ts, w)
		case "void":
			t |= tVoid
			ts = append(ts, w)
		}
	}

	if t == 0 {
		t |= tInt
	}

	ty = builtinTypes[t]
	if ty == nil {
		fmt.Printf("unsupported type %q\n", strings.Join(ts, " "))
	}

	return c, q, builtinTypes[t]
}

func newType(k TypeKind) *Type {
	return &Type{Kind: k}
}

func (t *Type) String() string {
	if t == nil {
		return "<nil>"
	}
	switch t.Kind {
	default:
		return t.Kind.String()
	case TypedefType:
		if t.Name == "" {
			return "missing_typedef_name"
		}
		return t.Name
	case Ptr:
		return t.Base.String() + "*"
	case Struct, Union, Enum:
		if t.Tag == "" {
			return t.Kind.String()
		}
		return t.Kind.String() + " " + t.Tag
	case Array:
		return t.Base.String() + "[]"
	case Func:
		s := "func("
		for i, d := range t.Decls {
			if i > 0 {
				s += ", "
			}
			s += d.Name + " " + d.Type.String()
		}
		if t.Base == t {
			s += ") SELF"
		} else {
			s += ") " + t.Base.String()
		}
		return s
	}
}

type Decl struct {
	SyntaxInfo
	Name    string
	Type    *Type
	Storage Storage
	Init    *Init
	Body    *Stmt

	XOuter    *Decl
	CurFn     *Decl
	OuterType *Type
	GoPackage string
}

func (d *Decl) String() string {
	if d == nil {
		return "nil Decl"
	}
	return fmt.Sprintf("Decl{%s, %s}", d.Name, d.Type)
}
