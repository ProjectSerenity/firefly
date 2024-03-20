// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/constant"
	"firefly-os.dev/tools/ruse/parser"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
)

// TestExpression is a somewhat more friendly
// set of tests than TestCheck. This is for
// testing that individual expressions result
// in the correct type (and optionally, value).
func TestExpression(t *testing.T) {
	tests := []struct {
		Name  string
		Path  string
		Text  string
		Ident string // Identifier to check.
		Want  TypeAndValue
	}{
		{
			Name: "untyped integer constant",
			Path: "tests/minimal",
			Text: `
				(package minimal)
				(let foo 3)
			`,
			Ident: "foo",
			Want: TypeAndValue{
				Type:  UntypedInt,
				Value: constant.MakeInt64(3),
			},
		},
		{
			Name: "untyped string constant",
			Path: "tests/minimal",
			Text: `
				(package minimal)
				(let foo "bar")
			`,
			Ident: "foo",
			Want: TypeAndValue{
				Type:  UntypedString,
				Value: constant.MakeString("bar"),
			},
		},
		{
			Name: "size-of int",
			Path: "tests/minimal",
			Text: `
				(package minimal)
				(let foo (size-of int))
			`,
			Ident: "foo",
			Want: TypeAndValue{
				Type:  UntypedInt,
				Value: constant.MakeInt64(8), // 64 bits on x86-64.
			},
		},
		{
			Name: "size-of array",
			Path: "tests/minimal",
			Text: `
				(package minimal)
				(let foo (size-of array/3/uint16))
			`,
			Ident: "foo",
			Want: TypeAndValue{
				Type:  UntypedInt,
				Value: constant.MakeInt64(3 * 2),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.ruse", test.Text, 0)
			if err != nil {
				t.Fatalf("failed to parse text: %v", err)
			}

			// Find the expression to check.
			var expr ast.Expression
			ast.Inspect(file, func(n ast.Node) bool {
				if expr != nil {
					return false
				}

				if n == nil {
					return true
				}

				ident, ok := n.(*ast.Identifier)
				if !ok || ident.Name != test.Ident {
					return true
				}

				expr = ident
				return false
			})

			files := []*ast.File{file}
			info := &Info{
				Types: make(map[ast.Expression]TypeAndValue),
			}

			_, err = Check(test.Path, fset, files, sys.X86_64, info)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := info.Types[expr]
			if diff := cmp.Diff(test.Want, got, cmp.Exporter(func(t reflect.Type) bool { return true })); diff != "" {
				t.Fatalf("Check(): (-want, +got)\n%s", diff)
			}
		})
	}
}

// TestCheck is quite a complex set of tests
// that cover the Check function. New non-error
// tests should probably be added to
// TestExpression instead.
func TestCheck(t *testing.T) {
	tests := []struct {
		Name  string
		Path  string
		Text  string   // Full code text (not compatible with Files).
		Files []string // List of filenames (not compatible with Text).
		Err   string
		Want  *Package
	}{
		// Valid packages.
		{
			Name:  "minimal",
			Path:  "tests/minimal",
			Files: []string{"minimal"},
			Want: (func() *Package {
				pkg := &Package{
					Name: "minimal",
					Path: "tests/minimal",
					scope: &Scope{
						parent:  Universe,
						comment: "package tests/minimal",
					},
				}

				file0 := NewScope(pkg.scope, 58, 74, "file 0")
				file0.readonly = true

				return pkg
			})(),
		},
		{
			Name: "multifile",
			Path: "tests/minimal",
			Files: []string{
				"minimal",
				"multifile",
			},
			Want: (func() *Package {
				pkg := &Package{
					Name: "minimal",
					Path: "tests/minimal",
					scope: &Scope{
						parent:  Universe,
						comment: "package tests/minimal",
					},
				}

				file0 := NewScope(pkg.scope, 58, 74, "file 0")
				file1 := NewScope(pkg.scope, 134, 150, "file 1")
				file0.readonly = true
				file1.readonly = true

				return pkg
			})(),
		},
		{
			Name:  "constants",
			Path:  "tests/constants",
			Files: []string{"constants"},
			Want: (func() *Package {
				pkg := &Package{
					Name: "constants",
					Path: "tests/constants",
					scope: &Scope{
						parent:  Universe,
						comment: "package tests/constants",
					},
				}

				file0 := NewScope(pkg.scope, 58, 548, "file 0")
				file0.readonly = true

				pkg.scope.Insert(&Constant{
					object: object{
						pos:  79,
						end:  110,
						pkg:  pkg,
						name: "INFERRED-STRING",
						typ:  UntypedString,
					},
					value: constant.MakeString("string 1"),
				})

				pkg.scope.Insert(&Constant{
					object: object{
						pos:  113,
						end:  133,
						pkg:  pkg,
						name: "InferredInt",
						typ:  UntypedInt,
					},
					value: constant.MakeInt64(123),
				})

				pkg.scope.Insert(&Constant{
					object: object{
						pos:  136,
						end:  155,
						pkg:  pkg,
						name: "big",
						typ:  Uint64,
					},
					value: constant.MakeUint64(1),
				})

				pkg.scope.Insert(&Constant{
					object: object{
						pos:  158,
						end:  188,
						pkg:  pkg,
						name: "NAMED",
						typ:  String,
					},
					value: constant.MakeString("string 2"),
				})

				pkg.scope.Insert(&Constant{
					object: object{
						pos:  191,
						end:  213,
						pkg:  pkg,
						name: "SMALL",
						typ:  Int8,
					},
					value: constant.MakeInt64(-127),
				})

				pkg.scope.Insert(&Constant{
					object: object{
						pos:  216,
						end:  240,
						pkg:  pkg,
						name: "derived",
						typ:  Int,
					},
					value: constant.MakeInt64(3),
				})

				pkg.scope.Insert(&Constant{
					object: object{
						pos:  243,
						end:  356,
						pkg:  pkg,
						name: "complex-constant-op",
						typ:  Int,
					},
					value: constant.MakeInt64(
						int64(len("foo")) +
							3 +
							int64(len("other")) +
							((0xff - 250) - 2) +
							((2 * 3) * 4) +
							((12 / 6) / 2)),
				})

				pkg.scope.Insert(&Constant{
					object: object{
						pos:  359,
						end:  395,
						pkg:  pkg,
						name: "compound-string",
						typ:  String,
					},
					value: constant.Operation(constant.OpAdd, constant.MakeString("string 2"), constant.MakeString("foo")),
				})

				pkg.scope.Insert(&Constant{
					object: object{
						pos:  398,
						end:  421,
						pkg:  pkg,
						name: "typecast",
						typ:  Int64,
					},
					value: constant.MakeInt64(1),
				})

				pkg.scope.Insert(&Constant{
					object: object{
						pos:  424,
						end:  482,
						pkg:  pkg,
						name: "strings",
						typ: &Array{
							length:  4,
							element: String,
						},
					},
					value: constant.MakeArray("array/4/string", []constant.Value{
						constant.MakeString("foo"),
						constant.MakeString("bar"),
						constant.MakeString("foobar"),
						constant.MakeString("baz"),
					}),
				})

				pkg.scope.Insert(&Constant{
					object: object{
						pos:  485,
						end:  518,
						pkg:  pkg,
						name: "strings-length",
						typ:  Int,
					},
					value: constant.MakeInt64(4),
				})

				pkg.scope.Insert(&Constant{
					object: object{
						pos:  521,
						end:  547,
						pkg:  pkg,
						name: "second",
						typ:  String,
					},
					value: constant.MakeString("bar"),
				})

				return pkg
			})(),
		},
		{
			Name:  "functions",
			Path:  "tests/functions",
			Files: []string{"functions"},
			Want: (func() *Package {
				pkg := &Package{
					Name: "functions",
					Path: "tests/functions",
					scope: &Scope{
						parent:  Universe,
						comment: "package tests/functions",
					},
				}

				file0 := NewScope(pkg.scope, 46, 554, "file 0")
				file0.readonly = true

				fun1Scope := NewScope(file0, 93, 109, "function nullary-function")
				fun1Scope.Insert(NewConstant(fun1Scope, 108, 109, pkg, "_", Int8, constant.MakeInt64(0)))
				pkg.scope.Insert(&Function{
					object: object{
						pos:  67,
						end:  109,
						pkg:  pkg,
						name: "nullary-function",
						typ: &Signature{
							name:   "(func)",
							params: []*Variable{},
						},
					},
				})

				fun2Scope := NewScope(file0, 145, 154, "function unary-function")
				param1 := NewParameter(fun2Scope, 134, 142, pkg, "x", Byte)
				fun2Scope.Insert(param1)
				fun2Scope.Insert(NewVariable(fun2Scope, 153, 154, pkg, "_", Byte))
				pkg.scope.Insert(&Function{
					object: object{
						pos:  112,
						end:  154,
						pkg:  pkg,
						name: "unary-function",
						typ: &Signature{
							name:   "(func (byte))",
							params: []*Variable{param1},
						},
					},
				})

				fun3Scope := NewScope(file0, 203, 237, "function binary-function")
				param1 = NewParameter(fun3Scope, 180, 189, pkg, "x", Int64)
				param2 := NewParameter(fun3Scope, 190, 200, pkg, "y", String)
				fun3Scope.Insert(param1)
				fun3Scope.Insert(param2)
				fun3Scope.Insert(NewVariable(fun3Scope, 236, 237, pkg, "_", Int64))
				pkg.scope.Insert(&Function{
					object: object{
						pos:  157,
						end:  237,
						pkg:  pkg,
						name: "binary-function",
						typ: &Signature{
							name:   "(func (int64) (string))",
							params: []*Variable{param1, param2},
						},
					},
				})

				fun4Scope := NewScope(file0, 268, 275, "function add1")
				param1 = NewParameter(fun4Scope, 252, 260, pkg, "x", Int8)
				fun4Scope.Insert(param1)
				pkg.scope.Insert(&Function{
					object: object{
						pos:  240,
						end:  275,
						pkg:  pkg,
						name: "add1",
						typ: &Signature{
							name:   "(func (int8) int8)",
							params: []*Variable{param1},
							result: Int8,
						},
					},
				})

				invertedStack := &ast.Identifier{NamePos: 0, Name: "false"}
				params := []*ast.Identifier{
					{NamePos: 0, Name: "rdi"},
					{NamePos: 0, Name: "rsi"},
					{NamePos: 0, Name: "rdx"},
					{NamePos: 0, Name: "rcx"},
					{NamePos: 0, Name: "r8"},
					{NamePos: 0, Name: "r9"},
				}
				result := []*ast.Identifier{
					{NamePos: 0, Name: "rax"},
					{NamePos: 0, Name: "rdx"},
				}
				scratch := []*ast.Identifier{
					{NamePos: 0, Name: "rax"},
					{NamePos: 0, Name: "rdi"},
					{NamePos: 0, Name: "rsi"},
					{NamePos: 0, Name: "rdx"},
					{NamePos: 0, Name: "rcx"},
					{NamePos: 0, Name: "r8"},
					{NamePos: 0, Name: "r9"},
					{NamePos: 0, Name: "r10"},
					{NamePos: 0, Name: "r11"},
				}
				unused := []*ast.Identifier(nil)
				abi, err := NewRawABI(sys.X86_64, invertedStack, params, result, scratch, unused)
				if err != nil {
					panic(err.Error())
				}
				pkg.scope.Insert(&Constant{
					object: object{
						pos:  293,
						end:  431,
						pkg:  pkg,
						name: "system-v",
						typ:  abi,
					},
				})

				fun5Scope := NewScope(file0, 504, 519, "function product")
				param1 = NewParameter(fun5Scope, 465, 478, pkg, "base", Uint64)
				param2 = NewParameter(fun5Scope, 479, 494, pkg, "scalar", Uint64)
				fun5Scope.Insert(param1)
				fun5Scope.Insert(param2)
				pkg.scope.Insert(&Function{
					object: object{
						pos:  450,
						end:  519,
						pkg:  pkg,
						name: "product",
						typ: &Signature{
							name:   "(func (uint64) (uint64) uint64)",
							params: []*Variable{param1, param2},
							result: Uint64,
						},
					},
					abi: abi.abi,
				})

				// Check that we correctly handle an untyped
				// constant being used as the return value for
				// a function. That is, the result can be
				// assignable to the result type without being
				// the exact same type.
				NewScope(file0, 551, 553, "function return-constant")
				pkg.scope.Insert(&Function{
					object: object{
						pos:  522,
						end:  553,
						pkg:  pkg,
						name: "return-constant",
						typ: &Signature{
							name:   "(func int)",
							params: []*Variable{},
							result: Int,
						},
					},
				})

				return pkg
			})(),
		},
		{
			Name:  "assembly",
			Path:  "tests/assembly",
			Files: []string{"assembly"},
			Want: (func() *Package {
				pkg := &Package{
					Name: "assembly",
					Path: "tests/assembly",
					scope: &Scope{
						parent:  Universe,
						comment: "package tests/assembly",
					},
				}

				file0 := NewScope(pkg.scope, 48, 361, "file 0")
				file0.readonly = true

				invertedStack := (*ast.Identifier)(nil)
				params := []*ast.Identifier{
					{NamePos: 0, Name: "rax"},
					{NamePos: 0, Name: "rdi"},
					{NamePos: 0, Name: "rsi"},
					{NamePos: 0, Name: "rdx"},
					{NamePos: 0, Name: "r10"},
					{NamePos: 0, Name: "r8"},
					{NamePos: 0, Name: "r9"},
				}
				result := []*ast.Identifier{
					{NamePos: 0, Name: "rax"},
				}
				scratch := []*ast.Identifier{
					{NamePos: 0, Name: "rcx"},
					{NamePos: 0, Name: "r11"},
				}
				unused := []*ast.Identifier(nil)
				abi, err := NewRawABI(sys.X86_64, invertedStack, params, result, scratch, unused)
				if err != nil {
					panic(err.Error())
				}
				pkg.scope.Insert(&Constant{
					object: object{
						pos:  273,
						end:  360,
						pkg:  pkg,
						name: "syscall",
						typ:  abi,
					},
				})

				funcScope := NewScope(file0, 246, 255, "function syscall6")
				syscall := NewParameter(funcScope, 120, 133, pkg, "sys", Uintptr)
				arg1 := NewParameter(funcScope, 136, 150, pkg, "arg1", Uintptr)
				arg2 := NewParameter(funcScope, 153, 167, pkg, "arg2", Uintptr)
				arg3 := NewParameter(funcScope, 170, 184, pkg, "arg3", Uintptr)
				arg4 := NewParameter(funcScope, 187, 201, pkg, "arg4", Uintptr)
				arg5 := NewParameter(funcScope, 204, 218, pkg, "arg5", Uintptr)
				arg6 := NewParameter(funcScope, 221, 235, pkg, "arg6", Uintptr)
				funcScope.Insert(syscall)
				funcScope.Insert(arg1)
				funcScope.Insert(arg2)
				funcScope.Insert(arg3)
				funcScope.Insert(arg4)
				funcScope.Insert(arg5)
				funcScope.Insert(arg6)
				pkg.scope.Insert(&Function{
					object: object{
						pos:  98,
						end:  255,
						pkg:  pkg,
						name: "syscall6",
						typ: &Signature{
							name:   "(func (uintptr) (uintptr) (uintptr) (uintptr) (uintptr) (uintptr) (uintptr) uintptr)",
							params: []*Variable{syscall, arg1, arg2, arg3, arg4, arg5, arg6},
							result: Uintptr,
						},
					},
					abi: abi.abi,
				})

				return pkg
			})(),
		},
		// Invalid packages.
		{
			Name: "no files",
			Path: "tests/no_files",
			Err:  "package has no files",
		},
		{
			Name: "bad multifile",
			Path: "tests/minimal",
			Files: []string{
				"minimal",
				"bad-multifile",
			},
			Err: `found package name "wrong", expected "minimal"`,
		},
		{
			Name: "bad package name",
			Path: "tests/minimal",
			Files: []string{
				"minimal",
				"bad-multifile",
			},
			Err: `found package name "wrong", expected "minimal"`,
		},
		{
			Name: "incorrect package name",
			Path: "tests/foo",
			Text: `(package bar)`,
			Err:  `found package name "bar", expected "main" or "foo"`,
		},
		{
			Name: "incorrect package annotation",
			Path: "tests/foo",
			Text: `'(bar)(package foo)`,
			Err:  `invalid package annotation: unrecognised annotation type: bar`,
		},
		{
			Name: "incorrect import annotation",
			Path: "tests/foo",
			Text: `(package foo)
			       '(bar)(import "baz")`,
			Err: `invalid import annotation: unrecognised annotation type: bar`,
		},
		{
			Name: "incorrect imports annotation",
			Path: "tests/foo",
			Text: `(package foo)
			       '(bar)(import (other "baz"))`,
			Err: `invalid import annotation: unrecognised annotation type: bar`,
		},
		{
			Name: "incorrect imports local annotations",
			Path: "tests/foo",
			Text: `(package foo)
			       (import '(bar)(other "baz"))`,
			Err: `invalid import annotation: unrecognised annotation type: bar`,
		},
		{
			Name: "incorrect let annotation",
			Path: "tests/foo",
			Text: `(package foo)
			       '(bar)(let baz "text")`,
			Err: `invalid annotation: unrecognised annotation type: bar`,
		},
		{
			Name: "let assignment to integer and type",
			Path: "tests/foo",
			Text: `(package foo)
			       (let (1 baz) foo)`,
			Err: "definition must be an identifier, found literal",
		},
		{
			Name: "let assignment to integer type",
			Path: "tests/foo",
			Text: `(package foo)
			       (let (bar 1) foo)`,
			Err: "type must be an identifier, found literal",
		},
		{
			Name: "let assignment to unknown integer type",
			Path: "tests/foo",
			Text: `(package foo)
			       (let (BAR baz) 1)`,
			Err: "undefined type: baz",
		},
		{
			Name: "let assignment of integer to string type",
			Path: "tests/foo",
			Text: `(package foo)
			       (let (BAR string) 1)`,
			Err: "cannot assign integer literal to constant of type string",
		},
		{
			Name: "duplicate let assignment of integer type",
			Path: "tests/foo",
			Text: `(package foo)
			       (let (BAR uint8) 1)
			       (let (BAR uint8) 2)`,
			Err: "BAR redeclared",
		},
		{
			Name: "let assignment to unknown string type",
			Path: "tests/foo",
			Text: `(package foo)
			       (let (BAR baz) "foo")`,
			Err: "undefined type: baz",
		},
		{
			Name: "let assignment to unknown string type",
			Path: "tests/foo",
			Text: `(package foo)
			       (let (BAR bool) "foo")`,
			Err: "cannot assign string literal to constant of type bool",
		},
		{
			Name: "duplicate let assignment of string type",
			Path: "tests/foo",
			Text: `(package foo)
			       (let BAR "foo")
			       (let BAR "bar")`,
			Err: "BAR redeclared",
		},
		{
			Name: "unrecognised architecture",
			Path: "tests/foo",
			Text: `(package foo)
			       '(arch pdp11)
			       (func (foo int) 1)`,
			Err: "unrecognised architecture: pdp11",
		},
		{
			Name: "incorrect asm-func annotation",
			Path: "tests/foo",
			Text: `(package foo)
			       '(bar)(asm-func (baz))`,
			Err: `invalid function annotation: unrecognised annotation type: bar`,
		},
		{
			Name: "incorrect asm-func signature annotation",
			Path: "tests/foo",
			Text: `(package foo)
			       (asm-func '(bar)(baz))`,
			Err: `invalid function annotation: unrecognised annotation type: bar`,
		},
		{
			Name: "incorrect asm-func parameter annotation",
			Path: "tests/foo",
			Text: `(package foo)
			       (asm-func (baz '(bar)(msg string)))`,
			Err: `invalid function annotation: unrecognised annotation type: bar`,
		},
		{
			Name: "incorrect func annotation",
			Path: "tests/foo",
			Text: `(package foo)
			       '(bar)(func (baz))`,
			Err: `invalid function annotation: unrecognised annotation type: bar`,
		},
		{
			Name: "incorrect func signature annotation",
			Path: "tests/foo",
			Text: `(package foo)
			       (func '(bar)(baz))`,
			Err: `invalid function annotation: unrecognised annotation type: bar`,
		},
		{
			Name: "incorrect func parameter annotation",
			Path: "tests/foo",
			Text: `(package foo)
			       (func (baz '(bar)(msg string)))`,
			Err: `invalid function annotation: unrecognised annotation type: bar`,
		},
		{
			Name: "incorrect expression annotation",
			Path: "tests/foo",
			Text: `(package foo)
			       (func (baz int)
			           '(bar)(let (num int) 1)
			           num)`,
			Err: `invalid expression annotation: unrecognised annotation type: bar`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fset := token.NewFileSet()
			var files []*ast.File
			if test.Text != "" {
				file, err := parser.ParseFile(fset, "test.ruse", test.Text, 0)
				if err != nil {
					t.Fatalf("failed to parse text: %v", err)
				}

				files = []*ast.File{file}
			} else {
				files = make([]*ast.File, len(test.Files))
				for i, name := range test.Files {
					full := filepath.Join("testdata", name+".ruse")
					file, err := parser.ParseFile(fset, full, nil, 0)
					if err != nil {
						t.Fatalf("failed to parse %s: %v", name, err)
					}

					files[i] = file
				}
			}

			info := &Info{
				Types:       make(map[ast.Expression]TypeAndValue),
				Definitions: make(map[*ast.Identifier]Object),
				Uses:        make(map[*ast.Identifier]Object),
			}

			pkg, err := Check(test.Path, fset, files, sys.X86_64, info)
			if test.Err != "" {
				if err == nil {
					t.Fatalf("unexpected success, wanted error %q", test.Err)
				}

				e := err.Error()
				if !strings.Contains(e, test.Err) {
					t.Fatalf("got error %q, want %q", e, test.Err)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, pkg, cmp.Exporter(func(t reflect.Type) bool { return true })); diff != "" {
				t.Fatalf("Check(): (-want, +got)\n%s", diff)
			}

			// Check that all the info is populated.
			for _, file := range files {
				ast.Inspect(file, func(n ast.Node) bool {
					if n == nil {
						return false
					}

					// Ignore quoted expressions and any
					// assembly snippets, as they exist
					// outside the type system.
					switch n := n.(type) {
					case *ast.QuotedIdentifier:
						return false
					case *ast.QuotedList:
						return false
					case *ast.List:
						ident, ok := n.Elements[0].(*ast.Identifier)
						if !ok {
							break
						}

						obj, ok := info.Uses[ident].(*SpecialForm)
						if !ok {
							break
						}

						switch obj.ID() {
						case SpecialFormAsmFunc, SpecialFormABI:
							return false
						}
					}

					// Every identifier should be in either
					// info.Definitions or info.Uses, except
					// the package name in the package
					// statement in each file.
					if ident, ok := n.(*ast.Identifier); ok && info.Definitions[ident] == nil && info.Uses[ident] == nil && ident != file.Name {
						t.Errorf("%s: identifier %q is absent from info.Definitions and info.Uses", fset.Position(ident.NamePos), ident.Name)
					}

					// Every expression should have a type
					// (and possibly value), except the
					// package statement in each file.
					if expr, ok := n.(ast.Expression); ok && info.Types[expr].Type == nil && expr.Pos() != file.Package.ParenOpen && expr.Pos() != file.Name.NamePos {
						t.Errorf("%s: expression %s is absent from info.Types", fset.Position(expr.Pos()), expr.Print())
					}

					return true
				})
			}
		})
	}
}

func TestImports(t *testing.T) {
	// Test that we correctly type-check
	// one package that imports another.
	depPkgName := "example/imported"
	depCode := `
		(package imported)

		(let msg "Hello, world!")

		(func (double-string-length (s string) int)
			(* (len s) 2))
	`

	mainPkgName := "example/main"
	mainCode := `
		(package main)

		(import
			("example/imported"))

		(let text imported.msg)
		(let double-length (* (len imported.msg) 2))

		(func (test-imported int)
			(+ double-length (imported.double-string-length text)))
	`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "dep.ruse", depCode, 0)
	if err != nil {
		t.Fatalf("failed to parse dep text: %v", err)
	}

	files := []*ast.File{file}

	depInfo := &Info{
		Types:       make(map[ast.Expression]TypeAndValue),
		Definitions: make(map[*ast.Identifier]Object),
		Uses:        make(map[*ast.Identifier]Object),
	}

	depPkg, err := Check(depPkgName, fset, files, sys.X86_64, depInfo)
	if err != nil {
		t.Fatalf("failed to check dep text: %v", err)
	}

	file, err = parser.ParseFile(fset, "main.ruse", mainCode, 0)
	if err != nil {
		t.Fatalf("failed to parse main text: %v", err)
	}

	files = []*ast.File{file}

	mainInfo := &Info{
		Types:       make(map[ast.Expression]TypeAndValue),
		Definitions: make(map[*ast.Identifier]Object),
		Uses:        make(map[*ast.Identifier]Object),
		Packages: map[string]*Package{
			depPkgName: depPkg,
		},
	}

	mainPkg, err := Check(mainPkgName, fset, files, sys.X86_64, mainInfo)
	if err != nil {
		t.Fatalf("failed to check main text: %v", err)
	}

	// Check that it's all joined up.
	text := mainPkg.scope.Lookup("text")
	con, ok := text.(*Constant)
	if !ok {
		t.Fatalf("failed to check main constant: got %#v", text)
	}

	if con.Type() != UntypedString {
		t.Fatalf("incorrect main constant type: got %v, want %v", con.Type(), UntypedString)
	}
}
