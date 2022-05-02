// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package lexer includes functionality for scanning a Plan source file into a
// sequence of tokens.
//
package lexer

import (
	"bytes"
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/ProjectSerenity/firefly/tools/plan/token"
)

// Lexeme describes a token, its position, and its textual
// value.
//
type Lexeme struct {
	Token    token.Token
	Position token.Position
	Value    string
}

func (l Lexeme) String() string {
	return fmt.Sprintf("%s (%s) at %#v", l.Token, l.Value, l.Position)
}

// lexer scans a sequence of bytes, producing a sequence of Plan
// tokens.
//
type lexer struct {
	// Immutable state.
	src     []byte
	lexemes chan<- Lexeme

	// Mutable state as we progress through the source.
	offset     int            // Offset into the file where the current token starts.
	nextOffset int            // Offset into the file of the current location.
	pos        token.Position // Position where the current token starts.
	line       int            // Line number of the current location.
	column     int            // Column number of the current location.
	prevColumn int            // The column number of the end of the previous line.
	width      int            // Number of bytes in the last code point read.
}

// Scan scans the given Plan source, producing a sequence of
// lexical tokens.
//
// Once the end of the file is reached, or an error is encountered,
// the channel will be closed, resulting in an endless sequence
// of EndOfFile tokens.
//
func Scan(source []byte) <-chan Lexeme {
	c := make(chan Lexeme)
	l := &lexer{
		src:     source,
		lexemes: c,

		offset:     0,
		nextOffset: 0,
		line:       1,
		column:     1,
		prevColumn: 1,
		width:      0,
	}

	go l.run()

	return c
}

// run scans through the lexer's source, emitting tokens
// until the end of the file is reached or an error is
// encountered. In either case, the channel of lexemes
// is then closed.
//
func (l *lexer) run() {
	// Close the channel, producing an endless sequence
	// of EndOfFile tokens once we're done.
	defer close(l.lexemes)

	if len(l.src) > token.MaxOffset {
		l.errorf("source file is too large")
		return
	}

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
		case isLetter(r):
			// Keep going until we get a non-identifier
			// token.
			for r = l.next(); isAlphanumeric(r); r = l.next() {
			}

			// Check the next token is appropriate; a
			// closing parenthesis, comment, or space.
			ok := isClosing(r) || (l.eof() && l.width == 0)
			if l.width != 0 {
				l.backup() // Don't include the next rune.
			}

			// Check the identifier is all lower case.
			if !bytes.Equal(l.src[l.offset:l.nextOffset], bytes.ToLower(l.src[l.offset:l.nextOffset])) {
				l.errorf("invalid identifier: identifiers must be lower case")
				return
			}

			l.lexeme(token.Identifier)
			if !ok {
				l.next()
				l.errorf("invalid composite token: %q after identifier", r)
				return
			}
		case isDigit(r):
			// For now, we only support non-negative
			// decimal integers, as numbers are only
			// used to define structure padding lengths.
			n := digitVal(r)
			base := 10
			leadingZero := r == '0'

			// Keep going until we get a non-digit
			// token.
			for r = l.next(); isDigit(r); r = l.next() {
				if leadingZero {
					l.errorf("invalid number: short octal literals are not supported")
					return
				}

				d := digitVal(r)
				if d >= base {
					l.errorf("illegal character %q in number", r)
					return
				}

				n = n*base + d
			}

			// Check the next token is appropriate; a
			// closing parenthesis, comment, or space.
			ok := isClosing(r) || (l.eof() && l.width == 0)
			if l.width != 0 {
				l.backup() // Don't include the next rune.
			}

			l.lexeme(token.Number)
			if !ok {
				l.next()
				l.errorf("invalid composite token: %q after number", r)
				return
			}
		case r == '"':
			l.scanString()
		case r == '*':
			l.lexeme(token.Asterisk)
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
			return
		}
	}
}

const (
	// End of file pseudo-rune.
	eof = -1

	// Byte order mark.
	bom = 0xfeff
)

// eof returns whether the lexer has reached the end
// of the source.
//
func (l *lexer) eof() bool {
	return l.nextOffset >= len(l.src)
}

// errorf records the given error message.
//
func (l *lexer) errorf(format string, v ...any) {
	l.lexemes <- Lexeme{Token: token.Error, Position: l.pos, Value: fmt.Sprintf(format, v...)}
}

// next consumes the next code point, returning it.
//
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
		} else if r == bom {
			l.errorf("illegal byte order mark")
		}
	}

	l.nextOffset += l.width
	l.column++
	if r == '\n' {
		l.prevColumn = l.column - 1
		l.column = 1
		l.line++
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
//
func (l *lexer) backup() {
	if l.width == 0 && !l.eof() {
		panic("Lexer.backup() called without preceeding call to Lexer.next()")
	}

	if l.nextOffset == 0 && l.eof() {
		return
	}

	l.nextOffset -= l.width
	l.width = 0
	l.column--
	if l.column == 0 {
		l.line--
		l.column = l.prevColumn
	}
}

// peek returns the next rune, without consuming
// it from the source.
//
func (l *lexer) peek() rune {
	r := l.next()
	l.backup()

	return r
}

// advance the source position.
//
func (l *lexer) advance() {
	l.offset = l.nextOffset
	var err error
	l.pos, err = token.NewPosition(l.offset, l.line, l.column)
	if err != nil {
		l.errorf("%v", err)
	}
}

// lexeme returns a Lexeme from the current position,
// with the given token type.
//
func (l *lexer) lexeme(tok token.Token) {
	val := string(l.src[l.offset:l.nextOffset])
	l.lexemes <- Lexeme{Token: tok, Position: l.pos, Value: val}
	l.advance()
}

// scanString is called after the opening quote
// has been scanned.
//
func (l *lexer) scanString() {
	for {
		r := l.next()
		switch r {
		case '"':
			l.lexeme(token.String)

			// Check the next token is appropriate; a
			// closing parenthesis, comment, or space.
			if !isClosing(l.peek()) && !l.eof() {
				l.errorf("invalid composite token: %q after string", l.peek())
			}

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
	return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || r >= utf8.RuneSelf && unicode.IsLetter(r)
}

func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

func isAlphanumeric(r rune) bool {
	return isLetter(r) || isDigit(r)
}

func isWhitespace(r rune) bool {
	return unicode.IsSpace(r)
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
