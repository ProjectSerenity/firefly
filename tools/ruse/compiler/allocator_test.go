// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package compiler

import (
	"go/constant"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"rsc.io/diff"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/internal/x86"
	"firefly-os.dev/tools/ruse/parser"
	"firefly-os.dev/tools/ruse/ssafir"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

func TestAllocator(t *testing.T) {
	tests := []struct {
		Name string
		Code string
		Want []*TestValue
		Text []string
	}{
		{
			Name: "no-op",
			Code: `
				(package test)

				(func (test (a string) (b int))
					(let _ a))
			`,
			Want: []*TestValue{
				{ID: 2, Op: ssafir.OpParameter, Extra: &Alloc{Dst: x86.RDI, Data: int64(0)}, Uses: 0, Code: `(a string)`},
				{ID: 2, Op: ssafir.OpParameter, Extra: &Alloc{Dst: x86.RSI, Data: int64(0)}, Uses: 0, Code: `(a string)`},
				{ID: 3, Op: ssafir.OpParameter, Extra: &Alloc{Dst: x86.RDX, Data: int64(1)}, Uses: 0, Code: `(b int)`},
			},
			Text: []string{
				"allocator for test (func (string) (int))",
				"  rax:  [free]",
				"  rcx:  [free]",
				"  rdx:  [free]",
				"  rsi:  [free]",
				"  rdi:  [free]",
				"  r8:   [free]",
				"  r9:   [free]",
				"  r10:  [free]",
				"  r11:  [free]",
				"  rbx:  [free]",
				"  rbp:  [free]",
				"  r12:  [free]",
				"  r13:  [free]",
				"  r14:  [free]",
				"  r15:  [free]",
			},
		},
		{
			Name: "passthrough",
			Code: `
				(package test)

				(func (test (a string) (b int) int)
					(let c b)
					c)
			`,
			Want: []*TestValue{
				{ID: 2, Op: ssafir.OpParameter, Extra: &Alloc{Dst: x86.RDI, Data: int64(0)}, Uses: 0, Code: `(a string)`},
				{ID: 2, Op: ssafir.OpParameter, Extra: &Alloc{Dst: x86.RSI, Data: int64(0)}, Uses: 0, Code: `(a string)`},
				{ID: 3, Op: ssafir.OpParameter, Extra: &Alloc{Dst: x86.RDX, Data: int64(1)}, Uses: 1, Code: `(b int)`},
				{ID: 4, Op: ssafir.OpCopy, Extra: &Alloc{Dst: x86.RDX, Src: x86.RDX}, Uses: 1, Code: `(let c b)`},
				{ID: 5, Op: ssafir.OpMakeResult, Extra: &Alloc{Dst: x86.RAX, Src: x86.RDX}, Uses: 1, Code: `c`},
				{ID: 4, Op: ssafir.OpDrop, Extra: &Alloc{Src: x86.RDX}, Uses: 1, Code: `(let c b)`},
			},
			Text: []string{
				"allocator for test (func (string) (int) int)",
				"  rax:  v5",
				"  rcx:  [free]",
				"  rdx:  [free]",
				"  rsi:  [free]",
				"  rdi:  [free]",
				"  r8:   [free]",
				"  r9:   [free]",
				"  r10:  [free]",
				"  r11:  [free]",
				"  rbx:  [free]",
				"  rbp:  [free]",
				"  r12:  [free]",
				"  r13:  [free]",
				"  r14:  [free]",
				"  r15:  [free]",
			},
		},
		{
			Name: "call",
			Code: `
				(package test)

				'(abi (abi
					(params rdi)
					(result rax)))
				(asm-func (double (in int) int)
					(mov rax rdi)
					(add rax rax)
					(ret))

				(func (test int)
					(let length (len "foobar"))
					(double (len "bar"))
					(double length)
					(double 7)
					(let (val int) 17)
					(double val))
			`,
			Want: []*TestValue{
				{ID: 4, Op: ssafir.OpConstantInt64, Extra: &Alloc{Dst: x86.RDI, Data: int64(3)}, Uses: 1, Code: `(len "bar")`},
				{ID: 5, Op: ssafir.OpFunctionCall, Extra: new(types.Function), Uses: 0, Code: `(double (len "bar"))`},
				{ID: 3, Op: ssafir.OpCopy, Extra: &Alloc{Dst: x86.RDI, Data: int64(6)}, Uses: 1, Code: `(let length (len "foobar"))`},
				{ID: 6, Op: ssafir.OpFunctionCall, Extra: new(types.Function), Uses: 0, Code: `(double length)`},
				{ID: 7, Op: ssafir.OpConstantUntypedInt, Extra: &Alloc{Dst: x86.RDI, Data: constant.MakeInt64(7)}, Uses: 1, Code: `7`},
				{ID: 8, Op: ssafir.OpFunctionCall, Extra: new(types.Function), Uses: 0, Code: `(double 7)`},
				{ID: 10, Op: ssafir.OpConstantUntypedInt, Extra: &Alloc{Dst: x86.RDI, Data: constant.MakeInt64(17)}, Uses: 1, Code: `val`},
				{ID: 11, Op: ssafir.OpFunctionCall, Extra: new(types.Function), Uses: 1, Code: `(double val)`},
				{ID: 11, Op: ssafir.OpMakeResult, Extra: &Alloc{Dst: x86.RAX, Src: x86.RAX}, Uses: 1, Code: `(double val)`},
				{ID: 12, Op: ssafir.OpMakeResult, Extra: &Alloc{Dst: x86.RAX, Src: x86.RAX}, Uses: 1, Code: `(double val)`},
			},
			Text: []string{
				"allocator for test (func int)",
				"  rax:  v12",
				"  rcx:  [free]",
				"  rdx:  [free]",
				"  rsi:  [free]",
				"  rdi:  v10",
				"  r8:   [free]",
				"  r9:   [free]",
				"  r10:  [free]",
				"  r11:  [free]",
				"  rbx:  [free]",
				"  rbp:  [free]",
				"  r12:  [free]",
				"  r13:  [free]",
				"  r14:  [free]",
				"  r15:  [free]",
			},
		},
	}

	compareOptions := []cmp.Option{
		cmpopts.IgnoreTypes(new(types.Function)),
	}

	arch := sys.X86_64
	sizes := types.SizesFor(arch)
	if err := arch.Validate(&arch.DefaultABI); err != nil {
		t.Fatalf("invalid test ABI: %v", err)
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			// Compile the code.
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.ruse", test.Code, 0)
			if err != nil {
				t.Fatalf("failed to parse text: %v", err)
			}

			files := []*ast.File{file}

			info := &types.Info{
				Types:       make(map[ast.Expression]types.TypeAndValue),
				Definitions: make(map[*ast.Identifier]types.Object),
				Uses:        make(map[*ast.Identifier]types.Object),
			}

			testPath := "tests/test"
			pkg, err := types.Check(testPath, fset, files, arch, info)
			if err != nil {
				t.Fatalf("failed to type-check package: %v", err)
			}

			p, err := Compile(fset, arch, pkg, files, info, sizes)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Find the test function.
			var testFunc *ssafir.Function
			for _, fun := range p.Functions {
				if fun.Name == "test" {
					testFunc = fun
					break
				}
			}

			if testFunc == nil {
				names := make([]string, len(p.Functions))
				for i, fun := range p.Functions {
					names[i] = fun.Name
				}

				t.Fatalf("failed to find test function: found %s", strings.Join(names, ", "))
			}

			// Use the allocator.
			a := newAllocator(fset, arch, sizes, testFunc)
			err = a.run()
			if err != nil {
				t.Fatalf("Allocate(): unexpected error: %v", err)
			}

			var testValues []*TestValue
			for _, b := range testFunc.Blocks {
				testValues = append(testValues, ConvertTestValues(fset, test.Code, b.Values)...)
			}

			if diff := cmp.Diff(test.Want, testValues, compareOptions...); diff != "" {
				t.Fatalf("Allocate(): (-want, +got)\n%s", diff)
			}

			t.Log(testFunc.Print())
			gotText := a.Debug()
			text := strings.Join(test.Text, "\n") + "\n"
			if gotText != text {
				t.Fatalf("allocator.Debug(): (+got, -want)\n%s", diff.Format(text, gotText))
			}
		})
	}
}
