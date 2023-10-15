// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package parser

import (
	"bytes"
	"go/scanner"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/token"
)

func TestParseExpression(t *testing.T) {
	tests := []struct {
		Name string
		Src  string
		Want ast.Expression
		Err  string
	}{
		{
			Name: "identifier",
			Src:  "x",
			Want: &ast.Identifier{
				NamePos: 1,
				Name:    "x",
			},
		},
		{
			Name: "decimal integer",
			Src:  "1234",
			Want: &ast.Literal{
				ValuePos: 1,
				Kind:     token.Integer,
				Value:    "1234",
			},
		},
		{
			Name: "hexadecimal integer",
			Src:  "0xdeadbeef",
			Want: &ast.Literal{
				ValuePos: 1,
				Kind:     token.Integer,
				Value:    "0xdeadbeef",
			},
		},
		{
			Name: "binary integer",
			Src:  "0b10101111",
			Want: &ast.Literal{
				ValuePos: 1,
				Kind:     token.Integer,
				Value:    "0b10101111",
			},
		},
		{
			Name: "string",
			Src:  "\"foo\"",
			Want: &ast.Literal{
				ValuePos: 1,
				Kind:     token.String,
				Value:    "\"foo\"",
			},
		},
		{
			Name: "pair",
			Src:  "(1 . \"foo\")",
			Want: &ast.List{
				ParenOpen: 1,
				Elements: []ast.Expression{
					&ast.Literal{ValuePos: 2, Kind: token.Integer, Value: "1"},
					&ast.Identifier{NamePos: 4, Name: "."},
					&ast.Literal{ValuePos: 6, Kind: token.String, Value: "\"foo\""},
				},
				ParenClose: 11,
			},
		},
		{
			Name: "list expression",
			Src:  "(+ a b)",
			Want: &ast.List{
				ParenOpen: 1,
				Elements: []ast.Expression{
					&ast.Identifier{NamePos: 2, Name: "+"},
					&ast.Identifier{NamePos: 4, Name: "a"},
					&ast.Identifier{NamePos: 6, Name: "b"},
				},
				ParenClose: 7,
			},
		},
		{
			Name: "invalid list",
			Src:  "(+ ()   b)",
			Want: &ast.List{
				ParenOpen: 1,
				Elements: []ast.Expression{
					&ast.Identifier{NamePos: 2, Name: "+"},
					&ast.List{ParenOpen: 4, Elements: []ast.Expression{}, ParenClose: 5},
					&ast.Identifier{NamePos: 9, Name: "b"},
				},
				ParenClose: 10,
			},
		},
		{
			Name: "invalid list with newline",
			Src:  "(+ ()\nb)",
			Want: &ast.List{
				ParenOpen: 1,
				Elements: []ast.Expression{
					&ast.Identifier{NamePos: 2, Name: "+"},
					&ast.List{ParenOpen: 4, Elements: []ast.Expression{}, ParenClose: 5},
					&ast.Identifier{NamePos: 7, Name: "b"},
				},
				ParenClose: 8,
			},
		},
		{
			Name: "nested list",
			Src:  "(+ () ((a b) c))",
			Want: &ast.List{
				ParenOpen: 1,
				Elements: []ast.Expression{
					&ast.Identifier{NamePos: 2, Name: "+"},
					&ast.List{ParenOpen: 4, Elements: []ast.Expression{}, ParenClose: 5},
					&ast.List{ParenOpen: 7, Elements: []ast.Expression{
						&ast.List{ParenOpen: 8, Elements: []ast.Expression{
							&ast.Identifier{NamePos: 9, Name: "a"},
							&ast.Identifier{NamePos: 11, Name: "b"},
						}, ParenClose: 12},
						&ast.Identifier{NamePos: 14, Name: "c"},
					}, ParenClose: 15},
				},
				ParenClose: 16,
			},
		},
		{
			Name: "quoted identifier",
			Src:  "'x",
			Want: &ast.QuotedIdentifier{
				Quote: 1,
				X: &ast.Identifier{
					NamePos: 2,
					Name:    "x",
				},
			},
		},
		{
			Name: "quoted list",
			Src:  "'(x)(y)",
			Want: &ast.List{
				ParenOpen: 5,
				Annotations: []*ast.QuotedList{
					{
						Quote: 1,
						X: &ast.List{
							ParenOpen: 2,
							Elements: []ast.Expression{
								&ast.Identifier{NamePos: 3, Name: "x"},
							},
							ParenClose: 4,
						},
					},
				},
				Elements: []ast.Expression{
					&ast.Identifier{NamePos: 6, Name: "y"},
				},
				ParenClose: 7,
			},
		},
		{
			Name: "spaced quoted list",
			Src:  "\t\t'(x)\n\t\t(y)",
			Want: &ast.List{
				ParenOpen: 10,
				Annotations: []*ast.QuotedList{
					{
						Quote: 3,
						X: &ast.List{
							ParenOpen: 4,
							Elements: []ast.Expression{
								&ast.Identifier{NamePos: 5, Name: "x"},
							},
							ParenClose: 6,
						},
					},
				},
				Elements: []ast.Expression{
					&ast.Identifier{NamePos: 11, Name: "y"},
				},
				ParenClose: 12,
			},
		},
		{
			Name: "qualified identifier",
			Src:  "x.y",
			Want: &ast.Qualified{
				X: &ast.Identifier{
					NamePos: 1,
					Name:    "x",
				},
				Period: 2,
				Y: &ast.Identifier{
					NamePos: 3,
					Name:    "y",
				},
			},
		},
		{
			Name: "invalid token",
			Src:  "£",
			Err:  "invalid token '£'",
		},
		{
			Name: "incomplete list",
			Src:  "(foo",
			Err:  "unclosed list: unexpected EOF",
		},
		{
			Name: "quoted decimal integer",
			Src:  "'1234",
			Err:  "expected identifier or list, found 1234",
		},
		{
			Name: "quoted hexadecimal integer",
			Src:  "'0xdeadbeef",
			Err:  "expected identifier or list, found 0xdeadbeef",
		},
		{
			Name: "quoted binary integer",
			Src:  "'0b10101111",
			Err:  "expected identifier or list, found 0b10101111",
		},
		{
			Name: "quoted list with no elements",
			Src:  "'()",
			Err:  "annotations must contain at least one expression",
		},
		{
			Name: "quoted list without identifier",
			Src:  "'(1)",
			Err:  "expected identifier",
		},
		{
			Name: "quoted list with gap before list",
			Src:  "'(x)\n\n(y)",
			Err:  "expected attached list or annotation",
		},
		{
			Name: "quoted list with gap before next quoted list",
			Src:  "'(x)\n\n'(y)(z)",
			Err:  "expected attached list or annotation",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			x, err := ParseExpression(test.Src)
			if test.Err != "" {
				if err == nil {
					t.Fatalf("ParseExpression(): got %#v, want error %q", x, test.Err)
				}

				e := err.Error()
				if !strings.Contains(e, test.Err) {
					t.Fatalf("ParseExpression(): got error %q, want %q", e, test.Err)
				}

				return
			}

			if err != nil {
				t.Errorf("ParseExpression(%q): %v", test.Src, err)
				return
			}

			if diff := cmp.Diff(test.Want, x); diff != "" {
				t.Errorf("ParseExpression(%q): (-want, +got)\n%s", test.Src, diff)
				return
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	tests := []struct {
		Name string
		Src  string
		Want *ast.File
		Bad  string
		Err  string
	}{
		{
			Name: "empty file",
			Src:  ``,
			Err:  "expected package name",
		},
		{
			Name: "no package name",
			Src:  `1`,
			Err:  "expected package name",
		},
		{
			Name: "empty list",
			Src:  `()`,
			Bad:  `()`,
			Err:  "expected package name",
		},
		{
			Name: "bad list",
			Src:  `(package foo bar)`,
			Bad:  `(package foo bar)`,
			Err:  "expected package name",
		},
		{
			Name: "bad name",
			Src:  `(1 foo)`,
			Bad:  `(1 foo)`,
			Err:  "expected package name",
		},
		{
			Name: "different name",
			Src:  `(pkg foo)`,
			Bad:  `(pkg foo)`,
			Err:  "expected package name",
		},
		{
			Name: "bad package name",
			Src:  `(package "foo")`,
			Err:  "expected package name",
		},
		{
			Name: "invalid package name",
			Src:  `(package _)`,
			Bad:  `_`,
			Err:  "invalid package name",
		},
		{
			Name: "invalid top-level identifier",
			Src: `(package foo)
			      bar`,
			Bad: `bar`,
			Err: "expected list",
		},
		{
			Name: "invalid top-level list",
			Src: `(package foo)
			      ()`,
			Bad: `()`,
			Err: "invalid top-level list",
		},
		{
			Name: "unterminated import",
			Src: `(package foo)
			      (import`,
			Err: "test.ruse:2:10: unclosed list: unexpected EOF",
		},
		{
			Name: "empty import",
			Src: `(package foo)
			      (import)`,
			Bad: `(import)`,
			Err: "no import path",
		},
		{
			Name: "bad single import path",
			Src: `(package foo)
			      (import 1)`,
			Bad: `1`,
			Err: "expected import path string",
		},
		{
			Name: "bad single named import name",
			Src: `(package foo)
			      (import "foo" "bar")`,
			Bad: `"foo"`,
			Err: "expected import name symbol",
		},
		{
			Name: "bad single named import path",
			Src: `(package foo)
			      (import foo bar)`,
			Bad: `bar`,
			Err: "expected import path string",
		},
		{
			Name: "bad single import",
			Src: `(package foo)
			      (import foo bar baz)`,
			Bad: `baz`,
			Err: "unexpected expression after import",
		},
		{
			Name: "bad multi import",
			Src: `(package foo)
			      (import ("foo") 1)`,
			Bad: `1`,
			Err: "expected import list expression",
		},
		{
			Name: "bad multi import list",
			Src: `(package foo)
			      (import ("foo") ())`,
			Bad: `()`,
			Err: "no import path",
		},
		{
			Name: "bad multi import path",
			Src: `(package foo)
			      (import (1))`,
			Bad: `1`,
			Err: "expected import path string",
		},
		{
			Name: "bad multi named import name",
			Src: `(package foo)
			      (import ("foo" "bar"))`,
			Bad: `"foo"`,
			Err: "expected import name symbol",
		},
		{
			Name: "bad multi named import path",
			Src: `(package foo)
			      (import (foo bar))`,
			Bad: `bar`,
			Err: "expected import path string",
		},
		{
			Name: "bad multi import expression",
			Src: `(package foo)
			      (import (foo bar baz))`,
			Bad: `baz`,
			Err: "unexpected expression after import list expression",
		},
		{
			Name: "illegal token in list",
			Src: `(package foo)
			      (£)`,
			Bad: `£)`,
			Err: "invalid token '£'",
		},
		{
			Name: "single definition",
			Src: `(package foo)
			      (let x 1)`,
			Want: &ast.File{
				Package: 1,
				Name:    &ast.Identifier{NamePos: 10, Name: "foo"},
				Expressions: []*ast.List{
					{
						ParenOpen: 24,
						Elements: []ast.Expression{
							&ast.Identifier{NamePos: 25, Name: "let"},
							&ast.Identifier{NamePos: 29, Name: "x"},
							&ast.Literal{ValuePos: 31, Kind: token.Integer, Value: "1"},
						},
						ParenClose: 32,
					},
				},
			},
		},
		{
			Name: "single import and definitions",
			Src: `(package foo)
			      (import b "bar")
			      (let x 1)
			      (let y 2)`,
			Want: &ast.File{
				Package: 1,
				Name:    &ast.Identifier{NamePos: 10, Name: "foo"},
				Imports: []*ast.Import{
					{
						List: &ast.List{
							ParenOpen: 24,
							Elements: []ast.Expression{
								&ast.Identifier{NamePos: 25, Name: "import"},
								&ast.Identifier{NamePos: 32, Name: "b"},
								&ast.Literal{ValuePos: 34, Kind: token.String, Value: `"bar"`},
							},
							ParenClose: 39,
						},
						Name: &ast.Identifier{NamePos: 32, Name: "b"},
						Path: &ast.Literal{ValuePos: 34, Kind: token.String, Value: `"bar"`},
					},
				},
				Expressions: []*ast.List{
					{
						ParenOpen: 50,
						Elements: []ast.Expression{
							&ast.Identifier{NamePos: 51, Name: "let"},
							&ast.Identifier{NamePos: 55, Name: "x"},
							&ast.Literal{ValuePos: 57, Kind: token.Integer, Value: "1"},
						},
						ParenClose: 58,
					},
					{
						ParenOpen: 69,
						Elements: []ast.Expression{
							&ast.Identifier{NamePos: 70, Name: "let"},
							&ast.Identifier{NamePos: 74, Name: "y"},
							&ast.Literal{ValuePos: 76, Kind: token.Integer, Value: "2"},
						},
						ParenClose: 77,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fset := token.NewFileSet()
			got, err := ParseFile(fset, "test.ruse", test.Src, ParseComments)
			if test.Err != "" {
				if err == nil {
					t.Fatalf("ParseFile(): got %#v, want error %q", got, test.Err)
				}

				e := err.Error()
				if !strings.Contains(e, test.Err) {
					t.Fatalf("ParseFile(): got error %q, want %q", e, test.Err)
				}

				if test.Bad == "" {
					return
				}

				if el, ok := err.(scanner.ErrorList); ok && len(el) == 1 {
					err = el[0]
				}

				se := err.(*scanner.Error)
				bad := test.Src[se.Pos.Offset:]
				if !strings.HasPrefix(bad, test.Bad) {
					t.Fatalf("ParseFile(): wrong bad code\nGot:  %q\nWant: %q", bad, test.Bad)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseFile(): got error %v, want %#v", err, test.Want)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Fatalf("ParseFile(): (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestTraceExpression(t *testing.T) {
	tests := []struct {
		Name string
		Src  string
		Want []string
	}{
		{
			Name: "identifier",
			Src:  "(bar)",
			Want: []string{
				`    1:  1: parseExpression (`,
				`    1:  1: . parseList (`,
				`    1:  1: . . opening parenthesis`,
				`    1:  2: . . identifier "bar"`,
				`    1:  5: . )`,
				`    1:  5: . closing parenthesis`,
				`    0:  0: )`,
				``,
			},
		},
		{
			Name: "pair",
			Src:  "(1 . \"foo\")",
			Want: []string{
				`    1:  1: parseExpression (`,
				`    1:  1: . parseList (`,
				`    1:  1: . . opening parenthesis`,
				`    1:  2: . . integer 1`,
				`    1:  4: . . identifier "."`,
				`    1:  6: . . string "foo"`,
				`    1: 11: . )`,
				`    1: 11: . closing parenthesis`,
				`    0:  0: )`,
				``,
			},
		},
		{
			Name: "list expression",
			Src:  "(+ a b)",
			Want: []string{
				`    1:  1: parseExpression (`,
				`    1:  1: . parseList (`,
				`    1:  1: . . opening parenthesis`,
				`    1:  2: . . identifier "+"`,
				`    1:  4: . . identifier "a"`,
				`    1:  6: . . identifier "b"`,
				`    1:  7: . )`,
				`    1:  7: . closing parenthesis`,
				`    0:  0: )`,
				``,
			},
		},
		{
			Name: "invalid list",
			Src:  "(+ ()   b)",
			Want: []string{
				`    1:  1: parseExpression (`,
				`    1:  1: . parseList (`,
				`    1:  1: . . opening parenthesis`,
				`    1:  2: . . identifier "+"`,
				`    1:  4: . . parseList (`,
				`    1:  4: . . . opening parenthesis`,
				`    1:  5: . . )`,
				`    1:  5: . . closing parenthesis`,
				`    1:  9: . . identifier "b"`,
				`    1: 10: . )`,
				`    1: 10: . closing parenthesis`,
				`    0:  0: )`,
				``,
			},
		},
		{
			Name: "invalid list with newline",
			Src:  "(+ ()\nb)",
			Want: []string{
				`    1:  1: parseExpression (`,
				`    1:  1: . parseList (`,
				`    1:  1: . . opening parenthesis`,
				`    1:  2: . . identifier "+"`,
				`    1:  4: . . parseList (`,
				`    1:  4: . . . opening parenthesis`,
				`    1:  5: . . )`,
				`    1:  5: . . closing parenthesis`,
				`    2:  1: . . identifier "b"`,
				`    2:  2: . )`,
				`    2:  2: . closing parenthesis`,
				`    0:  0: )`,
				``,
			},
		},
		{
			Name: "nested list",
			Src:  "(+ () ((a b) c))",
			Want: []string{
				`    1:  1: parseExpression (`,
				`    1:  1: . parseList (`,
				`    1:  1: . . opening parenthesis`,
				`    1:  2: . . identifier "+"`,
				`    1:  4: . . parseList (`,
				`    1:  4: . . . opening parenthesis`,
				`    1:  5: . . )`,
				`    1:  5: . . closing parenthesis`,
				`    1:  7: . . parseList (`,
				`    1:  7: . . . opening parenthesis`,
				`    1:  8: . . . parseList (`,
				`    1:  8: . . . . opening parenthesis`,
				`    1:  9: . . . . identifier "a"`,
				`    1: 11: . . . . identifier "b"`,
				`    1: 12: . . . )`,
				`    1: 12: . . . closing parenthesis`,
				`    1: 14: . . . identifier "c"`,
				`    1: 15: . . )`,
				`    1: 15: . . closing parenthesis`,
				`    1: 16: . )`,
				`    1: 16: . closing parenthesis`,
				`    0:  0: )`,
				``,
			},
		},
		{
			Name: "list with line breaks and comments",
			Src:  "(+ ()\n; commentary\n((a b) ; another\n; comment\nc))",
			Want: []string{
				`    1:  1: parseExpression (`,
				`    1:  1: . parseList (`,
				`    1:  1: . . opening parenthesis`,
				`    1:  2: . . identifier "+"`,
				`    1:  4: . . parseList (`,
				`    1:  4: . . . opening parenthesis`,
				`    1:  5: . . )`,
				`    1:  5: . . closing parenthesis`,
				`    2:  1: . . comment`,
				`    3:  1: . . parseList (`,
				`    3:  1: . . . opening parenthesis`,
				`    3:  2: . . . parseList (`,
				`    3:  2: . . . . opening parenthesis`,
				`    3:  3: . . . . identifier "a"`,
				`    3:  5: . . . . identifier "b"`,
				`    3:  6: . . . )`,
				`    3:  6: . . . closing parenthesis`,
				`    3:  8: . . . comment`,
				`    4:  1: . . . comment`,
				`    5:  1: . . . identifier "c"`,
				`    5:  2: . . )`,
				`    5:  2: . . closing parenthesis`,
				`    5:  3: . )`,
				`    5:  3: . closing parenthesis`,
				`    0:  0: )`,
				``,
			},
		},
		{
			Name: "quoted identifier",
			Src:  "'x",
			Want: []string{
				`    1:  1: parseExpression (`,
				`    1:  1: . quote`,
				`    1:  2: . parseExpression (`,
				`    1:  2: . . identifier "x"`,
				`    0:  0: . )`,
				`    0:  0: )`,
				``,
			},
		},
		{
			Name: "quoted list",
			Src:  "'(x)(y)",
			Want: []string{
				`    1:  1: parseExpression (`,
				`    1:  1: . quote`,
				`    1:  2: . parseExpression (`,
				`    1:  2: . . parseList (`,
				`    1:  2: . . . opening parenthesis`,
				`    1:  3: . . . identifier "x"`,
				`    1:  4: . . )`,
				`    1:  4: . . closing parenthesis`,
				`    1:  5: . )`,
				`    1:  5: . expectList (`,
				`    1:  5: . . parseList (`,
				`    1:  5: . . . opening parenthesis`,
				`    1:  6: . . . identifier "y"`,
				`    1:  7: . . )`,
				`    1:  7: . . closing parenthesis`,
				`    0:  0: . )`,
				`    0:  0: )`,
				``,
			},
		},
		{
			Name: "qualified identifier",
			Src:  "x.y",
			Want: []string{
				`    1:  1: parseExpression (`,
				`    1:  1: . identifier "x"`,
				`    1:  2: . period`,
				`    1:  3: . expectIdentifier (`,
				`    1:  3: . . identifier "y"`,
				`    0:  0: . )`,
				`    0:  0: )`,
				``,
			},
		},
	}

	var buf bytes.Buffer
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			buf.Reset()
			old := traceOutput
			traceOutput = &buf
			defer func() { traceOutput = old }()

			fset := token.NewFileSet()
			_, err := ParseExpressionFrom(fset, "test.ruse", test.Src, Trace)
			if err != nil {
				t.Errorf("ParseExpression(): %v", err)
				return
			}

			if diff := cmp.Diff(strings.Join(test.Want, "\n"), buf.String()); diff != "" {
				t.Errorf("ParseExpression(%q, Trace): (-want, +got)\n%s", test.Src, diff)
				return
			}
		})
	}
}

func TestTraceFile(t *testing.T) {
	tests := []struct {
		Name  string
		Lines []string
		Want  []string
	}{
		{
			Name: "identifier",
			Lines: []string{
				"(package foo)",
				"(bar)",
			},
			Want: []string{
				`    1:  1: parseFile (`,
				`    1:  1: . parseList (`,
				`    1:  1: . . opening parenthesis`,
				`    1:  2: . . identifier "package"`,
				`    1: 10: . . identifier "foo"`,
				`    1: 13: . )`,
				`    1: 13: . closing parenthesis`,
				`    2:  1: . parseExpression (`,
				`    2:  1: . . parseList (`,
				`    2:  1: . . . opening parenthesis`,
				`    2:  2: . . . identifier "bar"`,
				`    2:  5: . . )`,
				`    2:  5: . . closing parenthesis`,
				`    0:  0: . )`,
				`    0:  0: )`,
				``,
			},
		},
		{
			Name: "pair",
			Lines: []string{
				"(package foo)",
				"(1 . \"foo\")",
			},
			Want: []string{
				`    1:  1: parseFile (`,
				`    1:  1: . parseList (`,
				`    1:  1: . . opening parenthesis`,
				`    1:  2: . . identifier "package"`,
				`    1: 10: . . identifier "foo"`,
				`    1: 13: . )`,
				`    1: 13: . closing parenthesis`,
				`    2:  1: . parseExpression (`,
				`    2:  1: . . parseList (`,
				`    2:  1: . . . opening parenthesis`,
				`    2:  2: . . . integer 1`,
				`    2:  4: . . . identifier "."`,
				`    2:  6: . . . string "foo"`,
				`    2: 11: . . )`,
				`    2: 11: . . closing parenthesis`,
				`    0:  0: . )`,
				`    0:  0: )`,
				``,
			},
		},
		{
			Name: "list expression",
			Lines: []string{
				"(package foo)",
				"(+ a b)",
			},
			Want: []string{
				`    1:  1: parseFile (`,
				`    1:  1: . parseList (`,
				`    1:  1: . . opening parenthesis`,
				`    1:  2: . . identifier "package"`,
				`    1: 10: . . identifier "foo"`,
				`    1: 13: . )`,
				`    1: 13: . closing parenthesis`,
				`    2:  1: . parseExpression (`,
				`    2:  1: . . parseList (`,
				`    2:  1: . . . opening parenthesis`,
				`    2:  2: . . . identifier "+"`,
				`    2:  4: . . . identifier "a"`,
				`    2:  6: . . . identifier "b"`,
				`    2:  7: . . )`,
				`    2:  7: . . closing parenthesis`,
				`    0:  0: . )`,
				`    0:  0: )`,
				``,
			},
		},
		{
			Name: "invalid list",
			Lines: []string{
				"(package foo)",
				"(+ ()   b)",
			},
			Want: []string{
				`    1:  1: parseFile (`,
				`    1:  1: . parseList (`,
				`    1:  1: . . opening parenthesis`,
				`    1:  2: . . identifier "package"`,
				`    1: 10: . . identifier "foo"`,
				`    1: 13: . )`,
				`    1: 13: . closing parenthesis`,
				`    2:  1: . parseExpression (`,
				`    2:  1: . . parseList (`,
				`    2:  1: . . . opening parenthesis`,
				`    2:  2: . . . identifier "+"`,
				`    2:  4: . . . parseList (`,
				`    2:  4: . . . . opening parenthesis`,
				`    2:  5: . . . )`,
				`    2:  5: . . . closing parenthesis`,
				`    2:  9: . . . identifier "b"`,
				`    2: 10: . . )`,
				`    2: 10: . . closing parenthesis`,
				`    0:  0: . )`,
				`    0:  0: )`,
				``,
			},
		},
		{
			Name: "invalid list with newline",
			Lines: []string{
				"(package foo)",
				"(+ ()\nb)",
			},
			Want: []string{
				`    1:  1: parseFile (`,
				`    1:  1: . parseList (`,
				`    1:  1: . . opening parenthesis`,
				`    1:  2: . . identifier "package"`,
				`    1: 10: . . identifier "foo"`,
				`    1: 13: . )`,
				`    1: 13: . closing parenthesis`,
				`    2:  1: . parseExpression (`,
				`    2:  1: . . parseList (`,
				`    2:  1: . . . opening parenthesis`,
				`    2:  2: . . . identifier "+"`,
				`    2:  4: . . . parseList (`,
				`    2:  4: . . . . opening parenthesis`,
				`    2:  5: . . . )`,
				`    2:  5: . . . closing parenthesis`,
				`    3:  1: . . . identifier "b"`,
				`    3:  2: . . )`,
				`    3:  2: . . closing parenthesis`,
				`    0:  0: . )`,
				`    0:  0: )`,
				``,
			},
		},
		{
			Name: "nested list",
			Lines: []string{
				"(package foo)",
				"(+ () ((a b) c))",
			},
			Want: []string{
				`    1:  1: parseFile (`,
				`    1:  1: . parseList (`,
				`    1:  1: . . opening parenthesis`,
				`    1:  2: . . identifier "package"`,
				`    1: 10: . . identifier "foo"`,
				`    1: 13: . )`,
				`    1: 13: . closing parenthesis`,
				`    2:  1: . parseExpression (`,
				`    2:  1: . . parseList (`,
				`    2:  1: . . . opening parenthesis`,
				`    2:  2: . . . identifier "+"`,
				`    2:  4: . . . parseList (`,
				`    2:  4: . . . . opening parenthesis`,
				`    2:  5: . . . )`,
				`    2:  5: . . . closing parenthesis`,
				`    2:  7: . . . parseList (`,
				`    2:  7: . . . . opening parenthesis`,
				`    2:  8: . . . . parseList (`,
				`    2:  8: . . . . . opening parenthesis`,
				`    2:  9: . . . . . identifier "a"`,
				`    2: 11: . . . . . identifier "b"`,
				`    2: 12: . . . . )`,
				`    2: 12: . . . . closing parenthesis`,
				`    2: 14: . . . . identifier "c"`,
				`    2: 15: . . . )`,
				`    2: 15: . . . closing parenthesis`,
				`    2: 16: . . )`,
				`    2: 16: . . closing parenthesis`,
				`    0:  0: . )`,
				`    0:  0: )`,
				``,
			},
		},
		{
			Name: "list with line breaks and comments",
			Lines: []string{
				"(package foo)",
				"(+ ()\n; commentary\n((a b) ; another\n; comment\nc))",
			},
			Want: []string{
				`    1:  1: parseFile (`,
				`    1:  1: . parseList (`,
				`    1:  1: . . opening parenthesis`,
				`    1:  2: . . identifier "package"`,
				`    1: 10: . . identifier "foo"`,
				`    1: 13: . )`,
				`    1: 13: . closing parenthesis`,
				`    2:  1: . parseExpression (`,
				`    2:  1: . . parseList (`,
				`    2:  1: . . . opening parenthesis`,
				`    2:  2: . . . identifier "+"`,
				`    2:  4: . . . parseList (`,
				`    2:  4: . . . . opening parenthesis`,
				`    2:  5: . . . )`,
				`    2:  5: . . . closing parenthesis`,
				`    3:  1: . . . comment`,
				`    4:  1: . . . parseList (`,
				`    4:  1: . . . . opening parenthesis`,
				`    4:  2: . . . . parseList (`,
				`    4:  2: . . . . . opening parenthesis`,
				`    4:  3: . . . . . identifier "a"`,
				`    4:  5: . . . . . identifier "b"`,
				`    4:  6: . . . . )`,
				`    4:  6: . . . . closing parenthesis`,
				`    4:  8: . . . . comment`,
				`    5:  1: . . . . comment`,
				`    6:  1: . . . . identifier "c"`,
				`    6:  2: . . . )`,
				`    6:  2: . . . closing parenthesis`,
				`    6:  3: . . )`,
				`    6:  3: . . closing parenthesis`,
				`    0:  0: . )`,
				`    0:  0: )`,
				``,
			},
		},
	}

	var buf bytes.Buffer
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			buf.Reset()
			old := traceOutput
			traceOutput = &buf
			defer func() { traceOutput = old }()

			fset := token.NewFileSet()
			_, err := ParseFile(fset, "test.ruse", strings.Join(test.Lines, "\n"), Trace)
			if err != nil {
				t.Errorf("ParseExpr(): %v", err)
				return
			}

			if diff := cmp.Diff(strings.Join(test.Want, "\n"), buf.String()); diff != "" {
				t.Errorf("ParseFile(Trace): (-want, +got)\n%s", diff)
				return
			}
		})
	}
}
