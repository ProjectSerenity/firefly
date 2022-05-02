// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package parser

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ProjectSerenity/firefly/tools/plan/ast"
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

func TestParseFile(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want *ast.File
	}{
		{
			name: "empty file",
			src:  `; foo`,
			want: &ast.File{
				Comments: []*ast.CommentGroup{
					{
						List: []*ast.Comment{
							{
								Semicolon: position(t, 0, 1, 1),
								Text:      "; foo",
							},
						},
					},
				},
			},
		},
		{
			name: "comment group",
			src:  "; foo\n; bar",
			want: &ast.File{
				Comments: []*ast.CommentGroup{
					{
						List: []*ast.Comment{
							{
								Semicolon: position(t, 0, 1, 1),
								Text:      "; foo",
							},
							{
								Semicolon: position(t, 6, 2, 1),
								Text:      "; bar",
							},
						},
					},
				},
			},
		},
		{
			name: "separate comments",
			src:  "; foo\n\n; bar",
			want: &ast.File{
				Comments: []*ast.CommentGroup{
					{
						List: []*ast.Comment{
							{
								Semicolon: position(t, 0, 1, 1),
								Text:      "; foo",
							},
						},
					},
					{
						List: []*ast.Comment{
							{
								Semicolon: position(t, 7, 3, 1),
								Text:      "; bar",
							},
						},
					},
				},
			},
		},
		{
			name: "trailing comment",
			src:  "(foo (\"bar\"))\n; baz",
			want: &ast.File{
				Comments: []*ast.CommentGroup{
					{
						List: []*ast.Comment{
							{
								Semicolon: position(t, 14, 2, 1),
								Text:      "; baz",
							},
						},
					},
				},
				Lists: []*ast.List{
					{
						ParenOpen: position(t, 0, 1, 1),
						Elements: []ast.Expr{
							&ast.Identifier{NamePos: position(t, 1, 1, 2), Name: "foo"},
							&ast.List{
								ParenOpen: position(t, 5, 1, 6),
								Elements: []ast.Expr{
									&ast.String{QuotePos: position(t, 6, 1, 7), Text: `"bar"`},
								},
								ParenClose: position(t, 11, 1, 12),
							},
						},
						ParenClose: position(t, 12, 1, 13),
					},
				},
			},
		},
		{
			name: "list containing comment",
			src:  "(foo\n; bar\n1)",
			want: &ast.File{
				Comments: []*ast.CommentGroup{
					{
						List: []*ast.Comment{
							{
								Semicolon: position(t, 5, 2, 1),
								Text:      "; bar",
							},
						},
					},
				},
				Lists: []*ast.List{
					{
						ParenOpen: position(t, 0, 1, 1),
						Elements: []ast.Expr{
							&ast.Identifier{NamePos: position(t, 1, 1, 2), Name: "foo"},
							&ast.Number{ValuePos: position(t, 11, 3, 1), Value: "1"},
						},
						ParenClose: position(t, 12, 3, 2),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseFile("test.plan", test.src)
			if err != nil {
				t.Fatalf("ParseFile(): unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, test.want) {
				// Encoding the values in JSON makes the error
				// message more useful and legible.
				gotJSON, err := json.MarshalIndent(got, "", "\t")
				if err != nil {
					t.Fatal(err)
				}

				wantJSON, err := json.MarshalIndent(test.want, "", "\t")
				if err != nil {
					t.Fatal(err)
				}

				t.Fatalf("ParseFile():\nGot  %s\nWant %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestParseFileErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "top-level identifier",
			src:  `x`,
			want: `test.plan:1:1: expected list, found identifier "x"`,
		},
		{
			name: "syntax error",
			src:  `(foo bar) !`,
			want: `test.plan:1:11: expected list, found error "invalid token '!'"`,
		},
		{
			name: "incomplete list",
			src:  `(1 *constant byte`,
			want: `expected closing parenthesis, found end of file`,
		},
		{
			name: "invalid list",
			src:  `(1 !)`,
			want: `test.plan:1:4: invalid token '!'`,
		},
		{
			name: "incomplete pointer",
			src:  `(*1)`,
			want: `test.plan:1:3: expected identifier, found number "1"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f, err := ParseFile("test.plan", test.src)
			if err == nil {
				t.Fatalf("ParseFile(): got %v, expected error %q", f, test.want)
			}

			e := err.Error()
			if e != test.want {
				t.Fatalf("ParseFile():\nGot  %q\nWant %q", e, test.want)
			}
		})
	}
}

func FuzzParser(f *testing.F) {
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
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		_, _ = ParseFile("test.plan", input)
	})
}
