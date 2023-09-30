// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package compiler

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/internal/x86"
	"firefly-os.dev/tools/ruse/parser"
	"firefly-os.dev/tools/ruse/ssafir"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

func TestLower(t *testing.T) {
	tests := []struct {
		Name string
		Code string
		Want []*TestValue
	}{
		{
			Name: "no-op",
			Code: `
				(package test)

				(func (test (a string) (b int))
					(let _ a))
			`,
			Want: []*TestValue{
				{ID: 0, Op: ssafir.OpX86RET, Extra: &x86InstructionData{Length: 1}, Uses: 0, Code: `)`},
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
				{ID: 5, Op: ssafir.OpX86MOV_R64_Rmr64_REX, Extra: &x86InstructionData{Args: [4]any{x86.RAX, x86.RDX}, Length: 3}, Uses: 1, Code: `c`},
				{ID: 5, Op: ssafir.OpX86RET, Extra: &x86InstructionData{Length: 1}, Uses: 1, Code: `c`},
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
				{
					ID: 4,
					Op: ssafir.OpX86MOV_R32op_Imm32,
					Extra: &x86InstructionData{
						Args: [4]any{
							x86.EDI,
							uint64(3),
						},
						Length: 5,
					},
					Uses: 1,
					Code: `(len "bar")`,
				},
				{
					ID: 5,
					Op: ssafir.OpX86CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     221,
								Name:    "tests/test.double",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  6,
								Address: 0x0a,
							},
						},
						Length: 5,
					},
					Uses: 0,
					Code: `(double (len "bar"))`,
				},
				{
					ID: 3,
					Op: ssafir.OpX86MOV_R32op_Imm32,
					Extra: &x86InstructionData{
						Args: [4]any{
							x86.EDI,
							uint64(6),
						},
						Length: 5,
					},
					Uses: 1,
					Code: `(let length (len "foobar"))`,
				},
				{
					ID: 6,
					Op: ssafir.OpX86CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     247,
								Name:    "tests/test.double",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  16,
								Address: 0x14,
							},
						},
						Length: 5,
					},
					Uses: 0,
					Code: `(double length)`,
				},
				{
					ID: 7,
					Op: ssafir.OpX86MOV_R32op_Imm32,
					Extra: &x86InstructionData{
						Args: [4]any{
							x86.EDI,
							uint64(7),
						},
						Length: 5,
					},
					Uses: 1,
					Code: `7`,
				},
				{
					ID: 8,
					Op: ssafir.OpX86CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     268,
								Name:    "tests/test.double",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  26,
								Address: 0x1e,
							},
						},
						Length: 5,
					},
					Uses: 0,
					Code: `(double 7)`,
				},
				{
					ID: 10,
					Op: ssafir.OpX86MOV_R32op_Imm32,
					Extra: &x86InstructionData{
						Args: [4]any{
							x86.EDI,
							uint64(17),
						},
						Length: 5,
					},
					Uses: 1,
					Code: `val`,
				},
				{
					ID: 11,
					Op: ssafir.OpX86CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     308,
								Name:    "tests/test.double",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  36,
								Address: 0x28,
							},
						},
						Length: 5,
					},
					Uses: 1,
					Code: `(double val)`,
				},
				{
					ID: 12,
					Op: ssafir.OpX86RET,
					Extra: &x86InstructionData{
						Length: 1,
					},
					Uses: 1,
					Code: `(double val)`,
				},
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
			err = Allocate(fset, arch, sizes, testFunc)
			if err != nil {
				t.Fatalf("Allocate(): unexpected error: %v", err)
			}

			// Lower the instructions.
			err = Lower(fset, arch, sizes, testFunc)
			if err != nil {
				t.Fatalf("Lower(): unexpected error: %v", err)
			}

			var testValues []*TestValue
			for _, b := range testFunc.Blocks {
				testValues = append(testValues, ConvertTestValues(fset, test.Code, b.Values)...)
			}

			if diff := cmp.Diff(test.Want, testValues, compareOptions...); diff != "" {
				t.Fatalf("Lower(): (-want, +got)\n%s", diff)
			}
		})
	}
}
