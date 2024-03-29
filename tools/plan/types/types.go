// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Types that represent data types.

package types

import (
	"fmt"
	"math"

	"firefly-os.dev/tools/plan/ast"
	"firefly-os.dev/tools/plan/token"
)

// Type represents any type that can be
// referenced in a Plan document, including
// complex structure types.
type Type interface {
	// Alignment returns the memory alignment
	// of the type on the specified architecture.
	//
	// That is, the offset in bytes into a
	// structure at which a field of this type
	// appears must be an exact multiple of its
	// alignment. For example, if type `A` has
	// an alignment of 8, then any structure
	// fields of type `A` must have an offset
	// into the structure that is a multiple of
	// 8.
	//
	// Alignment values must be larger than
	// 0, and must be an exact power of 2.
	Alignment(Arch) int

	// Size returns the number of bytes that
	// a value of the type will consume in
	// memory.
	//
	// Size values must be an exact multiple
	// of the type's alignment, and must be
	// non-negative.
	Size(Arch) int

	// Parameter returns whether the type
	// can be passed in a syscall parameter.
	Parameter(Arch) bool

	// String returns a brief textual
	// representation for the type.
	String() string
}

// Underlying returns the base type, dereferencing
// any References if necessary.
func Underlying(typ Type) Type {
	for {
		ref, ok := typ.(*Reference)
		if !ok {
			return typ
		}

		typ = ref.Underlying
	}
}

// Array represents a fixed-size
// sequence of another data type.
type Array struct {
	Name   Name
	Node   *ast.List
	Docs   Docs
	Groups []Name
	Count  uint64
	Type   Type
}

var (
	_ Type     = (*Array)(nil)
	_ ast.Node = (*Array)(nil)
)

func (a *Array) Pos() token.Position { return a.Node.Pos() }
func (a *Array) End() token.Position { return a.Node.End() }

func (a *Array) Parameter(arch Arch) bool {
	// Even though an array may be small
	// enough to fit in a register, we
	// store it separately in case the
	// array's type/size changes later.
	return false
}

func (a *Array) Alignment(arch Arch) int {
	return a.Type.Alignment(arch)
}

func (a *Array) Size(arch Arch) int {
	return int(a.Count) * a.Type.Size(arch)
}

func (a *Array) String() string {
	return fmt.Sprintf("array(%d x %s)", a.Count, a.Type.String())
}

// Bitfield represents a numerical type
// with a constrained set of valid values
// in a syscalls plan. The values can be
// combined together, but each value uses
// one bit in the underlying integer,
// resulting in a smaller number of
// individual values than an enumeration
// of the same size.
type Bitfield struct {
	Name   Name
	Node   *ast.List
	Docs   Docs
	Groups []Name
	Type   Integer
	Values []*Value
}

var (
	_ Type     = (*Bitfield)(nil)
	_ ast.Node = (*Bitfield)(nil)
)

func (b *Bitfield) Pos() token.Position   { return b.Node.Pos() }
func (b *Bitfield) End() token.Position   { return b.Node.End() }
func (b *Bitfield) Parameter(a Arch) bool { return true }

func (e *Bitfield) Alignment(a Arch) int {
	return e.Type.Alignment(a)
}

func (e *Bitfield) Size(a Arch) int {
	return e.Type.Size(a)
}

func (b *Bitfield) String() string {
	return fmt.Sprintf("bitfield %s (%s)", b.Name.Spaced(), b.Type.String())
}

// Enumeration represents a numerical type
// with a constrained set of valid values
// in a syscalls plan.
type Enumeration struct {
	Name   Name
	Node   *ast.List
	Docs   Docs
	Groups []Name
	Type   Integer
	Embeds []*Enumeration
	Values []*Value
}

var (
	_ Type     = (*Enumeration)(nil)
	_ ast.Node = (*Enumeration)(nil)
)

func (e *Enumeration) Pos() token.Position   { return e.Node.Pos() }
func (e *Enumeration) End() token.Position   { return e.Node.End() }
func (e *Enumeration) Parameter(a Arch) bool { return true }

func (e *Enumeration) Alignment(a Arch) int {
	return e.Type.Alignment(a)
}

func (e *Enumeration) Size(a Arch) int {
	return e.Type.Size(a)
}

func (e *Enumeration) String() string {
	return fmt.Sprintf("enumeration %s (%s)", e.Name.Spaced(), e.Type.String())
}

// Integer represents a primitive integer,
// type.
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
	"sint8":  Sint8,
	"sint16": Sint16,
	"sint32": Sint32,
	"sint64": Sint64,
}

func (b Integer) Parameter(a Arch) bool { return true }

func (b Integer) Docs() Docs {
	docs := map[Integer]Text{
		Byte:   "An 8-bit unsigned integer, representing an octet of memory.",
		Uint8:  "An 8-bit unsigned integer type.",
		Uint16: "A 16-bit unsigned integer type.",
		Uint32: "A 32-bit unsigned integer type.",
		Uint64: "A 64-bit unsigned integer type.",
		Sint8:  "An 8-bit signed integer type.",
		Sint16: "A 16-bit signed integer type.",
		Sint32: "A 32-bit signed integer type.",
		Sint64: "A 64-bit signed integer type.",
	}

	text, ok := docs[b]
	if !ok {
		panic(fmt.Sprintf("unrecognised integer type %d", b))
	}

	return Docs{text}
}

func (b Integer) Bits() int {
	bits := map[Integer]int{
		Byte:   8,
		Uint8:  8,
		Uint16: 16,
		Uint32: 32,
		Uint64: 64,
		Sint8:  8,
		Sint16: 16,
		Sint32: 32,
		Sint64: 64,
	}

	bit, ok := bits[b]
	if !ok {
		panic(fmt.Sprintf("unrecognised integer type %d", b))
	}

	return bit
}

func (b Integer) Min() int64 {
	mins := map[Integer]int64{
		Byte:   0,
		Uint8:  0,
		Uint16: 0,
		Uint32: 0,
		Uint64: 0,
		Sint8:  math.MinInt8,
		Sint16: math.MinInt16,
		Sint32: math.MinInt32,
		Sint64: math.MinInt64,
	}

	min, ok := mins[b]
	if !ok {
		panic(fmt.Sprintf("unrecognised integer type %d", b))
	}

	return min
}

func (b Integer) Max() uint64 {
	maxs := map[Integer]uint64{
		Byte:   math.MaxUint8,
		Uint8:  math.MaxUint8,
		Uint16: math.MaxUint16,
		Uint32: math.MaxUint32,
		Uint64: math.MaxUint64,
		Sint8:  math.MaxInt8,
		Sint16: math.MaxInt16,
		Sint32: math.MaxInt32,
		Sint64: math.MaxInt64,
	}

	max, ok := maxs[b]
	if !ok {
		panic(fmt.Sprintf("unrecognised integer type %d", b))
	}

	return max
}

func (b Integer) Alignment(a Arch) int {
	alignments := map[Arch]map[Integer]int{
		X86_64: {
			Byte:   1,
			Uint8:  1,
			Uint16: 2,
			Uint32: 4,
			Uint64: 8,
			Sint8:  1,
			Sint16: 2,
			Sint32: 4,
			Sint64: 8,
		},
	}

	aligns, ok := alignments[a]
	if !ok {
		panic(fmt.Sprintf("unrecognised architecture %d", a))
	}

	align, ok := aligns[b]
	if !ok {
		panic(fmt.Sprintf("unrecognised integer type %d", b))
	}

	return align
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
		panic(fmt.Sprintf("unrecognised integer type %d", b))
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
		panic(fmt.Sprintf("unrecognised integer type %d", b))
	}

	return s
}

// Keep the integer types together.

// NewInteger represents a new type
// that has been defined, with an
// underlying integer type.
type NewInteger struct {
	Name   Name
	Node   *ast.List
	Docs   Docs
	Groups []Name
	Type   Integer
}

var (
	_ Type     = (*NewInteger)(nil)
	_ ast.Node = (*NewInteger)(nil)
)

func (i *NewInteger) Pos() token.Position   { return i.Node.Pos() }
func (i *NewInteger) End() token.Position   { return i.Node.End() }
func (i *NewInteger) Parameter(a Arch) bool { return true }
func (i *NewInteger) Alignment(a Arch) int  { return i.Type.Alignment(a) }
func (i *NewInteger) Size(a Arch) int       { return i.Type.Size(a) }

func (i *NewInteger) String() string {
	return fmt.Sprintf("%s (%s)", i.Name.Spaced(), i.Type)
}

// Padding represents unused space that is included
// after a field in a structure to ensure the fields
// and structure remain correctly aligned.
type Padding uint16

var _ Type = Padding(0)

func (p Padding) Parameter(a Arch) bool {
	// Padding can only be used within a
	// structure to manage alignment, so
	// even if the padding is small enough
	// to fit in a register, we don't allow
	// it to be passed in one.
	return false
}

func (p Padding) Alignment(a Arch) int {
	return 1
}

func (p Padding) Size(a Arch) int {
	return int(p)
}

func (p Padding) String() string {
	return fmt.Sprintf("%d padding bytes", p)
}

// Pointer represents a pointer to
// another data type.
type Pointer struct {
	Mutable    bool
	Underlying Type
}

var _ Type = (*Pointer)(nil)

func (p *Pointer) Parameter(a Arch) bool { return true }

func (p *Pointer) Alignment(a Arch) int {
	aligns := map[Arch]int{
		X86_64: 8,
	}

	align, ok := aligns[a]
	if !ok {
		panic(fmt.Sprintf("unrecognised architecture %d", a))
	}

	return align
}

func (p *Pointer) Size(a Arch) int {
	sizes := map[Arch]int{
		X86_64: 8,
	}

	size, ok := sizes[a]
	if !ok {
		panic(fmt.Sprintf("unrecognised architecture %d", a))
	}

	return size
}

func (p *Pointer) String() string {
	if p.Mutable {
		return fmt.Sprintf("*mutable %s", p.Underlying.String())
	}

	return fmt.Sprintf("*constant %s", p.Underlying.String())
}

// Reference represents a name used to reference a
// type that has already been defined elsewhere.
type Reference struct {
	Name       Name
	Underlying Type
}

var _ Type = (*Reference)(nil)

func (r *Reference) Parameter(a Arch) bool {
	return r.Underlying.Parameter(a)
}

func (r *Reference) Alignment(a Arch) int {
	return r.Underlying.Alignment(a)
}

func (r *Reference) Size(a Arch) int {
	return r.Underlying.Size(a)
}

func (r *Reference) String() string {
	return r.Name.Spaced()
}

// Structure represents a structure defined
// in a syscalls plan.
type Structure struct {
	Name   Name
	Node   *ast.List
	Docs   Docs
	Groups []Name
	Fields []*Field
}

var (
	_ Type     = (*Structure)(nil)
	_ ast.Node = (*Structure)(nil)
)

func (s *Structure) Pos() token.Position { return s.Node.Pos() }
func (s *Structure) End() token.Position { return s.Node.End() }

func (s *Structure) Parameter(a Arch) bool {
	// Even when a structure consists of a single
	// field, which would fit in a register, we
	// still reject it so that the structure can
	// change without that changing the answer.
	return false
}

func (s *Structure) Alignment(a Arch) int {
	maxAlign := 1
	for _, field := range s.Fields {
		align := field.Alignment(a)
		if maxAlign < align {
			maxAlign = align
		}
	}

	return maxAlign
}

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

// Syscall describes a system call, including
// its parameters and results.
type Syscall struct {
	Name    Name
	Node    *ast.List
	Docs    Docs
	Groups  []Name
	Args    Parameters
	Results Parameters
}

var _ ast.Node = (*Syscall)(nil)

func (s *Syscall) Pos() token.Position { return s.Node.Pos() }
func (s *Syscall) End() token.Position { return s.Node.End() }

func (s *Syscall) String() string {
	return fmt.Sprintf("syscall %s", s.Name.Spaced())
}

// SyscallReference can be used in documentation
// references to link to a system call and
// is used internally to prevent syscalls
// and types clashing in the name space.
type SyscallReference struct {
	Name Name
}

var _ Type = (*SyscallReference)(nil)

func (r *SyscallReference) Parameter(a Arch) bool { return false }
func (r *SyscallReference) Alignment(a Arch) int  { return 1 }
func (r *SyscallReference) Size(a Arch) int       { return 0 }
func (r *SyscallReference) String() string        { return fmt.Sprintf("syscall %s", r.Name.Spaced()) }

// File represents a parsed syscalls plan.
type File struct {
	// Data structures.
	Arrays       []*Array
	Bitfields    []*Bitfield
	Enumerations []*Enumeration
	NewIntegers  []*NewInteger
	Structures   []*Structure

	// System calls.
	Syscalls []*Syscall

	// Item groups.
	Groups []*Group
}

// SyscallsEnumeration returns a synthetic
// enumeration (with no AST data) describing
// the set of syscalls. This can be used to
// iterate over the set of syscalls in a target
// language.
func (f *File) SyscallsEnumeration() *Enumeration {
	enum := &Enumeration{
		Name:   Name{"syscalls"},
		Docs:   Docs{Text("An enumeration describing the set of system calls.")},
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
func (f *File) DropAST() {
	for _, array := range f.Arrays {
		array.Node = nil
	}
	for _, bitfield := range f.Bitfields {
		bitfield.Node = nil
		for _, value := range bitfield.Values {
			value.Node = nil
		}
	}
	for _, enumeration := range f.Enumerations {
		enumeration.Node = nil
		for _, value := range enumeration.Values {
			value.Node = nil
		}
	}
	for _, integer := range f.NewIntegers {
		integer.Node = nil
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
	for _, group := range f.Groups {
		group.Node = nil
		for _, item := range group.List {
			item.Node = nil
		}
	}
}
