// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Go types used by Plan types, but not Plan types (or syscalls)
// themselves.

package types

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"firefly-os.dev/tools/plan/ast"
)

// Name represents a name defined in a
// Plan source file.
type Name []string

// toTitle returns the word with the first rune
// in upper case and the remaining runes in lower
// case.
//
// This is similar to strings.Title, but we
// already know the string is alphanumeric only
// and contains no UTF-8 encoding errors.
func toTitle(s string) string {
	first, width := utf8.DecodeRuneInString(s)
	rest := s[width:]

	return string(unicode.ToUpper(first)) + strings.ToLower(rest)
}

// Spaced returns the name, separated by spaces.
func (n Name) Spaced() string {
	return strings.Join(n, " ")
}

// CamelCase returns the name in 'camel case',
// such as "camelCase".
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

// ScreamCase returns the name in 'scream case',
// such as "SCREAM_CASE".
func (n Name) ScreamCase() string {
	if len(n) == 1 {
		return strings.ToUpper(n[0])
	}

	lower := make([]string, len(n))
	for i, s := range n {
		lower[i] = strings.ToUpper(s)
	}

	return strings.Join(lower, "_")
}

// KebabCase returns the name in 'kebab case',
// such as "kebab-case".
func (n Name) KebabCase() string {
	if len(n) == 1 {
		return strings.ToLower(n[0])
	}

	lower := make([]string, len(n))
	for i, s := range n {
		lower[i] = strings.ToLower(s)
	}

	return strings.Join(lower, "-")
}

// TrainCase returns the name in 'train case',
// such as "TRAIN-CASE".
func (n Name) TrainCase() string {
	if len(n) == 1 {
		return strings.ToUpper(n[0])
	}

	lower := make([]string, len(n))
	for i, s := range n {
		lower[i] = strings.ToUpper(s)
	}

	return strings.Join(lower, "-")
}

// Docs represents a set of documentation
// for a type Plan a source file. The docs
// are split into lines.
type Docs []DocsItem

// DocsItem represents an item in the set
// of documentation for a type, syscall,
// or field.
type DocsItem interface {
	docsItem()
}

type (
	// Text represents plain text in a set of
	// documentation.
	//
	Text string

	// CodeText represents plain text in a set
	// of documentation that should be formatted
	// as source code.
	//
	CodeText string

	// ReferenceText represents plain text that
	// refers to a type defined in the Plan document.
	//
	// This will normally be turned into a link
	// to the relevant type definition when
	// rendered in documentation.
	//
	ReferenceText struct {
		Type
	}

	// Newline represents a line break in the
	// text of a set of documentation.
	//
	Newline struct{}
)

func (t Text) docsItem()          {}
func (t CodeText) docsItem()      {}
func (r ReferenceText) docsItem() {}
func (n Newline) docsItem()       {}

// Arch represents an instruction set
// architecture, which is used to customise
// types for a particular architecture.
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

// Field represents a single field in a structure
// type.
type Field struct {
	Name Name
	Node *ast.List
	Docs Docs
	Type Type
}

func (f *Field) Register(a Arch) bool {
	// Fields can only be used within a
	// larger structure.
	return false
}

func (f *Field) Alignment(a Arch) int {
	return f.Type.Alignment(a)
}

func (f *Field) Size(a Arch) int {
	return f.Type.Size(a)
}

func (f *Field) String() string {
	return fmt.Sprintf("field %s: %s", f.Name.Spaced(), f.Type.String())
}

// Value represents a single value in an
// enumeration type.
type Value struct {
	Name Name
	Node *ast.List
	Docs Docs
}

// Parameter represents a single argument
// or result in a function call.
type Parameter struct {
	Name Name
	Node *ast.List
	Docs Docs
	Type Type
}

func (p *Parameter) Enumeration() *Enumeration {
	return Underlying(p.Type).(*Enumeration)
}

func (p *Parameter) String() string {
	return fmt.Sprintf("parameter %s: %s", p.Name.Spaced(), p.Type.String())
}

// Parameters is an ordered set of function
// parameters, such as its arguments or
// results.
type Parameters []*Parameter

// ItemReference is like Reference, but it
// names the type of item in Underlying.
type ItemReference struct {
	Type       string
	Name       Name
	Node       *ast.List
	Underlying any
}

// Group represents a group of logically
// related items.
type Group struct {
	Name Name
	Node *ast.List
	Docs Docs
	List []*ItemReference
}

func (g *Group) String() string {
	return fmt.Sprintf("group %s", g.Name.Spaced())
}

func (g *Group) Syscalls() []*Syscall {
	out := make([]*Syscall, 0, len(g.List))
	for _, item := range g.List {
		if syscall, ok := item.Underlying.(*Syscall); ok {
			out = append(out, syscall)
		}
	}

	return out
}
