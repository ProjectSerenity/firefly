// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package lexer

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ProjectSerenity/firefly/tools/plan/token"
)

func position(t *testing.T, offset, line, column int) token.Position {
	pos, err := token.NewPosition(offset, line, column)
	if err != nil {
		t.Helper()
		t.Fatalf("invalid position: %v", err)
	}

	return pos
}

func TestLexer(t *testing.T) {
	tests := []struct {
		src  string
		want []Lexeme
	}{
		{
			src: "foo",
			want: []Lexeme{
				{token.Identifier, position(t, 0, 1, 1), "foo"},
			},
		},
		{
			src: "0",
			want: []Lexeme{
				{token.Number, position(t, 0, 1, 1), "0"},
			},
		},
		{
			src: "4",
			want: []Lexeme{
				{token.Number, position(t, 0, 1, 1), "4"},
			},
		},
		{
			src: "123456789",
			want: []Lexeme{
				{token.Number, position(t, 0, 1, 1), "123456789"},
			},
		},
		{
			src: `""`,
			want: []Lexeme{
				{token.String, position(t, 0, 1, 1), `""`},
			},
		},
		{
			src: `""`,
			want: []Lexeme{
				{token.String, position(t, 0, 1, 1), `""`},
			},
		},
		{
			src: `"\t"`,
			want: []Lexeme{
				{token.String, position(t, 0, 1, 1), `"\t"`},
			},
		},
		{
			src: `"\xff \u00ff \U00008a9e"`,
			want: []Lexeme{
				{token.String, position(t, 0, 1, 1), `"\xff \u00ff \U00008a9e"`},
			},
		},
		{
			src: "()",
			want: []Lexeme{
				{token.ParenOpen, position(t, 0, 1, 1), "("},
				{token.ParenClose, position(t, 1, 1, 2), ")"},
			},
		},
		{
			src: "(foo)",
			want: []Lexeme{
				{token.ParenOpen, position(t, 0, 1, 1), "("},
				{token.Identifier, position(t, 1, 1, 2), "foo"},
				{token.ParenClose, position(t, 4, 1, 5), ")"},
			},
		},
		{
			src: "(foo *baz)",
			want: []Lexeme{
				{token.ParenOpen, position(t, 0, 1, 1), "("},
				{token.Identifier, position(t, 1, 1, 2), "foo"},
				{token.Asterisk, position(t, 5, 1, 6), "*"},
				{token.Identifier, position(t, 6, 1, 7), "baz"},
				{token.ParenClose, position(t, 9, 1, 10), ")"},
			},
		},
		{
			src: "(foo \"bar\")",
			want: []Lexeme{
				{token.ParenOpen, position(t, 0, 1, 1), "("},
				{token.Identifier, position(t, 1, 1, 2), "foo"},
				{token.String, position(t, 5, 1, 6), `"bar"`},
				{token.ParenClose, position(t, 10, 1, 11), ")"},
			},
		},
		{
			src: " (\tfoo\n\r)",
			want: []Lexeme{
				{token.ParenOpen, position(t, 1, 1, 2), "("},
				{token.Identifier, position(t, 3, 1, 4), "foo"},
				{token.ParenClose, position(t, 8, 2, 2), ")"},
			},
		},
		{
			src: "; foobar",
			want: []Lexeme{
				{token.Comment, position(t, 0, 1, 1), "; foobar"},
			},
		},
		{
			src: "foo; foobar",
			want: []Lexeme{
				{token.Identifier, position(t, 0, 1, 1), "foo"},
				{token.Comment, position(t, 3, 1, 4), "; foobar"},
			},
		},
		{
			src: ";foo\n;bar",
			want: []Lexeme{
				{token.Comment, position(t, 0, 1, 1), ";foo"},
				{token.Comment, position(t, 5, 2, 1), ";bar"},
			},
		},
	}

	for _, test := range tests {
		lexemes := Scan([]byte(test.src))
		got := make([]Lexeme, 0, len(test.want))
		for lexeme := range lexemes {
			got = append(got, lexeme)
		}

		if !reflect.DeepEqual(got, test.want) {
			g := make([]string, len(got))
			for i, got := range got {
				g[i] = got.String()
			}

			w := make([]string, len(test.want))
			for i, want := range test.want {
				w[i] = want.String()
			}

			t.Errorf("Lexing %q:\nGot:\n  %s\nWant:\n  %s", test.src, strings.Join(g, "\n  "), strings.Join(w, "\n  "))
		}
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		src  string
		want Lexeme
	}{
		{
			src:  "",
			want: Lexeme{token.EndOfFile, 0, ""},
		},
		{
			src:  "ABC",
			want: Lexeme{token.Error, position(t, 0, 1, 1), "invalid identifier: identifiers must be lower case"},
		},
		{
			src:  "01",
			want: Lexeme{token.Error, position(t, 0, 1, 1), "invalid number: short octal literals are not supported"},
		},
		{
			src:  "1z",
			want: Lexeme{token.Error, position(t, 1, 1, 2), "invalid composite token: 'z' after number"},
		},
		{
			src:  `"1`,
			want: Lexeme{token.Error, position(t, 0, 1, 1), "string literal not terminated"},
		},
		{
			src: `"1
			          "`,
			want: Lexeme{token.Error, position(t, 0, 1, 1), "string literal not terminated"},
		},
		{
			src:  `"\`,
			want: Lexeme{token.Error, position(t, 0, 1, 1), "escape sequence not terminated"},
		},
		{
			src:  `"\p"`,
			want: Lexeme{token.Error, position(t, 0, 1, 1), "unrecognised escape sequence character 'p'"},
		},
		{
			src:  `"\xf`,
			want: Lexeme{token.Error, position(t, 0, 1, 1), "escape sequence not terminated"},
		},
		{
			src:  `"\xF"`,
			want: Lexeme{token.Error, position(t, 0, 1, 1), "illegal character '\"' in escape sequence"},
		},
		{
			src:  `"\ud812"`,
			want: Lexeme{token.Error, position(t, 0, 1, 1), "escape sequence is not a valid Unicode code point"},
		},
		{
			src:  `"foo"a`,
			want: Lexeme{token.Error, position(t, 5, 1, 6), "invalid composite token: 'a' after string"},
		},
		{
			src:  "a\"foo\"",
			want: Lexeme{token.Error, position(t, 1, 1, 2), "invalid composite token: '\"' after identifier"},
		},
		{
			src:  "(\"foo)",
			want: Lexeme{token.Error, position(t, 1, 1, 2), "string literal not terminated"},
		},
		{
			src:  "(\"foo\x00\")",
			want: Lexeme{token.Error, position(t, 1, 1, 2), "illegal character NUL"},
		},
		{
			src:  "(\"foo\x80\")",
			want: Lexeme{token.Error, position(t, 1, 1, 2), "source is not valid UTF-8"},
		},
		{
			src:  "!",
			want: Lexeme{token.Error, position(t, 0, 1, 1), "invalid token '!'"},
		},
	}

	for _, test := range tests {
		lexemes := Scan([]byte(test.src))
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
			if lex == test.want {
				ok = true
				break
			}
		}

		if ok {
			continue
		}

		g := make([]string, len(got))
		for i, got := range got {
			g[i] = got.String()
		}

		t.Errorf("Lexing %q:\nGot:\n  %s\nWant:\n  %s", test.src, strings.Join(g, "\n  "), test.want)
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
		lexemes := Scan(input)
		for lexeme := range lexemes {
			// Check that the token is one we recognise.
			tok := lexeme.Token.String()
			if strings.HasPrefix(tok, "Token(") {
				t.Errorf("invalid token: %s", tok)
			}
		}
	})
}
