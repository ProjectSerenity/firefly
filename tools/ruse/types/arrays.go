// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"fmt"
)

// Array represents a Ruse array type, which has
// a length and an element type.
type Array struct {
	length  uint
	element Type
}

var _ Type = (*Array)(nil)

// NewArray returns a new array type with the given
// length and element type.
func NewArray(length uint, element Type) *Array {
	return &Array{
		length:  length,
		element: element,
	}
}

// Length returns the array's length.
func (a *Array) Length() uint { return a.length }

// Element returns the array's element type.
func (a *Array) Element() Type { return a.element }

func (a *Array) Underlying() Type { return a }
func (a *Array) String() string   { return fmt.Sprintf("array/%d/%s", a.length, a.element) }
