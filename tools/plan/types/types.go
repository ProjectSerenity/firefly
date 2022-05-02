// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ProjectSerenity/firefly/tools/plan/ast"
	"github.com/ProjectSerenity/firefly/tools/plan/token"
)

// Name represents a name defined in a
// Plan source file.
//
type Name []string

// toTitle returns the word with the first rune
// in upper case and the remaining runes in lower
// case.
//
// This is similar to strings.Title, but we
// already know the string is alphanumeric only
// and contains no UTF-8 encoding errors.
//
func toTitle(s string) string {
	first, width := utf8.DecodeRuneInString(s)
	rest := s[width:]

	return string(unicode.ToUpper(first)) + strings.ToLower(rest)
}

// Spaced returns the name, separated by spaces.
//
func (n Name) Spaced() string {
	return strings.Join(n, " ")
}

// CamelCase returns the name in 'camel case',
// such as "camelCase".
//
func (n Name) CamelCase() string {
	if len(n) == 1 {
		return strings.ToLower(n[0])
	}

	title := make([]string, len(n)-1)
	for i, s := range n[1:] {
		title[i] = toTitle(s)
	}

	return strings.ToLower(n[0]) + strings.Join(title, "")
}

// PascalCase returns the name in 'Pascal case',
// such as "PascalCase".
//
func (n Name) PascalCase() string {
	if len(n) == 1 {
		return toTitle(n[0])
	}

	title := make([]string, len(n))
	for i, s := range n {
		title[i] = toTitle(s)
	}

	return strings.Join(title, "")
}

// SnakeCase returns the name in 'snake case',
// such as "snake_case".
//
func (n Name) SnakeCase() string {
	if len(n) == 1 {
		return strings.ToLower(n[0])
	}

	lower := make([]string, len(n))
	for i, s := range n {
		lower[i] = strings.ToLower(s)
	}

	return strings.Join(lower, "_")
}

// Docs represents a set of documentation
// for a type Plan a source file. The docs
// are split into lines.
//
type Docs []string

// NewDocs returns the given documentation,
// split into lines.
//
func NewDocs(text []string) Docs {
	joined := strings.Join(text, "")
	split := strings.Split(joined, "\n")

	return Docs(split)
}

// Arch represents an instruction set
// architecture, which is used to customise
// types for a particular architecture.
//
type Arch uint8

const (
	InvalidArch Arch = iota
	X86_64
)

func (a Arch) String() string {
	ss := map[Arch]string{
		X86_64: "x86-64",
	}

	s, ok := ss[a]
	if !ok {
		panic(fmt.Sprintf("unrecognised architecture %d", a))
	}

	return s
}

// Types that represent data types.

// Type represents any type that can be
// referenced in a Plan document, including
// complex structure types.
//
type Type interface {
	// Size returns the number of bytes that
	// a value of the type will consume in
	// memory.
	Size(Arch) int

	// String returns a brief textual
	// representation for the type.
	String() string
}

// Integer represents a primitive integer,
// type.
//
type Integer uint8

var _ Type = Integer(0)

const (
	InvalidInteger Integer = iota
	Byte
	Uint8
	Uint16
	Uint32
	Uint64
	Sint8
	Sint16
	Sint32
	Sint64
)

var integers = map[string]Integer{
	"byte":   Byte,
	"uint8":  Uint8,
	"uint16": Uint16,
	"uint32": Uint32,
	"uint64": Uint64,
	"int8":   Sint8,
	"int16":  Sint16,
	"int32":  Sint32,
	"int64":  Sint64,
}

func (b Integer) Size(a Arch) int {
	sizes := map[Integer]int{
		Byte:   1,
		Uint8:  1,
		Uint16: 2,
		Uint32: 4,
		Uint64: 8,
		Sint8:  1,
		Sint16: 2,
		Sint32: 4,
		Sint64: 8,
	}

	size, ok := sizes[b]
	if !ok {
		panic(fmt.Sprintf("unrecognised basic type %d", b))
	}

	return size
}

func (b Integer) String() string {
	ss := map[Integer]string{
		Byte:   "byte",
		Uint8:  "uint8",
		Uint16: "uint16",
		Uint32: "uint32",
		Uint64: "uint64",
		Sint8:  "sint8",
		Sint16: "sint16",
		Sint32: "sint32",
		Sint64: "sint64",
	}

	s, ok := ss[b]
	if !ok {
		panic(fmt.Sprintf("unrecognised basic type %d", b))
	}

	return s
}

// Pointer represents a pointer to
// another data type.
//
type Pointer struct {
	Mutable    bool
	Underlying Type
}

var _ Type = (*Pointer)(nil)

func (p *Pointer) Size(a Arch) int {
	sizes := [...]int{
		X86_64: 8,
	}

	if int(a) >= len(sizes) {
		panic(fmt.Sprintf("unrecognised architecture %d", a))
	}

	return sizes[a]
}

func (p *Pointer) String() string {
	if p.Mutable {
		return fmt.Sprintf("*mutable %s", p.Underlying.String())
	}

	return fmt.Sprintf("*constant %s", p.Underlying.String())
}

// Reference represents a name used to reference a
// type that has already been defined elsewhere.
//
type Reference struct {
	Name       Name
	Underlying Type
}

var _ Type = (*Reference)(nil)

func (r *Reference) Size(a Arch) int {
	return r.Underlying.Size(a)
}

func (r *Reference) String() string {
	return r.Name.Spaced()
}

// Padding represents unused space that is included
// after a field in a structure to ensure the fields
// and structure remain correctly aligned.
//
type Padding uint16

var _ Type = Padding(0)

func (p Padding) Size(a Arch) int {
	return int(p)
}

func (p Padding) String() string {
	return fmt.Sprintf("%d padding bytes", p)
}

// Field represents a single field in a structure
// type.
//
type Field struct {
	Name Name
	Node *ast.List
	Docs Docs
	Type Type
}

func (f *Field) Size(a Arch) int {
	return f.Type.Size(a)
}

func (f *Field) String() string {
	return fmt.Sprintf("field %s: %s", f.Name.Spaced(), f.Type.String())
}

// Value represents a single value in an
// enumeration type.
//
type Value struct {
	Name Name
	Node *ast.List
	Docs Docs
}

// Enumeration represents a numerical type
// with a constrained set of valid values
// in a syscalls plan.
//
type Enumeration struct {
	Name   Name
	Node   *ast.List
	Docs   Docs
	Type   Integer
	Values []*Value
}

var (
	_ Type     = (*Enumeration)(nil)
	_ ast.Node = (*Enumeration)(nil)
)

func (e *Enumeration) Pos() token.Position { return e.Node.Pos() }
func (e *Enumeration) End() token.Position { return e.Node.End() }

func (e *Enumeration) Size(a Arch) int {
	return e.Type.Size(a)
}

func (e *Enumeration) String() string {
	return fmt.Sprintf("enumeration %s (%s)", e.Name.Spaced(), e.Type.String())
}

func (e *Enumeration) GoString() string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "enumeration %s (%s) {\n", e.Name.Spaced(), e.Type.String())
	for _, value := range e.Values {
		buf.WriteByte('\t')
		buf.WriteString("value ")
		buf.WriteString(value.Name.Spaced())
		buf.WriteByte('\n')
	}

	buf.WriteByte('}')

	return buf.String()
}

// Structure represents a structure defined
// in a syscalls plan.
//
type Structure struct {
	Name   Name
	Node   *ast.List
	Docs   Docs
	Fields []*Field
}

var (
	_ Type     = (*Structure)(nil)
	_ ast.Node = (*Structure)(nil)
)

func (s *Structure) Pos() token.Position { return s.Node.Pos() }
func (s *Structure) End() token.Position { return s.Node.End() }

func (s *Structure) Size(a Arch) int {
	// We assume the structure is already
	// aligned.
	size := 0
	for _, field := range s.Fields {
		size += field.Size(a)
	}

	return size
}

func (s *Structure) String() string {
	return fmt.Sprintf("structure %s", s.Name.Spaced())
}

func (s *Structure) GoString() string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "structure %s {\n", s.Name.Spaced())
	for _, field := range s.Fields {
		buf.WriteByte('\t')
		buf.WriteString(field.String())
		buf.WriteByte('\n')
	}

	buf.WriteByte('}')

	return buf.String()
}

// Parameter represents a single argument
// or result in a function call.
//
type Parameter struct {
	Name Name
	Node *ast.List
	Docs Docs
	Type Type
}

func (p *Parameter) IsEnumeration() bool {
	paramType := p.Type
	if ref, ok := paramType.(*Reference); ok {
		paramType = ref.Underlying
	}

	_, ok := paramType.(*Enumeration)
	return ok
}

func (p *Parameter) Enumeration() *Enumeration {
	paramType := p.Type
	if ref, ok := paramType.(*Reference); ok {
		paramType = ref.Underlying
	}

	return paramType.(*Enumeration)
}

func (p *Parameter) Size(a Arch) int {
	return p.Type.Size(a)
}

func (p *Parameter) String() string {
	return fmt.Sprintf("parameter %s: %s", p.Name.Spaced(), p.Type.String())
}

// Parameters is an ordered set of function
// parameters, such as its arguments or
// results.
//
type Parameters []*Parameter

// Syscall describes a system call, including
// its parameters and results.
//
type Syscall struct {
	Name    Name
	Node    *ast.List
	Docs    Docs
	Args    Parameters
	Results Parameters
}

var _ ast.Node = (*Syscall)(nil)

func (s *Syscall) Pos() token.Position { return s.Node.Pos() }
func (s *Syscall) End() token.Position { return s.Node.End() }

func (s *Syscall) String() string {
	return fmt.Sprintf("syscall %s: (%d -> %d)", s.Name.Spaced(), len(s.Args), len(s.Results))
}

// File represents a parsed syscalls plan.
//
type File struct {
	// Data structures.
	Enumerations []*Enumeration
	Structures   []*Structure

	// System calls.
	Syscalls []*Syscall
}

// SyscallsEnumeration returns a synthetic
// enumeration (with no AST data) describing
// the set of syscalls. This can be used to
// iterate over the set of syscalls in a target
// language.
//
func (f *File) SyscallsEnumeration() *Enumeration {
	enum := &Enumeration{
		Name:   Name{"syscalls"},
		Docs:   Docs{"An enumeration describing the set of system calls."},
		Type:   Uint64,
		Values: make([]*Value, len(f.Syscalls)),
	}

	for i, syscall := range f.Syscalls {
		enum.Values[i] = &Value{
			Name: syscall.Name,
			Docs: syscall.Docs,
		}
	}

	return enum
}

// DropAST can be used to remove the AST nodes
// from a file, to make it easier to reproduce
// in tests.
//
func (f *File) DropAST() {
	for _, enumeration := range f.Enumerations {
		enumeration.Node = nil
		for _, value := range enumeration.Values {
			value.Node = nil
		}
	}
	for _, structure := range f.Structures {
		structure.Node = nil
		for _, field := range structure.Fields {
			field.Node = nil
		}
	}
	for _, syscall := range f.Syscalls {
		syscall.Node = nil
		for _, arg := range syscall.Args {
			arg.Node = nil
		}
		for _, result := range syscall.Results {
			result.Node = nil
		}
	}
}