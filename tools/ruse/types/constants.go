// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"fmt"
	"go/constant"

	"firefly-os.dev/tools/ruse/token"
)

// Constant represents an immutable data value,
// declared at compile time.
type Constant struct {
	object
	value constant.Value
}

var _ Object = (*Constant)(nil)

func NewConstant(scope *Scope, pos, end token.Pos, pkg *Package, name string, typ Type, value constant.Value) *Constant {
	return &Constant{
		object: object{
			parent: scope,
			pos:    pos,
			end:    end,
			pkg:    pkg,
			name:   name,
			typ:    typ,
		},
		value: value,
	}
}

func (c *Constant) String() string {
	return fmt.Sprintf("constant %s (%s)", c.object.name, c.object.typ)
}

func (c *Constant) Value() constant.Value {
	return c.value
}
