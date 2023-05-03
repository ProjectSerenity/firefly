// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"fmt"

	"firefly-os.dev/tools/ruse/token"
)

// Object represents a Ruse object, such as a package,
// constant, or function.
type Object interface {
	Parent() *Scope    // The scope in which the object is declared.
	Pos() token.Pos    // The position where the object's declaration starts.
	End() token.Pos    // The position where the object's declaration ends.
	Package() *Package // The package in which this object was declared, or nil for builtins.
	Name() string      // The name to which the object is bound at the point of declaration.
	Exported() bool    // Whether the object's name starts with a capital letter.
	Type() Type        // The object's type.

	String() string // Returns a human-readable description of the object.

	setParent(*Scope)
}

// object contains the common fields of an object type.
type object struct {
	parent   *Scope
	pos, end token.Pos
	pkg      *Package
	name     string
	typ      Type
}

func (o *object) Parent() *Scope    { return o.parent }
func (o *object) Pos() token.Pos    { return o.pos }
func (o *object) End() token.Pos    { return o.end }
func (o *object) Package() *Package { return o.pkg }
func (o *object) Name() string      { return o.name }
func (o *object) Type() Type        { return o.typ }
func (o *object) Exported() bool    { return token.IsExported(o.name) }

func (o *object) setParent(scope *Scope) { o.parent = scope }

// TypeName represents a name for a (builtin or
// alias) type.
type TypeName struct {
	object
}

var _ Object = (*TypeName)(nil)

func NewTypeName(scope *Scope, pos, end token.Pos, pkg *Package, name string, typ Type) *TypeName {
	return &TypeName{
		object: object{
			parent: scope,
			pos:    pos,
			end:    end,
			pkg:    pkg,
			name:   name,
			typ:    typ,
		},
	}
}

func (n *TypeName) IsAlias() bool {
	switch t := n.typ.(type) {
	case *Basic:
		return n.pkg != nil || t.name != n.name || t == Byte
	default:
		return true
	}
}

func (n *TypeName) String() string {
	return fmt.Sprintf("type %s (%s)", n.object.name, n.object.typ)
}
