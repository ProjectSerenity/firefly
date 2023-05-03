// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package token defines constants representing the lexical tokens of the Ruse
// programming language.
package token

import (
	"strconv"
	"unicode"
	"unicode/utf8"
)

// IsExported returns whether the given string starts with
// an upper-case letter.
func IsExported(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsUpper(r)
}

// Token is the set of lexical tokens of the Ruse programming language.
type Token int

// Note that EndOfFile deliberately has the value zero so that an infinite
// stream of EndOfFile tokens is emitted by a closed channel of tokens.

// The list of tokens.
const (
	// Special tokens
	EndOfFile Token = iota
	Error
	Comment

	literal_beg
	// Identifiers and basic type literals
	// (these tokens stand for classes of literals)
	Identifier // main
	Integer    // 12345
	String     // "abc"
	literal_end

	// Operators and delimiters
	ParenOpen  // (
	Period     // .
	Quote      // '
	ParenClose // )
)

var tokens = [...]string{
	EndOfFile: "end of file",
	Error:     "error",
	Comment:   "comment",

	Identifier: "identifier",
	Integer:    "integer",
	String:     "string",

	ParenOpen:  "opening parenthesis",
	Period:     "period",
	Quote:      "quote",
	ParenClose: "closing parenthesis",
}

// String returns the string corresponding to the token tok.
// For operators, delimiters, and keywords the string is the actual
// token character sequence (e.g., for the token ADD, the string is
// "+"). For all other tokens the string corresponds to the token
// constant name (e.g. for the token IDENT, the string is "IDENT").
func (tok Token) String() string {
	s := ""
	if 0 <= tok && tok < Token(len(tokens)) {
		s = tokens[tok]
	}

	if s == "" {
		s = "token(" + strconv.Itoa(int(tok)) + ")"
	}

	return s
}

// Predicates

// IsLiteral returns true for tokens corresponding to identifiers
// and basic type literals; it returns false otherwise.
func (tok Token) IsLiteral() bool { return literal_beg < tok && tok < literal_end }
