// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"firefly-os.dev/tools/ruse/token"
)

// Scope represents a lexical scope, which has
// one parent scope (in which it belongs) and
// an arbitrary number of child scopes, which
// it contains. A Scope also has a position,
// and an arbitrary number of object declarations.
type Scope struct {
	parent   *Scope
	children []*Scope
	number   int               // s.parent.children[e.number - 1] == e. Iff e.parent == nil, e.number == 0.
	decls    map[string]Object // Lazily allocated.
	pos, end token.Pos
	comment  string // A context string for debugging purposes.

	// If readonly is true, insertions to this scope
	// are added to the parent instead. This is used
	// in file scopes, as we want to read from them
	// but write to the parent (package) scope.
	readonly bool
}

// NewScope creates a new, empty scope.
func NewScope(parent *Scope, pos, end token.Pos, comment string) *Scope {
	s := &Scope{
		parent:  parent,
		pos:     pos,
		end:     end,
		comment: comment,
	}

	if parent != nil && parent != Universe {
		parent.children = append(parent.children, s)
		s.number = len(parent.children)
	}

	return s
}

// Child returns the ith child scope in s,
// which may be nil. i must be in the range
// 0 <= i < s.NumChildren().
func (s *Scope) Child(i int) *Scope {
	return s.children[i]
}

// NumChildren returns the number of child
// scopes in s.
func (s *Scope) NumChildren() int { return len(s.children) }

// Parent returns s's parent scope.
func (s *Scope) Parent() *Scope { return s.parent }

// Pos returns the position where s starts.
func (s *Scope) Pos() token.Pos { return s.pos }

// End returns the position where s ends.
func (s *Scope) End() token.Pos { return s.end }

// Contains returns whether s contains the
// given position.
func (s *Scope) Contains(pos token.Pos) bool {
	return s.pos <= pos && pos < s.end
}

// Innermost returns the smallest child scope
// that contains pos. If s does not contain
// pos, Innermost returns nil.
func (s *Scope) Innermost(pos token.Pos) *Scope {
	// Package scopes don't have bounds, so we
	// just iterate through their child (file)
	// scopes.
	if s.parent == Universe {
		for _, s := range s.children {
			if inner := s.Innermost(pos); inner != nil {
				return inner
			}
		}
	}

	if s.Contains(pos) {
		for _, s := range s.children {
			if s.Contains(pos) {
				return s.Innermost(pos)
			}
		}

		return s
	}

	return nil
}

// Len returns the number of objects declared
// in s.
func (s *Scope) Len() int { return len(s.decls) }

// Comment returns any debugging context for
// s.
func (s *Scope) Comment() string { return s.comment }

// Names returns the set of names declared in
// this scope, sorted lexographically.
//
// The sorting is not free.
func (s *Scope) Names() []string {
	names := make([]string, len(s.decls))

	i := 0
	for name := range s.decls {
		names[i] = name
		i++
	}

	sort.Strings(names)

	return names
}

// Lookup returns the object bound to the given
// name in the current scope (only). If no object
// has been bound to the given name within this
// scope, Lookup returns nil.
//
// See also LookupParent.
func (s *Scope) Lookup(name string) Object {
	return s.decls[name]
}

// LookupParent returns the object bound to the
// given name in the current scope and its
// (recursive) parent scope(s).
//
// If a valid position is provided, only objects
// declared at or before pos are considered.
//
// If no such object exists, LookupParent returns
// nil, nil.
func (s *Scope) LookupParent(name string, pos token.Pos) (*Scope, Object) {
	for ; s != nil; s = s.parent {
		if obj := s.Lookup(name); obj != nil && (!pos.IsValid() || obj.Parent().Pos() < pos) {
			return s, obj
		}
	}

	return nil, nil
}

// Insert attempts to insert obj into scope s.
//
// If s already contains another object with
// the same name, Insert returns the existing
// object without modifying s.
//
// Otherwise, Insert adds obj to s and sets
// obj's parent scope if currently unset, then
// returns nil.
func (s *Scope) Insert(obj Object) Object {
	if s.readonly {
		if obj.Parent() == s {
			obj.setParent(s.parent)
		}
		return s.parent.Insert(obj)
	}

	name := obj.Name()
	if other := s.Lookup(name); other != nil {
		return other
	}

	if s.decls == nil {
		s.decls = make(map[string]Object)
	}

	s.decls[name] = obj

	if obj.Parent() == nil {
		obj.setParent(s)
	}

	return nil
}

// String returns the textual representation
// of the scope, as provided by WriteTo.
func (s *Scope) String() string {
	var buf strings.Builder
	s.WriteTo(&buf, 0, false)
	return buf.String()
}

// WriteTo writes a textual representation
// of the scope to w, with the scope's
// names sorted lexographically.
//
// The level of indentation is controlled
// by n, with n=0 for no indentation.
//
// If recurse is true, WriteTo recursively
// writes all child scopes as well.
func (s *Scope) WriteTo(w io.Writer, n int, recurse bool) error {
	const indent = ".  "
	indentation := strings.Repeat(indent, n)

	if s == nil {
		fmt.Fprintf(w, "%s<nil scope!!>\n", indentation)
		return nil
	}
	_, err := fmt.Fprintf(w, "%s%s scope %p {\n", indentation, s.comment, s)
	if err != nil {
		return err
	}

	indented := indentation + indent
	for _, name := range s.Names() {
		_, err = fmt.Fprintf(w, "%s%s\n", indented, s.Lookup(name))
		if err != nil {
			return err
		}
	}

	if recurse {
		for _, s := range s.children {
			if s == nil {
				fmt.Fprintf(w, "%s<nil scope!>\n", indented)
				continue
			}
			err = s.WriteTo(w, n+1, recurse)
			if err != nil {
				return err
			}
		}
	}

	_, err = fmt.Fprintf(w, "%s}\n", indentation)
	if err != nil {
		return err
	}

	return nil
}
