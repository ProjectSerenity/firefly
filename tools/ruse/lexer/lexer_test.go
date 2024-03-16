// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package lexer

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"firefly-os.dev/tools/ruse/token"
)

func TestLexeme_String(t *testing.T) {
	tests := []struct {
		Src  string
		Want []string
	}{
		{
			Src: "foo-bar",
			Want: []string{
				"test.ruse:1:1: identifier (foo-bar)",
			},
		},
		{
			Src: "0",
			Want: []string{
				"test.ruse:1:1: integer (0)",
			},
		},
		{
			Src: "4",
			Want: []string{
				"test.ruse:1:1: integer (4)",
			},
		},
		{
			Src: "123456789",
			Want: []string{
				"test.ruse:1:1: integer (123456789)",
			},
		},
	}

	for _, test := range tests {
		fset := token.NewFileSet()
		file := fset.AddFile("test.ruse", -1, len(test.Src))
		lexemes := Scan(file, []byte(test.Src))
		got := make([]Lexeme, 0, len(test.Want))
		for lexeme := range lexemes {
			got = append(got, lexeme)
		}

		gotStrings := make([]string, len(got))
		for i, got := range got {
			gotStrings[i] = got.String(fset)
		}

		if diff := cmp.Diff(test.Want, gotStrings); diff != "" {
			t.Errorf("Lexing %q: (-want, +got)\n%s", test.Src, diff)
		}
	}
}

func TestLexer(t *testing.T) {
	tests := []struct {
		Name string
		Src  string
		Want []Lexeme
	}{
		{
			Name: "identifier",
			Src:  "foo",
			Want: []Lexeme{
				{token.Identifier, 1, "foo"},
			},
		},
		{
			Name: "complex identifier",
			Src:  "foo-bar!",
			Want: []Lexeme{
				{token.Identifier, 1, "foo-bar!"},
			},
		},
		{
			Name: "decimal zero",
			Src:  "0",
			Want: []Lexeme{
				{token.Integer, 1, "0"},
			},
		},
		{
			Name: "decimal integer",
			Src:  "4",
			Want: []Lexeme{
				{token.Integer, 1, "4"},
			},
		},
		{
			Name: "separated integer",
			Src:  "0_4",
			Want: []Lexeme{
				{token.Integer, 1, "0_4"},
			},
		},
		{
			Name: "large decimal integer",
			Src:  "123456789",
			Want: []Lexeme{
				{token.Integer, 1, "123456789"},
			},
		},
		{
			Name: "separated large decimal integer",
			Src:  "12_34_56_78_9",
			Want: []Lexeme{
				{token.Integer, 1, "12_34_56_78_9"},
			},
		},
		{
			Name: "large hexadecimal integer",
			Src:  "0xdeadbeef123",
			Want: []Lexeme{
				{token.Integer, 1, "0xdeadbeef123"},
			},
		},
		{
			Name: "large binary integer",
			Src:  "0b1011010111",
			Want: []Lexeme{
				{token.Integer, 1, "0b1011010111"},
			},
		},
		{
			Name: "large negative binary integer",
			Src:  "-0xdeadbeef123",
			Want: []Lexeme{
				{token.Integer, 1, "-0xdeadbeef123"},
			},
		},
		{
			Name: "large positive binary integer",
			Src:  "+0b1011010111",
			Want: []Lexeme{
				{token.Integer, 1, "+0b1011010111"},
			},
		},
		{
			Name: "plus identifier",
			Src:  "+",
			Want: []Lexeme{
				{token.Identifier, 1, "+"},
			},
		},
		{
			Name: "minus identifier",
			Src:  "-",
			Want: []Lexeme{
				{token.Identifier, 1, "-"},
			},
		},
		{
			Name: "underscore identifier",
			Src:  "_",
			Want: []Lexeme{
				{token.Identifier, 1, "_"},
			},
		},
		{
			Name: "underscore prefixed identifier",
			Src:  "_foo",
			Want: []Lexeme{
				{token.Identifier, 1, "_foo"},
			},
		},
		{
			Name: "qualified identifier",
			Src:  "foo.bar",
			Want: []Lexeme{
				{token.Identifier, 1, "foo"},
				{token.Period, 4, "."},
				{token.Identifier, 5, "bar"},
			},
		},
		{
			Name: "empty string",
			Src:  `""`,
			Want: []Lexeme{
				{token.String, 1, `""`},
			},
		},
		{
			Name: "non-empty string",
			Src:  `"foo"`,
			Want: []Lexeme{
				{token.String, 1, `"foo"`},
			},
		},
		{
			Name: "string with escape code",
			Src:  `"\t"`,
			Want: []Lexeme{
				{token.String, 1, `"\t"`},
			},
		},
		{
			Name: "string with many escape codes",
			Src:  `"\xff \u00ff \U00008a9e"`,
			Want: []Lexeme{
				{token.String, 1, `"\xff \u00ff \U00008a9e"`},
			},
		},
		{
			Name: "quoted identifier",
			Src:  "'foo",
			Want: []Lexeme{
				{token.Quote, 1, "'"},
				{token.Identifier, 2, "foo"},
			},
		},
		{
			Name: "quoted integer",
			Src:  "'123",
			Want: []Lexeme{
				{token.Quote, 1, "'"},
				{token.Integer, 2, "123"},
			},
		},
		{
			Name: "quoted list",
			Src:  "'(123)",
			Want: []Lexeme{
				{token.Quote, 1, "'"},
				{token.ParenOpen, 2, "("},
				{token.Integer, 3, "123"},
				{token.ParenClose, 6, ")"},
			},
		},
		{
			Name: "nil list",
			Src:  "()",
			Want: []Lexeme{
				{token.ParenOpen, 1, "("},
				{token.ParenClose, 2, ")"},
			},
		},
		{
			Name: "list of one element",
			Src:  "(foo)",
			Want: []Lexeme{
				{token.ParenOpen, 1, "("},
				{token.Identifier, 2, "foo"},
				{token.ParenClose, 5, ")"},
			},
		},
		{
			Name: "list of two elements",
			Src:  "(foo \"bar\")",
			Want: []Lexeme{
				{token.ParenOpen, 1, "("},
				{token.Identifier, 2, "foo"},
				{token.String, 6, `"bar"`},
				{token.ParenClose, 11, ")"},
			},
		},
		{
			Name: "List with intermittent spacing",
			Src:  " (\tfoo\n\r)",
			Want: []Lexeme{
				{token.ParenOpen, 2, "("},
				{token.Identifier, 4, "foo"},
				{token.ParenClose, 9, ")"},
			},
		},
		{
			Name: "comment",
			Src:  "; foobar",
			Want: []Lexeme{
				{token.Comment, 1, "; foobar"},
			},
		},
		{
			Name: "identifier with abutting comment",
			Src:  "foo; foobar",
			Want: []Lexeme{
				{token.Identifier, 1, "foo"},
				{token.Comment, 4, "; foobar"},
			},
		},
		{
			Name: "multi-line comment",
			Src:  ";foo\n;bar",
			Want: []Lexeme{
				{token.Comment, 1, ";foo"},
				{token.Comment, 6, ";bar"},
			},
		},
		{
			Name: "illegal tokens",
			Src:  "£ foo ]",
			Want: []Lexeme{
				{token.Error, 1, "invalid token '£'"},
				{token.Identifier, 4, "foo"},
				{token.Error, 8, "invalid token ']'"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fset := token.NewFileSet()
			file := fset.AddFile("test.ruse", -1, len(test.Src))
			lexemes := Scan(file, []byte(test.Src))
			got := make([]Lexeme, 0, len(test.Want))
			for lexeme := range lexemes {
				got = append(got, lexeme)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("Lexing %q: (-want, +got)\n%s", test.Src, diff)
			}
		})
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		Name string
		Src  string
		Want Lexeme
	}{
		{
			Name: "empty file",
			Src:  "",
			Want: Lexeme{token.EndOfFile, 0, ""},
		},
		{
			Name: "short octal literal",
			Src:  "01",
			Want: Lexeme{token.Error, 1, "invalid number: short octal literals are not supported"},
		},
		{
			Name: "successive integer separator",
			Src:  "1__0",
			Want: Lexeme{token.Error, 1, "invalid number: multiple successive separators"},
		},
		{
			Name: "trailing integer separator",
			Src:  "1_",
			Want: Lexeme{token.Error, 1, "invalid number: trailing separator"},
		},
		{
			Name: "binary raix prefix with no digits",
			Src:  "0b",
			Want: Lexeme{token.Error, 1, "invalid number: radix prefix \"0b\" is not followed by a valid digit"},
		},
		{
			Name: "hexadecimal raix prefix with no digits",
			Src:  "0x",
			Want: Lexeme{token.Error, 1, "invalid number: radix prefix \"0x\" is not followed by a valid digit"},
		},
		{
			Name: "binary literal with invalid digit",
			Src:  "0b2",
			Want: Lexeme{token.Error, 1, "invalid number: '2' is not a valid digit in base 2"},
		},
		{
			Name: "binary literal with invalid subsequent digit",
			Src:  "0b12",
			Want: Lexeme{token.Error, 4, "invalid number: '2' is not a valid digit in base 2"},
		},
		{
			Name: "integer followed by letter",
			Src:  "1z",
			Want: Lexeme{token.Error, 2, "invalid attached token: 'z' after integer"},
		},
		{
			Name: "unterminated string literal",
			Src:  `"1`,
			Want: Lexeme{token.Error, 1, "string literal not terminated"},
		},
		{
			Name: "multi-line string",
			Src: `"1
			          "`,
			Want: Lexeme{token.Error, 1, "string literal not terminated"},
		},
		{
			Name: "unterminated escape sequence in string",
			Src:  `"\`,
			Want: Lexeme{token.Error, 1, "escape sequence not terminated"},
		},
		{
			Name: "invalid escape sequence character",
			Src:  `"\p"`,
			Want: Lexeme{token.Error, 1, "unrecognised escape sequence character 'p'"},
		},
		{
			Name: "partial escape sequence",
			Src:  `"\xf`,
			Want: Lexeme{token.Error, 1, "escape sequence not terminated"},
		},
		{
			Name: "truncated escape sequence",
			Src:  `"\xF"`,
			Want: Lexeme{token.Error, 1, "illegal character '\"' in escape sequence"},
		},
		{
			Name: "invalid Unicode escape sequence",
			Src:  `"\ud812"`,
			Want: Lexeme{token.Error, 1, "escape sequence is not a valid Unicode code point"},
		},
		{
			Name: "string literal with abutting identifier",
			Src:  `"foo"a`,
			Want: Lexeme{token.Error, 6, "invalid attached token: 'a' after string"},
		},
		{
			Name: "unattached quote",
			Src:  "' foo",
			Want: Lexeme{token.Error, 2, "invalid detached token: ' ' after quote"},
		},
		{
			Name: "identifier with abutting string literal",
			Src:  "a\"foo\"",
			Want: Lexeme{token.Error, 2, "invalid attached token: '\"' after identifier"},
		},
		{
			Name: "qualified identifier with abutting string literal",
			Src:  "a.\"foo\"",
			Want: Lexeme{token.Error, 3, "invalid token '\"' after period"},
		},
		{
			Name: "qualified identifier with abutting integer literal",
			Src:  "a.4",
			Want: Lexeme{token.Error, 3, "invalid token '4' after period"},
		},
		{
			Name: "qualified identifier with detached identifier",
			Src:  "a. b",
			Want: Lexeme{token.Error, 3, "invalid token ' ' after period"},
		},
		{
			Name: "unterminated string literal in list",
			Src:  "(\"foo)",
			Want: Lexeme{token.Error, 2, "string literal not terminated"},
		},
		{
			Name: "NUL in string literal",
			Src:  "(\"foo\x00\")",
			Want: Lexeme{token.Error, 2, "illegal character NUL"},
		},
		{
			Name: "invalid Unicode code point in string literal",
			Src:  "(\"foo\x80\")",
			Want: Lexeme{token.Error, 2, "source is not valid UTF-8"},
		},
		{
			Name: "invalid token",
			Src:  "£",
			Want: Lexeme{token.Error, 1, "invalid token '£'"},
		},
		{
			Name: "byte-order mark",
			Src:  "foo \xfeff",
			Want: Lexeme{token.Error, 4, "source is not valid UTF-8"},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fset := token.NewFileSet()
			file := fset.AddFile("test.ruse", -1, len(test.Src))
			lexemes := Scan(file, []byte(test.Src))
			var got []Lexeme
			for lexeme := range lexemes {
				got = append(got, lexeme)
			}

			// Include an EndOfFile too.
			got = append(got, <-lexemes)

			// We may get other tokens after the first error,
			// so we can't just check the last token.
			ok := false
			for _, lex := range got {
				if lex == test.Want {
					ok = true
					break
				}
			}

			if ok {
				return
			}

			g := make([]string, len(got))
			for i, got := range got {
				g[i] = got.String(fset)
			}

			t.Errorf("Lexing %q:\nGot:\n  %s\nwant:\n  %s", test.Src, strings.Join(g, "\n  "), test.Want.String(fset))
		})
	}
}

func FuzzLexer(f *testing.F) {
	tests := []string{
		`x`,
		`"foo"`,
		`123`,
		`*constant`,
		`()`,
		`; foo`,
	}

	for _, test := range tests {
		f.Add([]byte(test))
		f.Add([]byte(test + "\n"))
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		fset := token.NewFileSet()
		file := fset.AddFile("test.ruse", -1, len(input))
		lexemes := Scan(file, input)
		for lexeme := range lexemes {
			// Check that the token is one we recognise.
			tok := lexeme.Token.String()
			if strings.HasPrefix(tok, "Token(") {
				t.Errorf("invalid token: %s", tok)
			}
		}
	})
}
