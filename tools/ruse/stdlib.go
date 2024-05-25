// Copyright 2024 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	_ "embed"

	"firefly-os.dev/tools/ruse/cmd/internal/stdlib"
)

//go:embed stdlib_/stdlib.rstd
var data []byte

func init() {
	stdlib.Embedded = data
}
