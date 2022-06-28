// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"rsc.io/diff"

	"firefly-os.dev/tools/plan/ast"
	"firefly-os.dev/tools/plan/parser"
	"firefly-os.dev/tools/plan/token"
)

func position(t *testing.T, offset, line, column int) token.Position {
	pos, err := token.NewPosition(offset, line, column)
	if err != nil {
		t.Helper()
		t.Fatalf("invalid position: %v", err)
	}

	return pos
}

func TestInterpreter(t *testing.T) {
	tests := []struct {
		Name   string
		Source string
		AST    bool
		Want   *File
	}{
		{
			Name:   "Simple array",
			Source: `(array (name blah) (docs "xyz" "" "abc\n" "") (size 1024) (type byte))`,
			Want: &File{
				Arrays: []*Array{
					{
						Name:  Name{"blah"},
						Docs:  Docs{Text("xyz"), Newline{}, Text("abc")},
						Count: 1024,
						Type:  Byte,
					},
				},
			},
		},
		{
			Name:   "Simple bitfield",
			Source: `(bitfield (name blah) (docs "xyz") (type byte) (value (name foo) (docs "bar")) (value (name bar) (docs "abc")))`,
			Want: &File{
				Bitfields: []*Bitfield{
					{
						Name: Name{"blah"},
						Docs: Docs{Text("xyz")},
						Type: Byte,
						Values: []*Value{
							{
								Name: Name{"foo"},
								Docs: Docs{Text("bar")},
							},
							{
								Name: Name{"bar"},
								Docs: Docs{Text("abc")},
							},
						},
					},
				},
			},
		},
		{
			Name:   "Simple enumeration",
			Source: `(enumeration (name blah) (docs "xyz") (type byte) (value (name foo) (docs "bar")) (value (name bar) (docs "abc")))`,
			Want: &File{
				Enumerations: []*Enumeration{
					{
						Name: Name{"blah"},
						Docs: Docs{Text("xyz")},
						Type: Byte,
						Values: []*Value{
							{
								Name: Name{"foo"},
								Docs: Docs{Text("bar")},
							},
							{
								Name: Name{"bar"},
								Docs: Docs{Text("abc")},
							},
						},
					},
				},
			},
		},
		{
			Name: "Embedded enumeration",
			Source: `(enumeration (name blah) (docs "xyz") (type byte) (value (name foo) (docs "bar")) (value (name bar) (docs "abc")))
			         (enumeration (name two) (docs "abc") (type sint8) (value (name first) (docs "1")) (embed blah) (value (name four) (docs "4")))`,
			Want: &File{
				Enumerations: []*Enumeration{
					{
						Name: Name{"blah"},
						Docs: Docs{Text("xyz")},
						Type: Byte,
						Values: []*Value{
							{
								Name: Name{"foo"},
								Docs: Docs{Text("bar")},
							},
							{
								Name: Name{"bar"},
								Docs: Docs{Text("abc")},
							},
						},
					},
					{
						Name: Name{"two"},
						Docs: Docs{Text("abc")},
						Type: Sint8,
						Embeds: []*Enumeration{
							{
								Name: Name{"blah"},
								Docs: Docs{Text("xyz")},
								Type: Byte,
								Values: []*Value{
									{
										Name: Name{"foo"},
										Docs: Docs{Text("bar")},
									},
									{
										Name: Name{"bar"},
										Docs: Docs{Text("abc")},
									},
								},
							},
						},
						Values: []*Value{
							{
								Name: Name{"first"},
								Docs: Docs{Text("1")},
							},
							{
								Name: Name{"foo"},
								Docs: Docs{Text("bar")},
							},
							{
								Name: Name{"bar"},
								Docs: Docs{Text("abc")},
							},
							{
								Name: Name{"four"},
								Docs: Docs{Text("4")},
							},
						},
					},
				},
			},
		},
		{
			Name:   "Simple new integer",
			Source: `(integer (name blah) (docs "xyz") (type byte))`,
			Want: &File{
				NewIntegers: []*NewInteger{
					{
						Name: Name{"blah"},
						Docs: Docs{Text("xyz")},
						Type: Byte,
					},
				},
			},
		},
		{
			Name:   "Simple structure",
			Source: `(structure (name blah) (docs "xyz" "" "abc\n" "") (field (name foo) (docs "foo" "bar") (type *constant byte)))`,
			Want: &File{
				Structures: []*Structure{
					{
						Name: Name{"blah"},
						Docs: Docs{Text("xyz"), Newline{}, Text("abc")},
						Fields: []*Field{
							{
								Name: Name{"foo"},
								Docs: Docs{Text("foo"), Text(" "), Text("bar")},
								Type: &Pointer{
									Underlying: Byte,
								},
							},
						},
					},
				},
			},
		},
		{
			Name:   "Structure containing a syscall reference",
			Source: `(structure (name blah) (docs "xyz") (field (name foo) (docs "example" (reference syscalls) ". ref") (type syscalls)))`,
			Want: &File{
				Structures: []*Structure{
					{
						Name: Name{"blah"},
						Docs: Docs{Text("xyz")},
						Fields: []*Field{
							{
								Name: Name{"foo"},
								Docs: Docs{
									Text("example"),
									Text(" "),
									ReferenceText{
										&Reference{
											Name: Name{"syscalls"},
											Underlying: &Enumeration{
												Name: Name{"syscalls"},
												Type: Uint64,
											},
										},
									},
									Text(". ref"),
								},
								Type: &Reference{
									Name: Name{"syscalls"},
									Underlying: &Enumeration{
										Name: Name{"syscalls"},
										Type: Uint64,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "Padded structure",
			Source: `(structure
			             (name blah)
			             (docs "xyz")
			             (field
			                 (name foo)
			                 (docs "bar" (code "foo") "baz")
			                 (type *constant byte))
			             (field
			                 (name bar)
			                 (docs "padding")
			                 (padding 8)))`,
			Want: &File{
				Structures: []*Structure{
					{
						Name: Name{"blah"},
						Docs: Docs{Text("xyz")},
						Fields: []*Field{
							{
								Name: Name{"foo"},
								Docs: Docs{
									Text("bar"),
									Text(" "),
									CodeText("foo"),
									Text(" "),
									Text("baz"),
								},
								Type: &Pointer{
									Underlying: Byte,
								},
							},
							{
								Name: Name{"bar"},
								Docs: Docs{Text("padding")},
								Type: Padding(8),
							},
						},
					},
				},
			},
		},
		{
			Name:   "Simple structure with AST",
			Source: `(structure (name blah) (docs "xyz") (field (name foo) (docs "bar") (type *constant byte)))`,
			AST:    true,
			Want: &File{
				Structures: []*Structure{
					{
						Name: Name{"blah"},
						Node: &ast.List{
							ParenOpen: position(t, 0, 1, 1),
							Elements: []ast.Expr{
								&ast.Identifier{NamePos: position(t, 1, 1, 2), Name: "structure"},
								&ast.List{
									ParenOpen: position(t, 11, 1, 12),
									Elements: []ast.Expr{
										&ast.Identifier{NamePos: position(t, 12, 1, 13), Name: "name"},
										&ast.Identifier{NamePos: position(t, 17, 1, 18), Name: "blah"},
									},
									ParenClose: position(t, 21, 1, 22),
								},
								&ast.List{
									ParenOpen: position(t, 23, 1, 24),
									Elements: []ast.Expr{
										&ast.Identifier{NamePos: position(t, 24, 1, 25), Name: "docs"},
										&ast.String{QuotePos: position(t, 29, 1, 30), Text: `"xyz"`},
									},
									ParenClose: position(t, 34, 1, 35),
								},
								&ast.List{
									ParenOpen: position(t, 36, 1, 37),
									Elements: []ast.Expr{
										&ast.Identifier{NamePos: position(t, 37, 1, 38), Name: "field"},
										&ast.List{
											ParenOpen: position(t, 43, 1, 44),
											Elements: []ast.Expr{
												&ast.Identifier{NamePos: position(t, 44, 1, 45), Name: "name"},
												&ast.Identifier{NamePos: position(t, 49, 1, 50), Name: "foo"},
											},
											ParenClose: position(t, 52, 1, 53),
										},
										&ast.List{
											ParenOpen: position(t, 54, 1, 55),
											Elements: []ast.Expr{
												&ast.Identifier{NamePos: position(t, 55, 1, 56), Name: "docs"},
												&ast.String{QuotePos: position(t, 60, 1, 61), Text: `"bar"`},
											},
											ParenClose: position(t, 65, 1, 66),
										},
										&ast.List{
											ParenOpen: position(t, 67, 1, 68),
											Elements: []ast.Expr{
												&ast.Identifier{NamePos: position(t, 68, 1, 69), Name: "type"},
												&ast.Pointer{
													AsteriskPos: position(t, 73, 1, 74),
													NotePos:     position(t, 74, 1, 75),
													Note:        "constant",
												},
												&ast.Identifier{NamePos: position(t, 83, 1, 84), Name: "byte"},
											},
											ParenClose: position(t, 87, 1, 88),
										},
									},
									ParenClose: position(t, 88, 1, 89),
								},
							},
							ParenClose: position(t, 89, 1, 90),
						},
						Docs: Docs{Text("xyz")},
						Fields: []*Field{
							{
								Name: Name{"foo"},
								Node: &ast.List{
									ParenOpen: position(t, 36, 1, 37),
									Elements: []ast.Expr{
										&ast.Identifier{NamePos: position(t, 37, 1, 38), Name: "field"},
										&ast.List{
											ParenOpen: position(t, 43, 1, 44),
											Elements: []ast.Expr{
												&ast.Identifier{NamePos: position(t, 44, 1, 45), Name: "name"},
												&ast.Identifier{NamePos: position(t, 49, 1, 50), Name: "foo"},
											},
											ParenClose: position(t, 52, 1, 53),
										},
										&ast.List{
											ParenOpen: position(t, 54, 1, 55),
											Elements: []ast.Expr{
												&ast.Identifier{NamePos: position(t, 55, 1, 56), Name: "docs"},
												&ast.String{QuotePos: position(t, 60, 1, 61), Text: `"bar"`},
											},
											ParenClose: position(t, 65, 1, 66),
										},
										&ast.List{
											ParenOpen: position(t, 67, 1, 68),
											Elements: []ast.Expr{
												&ast.Identifier{NamePos: position(t, 68, 1, 69), Name: "type"},
												&ast.Pointer{
													AsteriskPos: position(t, 73, 1, 74),
													NotePos:     position(t, 74, 1, 75),
													Note:        "constant",
												},
												&ast.Identifier{NamePos: position(t, 83, 1, 84), Name: "byte"},
											},
											ParenClose: position(t, 87, 1, 88),
										},
									},
									ParenClose: position(t, 88, 1, 89),
								},
								Docs: Docs{Text("bar")},
								Type: &Pointer{
									Underlying: Byte,
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "Simple syscall",
			Source: `(syscall
			             (name blah)
			             (docs "xyz")
			             (arg1
			                 (name foo)
			                 (docs "bar")
			                 (type *constant byte))
			             (result1
			                 (name bar)
			                 (docs "x")
			                 (type other error)))
			         (enumeration
			             (name error)
			             (docs "")
			             (type uint8)
			             (value (name no error) (docs ""))
			             (value (name bad syscall) (docs ""))
			             (value (name illegal arg1) (docs ""))
			             (value (name illegal arg2) (docs ""))
			             (value (name illegal arg3) (docs ""))
			             (value (name illegal arg4) (docs ""))
			             (value (name illegal arg5) (docs ""))
			             (value (name illegal arg6) (docs "")))
			         (enumeration
			             (name other error)
			             (docs "")
			             (type uint8)
			             (embed error)
			             (value (name other) (docs "")))`,
			Want: &File{
				Enumerations: []*Enumeration{
					{
						Name: Name{"error"},
						Docs: Docs{},
						Type: Uint8,
						Values: []*Value{
							{
								Name: Name{"no", "error"},
								Docs: Docs{},
							},
							{
								Name: Name{"bad", "syscall"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg1"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg2"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg3"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg4"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg5"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg6"},
								Docs: Docs{},
							},
						},
					},
					{
						Name: Name{"other", "error"},
						Docs: Docs{},
						Type: Uint8,
						Embeds: []*Enumeration{
							{
								Name: Name{"error"},
								Docs: Docs{},
								Type: Uint8,
								Values: []*Value{
									{
										Name: Name{"no", "error"},
										Docs: Docs{},
									},
									{
										Name: Name{"bad", "syscall"},
										Docs: Docs{},
									},
									{
										Name: Name{"illegal", "arg1"},
										Docs: Docs{},
									},
									{
										Name: Name{"illegal", "arg2"},
										Docs: Docs{},
									},
									{
										Name: Name{"illegal", "arg3"},
										Docs: Docs{},
									},
									{
										Name: Name{"illegal", "arg4"},
										Docs: Docs{},
									},
									{
										Name: Name{"illegal", "arg5"},
										Docs: Docs{},
									},
									{
										Name: Name{"illegal", "arg6"},
										Docs: Docs{},
									},
								},
							},
						},
						Values: []*Value{
							{
								Name: Name{"no", "error"},
								Docs: Docs{},
							},
							{
								Name: Name{"bad", "syscall"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg1"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg2"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg3"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg4"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg5"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg6"},
								Docs: Docs{},
							},
							{
								Name: Name{"other"},
								Docs: Docs{},
							},
						},
					},
				},
				Syscalls: []*Syscall{
					{
						Name: Name{"blah"},
						Docs: Docs{Text("xyz")},
						Args: []*Parameter{
							{
								Name: Name{"foo"},
								Docs: Docs{Text("bar")},
								Type: &Pointer{
									Underlying: Byte,
								},
							},
						},
						Results: []*Parameter{
							{
								Name: Name{"bar"},
								Docs: Docs{Text("x")},
								Type: &Reference{
									Name: Name{"other", "error"},
									Underlying: &Enumeration{
										Name: Name{"other", "error"},
										Docs: Docs{},
										Type: Uint8,
										Embeds: []*Enumeration{
											{
												Name: Name{"error"},
												Docs: Docs{},
												Type: Uint8,
												Values: []*Value{
													{
														Name: Name{"no", "error"},
														Docs: Docs{},
													},
													{
														Name: Name{"bad", "syscall"},
														Docs: Docs{},
													},
													{
														Name: Name{"illegal", "arg1"},
														Docs: Docs{},
													},
													{
														Name: Name{"illegal", "arg2"},
														Docs: Docs{},
													},
													{
														Name: Name{"illegal", "arg3"},
														Docs: Docs{},
													},
													{
														Name: Name{"illegal", "arg4"},
														Docs: Docs{},
													},
													{
														Name: Name{"illegal", "arg5"},
														Docs: Docs{},
													},
													{
														Name: Name{"illegal", "arg6"},
														Docs: Docs{},
													},
												},
											},
										},
										Values: []*Value{
											{
												Name: Name{"no", "error"},
												Docs: Docs{},
											},
											{
												Name: Name{"bad", "syscall"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg1"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg2"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg3"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg4"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg5"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg6"},
												Docs: Docs{},
											},
											{
												Name: Name{"other"},
												Docs: Docs{},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "Sequential type reference",
			Source: `(structure (name blah) (docs "xyz") (field (name foo) (docs "bar") (type *constant byte)))
			         (structure (name two)  (docs "abc") (field (name first) (docs (reference blah)) (type *mutable blah)))`,
			Want: &File{
				Structures: []*Structure{
					{
						Name: Name{"blah"},
						Docs: Docs{Text("xyz")},
						Fields: []*Field{
							{
								Name: Name{"foo"},
								Docs: Docs{Text("bar")},
								Type: &Pointer{
									Underlying: Byte,
								},
							},
						},
					},
					{
						Name: Name{"two"},
						Docs: Docs{Text("abc")},
						Fields: []*Field{
							{
								Name: Name{"first"},
								Docs: Docs{
									ReferenceText{
										&Reference{
											Name: Name{"blah"},
											Underlying: &Structure{
												Name: Name{"blah"},
												Docs: Docs{Text("xyz")},
												Fields: []*Field{
													{
														Name: Name{"foo"},
														Docs: Docs{Text("bar")},
														Type: &Pointer{
															Underlying: Byte,
														},
													},
												},
											},
										},
									},
								},
								Type: &Pointer{
									Mutable: true,
									Underlying: &Reference{
										Name: Name{"blah"},
										Underlying: &Structure{
											Name: Name{"blah"},
											Docs: Docs{Text("xyz")},
											Fields: []*Field{
												{
													Name: Name{"foo"},
													Docs: Docs{Text("bar")},
													Type: &Pointer{
														Underlying: Byte,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "Nonsequential type reference",
			Source: `(structure (name two)  (docs "abc")
			             (field (name first) (docs "x") (type *mutable blah))
			             (field (name flags) (docs "") (type flag set))
			             (field (name pad) (docs "") (padding 4)))
			         (array (name flag set) (docs "A set of flags") (size 10) (type flags))
			         (structure (name blah) (docs (reference func)) (field (name foo) (docs "bar") (type *constant baz)))
			         (syscall (name func) (docs "xyz") (arg1 (name fd) (docs "") (type fd)) (result1 (name error) (docs "") (type error)))
			         (integer (name fd) (docs "file descriptor") (type uint32))
			         (bitfield (name flags) (docs "Flags") (type uint16) (value (name on) (docs "On")))
			         (enumeration (name baz) (docs "foo") (type byte) (value (name one) (docs "1")))
			         (enumeration (name error) (docs "") (type uint8)
			             (value (name no error) (docs ""))
			             (value (name bad syscall) (docs ""))
			             (value (name illegal arg1) (docs ""))
			             (value (name illegal arg2) (docs ""))
			             (value (name illegal arg3) (docs ""))
			             (value (name illegal arg4) (docs ""))
			             (value (name illegal arg5) (docs ""))
			             (value (name illegal arg6) (docs "")))`,
			Want: &File{
				Arrays: []*Array{
					{
						Name:  Name{"flag", "set"},
						Docs:  Docs{Text("A set of flags")},
						Count: 10,
						Type: &Reference{
							Name: Name{"flags"},
							Underlying: &Bitfield{
								Name: Name{"flags"},
								Docs: Docs{Text("Flags")},
								Type: Uint16,
								Values: []*Value{
									{
										Name: Name{"on"},
										Docs: Docs{Text("On")},
									},
								},
							},
						},
					},
				},
				Bitfields: []*Bitfield{
					{
						Name: Name{"flags"},
						Docs: Docs{Text("Flags")},
						Type: Uint16,
						Values: []*Value{
							{
								Name: Name{"on"},
								Docs: Docs{Text("On")},
							},
						},
					},
				},
				Enumerations: []*Enumeration{
					{
						Name: Name{"baz"},
						Docs: Docs{Text("foo")},
						Type: Byte,
						Values: []*Value{
							{
								Name: Name{"one"},
								Docs: Docs{Text("1")},
							},
						},
					},
					{
						Name: Name{"error"},
						Docs: Docs{},
						Type: Uint8,
						Values: []*Value{
							{
								Name: Name{"no", "error"},
								Docs: Docs{},
							},
							{
								Name: Name{"bad", "syscall"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg1"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg2"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg3"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg4"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg5"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg6"},
								Docs: Docs{},
							},
						},
					},
				},
				NewIntegers: []*NewInteger{
					{
						Name: Name{"fd"},
						Docs: Docs{Text("file descriptor")},
						Type: Uint32,
					},
				},
				Structures: []*Structure{
					{
						Name: Name{"two"},
						Docs: Docs{Text("abc")},
						Fields: []*Field{
							{
								Name: Name{"first"},
								Docs: Docs{Text("x")},
								Type: &Pointer{
									Mutable: true,
									Underlying: &Reference{
										Name: Name{"blah"},
										Underlying: &Structure{
											Name: Name{"blah"},
											Docs: Docs{
												ReferenceText{
													&Reference{
														Name: Name{"func"},
														Underlying: &SyscallReference{
															Name: Name{"func"},
														},
													},
												},
											},
											Fields: []*Field{
												{
													Name: Name{"foo"},
													Docs: Docs{Text("bar")},
													Type: &Pointer{
														Underlying: &Reference{
															Name: Name{"baz"},
															Underlying: &Enumeration{
																Name: Name{"baz"},
																Docs: Docs{Text("foo")},
																Type: Byte,
																Values: []*Value{
																	{
																		Name: Name{"one"},
																		Docs: Docs{Text("1")},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Name: Name{"flags"},
								Docs: Docs{},
								Type: &Reference{
									Name: Name{"flag", "set"},
									Underlying: &Array{
										Name:  Name{"flag", "set"},
										Docs:  Docs{Text("A set of flags")},
										Count: 10,
										Type: &Reference{
											Name: Name{"flags"},
											Underlying: &Bitfield{
												Name: Name{"flags"},
												Docs: Docs{Text("Flags")},
												Type: Uint16,
												Values: []*Value{
													{
														Name: Name{"on"},
														Docs: Docs{Text("On")},
													},
												},
											},
										},
									},
								},
							},
							{
								Name: Name{"pad"},
								Docs: Docs{},
								Type: Padding(4),
							},
						},
					},
					{
						Name: Name{"blah"},
						Docs: Docs{
							ReferenceText{
								&Reference{
									Name: Name{"func"},
									Underlying: &SyscallReference{
										Name: Name{"func"},
									},
								},
							},
						},
						Fields: []*Field{
							{
								Name: Name{"foo"},
								Docs: Docs{Text("bar")},
								Type: &Pointer{
									Underlying: &Reference{
										Name: Name{"baz"},
										Underlying: &Enumeration{
											Name: Name{"baz"},
											Docs: Docs{Text("foo")},
											Type: Byte,
											Values: []*Value{
												{
													Name: Name{"one"},
													Docs: Docs{Text("1")},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Syscalls: []*Syscall{
					{
						Name: Name{"func"},
						Docs: Docs{Text("xyz")},
						Args: []*Parameter{
							{
								Name: Name{"fd"},
								Docs: Docs{},
								Type: &Reference{
									Name: Name{"fd"},
									Underlying: &NewInteger{
										Name: Name{"fd"},
										Docs: Docs{Text("file descriptor")},
										Type: Uint32,
									},
								},
							},
						},
						Results: []*Parameter{
							{
								Name: Name{"error"},
								Docs: Docs{},
								Type: &Reference{
									Name: Name{"error"},
									Underlying: &Enumeration{
										Name: Name{"error"},
										Docs: Docs{},
										Type: Uint8,
										Values: []*Value{
											{
												Name: Name{"no", "error"},
												Docs: Docs{},
											},
											{
												Name: Name{"bad", "syscall"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg1"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg2"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg3"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg4"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg5"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg6"},
												Docs: Docs{},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "Item group",
			Source: `(structure (name two)  (docs "abc")
			             (field (name first) (docs "x") (type *mutable blah))
			             (field (name flag) (docs "") (type flags))
			             (field (name pad) (docs "") (padding 6)))
			         (structure (name blah) (docs (reference func)) (field (name foo) (docs "bar") (type *constant baz)))
			         (syscall (name func) (docs "xyz") (arg1 (name fd) (docs "") (type fd)) (result1 (name error) (docs "") (type error)))
			         (integer (name fd) (docs "file descriptor") (type uint32))
			         (group
			             (name fun features)
			             (docs "Where the good times roll.")
			             (integer fd)
			             (structure blah)
			             (syscall func)
			             (array ipv4 address)
			             (enumeration baz)
			             (bitfield flags))
			         (bitfield (name flags) (docs "Flags") (type uint16) (value (name on) (docs "On")))
			         (array (name ipv4 address) (docs "IPv4 address") (size 4) (type byte))
			         (enumeration (name baz) (docs "foo") (type byte) (value (name one) (docs "1")))
			         (enumeration (name error) (docs "") (type uint8)
			             (value (name no error) (docs ""))
			             (value (name bad syscall) (docs ""))
			             (value (name illegal arg1) (docs ""))
			             (value (name illegal arg2) (docs ""))
			             (value (name illegal arg3) (docs ""))
			             (value (name illegal arg4) (docs ""))
			             (value (name illegal arg5) (docs ""))
			             (value (name illegal arg6) (docs "")))`,
			Want: &File{
				Arrays: []*Array{
					{
						Name: Name{"ipv4", "address"},
						Docs: Docs{Text("IPv4 address")},
						Groups: []Name{
							{"fun", "features"},
						},
						Count: 4,
						Type:  Byte,
					},
				},
				Bitfields: []*Bitfield{
					{
						Name:   Name{"flags"},
						Docs:   Docs{Text("Flags")},
						Groups: []Name{{"fun", "features"}},
						Type:   Uint16,
						Values: []*Value{
							{
								Name: Name{"on"},
								Docs: Docs{Text("On")},
							},
						},
					},
				},
				Enumerations: []*Enumeration{
					{
						Name:   Name{"baz"},
						Docs:   Docs{Text("foo")},
						Groups: []Name{{"fun", "features"}},
						Type:   Byte,
						Values: []*Value{
							{
								Name: Name{"one"},
								Docs: Docs{Text("1")},
							},
						},
					},
					{
						Name: Name{"error"},
						Docs: Docs{},
						Type: Uint8,
						Values: []*Value{
							{
								Name: Name{"no", "error"},
								Docs: Docs{},
							},
							{
								Name: Name{"bad", "syscall"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg1"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg2"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg3"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg4"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg5"},
								Docs: Docs{},
							},
							{
								Name: Name{"illegal", "arg6"},
								Docs: Docs{},
							},
						},
					},
				},
				NewIntegers: []*NewInteger{
					{
						Name:   Name{"fd"},
						Docs:   Docs{Text("file descriptor")},
						Groups: []Name{{"fun", "features"}},
						Type:   Uint32,
					},
				},
				Structures: []*Structure{
					{
						Name: Name{"two"},
						Docs: Docs{Text("abc")},
						Fields: []*Field{
							{
								Name: Name{"first"},
								Docs: Docs{Text("x")},
								Type: &Pointer{
									Mutable: true,
									Underlying: &Reference{
										Name: Name{"blah"},
										Underlying: &Structure{
											Name: Name{"blah"},
											Docs: Docs{
												ReferenceText{
													&Reference{
														Name: Name{"func"},
														Underlying: &SyscallReference{
															Name: Name{"func"},
														},
													},
												},
											},
											Groups: []Name{{"fun", "features"}},
											Fields: []*Field{
												{
													Name: Name{"foo"},
													Docs: Docs{Text("bar")},
													Type: &Pointer{
														Underlying: &Reference{
															Name: Name{"baz"},
															Underlying: &Enumeration{
																Name:   Name{"baz"},
																Docs:   Docs{Text("foo")},
																Groups: []Name{{"fun", "features"}},
																Type:   Byte,
																Values: []*Value{
																	{
																		Name: Name{"one"},
																		Docs: Docs{Text("1")},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Name: Name{"flag"},
								Docs: Docs{},
								Type: &Reference{
									Name: Name{"flags"},
									Underlying: &Bitfield{
										Name:   Name{"flags"},
										Docs:   Docs{Text("Flags")},
										Groups: []Name{{"fun", "features"}},
										Type:   Uint16,
										Values: []*Value{
											{
												Name: Name{"on"},
												Docs: Docs{Text("On")},
											},
										},
									},
								},
							},
							{
								Name: Name{"pad"},
								Docs: Docs{},
								Type: Padding(6),
							},
						},
					},
					{
						Name: Name{"blah"},
						Docs: Docs{
							ReferenceText{
								&Reference{
									Name: Name{"func"},
									Underlying: &SyscallReference{
										Name: Name{"func"},
									},
								},
							},
						},
						Groups: []Name{{"fun", "features"}},
						Fields: []*Field{
							{
								Name: Name{"foo"},
								Docs: Docs{Text("bar")},
								Type: &Pointer{
									Underlying: &Reference{
										Name: Name{"baz"},
										Underlying: &Enumeration{
											Name:   Name{"baz"},
											Docs:   Docs{Text("foo")},
											Groups: []Name{{"fun", "features"}},
											Type:   Byte,
											Values: []*Value{
												{
													Name: Name{"one"},
													Docs: Docs{Text("1")},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Syscalls: []*Syscall{
					{
						Name:   Name{"func"},
						Docs:   Docs{Text("xyz")},
						Groups: []Name{{"fun", "features"}},
						Args: []*Parameter{
							{
								Name: Name{"fd"},
								Docs: Docs{},
								Type: &Reference{
									Name: Name{"fd"},
									Underlying: &NewInteger{
										Name:   Name{"fd"},
										Docs:   Docs{Text("file descriptor")},
										Groups: []Name{{"fun", "features"}},
										Type:   Uint32,
									},
								},
							},
						},
						Results: []*Parameter{
							{
								Name: Name{"error"},
								Docs: Docs{},
								Type: &Reference{
									Name: Name{"error"},
									Underlying: &Enumeration{
										Name: Name{"error"},
										Docs: Docs{},
										Type: Uint8,
										Values: []*Value{
											{
												Name: Name{"no", "error"},
												Docs: Docs{},
											},
											{
												Name: Name{"bad", "syscall"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg1"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg2"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg3"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg4"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg5"},
												Docs: Docs{},
											},
											{
												Name: Name{"illegal", "arg6"},
												Docs: Docs{},
											},
										},
									},
								},
							},
						},
					},
				},
				Groups: []*Group{
					{
						Name: Name{"fun", "features"},
						Docs: Docs{Text("Where the good times roll.")},
						List: []*ItemReference{
							{
								Type: "integer",
								Name: Name{"fd"},
								Underlying: &NewInteger{
									Name:   Name{"fd"},
									Docs:   Docs{Text("file descriptor")},
									Groups: []Name{{"fun", "features"}},
									Type:   Uint32,
								},
							},
							{
								Type: "structure",
								Name: Name{"blah"},
								Underlying: &Structure{
									Name: Name{"blah"},
									Docs: Docs{
										ReferenceText{
											&Reference{
												Name: Name{"func"},
												Underlying: &SyscallReference{
													Name: Name{"func"},
												},
											},
										},
									},
									Groups: []Name{{"fun", "features"}},
									Fields: []*Field{
										{
											Name: Name{"foo"},
											Docs: Docs{Text("bar")},
											Type: &Pointer{
												Underlying: &Reference{
													Name: Name{"baz"},
													Underlying: &Enumeration{
														Name:   Name{"baz"},
														Docs:   Docs{Text("foo")},
														Groups: []Name{{"fun", "features"}},
														Type:   Byte,
														Values: []*Value{
															{
																Name: Name{"one"},
																Docs: Docs{Text("1")},
															},
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Type: "syscall",
								Name: Name{"func"},
								Underlying: &Syscall{
									Name:   Name{"func"},
									Docs:   Docs{Text("xyz")},
									Groups: []Name{{"fun", "features"}},
									Args: []*Parameter{
										{
											Name: Name{"fd"},
											Docs: Docs{},
											Type: &Reference{
												Name: Name{"fd"},
												Underlying: &NewInteger{
													Name:   Name{"fd"},
													Docs:   Docs{Text("file descriptor")},
													Groups: []Name{{"fun", "features"}},
													Type:   Uint32,
												},
											},
										},
									},
									Results: []*Parameter{
										{
											Name: Name{"error"},
											Docs: Docs{},
											Type: &Reference{
												Name: Name{"error"},
												Underlying: &Enumeration{
													Name: Name{"error"},
													Docs: Docs{},
													Type: Uint8,
													Values: []*Value{
														{
															Name: Name{"no", "error"},
															Docs: Docs{},
														},
														{
															Name: Name{"bad", "syscall"},
															Docs: Docs{},
														},
														{
															Name: Name{"illegal", "arg1"},
															Docs: Docs{},
														},
														{
															Name: Name{"illegal", "arg2"},
															Docs: Docs{},
														},
														{
															Name: Name{"illegal", "arg3"},
															Docs: Docs{},
														},
														{
															Name: Name{"illegal", "arg4"},
															Docs: Docs{},
														},
														{
															Name: Name{"illegal", "arg5"},
															Docs: Docs{},
														},
														{
															Name: Name{"illegal", "arg6"},
															Docs: Docs{},
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Type: "array",
								Name: Name{"ipv4", "address"},
								Underlying: &Array{
									Name: Name{"ipv4", "address"},
									Docs: Docs{Text("IPv4 address")},
									Groups: []Name{
										{"fun", "features"},
									},
									Count: 4,
									Type:  Byte,
								},
							},
							{
								Type: "enumeration",
								Name: Name{"baz"},
								Underlying: &Enumeration{
									Name:   Name{"baz"},
									Docs:   Docs{Text("foo")},
									Groups: []Name{{"fun", "features"}},
									Type:   Byte,
									Values: []*Value{
										{
											Name: Name{"one"},
											Docs: Docs{Text("1")},
										},
									},
								},
							},
							{
								Type: "bitfield",
								Name: Name{"flags"},
								Underlying: &Bitfield{
									Name:   Name{"flags"},
									Docs:   Docs{Text("Flags")},
									Groups: []Name{{"fun", "features"}},
									Type:   Uint16,
									Values: []*Value{
										{
											Name: Name{"on"},
											Docs: Docs{Text("On")},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			f, err := parser.ParseFile("test.plan", test.Source)
			if err != nil {
				t.Fatalf("Failed to parse file: %v", err)
			}

			got, err := Interpret("test.plan", f, X86_64)
			if err != nil {
				t.Fatalf("Interpret(): unexpected error %v", err)
			}

			// Reproducing the AST by hand can be verbose
			// and tedious, so we have the option to ignore
			// it by omitting it from test.Want and removing
			// it from got.
			if !test.AST {
				got.DropAST()
			}

			if !reflect.DeepEqual(got, test.Want) {
				// Encoding the values in JSON makes the error
				// message more useful and legible.
				gotJSON, err := json.MarshalIndent(got, "", "\t")
				if err != nil {
					t.Fatal(err)
				}

				wantJSON, err := json.MarshalIndent(test.Want, "", "\t")
				if err != nil {
					t.Fatal(err)
				}

				t.Fatalf("Interpret():\n%s", diff.Format(string(gotJSON), string(wantJSON)))
			}
		})
	}
}

func TestInterpreterErrors(t *testing.T) {
	tests := []struct {
		Name   string
		Source string
		Want   string
	}{
		// General errors.
		{
			Name:   "invalid top-level definition",
			Source: `()`,
			Want:   `test.plan:1:1: invalid top-level definition: empty definition`,
		},
		{
			Name:   "unrecognised top-level definition",
			Source: `(foo bar)`,
			Want:   `test.plan:1:2: unrecognised definition kind "foo"`,
		},
		// Array errors.
		{
			Name:   "array with identifier element",
			Source: `(array foo)`,
			Want:   `test.plan:1:8: invalid array: expected a list, found identifier`,
		},
		{
			Name:   "array with empty definition",
			Source: `(array ())`,
			Want:   `test.plan:1:8: empty definition`,
		},
		{
			Name:   "array with bad definition",
			Source: `(array ("foo"))`,
			Want:   `test.plan:1:9: definition kind must be an identifier, found string`,
		},
		{
			Name:   "array with short definition",
			Source: `(array (foo))`,
			Want:   `test.plan:1:9: definition must have at least one field, found none`,
		},
		{
			Name:   "array with unrecognised definition",
			Source: `(array (foo bar))`,
			Want:   `test.plan:1:9: unrecognised array definition kind "foo"`,
		},
		{
			Name:   "array with invalid name",
			Source: `(array (name "bar"))`,
			Want:   `test.plan:1:14: invalid array name: expected an identifier, found string`,
		},
		{
			Name:   "array with duplicate name",
			Source: `(array (name bar) (name baz))`,
			Want:   `test.plan:1:20: invalid array definition: name already defined`,
		},
		{
			Name:   "array with invalid docs",
			Source: `(array (docs bar))`,
			Want:   `test.plan:1:14: invalid array docs: expected a string or formatting expression, found identifier`,
		},
		{
			Name:   "array with duplicate docs",
			Source: `(array (docs "bar") (docs "baz"))`,
			Want:   `test.plan:1:22: invalid array definition: docs already defined`,
		},
		{
			Name:   "array with invalid size",
			Source: `(array (size bar))`,
			Want:   `test.plan:1:14: invalid array size definition: expected a number, found identifier`,
		},
		{
			Name:   "array with excessive size",
			Source: `(array (size 99999999999999999999999999))`,
			Want:   `test.plan:1:14: invalid array size definition: invalid size: value out of range`,
		},
		{
			Name:   "array with zero size",
			Source: `(array (size 0))`,
			Want:   `test.plan:1:14: invalid array size definition: size must be larger than zero`,
		},
		{
			Name:   "array with extra field after size",
			Source: `(array (size 1 bar))`,
			Want:   `test.plan:1:16: invalid array size definition: unexpected identifier after size`,
		},
		{
			Name:   "array with duplicate size",
			Source: `(array (size 1) (size 2))`,
			Want:   `test.plan:1:18: invalid array definition: size already defined`,
		},
		{
			Name:   "array with invalid type",
			Source: `(array (type ()))`,
			Want:   `test.plan:1:14: invalid array element type: expected a type definition, found list`,
		},
		{
			Name:   "array with duplicate type",
			Source: `(array (type uint16) (type byte))`,
			Want:   `test.plan:1:23: invalid array definition: type already defined`,
		},
		{
			Name:   "array with missing name",
			Source: `(array (docs "blah") (size 1) (type byte))`,
			Want:   `test.plan:1:1: array has no name definition`,
		},
		{
			Name:   "array with missing docs",
			Source: `(array (name blah) (size 1) (type byte))`,
			Want:   `test.plan:1:1: array has no docs definition`,
		},
		{
			Name:   "array with missing size",
			Source: `(array (name blah) (docs "foo") (type byte))`,
			Want:   `test.plan:1:1: array has no size definition`,
		},
		{
			Name:   "array with missing type",
			Source: `(array (name blah) (docs "foo") (size 1))`,
			Want:   `test.plan:1:1: array has no type definition`,
		},
		{
			Name: "duplicate array",
			Source: `(array (name blah) (docs "xyz") (size 1) (type byte))
			         (array (name blah) (docs "abc") (size 2) (type *constant uint32))`,
			Want: `test.plan:2:13: type "blah" is already defined`,
		},
		// Bitfield errors.
		{
			Name:   "bitfield with identifier element",
			Source: `(bitfield foo)`,
			Want:   `test.plan:1:11: invalid bitfield: expected a list, found identifier`,
		},
		{
			Name:   "bitfield with empty definition",
			Source: `(bitfield ())`,
			Want:   `test.plan:1:11: empty definition`,
		},
		{
			Name:   "bitfield with bad definition",
			Source: `(bitfield ("foo"))`,
			Want:   `test.plan:1:12: definition kind must be an identifier, found string`,
		},
		{
			Name:   "bitfield with short definition",
			Source: `(bitfield (foo))`,
			Want:   `test.plan:1:12: definition must have at least one field, found none`,
		},
		{
			Name:   "bitfield with unrecognised definition",
			Source: `(bitfield (foo bar))`,
			Want:   `test.plan:1:12: unrecognised bitfield definition kind "foo"`,
		},
		{
			Name:   "bitfield with invalid name",
			Source: `(bitfield (name "bar"))`,
			Want:   `test.plan:1:17: invalid bitfield name: expected an identifier, found string`,
		},
		{
			Name:   "bitfield with duplicate name",
			Source: `(bitfield (name bar) (name baz))`,
			Want:   `test.plan:1:23: invalid bitfield definition: name already defined`,
		},
		{
			Name:   "bitfield with invalid docs",
			Source: `(bitfield (docs bar))`,
			Want:   `test.plan:1:17: invalid bitfield docs: expected a string or formatting expression, found identifier`,
		},
		{
			Name:   "bitfield with invalid docs formatting expression",
			Source: `(bitfield (docs (bar)))`,
			Want:   `test.plan:1:18: invalid bitfield docs: invalid formatting expression: definition must have at least one field, found none`,
		},
		{
			Name:   "bitfield with unsupported docs formatting expression",
			Source: `(bitfield (docs (bar foo)))`,
			Want:   `test.plan:1:18: invalid bitfield docs: unrecognised formatting expression kind "bar"`,
		},
		{
			Name:   "bitfield with invalid docs code formatting expression",
			Source: `(bitfield (docs (code foo)))`,
			Want:   `test.plan:1:23: invalid bitfield docs: invalid formatting expression: expected a string, found identifier`,
		},
		{
			Name:   "bitfield with invalid docs reference formatting expression",
			Source: `(bitfield (docs (reference "baz") "bar"))`,
			Want:   `test.plan:1:28: invalid bitfield docs: invalid reference formatting expression: invalid type reference: expected an identifier, found string`,
		},
		{
			Name:   "bitfield with duplicate docs",
			Source: `(bitfield (docs "bar") (docs "baz"))`,
			Want:   `test.plan:1:25: invalid bitfield definition: docs already defined`,
		},
		{
			Name:   "bitfield with invalid type",
			Source: `(bitfield (type "foo"))`,
			Want:   `test.plan:1:17: invalid bitfield type: expected a type definition, found string`,
		},
		{
			Name:   "bitfield with complex type",
			Source: `(bitfield (type *constant byte))`,
			Want:   `test.plan:1:17: invalid bitfield type: must be an integer type, found *constant byte`,
		},
		{
			Name:   "bitfield with duplicate type",
			Source: `(bitfield (type byte) (type sint8))`,
			Want:   `test.plan:1:24: invalid bitfield definition: type already defined`,
		},
		{
			Name:   "bitfield with invalid value",
			Source: `(bitfield (value bar))`,
			Want:   `test.plan:1:18: invalid value definition: expected a list, found identifier`,
		},
		{
			Name:   "bitfield with empty value element",
			Source: `(bitfield (value ()))`,
			Want:   `test.plan:1:18: invalid value definition: empty definition`,
		},
		{
			Name:   "bitfield with bad value element",
			Source: `(bitfield (value ("foo")))`,
			Want:   `test.plan:1:19: invalid value definition: definition kind must be an identifier, found string`,
		},
		{
			Name:   "bitfield with short value element",
			Source: `(bitfield (value (foo)))`,
			Want:   `test.plan:1:19: invalid value definition: definition must have at least one field, found none`,
		},
		{
			Name:   "bitfield with unrecognised value element",
			Source: `(bitfield (value (foo bar)))`,
			Want:   `test.plan:1:19: unrecognised value definition kind "foo"`,
		},
		{
			Name:   "bitfield with invalid value name",
			Source: `(bitfield (value (name 123)))`,
			Want:   `test.plan:1:24: invalid value name: expected an identifier, found number`,
		},
		{
			Name:   "bitfield with duplicate value name",
			Source: `(bitfield (value (name bar) (name baz)))`,
			Want:   `test.plan:1:30: invalid value definition: name already defined`,
		},
		{
			Name:   "bitfield with invalid value docs",
			Source: `(bitfield (value (docs bar)))`,
			Want:   `test.plan:1:24: invalid value docs: expected a string or formatting expression, found identifier`,
		},
		{
			Name:   "bitfield with duplicate value docs",
			Source: `(bitfield (value (docs "bar") (docs "baz")))`,
			Want:   `test.plan:1:32: invalid value definition: docs already defined`,
		},
		{
			Name:   "bitfield with value missing name",
			Source: `(bitfield (value (docs "bar")))`,
			Want:   `test.plan:1:11: value has no name definition`,
		},
		{
			Name:   "bitfield with value missing docs",
			Source: `(bitfield (value (name bar)))`,
			Want:   `test.plan:1:11: value has no docs definition`,
		},
		{
			Name:   "bitfield with duplicate value",
			Source: `(bitfield (type byte) (value (name bar ber) (docs "baz")) (value (name bar ber) (docs "foo")))`,
			Want:   `test.plan:1:59: value "bar ber" already defined at test.plan:1:23`,
		},
		{
			Name:   "bitfield with missing name",
			Source: `(bitfield (type byte) (docs "blah") (value (name foo) (docs "bar")))`,
			Want:   `test.plan:1:1: bitfield has no name definition`,
		},
		{
			Name:   "bitfield with missing docs",
			Source: `(bitfield (name blah) (type byte) (value (name foo) (docs "bar")))`,
			Want:   `test.plan:1:1: bitfield has no docs definition`,
		},
		{
			Name:   "bitfield with missing type",
			Source: `(bitfield (name blah) (docs "abc") (value (name foo) (docs "bar")))`,
			Want:   `test.plan:1:1: bitfield has no type definition`,
		},
		{
			Name:   "bitfield with missing values",
			Source: `(bitfield (name blah) (docs "foo") (type byte))`,
			Want:   `test.plan:1:1: bitfield has no value definitions`,
		},
		{
			Name: "bitfield with too many values",
			Source: `(bitfield (name blah) (docs "foo") (type uint8)` + func() string {
				var buf strings.Builder
				for i := 0; i < 9; i++ {
					fmt.Fprintf(&buf, `(value (name foo%d) (docs "bar"))`, i+1)
				}
				return buf.String()
			}() + `)`,
			Want: `test.plan:1:1: bitfield has 9 values, which exceeds capacity of uint8 (max 8)`,
		},
		{
			Name: "duplicate bitfield",
			Source: `(bitfield (name blah) (docs "xyz") (type byte) (value (name foo) (docs "bar")))
			         (bitfield (name blah) (docs "abc") (type sint8) (value (name this) (docs "some")))`,
			Want: `test.plan:2:13: type "blah" is already defined`,
		},
		// Enumeration errors.
		{
			Name:   "enumeration with identifier element",
			Source: `(enumeration foo)`,
			Want:   `test.plan:1:14: invalid enumeration: expected a list, found identifier`,
		},
		{
			Name:   "enumeration with empty definition",
			Source: `(enumeration ())`,
			Want:   `test.plan:1:14: empty definition`,
		},
		{
			Name:   "enumeration with bad definition",
			Source: `(enumeration ("foo"))`,
			Want:   `test.plan:1:15: definition kind must be an identifier, found string`,
		},
		{
			Name:   "enumeration with short definition",
			Source: `(enumeration (foo))`,
			Want:   `test.plan:1:15: definition must have at least one field, found none`,
		},
		{
			Name:   "enumeration with unrecognised definition",
			Source: `(enumeration (foo bar))`,
			Want:   `test.plan:1:15: unrecognised enumeration definition kind "foo"`,
		},
		{
			Name:   "enumeration with invalid name",
			Source: `(enumeration (name "bar"))`,
			Want:   `test.plan:1:20: invalid enumeration name: expected an identifier, found string`,
		},
		{
			Name:   "enumeration with duplicate name",
			Source: `(enumeration (name bar) (name baz))`,
			Want:   `test.plan:1:26: invalid enumeration definition: name already defined`,
		},
		{
			Name:   "enumeration with invalid docs",
			Source: `(enumeration (docs bar))`,
			Want:   `test.plan:1:20: invalid enumeration docs: expected a string or formatting expression, found identifier`,
		},
		{
			Name:   "enumeration with invalid docs formatting expression",
			Source: `(enumeration (docs (bar)))`,
			Want:   `test.plan:1:21: invalid enumeration docs: invalid formatting expression: definition must have at least one field, found none`,
		},
		{
			Name:   "enumeration with unsupported docs formatting expression",
			Source: `(enumeration (docs (bar foo)))`,
			Want:   `test.plan:1:21: invalid enumeration docs: unrecognised formatting expression kind "bar"`,
		},
		{
			Name:   "enumeration with invalid docs code formatting expression",
			Source: `(enumeration (docs (code foo)))`,
			Want:   `test.plan:1:26: invalid enumeration docs: invalid formatting expression: expected a string, found identifier`,
		},
		{
			Name:   "enumeration with invalid docs reference formatting expression",
			Source: `(enumeration (docs (reference "baz") "bar"))`,
			Want:   `test.plan:1:31: invalid enumeration docs: invalid reference formatting expression: invalid type reference: expected an identifier, found string`,
		},
		{
			Name:   "enumeration with duplicate docs",
			Source: `(enumeration (docs "bar") (docs "baz"))`,
			Want:   `test.plan:1:28: invalid enumeration definition: docs already defined`,
		},
		{
			Name:   "enumeration with invalid type",
			Source: `(enumeration (type "foo"))`,
			Want:   `test.plan:1:20: invalid enumeration type: expected a type definition, found string`,
		},
		{
			Name:   "enumeration with complex type",
			Source: `(enumeration (type *constant byte))`,
			Want:   `test.plan:1:20: invalid enumeration type: must be an integer type, found *constant byte`,
		},
		{
			Name:   "enumeration with duplicate type",
			Source: `(enumeration (type byte) (type sint8))`,
			Want:   `test.plan:1:27: invalid enumeration definition: type already defined`,
		},
		{
			Name:   "enumeration with invalid value",
			Source: `(enumeration (value bar))`,
			Want:   `test.plan:1:21: invalid value definition: expected a list, found identifier`,
		},
		{
			Name:   "enumeration with empty value element",
			Source: `(enumeration (value ()))`,
			Want:   `test.plan:1:21: invalid value definition: empty definition`,
		},
		{
			Name:   "enumeration with bad value element",
			Source: `(enumeration (value ("foo")))`,
			Want:   `test.plan:1:22: invalid value definition: definition kind must be an identifier, found string`,
		},
		{
			Name:   "enumeration with short value element",
			Source: `(enumeration (value (foo)))`,
			Want:   `test.plan:1:22: invalid value definition: definition must have at least one field, found none`,
		},
		{
			Name:   "enumeration with unrecognised value element",
			Source: `(enumeration (value (foo bar)))`,
			Want:   `test.plan:1:22: unrecognised value definition kind "foo"`,
		},
		{
			Name:   "enumeration with invalid value name",
			Source: `(enumeration (value (name 123)))`,
			Want:   `test.plan:1:27: invalid value name: expected an identifier, found number`,
		},
		{
			Name:   "enumeration with duplicate value name",
			Source: `(enumeration (value (name bar) (name baz)))`,
			Want:   `test.plan:1:33: invalid value definition: name already defined`,
		},
		{
			Name:   "enumeration with invalid value docs",
			Source: `(enumeration (value (docs bar)))`,
			Want:   `test.plan:1:27: invalid value docs: expected a string or formatting expression, found identifier`,
		},
		{
			Name:   "enumeration with duplicate value docs",
			Source: `(enumeration (value (docs "bar") (docs "baz")))`,
			Want:   `test.plan:1:35: invalid value definition: docs already defined`,
		},
		{
			Name:   "enumeration with value missing name",
			Source: `(enumeration (value (docs "bar")))`,
			Want:   `test.plan:1:14: value has no name definition`,
		},
		{
			Name:   "enumeration with value missing docs",
			Source: `(enumeration (value (name bar)))`,
			Want:   `test.plan:1:14: value has no docs definition`,
		},
		{
			Name:   "enumeration with duplicate value",
			Source: `(enumeration (type byte) (value (name bar ber) (docs "baz")) (value (name bar ber) (docs "foo")))`,
			Want:   `test.plan:1:62: value "bar ber" already defined at test.plan:1:26`,
		},
		{
			Name:   "enumeration with invalid embedding syntax",
			Source: `(enumeration (embed "foo"))`,
			Want:   `test.plan:1:21: invalid enumeration embedding: invalid type reference: expected an identifier, found string`,
		},
		{
			Name:   "enumeration with invalid embedding type",
			Source: `(enumeration (embed byte))`,
			Want:   `test.plan:1:21: invalid embedded type: expected an enumeration, found byte`,
		},
		{
			Name:   "enumeration with undefined embedding type",
			Source: `(enumeration (embed foo))`,
			Want:   `test.plan:1:21: invalid embedded type: type "foo" has not yet been defined`,
		},
		{
			Name: "enumeration with invalid embedding reference type",
			Source: `(structure (name foo) (docs "abc") (field (name bar) (docs "def") (type sint64)))
			         (enumeration (embed foo))`,
			Want: `test.plan:2:33: invalid embedded type: expected an enumeration, found structure foo`,
		},
		{
			Name: "enumeration with clashing embedding value",
			Source: `(enumeration (name foo) (docs "abc") (type byte) (value (name bar) (docs "def")))
			         (enumeration (value (name bar) (docs "xyz")) (embed foo))`,
			Want: `test.plan:2:65: embedded value "bar" already defined at test.plan:2:26`,
		},
		{
			Name: "enumeration with value clashing with embedding value",
			Source: `(enumeration (name foo) (docs "abc") (type byte) (value (name bar) (docs "def")))
			         (enumeration (embed foo) (value (name bar) (docs "xyz")))`,
			Want: `test.plan:2:38: value "bar" already defined at test.plan:1:50`,
		},
		{
			Name:   "enumeration with missing name",
			Source: `(enumeration (type byte) (docs "blah") (value (name foo) (docs "bar")))`,
			Want:   `test.plan:1:1: enumeration has no name definition`,
		},
		{
			Name:   "enumeration with missing docs",
			Source: `(enumeration (name blah) (type byte) (value (name foo) (docs "bar")))`,
			Want:   `test.plan:1:1: enumeration has no docs definition`,
		},
		{
			Name:   "enumeration with missing type",
			Source: `(enumeration (name blah) (docs "abc") (value (name foo) (docs "bar")))`,
			Want:   `test.plan:1:1: enumeration has no type definition`,
		},
		{
			Name:   "enumeration with missing values",
			Source: `(enumeration (name blah) (docs "foo") (type byte))`,
			Want:   `test.plan:1:1: enumeration has no value definitions`,
		},
		{
			Name: "enumeration with too many values",
			Source: `(enumeration (name blah) (docs "foo") (type sint8)` + func() string {
				var buf strings.Builder
				for i := 0; i < 128; i++ {
					fmt.Fprintf(&buf, `(value (name foo%d) (docs "bar"))`, i+1)
				}
				return buf.String()
			}() + `)`,
			Want: `test.plan:1:1: enumeration has 128 values, which exceeds capacity of sint8 (max 127)`,
		},
		{
			Name: "duplicate enumeration",
			Source: `(enumeration (name blah) (docs "xyz") (type byte) (value (name foo) (docs "bar")))
			         (enumeration (name blah) (docs "abc") (type sint8) (value (name this) (docs "some")))`,
			Want: `test.plan:2:13: type "blah" is already defined`,
		},
		// NewInteger errors.
		{
			Name:   "new integer with identifier element",
			Source: `(integer foo)`,
			Want:   `test.plan:1:10: invalid integer: expected a list, found identifier`,
		},
		{
			Name:   "new integer with empty definition",
			Source: `(integer ())`,
			Want:   `test.plan:1:10: empty definition`,
		},
		{
			Name:   "new integer with bad definition",
			Source: `(integer ("foo"))`,
			Want:   `test.plan:1:11: definition kind must be an identifier, found string`,
		},
		{
			Name:   "new integer with short definition",
			Source: `(integer (foo))`,
			Want:   `test.plan:1:11: definition must have at least one field, found none`,
		},
		{
			Name:   "new integer with unrecognised definition",
			Source: `(integer (foo bar))`,
			Want:   `test.plan:1:11: unrecognised integer definition kind "foo"`,
		},
		{
			Name:   "new integer with invalid name",
			Source: `(integer (name "bar"))`,
			Want:   `test.plan:1:16: invalid integer name: expected an identifier, found string`,
		},
		{
			Name:   "new integer with duplicate name",
			Source: `(integer (name bar) (name baz))`,
			Want:   `test.plan:1:22: invalid integer definition: name already defined`,
		},
		{
			Name:   "new integer with invalid docs",
			Source: `(integer (docs bar))`,
			Want:   `test.plan:1:16: invalid integer docs: expected a string or formatting expression, found identifier`,
		},
		{
			Name:   "new integer with invalid docs formatting expression",
			Source: `(integer (docs (bar)))`,
			Want:   `test.plan:1:17: invalid integer docs: invalid formatting expression: definition must have at least one field, found none`,
		},
		{
			Name:   "new integer with unsupported docs formatting expression",
			Source: `(integer (docs (bar foo)))`,
			Want:   `test.plan:1:17: invalid integer docs: unrecognised formatting expression kind "bar"`,
		},
		{
			Name:   "new integer with invalid docs code formatting expression",
			Source: `(integer (docs (code foo)))`,
			Want:   `test.plan:1:22: invalid integer docs: invalid formatting expression: expected a string, found identifier`,
		},
		{
			Name:   "new integer with invalid docs reference formatting expression",
			Source: `(integer (docs (reference "baz") "bar"))`,
			Want:   `test.plan:1:27: invalid integer docs: invalid reference formatting expression: invalid type reference: expected an identifier, found string`,
		},
		{
			Name:   "new integer with duplicate docs",
			Source: `(integer (docs "bar") (docs "baz"))`,
			Want:   `test.plan:1:24: invalid integer definition: docs already defined`,
		},
		{
			Name:   "new integer with invalid type",
			Source: `(integer (type "foo"))`,
			Want:   `test.plan:1:16: invalid integer type: expected a type definition, found string`,
		},
		{
			Name:   "new integer with complex type",
			Source: `(integer (type *constant byte))`,
			Want:   `test.plan:1:16: invalid integer type: must be an integer type, found *constant byte`,
		},
		{
			Name:   "new integer with duplicate type",
			Source: `(integer (type byte) (type sint8))`,
			Want:   `test.plan:1:23: invalid integer definition: type already defined`,
		},
		{
			Name:   "new integer with missing name",
			Source: `(integer (type byte) (docs "blah"))`,
			Want:   `test.plan:1:1: integer has no name definition`,
		},
		{
			Name:   "new integer with missing docs",
			Source: `(integer (name blah) (type byte))`,
			Want:   `test.plan:1:1: integer has no docs definition`,
		},
		{
			Name:   "new integer with missing type",
			Source: `(integer (name blah) (docs "abc"))`,
			Want:   `test.plan:1:1: integer has no type definition`,
		},
		{
			Name: "duplicate integer",
			Source: `(integer (name blah) (docs "xyz") (type byte))
			         (integer (name blah) (docs "abc") (type sint8))`,
			Want: `test.plan:2:13: type "blah" is already defined`,
		},
		// Structure errors.
		{
			Name:   "structure with identifier element",
			Source: `(structure foo)`,
			Want:   `test.plan:1:12: invalid structure: expected a list, found identifier`,
		},
		{
			Name:   "structure with empty definition",
			Source: `(structure ())`,
			Want:   `test.plan:1:12: empty definition`,
		},
		{
			Name:   "structure with bad definition",
			Source: `(structure ("foo"))`,
			Want:   `test.plan:1:13: definition kind must be an identifier, found string`,
		},
		{
			Name:   "structure with short definition",
			Source: `(structure (foo))`,
			Want:   `test.plan:1:13: definition must have at least one field, found none`,
		},
		{
			Name:   "structure with unrecognised definition",
			Source: `(structure (foo bar))`,
			Want:   `test.plan:1:13: unrecognised structure definition kind "foo"`,
		},
		{
			Name:   "structure with invalid name",
			Source: `(structure (name "bar"))`,
			Want:   `test.plan:1:18: invalid structure name: expected an identifier, found string`,
		},
		{
			Name:   "structure with duplicate name",
			Source: `(structure (name bar) (name baz))`,
			Want:   `test.plan:1:24: invalid structure definition: name already defined`,
		},
		{
			Name:   "structure with invalid docs",
			Source: `(structure (docs bar))`,
			Want:   `test.plan:1:18: invalid structure docs: expected a string or formatting expression, found identifier`,
		},
		{
			Name:   "structure with duplicate docs",
			Source: `(structure (docs "bar") (docs "baz"))`,
			Want:   `test.plan:1:26: invalid structure definition: docs already defined`,
		},
		{
			Name:   "structure with invalid field",
			Source: `(structure (field bar))`,
			Want:   `test.plan:1:19: invalid field definition: expected a list, found identifier`,
		},
		{
			Name:   "structure with empty field element",
			Source: `(structure (field ()))`,
			Want:   `test.plan:1:19: invalid field definition: empty definition`,
		},
		{
			Name:   "structure with bad field element",
			Source: `(structure (field ("foo")))`,
			Want:   `test.plan:1:20: invalid field definition: definition kind must be an identifier, found string`,
		},
		{
			Name:   "structure with short field element",
			Source: `(structure (field (foo)))`,
			Want:   `test.plan:1:20: invalid field definition: definition must have at least one field, found none`,
		},
		{
			Name:   "structure with unrecognised field element",
			Source: `(structure (field (foo bar)))`,
			Want:   `test.plan:1:20: unrecognised field definition kind "foo"`,
		},
		{
			Name:   "structure with invalid field name",
			Source: `(structure (field (name 123)))`,
			Want:   `test.plan:1:25: invalid field name: expected an identifier, found number`,
		},
		{
			Name:   "structure with duplicate field name",
			Source: `(structure (field (name bar) (name baz)))`,
			Want:   `test.plan:1:31: invalid field definition: name already defined`,
		},
		{
			Name:   "structure with invalid field docs",
			Source: `(structure (field (docs bar)))`,
			Want:   `test.plan:1:25: invalid field docs: expected a string or formatting expression, found identifier`,
		},
		{
			Name:   "structure with duplicate field docs",
			Source: `(structure (field (docs "bar") (docs "baz")))`,
			Want:   `test.plan:1:33: invalid field definition: docs already defined`,
		},
		{
			Name:   "structure with invalid field type",
			Source: `(structure (field (type "bar")))`,
			Want:   `test.plan:1:25: invalid field type: expected a type definition, found string`,
		},
		{
			Name:   "structure with duplicate field type",
			Source: `(structure (field (type bar) (type baz)))`,
			Want:   `test.plan:1:31: invalid field definition: type already defined`,
		},
		{
			Name:   "structure with field type and padding",
			Source: `(structure (field (type bar) (padding 2)))`,
			Want:   `test.plan:1:31: invalid field definition: type already defined`,
		},
		{
			Name:   "structure with field padding and type",
			Source: `(structure (field (padding 2) (type bar)))`,
			Want:   `test.plan:1:32: invalid field definition: type already defined`,
		},
		{
			Name:   "structure with invalid field padding size",
			Source: `(structure (field (padding "bar")))`,
			Want:   `test.plan:1:28: invalid padding definition: expected a number, found string`,
		},
		{
			Name:   "structure with invalid field padding elements",
			Source: `(structure (field (padding 3 3)))`,
			Want:   `test.plan:1:30: invalid padding definition: unexpected number after size`,
		},
		{
			Name:   "structure with excessive field padding size",
			Source: `(structure (field (padding 99999999)))`,
			Want:   `test.plan:1:28: invalid padding definition: invalid padding size: value out of range`,
		},
		{
			Name:   "structure with field missing name",
			Source: `(structure (field (docs "bar") (type byte)))`,
			Want:   `test.plan:1:12: field has no name definition`,
		},
		{
			Name:   "structure with field missing docs",
			Source: `(structure (field (name bar) (type byte)))`,
			Want:   `test.plan:1:12: field has no docs definition`,
		},
		{
			Name:   "structure with field missing type",
			Source: `(structure (field (name bar) (docs "baz")))`,
			Want:   `test.plan:1:12: field has no type definition`,
		},
		{
			Name:   "structure with duplicate field",
			Source: `(structure (field (name bar ber) (docs "baz") (type byte)) (field (name bar ber) (docs "foo") (type int8)))`,
			Want:   `test.plan:1:60: field "bar ber" already defined at test.plan:1:12`,
		},
		{
			Name:   "structure with missing name",
			Source: `(structure (docs "blah") (field (name foo) (docs "bar") (type byte)))`,
			Want:   `test.plan:1:1: structure has no name definition`,
		},
		{
			Name:   "structure with missing docs",
			Source: `(structure (name blah) (field (name foo) (docs "bar") (type byte)))`,
			Want:   `test.plan:1:1: structure has no docs definition`,
		},
		{
			Name:   "structure with missing fields",
			Source: `(structure (name blah) (docs "foo"))`,
			Want:   `test.plan:1:1: structure has no field definitions`,
		},
		{
			Name: "duplicate structure",
			Source: `(structure (name blah) (docs "xyz") (field (name foo) (docs "bar") (type byte)))
			         (structure (name blah) (docs "abc") (field (name this) (docs "some") (type int8)))`,
			Want: `test.plan:2:13: type "blah" is already defined`,
		},
		// Syscall errors.
		{
			Name:   "syscall with identifier element",
			Source: `(syscall foo)`,
			Want:   `test.plan:1:10: invalid syscall: expected a list, found identifier`,
		},
		{
			Name:   "syscall with empty definition",
			Source: `(syscall ())`,
			Want:   `test.plan:1:10: empty definition`,
		},
		{
			Name:   "syscall with bad definition",
			Source: `(syscall ("foo"))`,
			Want:   `test.plan:1:11: definition kind must be an identifier, found string`,
		},
		{
			Name:   "syscall with short definition",
			Source: `(syscall (foo))`,
			Want:   `test.plan:1:11: definition must have at least one field, found none`,
		},
		{
			Name:   "syscall with unrecognised definition",
			Source: `(syscall (foo bar))`,
			Want:   `test.plan:1:11: unrecognised syscall definition kind "foo"`,
		},
		{
			Name:   "syscall with invalid name",
			Source: `(syscall (name "bar"))`,
			Want:   `test.plan:1:16: invalid syscall name: expected an identifier, found string`,
		},
		{
			Name:   "syscall with duplicate name",
			Source: `(syscall (name bar) (name baz))`,
			Want:   `test.plan:1:22: invalid syscall definition: name already defined`,
		},
		{
			Name:   "syscall with invalid docs",
			Source: `(syscall (docs bar))`,
			Want:   `test.plan:1:16: invalid syscall docs: expected a string or formatting expression, found identifier`,
		},
		{
			Name:   "syscall with duplicate docs",
			Source: `(syscall (docs "bar") (docs "baz"))`,
			Want:   `test.plan:1:24: invalid syscall definition: docs already defined`,
		},
		{
			Name:   "syscall with invalid arg",
			Source: `(syscall (arg1 bar))`,
			Want:   `test.plan:1:16: invalid parameter definition: expected a list, found identifier`,
		},
		{
			Name:   "syscall with empty arg element",
			Source: `(syscall (arg1 ()))`,
			Want:   `test.plan:1:16: invalid parameter definition: empty definition`,
		},
		{
			Name:   "syscall with bad arg element",
			Source: `(syscall (arg1 ("foo")))`,
			Want:   `test.plan:1:17: invalid parameter definition: definition kind must be an identifier, found string`,
		},
		{
			Name:   "syscall with short arg element",
			Source: `(syscall (arg1 (foo)))`,
			Want:   `test.plan:1:17: invalid parameter definition: definition must have at least one field, found none`,
		},
		{
			Name:   "syscall with unrecognised arg element",
			Source: `(syscall (arg1 (foo bar)))`,
			Want:   `test.plan:1:17: unrecognised parameter definition kind "foo"`,
		},
		{
			Name:   "syscall with invalid arg name",
			Source: `(syscall (arg1 (name 123)))`,
			Want:   `test.plan:1:22: invalid parameter name: expected an identifier, found number`,
		},
		{
			Name:   "syscall with duplicate arg name",
			Source: `(syscall (arg1 (name bar) (name baz)))`,
			Want:   `test.plan:1:28: invalid parameter definition: name already defined`,
		},
		{
			Name:   "syscall with invalid arg docs",
			Source: `(syscall (arg1 (docs bar)))`,
			Want:   `test.plan:1:22: invalid parameter docs: expected a string or formatting expression, found identifier`,
		},
		{
			Name:   "syscall with duplicate arg docs",
			Source: `(syscall (arg1 (docs "bar") (docs "baz")))`,
			Want:   `test.plan:1:30: invalid parameter definition: docs already defined`,
		},
		{
			Name:   "syscall with invalid arg type",
			Source: `(syscall (arg1 (type "bar")))`,
			Want:   `test.plan:1:22: invalid parameter type: expected a type definition, found string`,
		},
		{
			Name:   "syscall with duplicate arg type",
			Source: `(syscall (arg1 (type bar) (type baz)))`,
			Want:   `test.plan:1:28: invalid parameter definition: type already defined`,
		},
		{
			Name:   "syscall with arg missing name",
			Source: `(syscall (arg1 (docs "bar") (type byte)))`,
			Want:   `test.plan:1:10: parameter has no name definition`,
		},
		{
			Name:   "syscall with arg missing docs",
			Source: `(syscall (arg1 (name bar) (type byte)))`,
			Want:   `test.plan:1:10: parameter has no docs definition`,
		},
		{
			Name:   "syscall with arg missing type",
			Source: `(syscall (arg1 (name bar) (docs "baz")))`,
			Want:   `test.plan:1:10: parameter has no type definition`,
		},
		{
			Name:   "syscall with duplicate arg",
			Source: `(syscall (arg1 (name bar ber) (docs "baz") (type byte)) (arg1 (name bar ber) (docs "foo") (type int8)))`,
			Want:   `test.plan:1:57: arg1 "bar ber" already defined at test.plan:1:10`,
		},
		{
			Name:   "syscall with invalid result",
			Source: `(syscall (result1 bar))`,
			Want:   `test.plan:1:19: invalid parameter definition: expected a list, found identifier`,
		},
		{
			Name:   "syscall with empty result element",
			Source: `(syscall (result1 ()))`,
			Want:   `test.plan:1:19: invalid parameter definition: empty definition`,
		},
		{
			Name:   "syscall with bad result element",
			Source: `(syscall (result1 ("foo")))`,
			Want:   `test.plan:1:20: invalid parameter definition: definition kind must be an identifier, found string`,
		},
		{
			Name:   "syscall with short result element",
			Source: `(syscall (result1 (foo)))`,
			Want:   `test.plan:1:20: invalid parameter definition: definition must have at least one field, found none`,
		},
		{
			Name:   "syscall with unrecognised result element",
			Source: `(syscall (result1 (foo bar)))`,
			Want:   `test.plan:1:20: unrecognised parameter definition kind "foo"`,
		},
		{
			Name:   "syscall with invalid result name",
			Source: `(syscall (result1 (name 123)))`,
			Want:   `test.plan:1:25: invalid parameter name: expected an identifier, found number`,
		},
		{
			Name:   "syscall with duplicate result name",
			Source: `(syscall (result1 (name bar) (name baz)))`,
			Want:   `test.plan:1:31: invalid parameter definition: name already defined`,
		},
		{
			Name:   "syscall with invalid result docs",
			Source: `(syscall (result1 (docs bar)))`,
			Want:   `test.plan:1:25: invalid parameter docs: expected a string or formatting expression, found identifier`,
		},
		{
			Name:   "syscall with duplicate result docs",
			Source: `(syscall (result1 (docs "bar") (docs "baz")))`,
			Want:   `test.plan:1:33: invalid parameter definition: docs already defined`,
		},
		{
			Name:   "syscall with invalid result type",
			Source: `(syscall (result1 (type "bar")))`,
			Want:   `test.plan:1:25: invalid parameter type: expected a type definition, found string`,
		},
		{
			Name:   "syscall with duplicate result type",
			Source: `(syscall (result1 (type bar) (type baz)))`,
			Want:   `test.plan:1:31: invalid parameter definition: type already defined`,
		},
		{
			Name:   "syscall with result missing name",
			Source: `(syscall (result1 (docs "bar") (type byte)))`,
			Want:   `test.plan:1:10: parameter has no name definition`,
		},
		{
			Name:   "syscall with result missing docs",
			Source: `(syscall (result1 (name bar) (type byte)))`,
			Want:   `test.plan:1:10: parameter has no docs definition`,
		},
		{
			Name:   "syscall with result missing type",
			Source: `(syscall (result1 (name bar) (docs "baz")))`,
			Want:   `test.plan:1:10: parameter has no type definition`,
		},
		{
			Name:   "syscall with duplicate result",
			Source: `(syscall (result1 (name bar ber) (docs "baz") (type byte)) (result1 (name bar ber) (docs "foo") (type int8)))`,
			Want:   `test.plan:1:60: result1 "bar ber" already defined at test.plan:1:10`,
		},
		{
			Name:   "syscall with missing name",
			Source: `(syscall (docs "blah"))`,
			Want:   `test.plan:1:1: syscall has no name definition`,
		},
		{
			Name:   "syscall with missing docs",
			Source: `(syscall (name blah))`,
			Want:   `test.plan:1:1: syscall has no docs definition`,
		},
		{
			Name:   "syscall with missing arg",
			Source: `(syscall (name blah) (docs "foo") (arg2 (name foo) (docs "abc") (type byte)))`,
			Want:   `test.plan:1:35: arg2 is defined but arg1 is missing`,
		},
		{
			Name:   "syscall with missing result",
			Source: `(syscall (name blah) (docs "foo") (result2 (name foo) (docs "abc") (type byte)))`,
			Want:   `test.plan:1:35: result2 is defined but result1 is missing`,
		},
		{
			Name: "duplicate syscall",
			Source: `(syscall (name blah) (docs "xyz"))
			         (syscall (name blah) (docs "abc"))`,
			Want: `test.plan:2:13: cannot define syscall: type "blah" is already defined`,
		},
		{
			Name:   "syscall clash",
			Source: `(syscall (name uint64) (docs "abc"))`,
			Want:   `test.plan:1:1: cannot define syscall: type "uint64" is already defined`,
		},
		// Group errors.
		{
			Name:   "group with identifier element",
			Source: `(group foo)`,
			Want:   `test.plan:1:8: invalid group: expected a list, found identifier`,
		},
		{
			Name:   "group with empty definition",
			Source: `(group ())`,
			Want:   `test.plan:1:8: empty definition`,
		},
		{
			Name:   "group with bad definition",
			Source: `(group ("foo"))`,
			Want:   `test.plan:1:9: definition kind must be an identifier, found string`,
		},
		{
			Name:   "group with short definition",
			Source: `(group (foo))`,
			Want:   `test.plan:1:9: definition must have at least one field, found none`,
		},
		{
			Name:   "group with unrecognised definition",
			Source: `(group (foo bar))`,
			Want:   `test.plan:1:9: unrecognised group definition kind "foo"`,
		},
		{
			Name:   "group with invalid name",
			Source: `(group (name "bar"))`,
			Want:   `test.plan:1:14: invalid group name: expected an identifier, found string`,
		},
		{
			Name:   "group with duplicate name",
			Source: `(group (name bar) (name baz))`,
			Want:   `test.plan:1:20: invalid group definition: name already defined`,
		},
		{
			Name:   "group with invalid docs",
			Source: `(group (docs bar))`,
			Want:   `test.plan:1:14: invalid group docs: expected a string or formatting expression, found identifier`,
		},
		{
			Name:   "group with invalid docs formatting expression",
			Source: `(group (docs (bar)))`,
			Want:   `test.plan:1:15: invalid group docs: invalid formatting expression: definition must have at least one field, found none`,
		},
		{
			Name:   "group with unsupported docs formatting expression",
			Source: `(group (docs (bar foo)))`,
			Want:   `test.plan:1:15: invalid group docs: unrecognised formatting expression kind "bar"`,
		},
		{
			Name:   "group with invalid docs code formatting expression",
			Source: `(group (docs (code foo)))`,
			Want:   `test.plan:1:20: invalid group docs: invalid formatting expression: expected a string, found identifier`,
		},
		{
			Name:   "group with invalid docs reference formatting expression",
			Source: `(group (docs (reference "baz") "bar"))`,
			Want:   `test.plan:1:25: invalid group docs: invalid reference formatting expression: invalid type reference: expected an identifier, found string`,
		},
		{
			Name:   "group with duplicate docs",
			Source: `(group (docs "bar") (docs "baz"))`,
			Want:   `test.plan:1:22: invalid group definition: docs already defined`,
		},
		{
			Name:   "group with invalid item",
			Source: `(group (integer "foo"))`,
			Want:   `test.plan:1:17: expected an identifier, found string`,
		},
		{
			Name:   "group with missing name",
			Source: `(group (docs "blah") (integer uint8))`,
			Want:   `test.plan:1:1: group has no name definition`,
		},
		{
			Name:   "group with missing docs",
			Source: `(group (name blah) (integer uint8))`,
			Want:   `test.plan:1:1: group has no docs definition`,
		},
		{
			Name:   "group with missing items",
			Source: `(group (name blah) (docs "foo"))`,
			Want:   `test.plan:1:1: group has no item definitions`,
		},
		{
			Name: "duplicate group",
			Source: `(integer (name blah) (docs "xyz") (type uint8))
			         (group (name blah) (docs "abc") (integer uint8))`,
			Want: `test.plan:2:13: type "blah" is already defined`,
		},
		// Type errors.
		{
			Name:   "type clashing with synthetic enumeration for syscalls",
			Source: `(structure (name syscalls) (docs "xyz") (field (name foo) (docs "bar") (type uint64)))`,
			Want:   `test.plan:1:1: type "syscalls" is already defined`,
		},
		{
			Name:   "undefined type",
			Source: `(structure (name blah) (docs "xyz") (field (name foo) (docs "bar") (type never before seen)))`,
			Want:   `test.plan:1:74: type "never before seen" is not defined`,
		},
		{
			Name:   "undefined pointer type",
			Source: `(structure (name blah) (docs "xyz") (field (name foo) (docs "bar") (type *mutable never before seen)))`,
			Want:   `test.plan:1:83: type "never before seen" is not defined`,
		},
		{
			Name:   "invalid pointer type",
			Source: `(structure (name blah) (docs "xyz") (field (name foo) (docs "bar") (type *other never before seen)))`,
			Want:   `test.plan:1:74: invalid field type: invalid pointer note: want "constant" or "mutable", found "other"`,
		},
		{
			Name:   "invalid pointer underlying type",
			Source: `(structure (name blah) (docs "xyz") (field (name foo) (docs "bar") (type *constant "blooper")))`,
			Want:   `test.plan:1:84: invalid field type: invalid type reference: expected an identifier, found string`,
		},
		{
			Name:   "error enumeration with too few values",
			Source: `(enumeration (name example error) (docs "abc") (type uint8) (value (name no error) (docs "xyz")))`,
			Want:   `test.plan:1:1: enumeration "example error" is not an error enumeration: missing value "bad syscall"`,
		},
		{
			Name:   "error enumeration with missing values",
			Source: `(enumeration (name example error) (docs "abc") (type uint8) (value (name foo) (docs "xyz")))`,
			Want:   `test.plan:1:1: enumeration "example error" is not an error enumeration: missing value "no error"`,
		},
		{
			Name:   "structure with only padding",
			Source: `(structure (name blah) (docs "xyz") (field (name foo) (docs "bar") (padding 1)))`,
			Want:   `test.plan:1:1: structure has no non-padding fields`,
		},
		{
			Name: "structure with unaligned field",
			Source: `(structure
			             (name foo)
			             (docs "x")
			             (field
			                 (name the first)
			                 (docs "baz")
			                 (type byte))
			             (field
			                 (name bar ber)
			                 (docs "foo")
			                 (type sint16)))`,
			Want: `test.plan:8:17: field "bar ber" is not aligned: 2-aligned field found at offset 1`,
		},
		{
			Name:   "syscall with undefined arg type",
			Source: `(syscall (name baz) (docs "abc") (arg1 (name foo) (docs "bar") (type blah)))`,
			Want:   `test.plan:1:70: type "blah" is not defined`,
		},
		{
			Name: "syscall with complex arg",
			Source: `(structure (name blah) (docs "xyz") (field (name foo) (docs "bar") (type byte)))
			         (syscall (name baz) (docs "abc") (arg1 (name foo) (docs "bar") (type blah)))`,
			Want: `test.plan:2:46: arg1 "foo" has invalid type: structure blah cannot be stored in a parameter`,
		},
		{
			Name: "syscall with complex result",
			Source: `(structure (name blah) (docs "xyz") (field (name foo) (docs "bar") (type byte)))
			         (syscall (name baz) (docs "abc") (result1 (name foo) (docs "bar") (type blah)))`,
			Want: `test.plan:2:46: result1 "foo" has invalid type: structure blah cannot be stored in a parameter`,
		},
		{
			Name:   "syscall with args but no results",
			Source: `(syscall (name baz) (docs "abc") (arg1 (name foo) (docs "bar") (type byte)))`,
			Want:   `test.plan:1:1: cannot handle errors in syscall baz: syscall has no results`,
		},
		{
			Name:   "group referencing a non-existant type",
			Source: `(group (name baz) (docs "abc") (integer foo))`,
			Want:   `test.plan:1:32: type "foo" is not defined`,
		},
		{
			Name:   "group referencing an unexpected type",
			Source: `(group (name baz) (docs "abc") (structure uint8))`,
			Want:   `test.plan:1:32: structure reference "uint8" resolved to a types.Integer`,
		},
		{
			Name: "group referencing the wrong type",
			Source: `(integer (name foo) (docs "xyz") (type uint8))
			         (group (name baz) (docs "abc") (structure foo))`,
			Want: `test.plan:2:44: structure reference "foo" resolved to a *types.NewInteger`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			f, err := parser.ParseFile("test.plan", test.Source)
			if err != nil {
				t.Fatalf("Failed to parse file: %v", err)
			}

			got, err := Interpret("test.plan", f, X86_64)
			if err == nil {
				t.Fatalf("Interpret(): wanted error %q, got:\n%#v", test.Want, got)
			}

			e := err.Error()
			if e != test.Want {
				t.Fatalf("Interpret():\nGot:  %s\nWant: %s", e, test.Want)
			}
		})
	}
}
