// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package compiler

import (
	"fmt"

	"firefly-os.dev/tools/ruse/ssafir"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

// Lower takes SSAFIR values with register allocations
// provided by `Allocate` and lowers the instructions
// to architecture-specific instructions.
func Lower(fset *token.FileSet, arch *sys.Arch, sizes types.Sizes, fun *ssafir.Function) error {
	switch arch {
	case sys.X86_64:
		return lowerX86(fset, arch, sizes, fun)
	default:
		return fmt.Errorf("unsupported architecture: %s", arch.Name)
	}
}
