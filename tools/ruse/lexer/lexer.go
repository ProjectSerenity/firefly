// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package lexer implements a lexical scanner for Ruse source text.
//
// It takes a []byte source, which is then tokenised by repeated
// calls to the Scan method.
package lexer

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"firefly-os.dev/tools/ruse/token"
)

// Lexeme describes a token, its position, and its textual
// value.
type Lexeme struct {
	Token    token.Token
	Position token.Pos
	Value    string
}

func (l Lexeme) String(fset *token.FileSet) string {
	return fmt.Sprintf("%s: %s (%s)", fset.Position(l.Position), l.Token, l.Value)
}

// lexer scans a sequence of bytes, producing a sequence of Ruse
// tokens.
type lexer struct {
	// Immutable state.
	src     []byte
	file    *token.File
	lexemes chan<- Lexeme

	// Mutable state as we progress through the source.
	offset     int // Offset into the file where the current token starts.
	nextOffset int // Offset into the file of the current location.
	width      int // Number of bytes in the last code point read.
}

// Scan scans the given Ruse source, producing a sequence of
// lexical tokens.
//
// Once the end of the file is reached, or an error is encountered,
// the channel will be closed, resulting in an endless sequence
// of EndOfFile tokens.
func Scan(file *token.File, source []byte) <-chan Lexeme {
	c := make(chan Lexeme)
	l := &lexer{
		src:     source,
		file:    file,
		lexemes: c,

		offset:     0,
		nextOffset: 0,
		width:      0,
	}

	go l.run()

	return c
}

// run scans through the lexer's source, emitting tokens
// until the end of the file is reached or an error is
// encountered. In either case, the channel of lexemes
// is then closed.
func (l *lexer) run() {
	// Close the channel, producing an endless sequence
	// of EndOfFile tokens once we're done.
	defer close(l.lexemes)

	for {
		// Skip over any whitespace.
		for isWhitespace(l.next()) {
		}

		l.backup()
		l.advance()

		// Read the next rune.
		r := l.next()
		if r == eof {
			return
		}

		switch {
		case r == '(':
			l.lexeme(token.ParenOpen)
		case r == ')':
			l.lexeme(token.ParenClose)
		case r == '.':
			// A period on its own is treated like an
			// identifier. It must be on its own, or
			// it would be ambiguous with a qualified
			// identifier (which uses the Period token).
			//
			// Check the next token is appropriate; a
			// closing parenthesis, comment, or space.
			l.detatchedLexeme(token.Identifier, l.next())
		case r == '-', r == '+':
			// These are ambiguous so we need to read
			// the next rune.
			if isDigit(l.next()) {
				l.backup()
				l.scanNumber(r)
				break
			}

			l.backup()

			// Identifier handling.
			fallthrough
		case isIdentifierInitial(r):
			// Keep going until we get a non-identifier
			// token.
			for r = l.next(); isIdentifierSubsequent(r); r = l.next() {
			}

			// Check for a qualified identifier.
			if r == '.' {
				l.backup()
				l.lexeme(token.Identifier)
				l.next()
				l.lexeme(token.Period)
				r = l.next()
				if !isIdentifierInitial(r) {
					l.errorf("invalid token %q after %s", r, token.Period)
					continue
				}

				for r = l.next(); isIdentifierSubsequent(r); r = l.next() {
				}
			}

			// Check the next token is appropriate; a
			// closing parenthesis, comment, or space.
			l.detatchedLexeme(token.Identifier, r)
		case isDigit(r):
			l.scanNumber(r)
		case r == '"':
			l.scanString()
		case r == '\'':
			// Check the next token is appropriate; an
			// opening parenthesis, identifier, or integer.
			l.atatchedLexeme(token.Quote, l.next())
		case r == ';':
			// Keep going until the end of the line or
			// the end of the file, whichever comes
			// first.
			for r = l.next(); r != eof && r != '\n'; r = l.next() {
			}

			// Don't include the trailing newline.
			if r == '\n' {
				l.backup()
			}

			l.lexeme(token.Comment)
		default:
			l.errorf("invalid token %q", r)
		}
	}
}

// End of file pseudo-rune.
const eof = -1

// eof returns whether the lexer has reached the end
// of the source.
func (l *lexer) eof() bool {
	return l.nextOffset >= len(l.src)
}

// errorf records the given error message.
func (l *lexer) errorf(format string, v ...any) {
	pos := l.file.Pos(l.offset)
	l.lexemes <- Lexeme{Token: token.Error, Position: pos, Value: fmt.Sprintf(format, v...)}
}

// next consumes the next code point, returning it.
func (l *lexer) next() (r rune) {
	if l.eof() {
		l.width = 0
		return eof
	}

	// Try an ASCII character first.
	r, l.width = rune(l.src[l.nextOffset]), 1
	if r >= utf8.RuneSelf {
		// Not ASCII.
		r, l.width = utf8.DecodeRune(l.src[l.nextOffset:])
		if r == utf8.RuneError && l.width == 1 {
			l.errorf("source is not valid UTF-8")
		}
	}

	l.nextOffset += l.width
	if r == '\n' {
		l.file.AddLine(l.nextOffset)
	}

	if r == 0 {
		l.errorf("illegal character NUL")
		return eof
	}

	return r
}

// backup steps back by one rune.
//
// If next was not called since the last call to
// backup, peek, or Init, backup will panic.
func (l *lexer) backup() {
	if l.width == 0 && !l.eof() {
		panic("internal error: lexer.backup() called without preceeding call to Lexer.next()")
	}

	if l.nextOffset == 0 && l.eof() || l.width == 0 {
		return
	}

	l.nextOffset -= l.width
	l.width = 0
}

// advance the source position.
func (l *lexer) advance() {
	l.offset = l.nextOffset
}

// lexeme emits a Lexeme at the current position,
// with the given token type.
func (l *lexer) lexeme(tok token.Token) {
	pos := l.file.Pos(l.offset)
	val := string(l.src[l.offset:l.nextOffset])
	l.lexemes <- Lexeme{Token: tok, Position: pos, Value: val}
	l.advance()
}

// detatchedLexeme emits a Lexeme at the current
// position, with the given token type and an
// error if the next rune is not a closing
// parenthesis, comment, or space.
func (l *lexer) detatchedLexeme(tok token.Token, next rune) {
	ok := isClosing(next) || (l.eof() && l.width == 0)
	l.backup()
	l.lexeme(tok)
	if !ok {
		r := l.next()
		l.errorf("invalid attached token: %q after %s", r, tok)
	}
}

// atatchedLexeme emits a Lexeme at the current
// position, with the given token type and an
// error if the next rune a comment, end of file,
// or space.
func (l *lexer) atatchedLexeme(tok token.Token, next rune) {
	ok := next == '(' || isDigit(next) || isIdentifierInitial(next)
	l.backup()
	l.lexeme(tok)
	if !ok {
		r := l.next()
		l.errorf("invalid detached token: %q after %s", r, tok)
	}
}

// scanNumber is called after the opening digit/sign
// has been scanned.
func (l *lexer) scanNumber(r rune) {
	if r == '+' || r == '-' {
		r = l.next()
	}

	base := 10
	var prefix string
	if r == '0' {
		// Either decimal zero, the start of a radix
		// prefix, or a short (illegal) octal literal.
		r = l.next()
		switch r {
		case 'x':
			// Hexadecimal literal.
			base = 16
			prefix = "0x"
		case 'b':
			// Binary literal.
			base = 2
			prefix = "0b"
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			// Octal short literal.
			l.errorf("invalid number: short octal literals are not supported")
			return
		case '_':
			r = l.next()
			if r == '_' {
				l.errorf("invalid number: multiple successive separators")
				return
			}
			if digitVal(r) == 16 {
				l.errorf("invalid number: trailing separator")
				return
			}
		default:
			// A decimal zero.
			//
			// Check the next token is appropriate; a
			// closing parenthesis, comment, or space.
			l.detatchedLexeme(token.Integer, r)
			return
		}
	}

	// If we've had a radix prefix, we need at
	// least one subsequent digit.
	if prefix != "" {
		r = l.next()
		if digitVal(r) >= base {
			if digitVal(r) < 16 {
				l.errorf("invalid number: %q is not a valid digit in base %d", r, base)
			} else {
				l.errorf("invalid number: radix prefix %q is not followed by a valid digit", prefix)
			}

			return
		}
	}

	r = l.next()
	if r == '_' {
		r = l.next()
		if r == '_' {
			l.errorf("invalid number: multiple successive separators")
			return
		}
		if digitVal(r) == 16 {
			l.errorf("invalid number: trailing separator")
			return
		}
	}
	for digitVal(r) < base {
		r = l.next()
		if r == '_' {
			r = l.next()
			if r == '_' {
				l.errorf("invalid number: multiple successive separators")
				return
			}
			if digitVal(r) == 16 {
				l.errorf("invalid number: trailing separator")
				return
			}
		}
	}

	// Check the next token is appropriate; a
	// closing parenthesis, comment, or space.
	ok := isClosing(r) || (l.eof() && l.width == 0)
	l.backup()
	l.lexeme(token.Integer)
	if !ok {
		r = l.next()
		if digitVal(r) < 16 {
			l.errorf("invalid number: %q is not a valid digit in base %d", r, base)
		} else {
			l.errorf("invalid attached token: %q after %s", r, token.Integer)
		}
	}
}

// scanString is called after the opening quote
// has been scanned.
func (l *lexer) scanString() {
	for {
		r := l.next()
		switch r {
		case '"':
			l.detatchedLexeme(token.String, l.next())
			return
		case '\n', eof:
			l.errorf("string literal not terminated")
			return
		case '\\':
			// Handle an escape sequence.
			var n int
			var base, max uint32
			switch r := l.next(); r {
			// Special characters.
			case 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\', '"':
				continue
			// Hexadecimal literal.
			case 'x':
				n, base, max = 2, 16, 255
			// Small Unicode code point hex literal.
			case 'u':
				n, base, max = 4, 16, unicode.MaxRune
			// Large Unicode code point hex literal.
			case 'U':
				n, base, max = 8, 16, unicode.MaxRune
			case eof:
				l.errorf("escape sequence not terminated")
				return
			default:
				l.errorf("unrecognised escape sequence character %q", r)
				return
			}

			// Handle the values of the escape sequence.
			var x uint32
			for n > 0 {
				r := l.next()
				if r == eof {
					l.errorf("escape sequence not terminated")
					return
				}

				d := uint32(digitVal(r))
				if d >= base {
					l.errorf("illegal character %q in escape sequence", r)
					return
				}

				x = x*base + d
				n--
			}

			if x > max || 0xD800 <= x && x < 0xE000 {
				l.errorf("escape sequence is not a valid Unicode code point")
				return
			}
		}
	}
}

// Rune predicates.

func isLetter(r rune) bool {
	return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || r == '_' || r >= utf8.RuneSelf && unicode.IsLetter(r)
}

func isIdentifierInitial(r rune) bool {
	if isLetter(r) {
		return true
	}

	switch r {
	case '!', '$', '%', '&', '*', '/', ':', '<', '=', '>', '?', '~', '_', '^', '|':
		return true
	}

	return false
}

func isIdentifierSubsequent(r rune) bool {
	return isIdentifierInitial(r) || isDigit(r) || r == '+' || r == '-'
}

func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

func isWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

func isClosing(r rune) bool {
	return r == ')' || r == ';' || isWhitespace(r)
}

func digitVal(r rune) int {
	switch {
	case '0' <= r && r <= '9':
		return int(r - '0')
	case 'a' <= r && r <= 'f':
		return int(r - 'a' + 10)
	case 'A' <= r && r <= 'F':
		return int(r - 'A' + 10)
	}

	return 16 // larger than any legal digit val
}
