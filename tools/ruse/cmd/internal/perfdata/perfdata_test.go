// Copyright 2024 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package perfdata

import (
	"testing"
	"time"
)

func TestTrimDuration(t *testing.T) {
	tests := []struct {
		D time.Duration
		S string
	}{
		{1111 * time.Microsecond, "1.11ms"},
		{9999 * time.Microsecond, "10ms"},
	}

	for _, test := range tests {
		t.Run(test.D.String(), func(t *testing.T) {
			got := trimDuration(test.D)
			if got != test.S {
				t.Fatalf("trimDuration(%s):\nGot:  %s\nWant: %s", test.D, got, test.S)
			}
		})
	}
}
