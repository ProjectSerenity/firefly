// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"io"

	"firefly-os.dev/tools/vendeps"
)

func init() {
	RegisterCommand("vendored", "Update the vendored dependencies used.", cmdVendored)
}

func cmdVendored(ctx context.Context, w io.Writer, args []string) error {
	return vendeps.UpdateDependencies("deps.bzl")
}
