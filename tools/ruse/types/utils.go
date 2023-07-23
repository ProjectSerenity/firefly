// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"errors"
	"fmt"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/token"
)

// positionalError represents an error that has
// occurred at a specific location within a Ruse
// file.
type positionalError struct {
	Pos token.Pos
	Msg string
}

func posErr(pos token.Pos, format string, v ...any) *positionalError {
	return &positionalError{
		Pos: pos,
		Msg: fmt.Sprintf(format, v...),
	}
}

func (e *positionalError) Context(format string, v ...any) *positionalError {
	msg := fmt.Sprintf(format, v...)
	return &positionalError{
		Pos: e.Pos,
		Msg: msg + ": " + e.Msg,
	}
}

// error passes the error to c.Error if that exists,
// then returns it as an error.
func (c *checker) error(e *positionalError) error {
	err := fmt.Errorf("%s: %s", c.fset.Position(e.Pos), e.Msg)
	if c.Error != nil {
		c.Error(err)
	}

	return err
}

// errorf prepares the error, passes it to c.Error if
// that exists, then returns it.
func (c *checker) errorf(pos token.Pos, format string, v ...any) error {
	text := fmt.Sprintf("%s: %s", c.fset.Position(pos), fmt.Sprintf(format, v...))
	err := errors.New(text)
	if c.Error != nil {
		c.Error(err)
	}

	return err
}

// interpretDefinition ensures that the given list
// consists of an identifier followed by at least
// one further element.
func (c *checker) interpretDefinition(list *ast.List, context string) (kind *ast.Identifier, rest []ast.Expression, err *positionalError) {
	if len(list.Elements) == 0 {
		return nil, nil, posErr(list.ParenOpen, "invalid %s: empty definition", context)
	}

	var ok bool
	kind, ok = list.Elements[0].(*ast.Identifier)
	if !ok {
		return nil, nil, posErr(list.Elements[0].Pos(), "invalid %s: definition kind must be an identifier, found %s", context, list.Elements[0])
	}

	rest = list.Elements[1:]
	if len(rest) == 0 {
		return nil, nil, posErr(list.ParenClose, "invalid %s: definition must have at least one field, found none", context)
	}

	return kind, rest, nil
}

// interpretIdentifiersDefinition ensures that the
// given list consists of an identifier followed
// by one or more further identifiers.
func (c *checker) interpretIdentifiersDefinition(list *ast.List, context string) (kind *ast.Identifier, rest []*ast.Identifier, err *positionalError) {
	if len(list.Elements) == 0 {
		return nil, nil, posErr(list.ParenOpen, "invalid %s: empty definition", context)
	}

	var ok bool
	kind, ok = list.Elements[0].(*ast.Identifier)
	if !ok {
		return nil, nil, posErr(list.Elements[0].Pos(), "invalid %s: definition kind must be an identifier, found %s", context, list.Elements[0])
	}

	rest = make([]*ast.Identifier, len(list.Elements[1:]))
	if len(rest) == 0 {
		return nil, nil, posErr(list.ParenClose, "invalid %s: definition must have at least one field, found none", context)
	}

	for i, elt := range list.Elements[1:] {
		ident, ok := elt.(*ast.Identifier)
		if !ok {
			return nil, nil, posErr(elt.Pos(), "invalid %s: value %s must be an identifier, found %s", context, elt.Print(), elt)
		}

		rest[i] = ident
	}

	return kind, rest, nil
}

// checkFixedArgsList ensures that the given list
// consists of one identifier, plus exactly
// len(names) elements.
//
// If the list is the wrong length, names is consulted to
// produce a clear error message.
func (c *checker) checkFixedArgsList(list *ast.List, context string, names ...string) *positionalError {
	if _, ok := list.Elements[0].(*ast.Identifier); !ok {
		return posErr(list.Elements[0].Pos(), "invalid %s: definition must be an identifier, found %s", context, list.Elements[0])
	}

	if len(list.Elements[1:]) < len(names) {
		return posErr(list.ParenClose, "invalid %s: %s missing", context, names[len(list.Elements[1:])])
	}

	if len(list.Elements[1:]) > len(names) {
		return posErr(list.Elements[len(names)+1].Pos(), "invalid %s: unexpected %s after %s", context, list.Elements[len(names)+1], names[len(names)-1])
	}

	return nil
}
