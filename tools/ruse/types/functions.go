// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"fmt"

	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
)

// Function represents a function signature.
type Function struct {
	object
	abi *sys.ABI
}

var _ Object = (*Function)(nil)

func NewFunction(scope *Scope, pos, end token.Pos, pkg *Package, name string, signature *Signature) *Function {
	var typ Type
	if signature != nil {
		typ = signature
	}

	return &Function{
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

func (f *Function) ABI() *sys.ABI       { return f.abi }
func (f *Function) SetABI(abi *sys.ABI) { f.abi = abi }
func (f *Function) String() string {
	return fmt.Sprintf("function %s (%s)", f.object.name, f.object.typ)
}

// Signature represents a function
// signature.
type Signature struct {
	name   string
	params []*Variable
	result Type
}

var _ Type = (*Signature)(nil)

func NewSignature(name string, params []*Variable, result Type) *Signature {
	return &Signature{
		name:   name,
		params: params,
		result: result,
	}
}

func (s *Signature) Underlying() Type    { return s }
func (s *Signature) String() string      { return s.name }
func (s *Signature) Params() []*Variable { return s.params }
func (s *Signature) Result() Type        { return s.result }
