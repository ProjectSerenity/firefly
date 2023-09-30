// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"firefly-os.dev/tools/ruse/internal/x86"
)

type registerSizeMapping struct {
	Register *x86.Register
	Others   []*x86.Register
}

// buildRegisterSegmentMappings uses [x86.RegisterSegments]
// to build the mappings from each register
// that forms a segment of a broader register
// to the other segments of the register.
//
// This is then used to implement [x86.Register.ToSize].
//
// We choose to include the cases where a
// register is mapped to itself, for consistency.
func buildRegisterSegmentMappings() []registerSizeMapping {
	out := make([]registerSizeMapping, 0, len(x86.Registers))
	for _, segments := range x86.RegisterSegments {
		for _, reg := range segments {
			out = append(out, registerSizeMapping{Register: reg, Others: segments})
		}
	}

	return out
}
