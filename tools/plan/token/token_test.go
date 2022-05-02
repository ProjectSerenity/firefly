// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package token

import (
	"strings"
	"testing"
)

func TestTokens(t *testing.T) {
	// Make sure the zero value is EOF, as we use
	// that fact in the lexer.
	if EndOfFile != 0 {
		t.Errorf("the zero value for Token is %q, not EndOfFile", Token(0))
	}

	// Make sure we have a string representation
	// for every token.
	for tok := EndOfFile; tok < endTokens; tok++ {
		if tokens[tok] == "" {
			t.Errorf("Token(%d) has no string value in tokens", tok)
		}

		got := tok.String()
		if strings.HasPrefix(got, "token(") {
			t.Errorf("Token(%d) gave string representation %q", tok, got)
		}
	}

	// Make sure we get something descriptive for
	// unexpected tokens.
	const (
		invalid = 127
		want    = "Token(127)"
	)

	got := Token(invalid).String()
	if got != want {
		t.Errorf("Token(%d).String(): got %q, want %q", invalid, got, want)
	}
}
