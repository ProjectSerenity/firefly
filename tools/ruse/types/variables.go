// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"fmt"

	"firefly-os.dev/tools/ruse/token"
)

// Variable represents an immutable data value,
// passed as a function parameter or created in
// a function body.
type Variable struct {
	object
	used bool // Set if the variable is used.
}

var _ Object = (*Variable)(nil)

func NewVariable(scope *Scope, pos, end token.Pos, pkg *Package, name string, typ Type) *Variable {
	return &Variable{
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

func NewParameter(scope *Scope, pos, end token.Pos, pkg *Package, name string, typ Type) *Variable {
	return &Variable{
		object: object{
			parent: scope,
			pos:    pos,
			end:    end,
			pkg:    pkg,
			name:   name,
			typ:    typ,
		},
		used: true, // Function parameters always count as used.
	}
}

func (v *Variable) String() string {
	return fmt.Sprintf("variable %s (%s)", v.object.name, v.object.typ)
}
