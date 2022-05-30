// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"testing"
)

func TestAcceptableLicense(t *testing.T) {
	tests := []struct {
		In   string
		Want string
		Ok   bool
	}{
		{
			In:   "MIT",
			Want: "MIT License",
			Ok:   true,
		},
		{
			In:   "MIT/Apache-2.0",
			Want: "MIT License",
			Ok:   true,
		},
		{
			In:   "Apache-2.0 OR MIT",
			Want: "Apache License 2.0",
			Ok:   true,
		},
		{
			In:   "",
			Want: "",
			Ok:   false,
		},
		{
			In:   "AGPL",
			Want: "",
			Ok:   false,
		},
	}

	for _, test := range tests {
		got, ok := AcceptableLicense(test.In)
		if got != test.Want || ok != test.Ok {
			t.Errorf("AcceptableLicense(%q): got (%q %v), want (%q %v)", test.In, got, ok, test.Want, test.Ok)
		}
	}
}
