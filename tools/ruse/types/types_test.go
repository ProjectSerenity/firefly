// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"go/constant"
	gotoken "go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/parser"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		Name  string
		Path  string
		Text  string
		Files []string
		Err   string
		Want  *Package
	}{
		// Valid packages.
		{
			Name:  "minimal",
			Path:  "tests/minimal",
			Files: []string{"minimal"},
			Want: &Package{
				Name: "minimal",
				Path: "tests/minimal",
				scope: &Scope{
					parent:  Universe,
					comment: "package tests/minimal",
				},
			},
		},
		{
			Name: "multifile",
			Path: "tests/minimal",
			Files: []string{
				"minimal",
				"multifile",
			},
			Want: &Package{
				Name: "minimal",
				Path: "tests/minimal",
				scope: &Scope{
					parent:  Universe,
					comment: "package tests/minimal",
				},
			},
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
					value: constant.BinaryOp(constant.MakeString("string 2"), gotoken.ADD, constant.MakeString("foo")),
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

				fun1Scope := NewScope(pkg.scope, 93, 109, "function nullary-function")
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

				fun2Scope := NewScope(pkg.scope, 145, 154, "function unary-function")
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

				fun3Scope := NewScope(pkg.scope, 203, 237, "function binary-function")
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

				fun4Scope := NewScope(pkg.scope, 268, 275, "function add1")
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

				fun5Scope := NewScope(pkg.scope, 332, 347, "function product")
				param1 = NewParameter(fun5Scope, 293, 306, pkg, "base", Uint64)
				param2 = NewParameter(fun5Scope, 307, 322, pkg, "scalar", Uint64)
				fun5Scope.Insert(param1)
				fun5Scope.Insert(param2)
				pkg.scope.Insert(&Function{
					object: object{
						pos:  278,
						end:  347,
						pkg:  pkg,
						name: "product",
						typ: &Signature{
							name:   "(func (uint64) (uint64) uint64)",
							params: []*Variable{param1, param2},
							result: Uint64,
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

				funcScope := NewScope(pkg.scope, 329, 338, "function syscall6")
				sys := NewParameter(funcScope, 91, 103, pkg, "sys", Uint64)
				arg1 := NewParameter(funcScope, 117, 130, pkg, "arg1", Uint64)
				arg2 := NewParameter(funcScope, 144, 157, pkg, "arg2", Uint64)
				arg3 := NewParameter(funcScope, 171, 184, pkg, "arg3", Uint64)
				arg4 := NewParameter(funcScope, 198, 211, pkg, "arg4", Uint64)
				arg5 := NewParameter(funcScope, 225, 238, pkg, "arg5", Uint64)
				arg6 := NewParameter(funcScope, 251, 264, pkg, "arg6", Uint64)
				funcScope.Insert(sys)
				funcScope.Insert(arg1)
				funcScope.Insert(arg2)
				funcScope.Insert(arg3)
				funcScope.Insert(arg4)
				funcScope.Insert(arg5)
				funcScope.Insert(arg6)
				pkg.scope.Insert(&Function{
					object: object{
						pos:  309,
						end:  338,
						pkg:  pkg,
						name: "syscall6",
						typ: &Signature{
							name:   "(func (uint64) (uint64) (uint64) (uint64) (uint64) (uint64) (uint64) uint64)",
							params: []*Variable{sys, arg1, arg2, arg3, arg4, arg5, arg6},
							result: Uint64,
						},
					},
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
			Name: "let assignment to integer",
			Path: "tests/foo",
			Text: `(package bar)`,
			Err:  `found package name "bar", expected "main" or "foo"`,
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
						if ok && obj.ID() == SpecialFormAsmFunc {
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
					if expr, ok := n.(ast.Expression); ok && info.Types[expr].Type == nil && expr.Pos() != file.Package && expr.Pos() != file.Name.NamePos {
						t.Errorf("%s: expression %s is absent from info.Types", fset.Position(expr.Pos()), expr.Print())
					}

					return true
				})
			}
		})
	}
}
