// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
)

// A Visitor's Visit method is invoked for each node encountered by Walk.
// If the result visitor w is not nil, Walk visits each of the children
// of node with the visitor w, followed by a call of w.Visit(nil).
type Visitor interface {
	Visit(node Node) (w Visitor)
}

// Helper functions for common node lists. They may be empty.

func walkIdentList(v Visitor, list []*Identifier) {
	for _, x := range list {
		Walk(v, x)
	}
}

func walkExprList(v Visitor, list []Expression) {
	for _, x := range list {
		Walk(v, x)
	}
}

func walkImports(v Visitor, imports []*Import) {
	for _, x := range imports {
		Walk(v, x)
	}
}

func walkList(v Visitor, list []*List) {
	for _, x := range list {
		Walk(v, x)
	}
}

// TODO(gri): Investigate if providing a closure to Walk leads to
//            simpler use (and may help eliminate Inspect in turn).

// Walk traverses an AST in depth-first order: It starts by calling
// v.Visit(node); node must not be nil. If the visitor w returned by
// v.Visit(node) is not nil, Walk is invoked recursively with visitor
// w for each of the non-nil children of node, followed by a call of
// w.Visit(nil).
func Walk(v Visitor, node Node) {
	if v = v.Visit(node); v == nil {
		return
	}

	// walk children
	// (the order of the cases matches the order
	// of the corresponding node types in ast.go)
	switch n := node.(type) {
	// Comments and fields
	case *Comment:
		// nothing to do

	case *CommentGroup:
		for _, c := range n.List {
			Walk(v, c)
		}

	// Expressions
	case *BadExpression, *Identifier, *Literal:
		// nothing to do

	case *List:
		walkExprList(v, n.Elements)

	case *QuotedIdentifier:
		Walk(v, n.X)

	case *QuotedList:
		Walk(v, n.X)

	case *Qualified:
		Walk(v, n.X)
		Walk(v, n.Y)

	// Files and packages
	case *File:
		if n.Doc != nil {
			Walk(v, n.Doc)
		}
		Walk(v, n.Name)
		walkImports(v, n.Imports)
		walkList(v, n.Expressions)
		// don't walk n.Comments - they have been
		// visited already through the individual
		// nodes

	case *Package:
		for _, f := range n.Files {
			Walk(v, f)
		}

	default:
		panic(fmt.Sprintf("ast.Walk: unexpected node type %T", n))
	}

	v.Visit(nil)
}

type inspector func(Node) bool

func (f inspector) Visit(node Node) Visitor {
	if f(node) {
		return f
	}
	return nil
}

// Inspect traverses an AST in depth-first order: It starts by calling
// f(node); node must not be nil.
//
// If f returns true, Inspect invokes f recursively for each of the
// non-nil children of node, followed by a call of f(nil).
func Inspect(node Node, f func(Node) bool) {
	Walk(inspector(f), node)
}
