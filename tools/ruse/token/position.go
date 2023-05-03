// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package token

import (
	"go/token"
)

// We use the position and FileSet types from the
// "go/token" package, as they're not Go-specific
// and they suit our needs.

type (
	Position = token.Position
	Pos      = token.Pos
	File     = token.File
	FileSet  = token.FileSet
)

func NewFileSet() *FileSet {
	return token.NewFileSet()
}

const NoPos = token.NoPos
