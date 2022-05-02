// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package token contains constants for the lexical tokens in the
// Plan interface description language and types to compactly store
// a position in a Plan source file.
//
package token

import (
	"strconv"
)

// Token is the set of lexical tokens in Plan.
//
type Token int

const (
	// Special tokens.
	EndOfFile Token = iota
	Error

	// Primitive tokens.
	ParenOpen  // (
	ParenClose // )
	Identifier // Alphanumeric identifier.
	String     // "foo"
	Number     // 13
	Comment    // ; Foo
	Asterisk   // *

	endTokens
)

var tokens = [...]string{
	EndOfFile: "end of file",
	Error:     "error",

	ParenOpen:  "opening parenthesis",
	ParenClose: "closing parenthesis",
	Identifier: "identifier",
	String:     "string",
	Number:     "number",
	Comment:    "comment",
	Asterisk:   "asterisk",
}

// String returns the textual representation for
// the token t.
//
func (t Token) String() string {
	if 0 <= t && t < endTokens {
		return tokens[t]
	}

	return "Token(" + strconv.Itoa(int(t)) + ")"
}
