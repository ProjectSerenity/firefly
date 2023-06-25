// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"strings"
)

// The headings are inconsistent about dash and superscript usage. Normalize.
var fixDash = strings.NewReplacer(
	"Compute 2 –1", "Compute 2^x-1",
	"Compute 2x-1", "Compute 2^x-1",
	"Compute 2x–1", "Compute 2^x-1",
	"/ FUCOMI", "/FUCOMI",
	"Compute y ∗ log x", "Compute y * log₂x",
	"Compute y * log2x", "Compute y * log₂x",
	"Compute y * log2(x +1)", "Compute y * log₂(x+1)",
	"Compute y ∗ log (x +1)", "Compute y * log₂(x+1)",
	" — ", "-",
	"— ", "-",
	" —", "-",
	"—", "-",
	" – ", "-",
	" –", "-",
	"– ", "-",
	"–", "-",
	" - ", "-",
	"- ", "-",
	" -", "-",
)
