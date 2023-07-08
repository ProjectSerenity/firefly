// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"fmt"

	"firefly-os.dev/tools/ruse/token"
)

// Import represents a reference to an
// imported package. An Import has no
// type.
type Import struct {
	object
	imported *Package
	used     bool // Set if the reference is used.
}

var _ Object = (*Import)(nil)

func NewImport(scope *Scope, pos, end token.Pos, pkg *Package, name string, imported *Package) *Import {
	return &Import{
		object: object{
			parent: scope,
			pos:    pos,
			end:    end,
			pkg:    pkg,
			name:   name,
		},
		imported: imported,
	}
}

func (i *Import) String() string {
	return fmt.Sprintf("import %s (%q)", i.object.name, i.imported.Path)
}
