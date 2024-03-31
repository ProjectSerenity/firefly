// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package compiler

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/arch/x86/x86asm"
	"rsc.io/diff"

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
		Name   string
		Code   string
		Disasm []string
		Want   []*TestValue
	}{
		{
			Name: "no-op",
			Code: `
				(package test)

				(func (test (a string) (b int))
					(let _ a))
			`,
			Disasm: []string{
				"000000:	c3                   	ret",
			},
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
			Disasm: []string{
				"000000:	48 8b c2             	mov rax, rdx",
				"000003:	c3                   	ret",
			},
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
			Disasm: []string{
				"000000:	bf 03 00 00 00       	mov edi, 0x3",    // Prepare arg (len "bar")
				"000005:	e8 3f 33 22 11       	call 0x11223349", // Call func   (double (len "bar"))
				"00000a:	bf 06 00 00 00       	mov edi, 0x6",    // Prepare arg (let length (len "foobar")))
				"00000f:	e8 3f 33 22 11       	call 0x11223353", // Call func   (double length)
				"000014:	bf 07 00 00 00       	mov edi, 0x7",    // Prepare arg 7
				"000019:	e8 3f 33 22 11       	call 0x1122335d", // Call func   (double 7)
				"00001e:	bf 11 00 00 00       	mov edi, 0x11",   // Prepare arg (let (val int) 17)
				"000023:	e8 3f 33 22 11       	call 0x11223367", // Call func   (double val)
				"000028:	c3                   	ret",             // Return      (double val)
			},
			Want: []*TestValue{
				{
					ID:    4,
					Op:    ssafir.OpX86MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EDI, uint64(3)}, Length: 5},
					Uses:  1,
					Code:  `(len "bar")`,
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
					ID:    3,
					Op:    ssafir.OpX86MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EDI, uint64(6)}, Length: 5},
					Uses:  1,
					Code:  `(let length (len "foobar"))`,
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
					ID:    7,
					Op:    ssafir.OpX86MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EDI, uint64(7)}, Length: 5},
					Uses:  1,
					Code:  `7`,
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
					ID:    10,
					Op:    ssafir.OpX86MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EDI, uint64(17)}, Length: 5},
					Uses:  1,
					Code:  `val`,
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
					ID:    12,
					Op:    ssafir.OpX86RET,
					Extra: &x86InstructionData{Length: 1},
					Uses:  1,
					Code:  `(double val)`,
				},
			},
		},
		{
			Name: "arithmetic",
			Code: `
				(package test)

				'(abi (abi
					(params rax)
					(result rax)))
				(asm-func (copy-n (n int) int)
					(ret))

				(func (test int)
					(let a (copy-n 7))
					(let b (copy-n 3))
					(let ub (int->uint b))
					(let sum (+ a b))     ; 10
					(let dif (- a b))     ;  4
					(let mul (* a b))     ; 21
					(let div (/ a b))     ;  2
					(let and (& a b))     ;  3
					(let big (<< a ub))   ; 56
					(let sml (>> mul ub)) ;  2
					(| sum dif mul div and big sml)) ; 63
			`,
			Disasm: []string{
				"000000:	b8 07 00 00 00       	mov eax, 0x7",    // Prepare arg 7
				"000005:	e8 3f 33 22 11       	call 0x11223349", // Call func   (copy-n 7)
				"00000a:	48 8b c8             	mov rcx, rax",    // Save result (let a (copy-n 7))
				"00000d:	b8 03 00 00 00       	mov eax, 0x3",    // Prepare arg 3
				"000012:	e8 3f 33 22 11       	call 0x11223356", // Call func   (copy-n 3)
				"000017:	48 8b d0             	mov rdx, rax",    // Save result (let b (copy-n 3))
				"00001a:	48 8b f1             	mov rsi, rcx",    // Prepare arg a
				"00001d:	48 03 f0             	add rsi, rax",    // Arithmetic  (+ a b)
				"000020:	48 8b f9             	mov rdi, rcx",    // Prepare arg a
				"000023:	48 2b f8             	sub rdi, rax",    // Arithmetic  (- a b)
				"000026:	4c 8b c2             	mov r8, rdx",     // Save result (let ub (int->uint b))
				"000029:	4c 8b c8             	mov r9, rax",     // Prepare arg b
				"00002c:	48 8b c1             	mov rax, rcx",    // Prepare arg a
				"00002f:	49 f7 e1             	mul r9",          // Arithmetic  (* a b)
				"000032:	4c 8b d0             	mov r10, rax",    // Save result (let mul (* a b))
				"000035:	48 8b c1             	mov rax, rcx",    // Prepare arg a
				"000038:	49 f7 f1             	div r9",          // Arithmetic  (/ a b)
				"00003b:	48 8b d1             	mov rdx, rcx",    // Prepare arg a
				"00003e:	49 23 d1             	and rdx, r9",     // Arithmetic  (& a b)
				"000041:	4c 8b c9             	mov r9, rcx",     // Prepare arg a
				"000044:	4d 8b d9             	mov r11, r9",     // Save arg    a
				"000047:	49 8b c8             	mov rcx, r8",     // Prepare arg ub
				"00004a:	49 d3 e3             	shl r11, cl",     // Arithmetic  (<< a ub)
				"00004d:	4c 8b c9             	mov r9, rcx",     // Save arg    b
				"000050:	49 8b da             	mov rbx, r10",    // Prepare arg mul
				"000053:	49 8b c8             	mov rcx, r8",     // Prepare arg ub
				"000056:	48 d3 fb             	sar rbx, cl",     // Arithmetic  (>> mul ub)
				"000059:	4c 8b c6             	mov r8, rsi",     // Prepare arg sum
				"00005c:	4c 0b c7             	or r8, rdi",      // Arithmetic  (| sum dif)
				"00005f:	4d 0b c2             	or r8, r10",      // Arithmetic  (| sum dif mul)
				"000062:	4c 0b c0             	or r8, rax",      // Arithmetic  (| sum ... mul div)
				"000065:	4c 0b c2             	or r8, rdx",      // Arithmetic  (| sum ... div and)
				"000068:	4d 0b c3             	or r8, r11",      // Arithmetic  (| sum ... and big)
				"00006b:	4c 0b c3             	or r8, rbx",      // Arithmetic  (| sum ... big sml)
				"00006e:	49 8b c0             	mov rax, r8",     // Save result (| sum ... sml)
				"000071:	c3                   	ret",
			},
			Want: []*TestValue{
				{
					ID:    2,
					Op:    ssafir.OpX86MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EAX, uint64(7)}, Length: 5},
					Uses:  1,
					Code:  `7`,
				},
				{
					ID: 3,
					Op: ssafir.OpX86CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     156,
								Name:    "tests/test.copy-n",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  6,
								Address: 0x0a,
							},
						},
						Length: 5,
					},
					Uses: 1,
					Code: `(copy-n 7)`,
				},
				{
					ID:    4,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RCX, x86.RAX}, Length: 3},
					Uses:  6,
					Code:  `(let a (copy-n 7))`,
				},
				{
					ID:    5,
					Op:    ssafir.OpX86MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EAX, uint64(3)}, Length: 5},
					Uses:  1,
					Code:  `3`,
				},
				{
					ID: 6,
					Op: ssafir.OpX86CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     180,
								Name:    "tests/test.copy-n",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  19,
								Address: 0x17,
							},
						},
						Length: 5,
					},
					Uses: 1,
					Code: `(copy-n 3)`,
				},
				{
					ID:    8,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDX, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  `(int->uint b)`,
				},
				{
					ID:    10,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RSI, x86.RCX}, Length: 3},
					Uses:  1,
					Code:  `a b`,
				},
				{
					ID:    10,
					Op:    ssafir.OpX86ADD_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RSI, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  `a b`,
				},
				{
					ID:    12,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDI, x86.RCX}, Length: 3},
					Uses:  1,
					Code:  `a b`,
				},
				{
					ID:    12,
					Op:    ssafir.OpX86SUB_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDI, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    9,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.RDX}, Length: 3},
					Uses:  2,
					Code:  "(let ub (int->uint b))",
				},
				{
					ID:    7,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R9, x86.RAX}, Length: 3},
					Uses:  6,
					Code:  "(let b (copy-n 3))",
				},
				{
					ID:    14,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RAX, x86.RCX}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    14,
					Op:    ssafir.OpX86MUL_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R9}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    15,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R10, x86.RAX}, Length: 3},
					Uses:  2,
					Code:  "(let mul (* a b))",
				},
				{
					ID:    16,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RAX, x86.RCX}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    16,
					Op:    ssafir.OpX86DIV_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R9}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    18,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDX, x86.RCX}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    18,
					Op:    ssafir.OpX86AND_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDX, x86.R9}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    4,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R9, x86.RCX}, Length: 3},
					Uses:  6,
					Code:  "(let a (copy-n 7))",
				},
				{
					ID:    20,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R11, x86.R9}, Length: 3},
					Uses:  1,
					Code:  "a ub",
				},
				{
					ID:    20,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RCX, x86.R8}, Length: 3},
					Uses:  1,
					Code:  "a ub",
				},
				{
					ID:    20,
					Op:    ssafir.OpX86SAL_Rmr64_CL_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R11, x86.CL}, Length: 3},
					Uses:  1,
					Code:  "a ub",
				},
				{
					ID:    9,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R9, x86.RCX}, Length: 3},
					Uses:  2,
					Code:  "(let ub (int->uint b))",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RBX, x86.R10}, Length: 3},
					Uses:  1,
					Code:  "mul ub",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RCX, x86.R8}, Length: 3},
					Uses:  1,
					Code:  "mul ub",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86SAR_Rmr64_CL_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RBX, x86.CL}, Length: 3},
					Uses:  1,
					Code:  "mul ub",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.RSI}, Length: 3},
					Uses:  1,
					Code:  "sum dif",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86OR_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.RDI}, Length: 3},
					Uses:  1,
					Code:  "sum dif",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86OR_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.R10}, Length: 3},
					Uses:  1,
					Code:  "dif mul",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86OR_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  "mul div",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86OR_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.RDX}, Length: 3},
					Uses:  1,
					Code:  "div and",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86OR_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.R11}, Length: 3},
					Uses:  1,
					Code:  "and big",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86OR_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.RBX}, Length: 3},
					Uses:  1,
					Code:  "big sml",
				},
				{
					ID:    25,
					Op:    ssafir.OpX86MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RAX, x86.R8}, Length: 3},
					Uses:  1,
					Code:  "(| sum dif mul div and big sml)",
				},
				{
					ID:    25,
					Op:    ssafir.OpX86RET,
					Extra: &x86InstructionData{Length: 1},
					Uses:  1,
					Code:  "(| sum dif mul div and big sml)",
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

	var code bytes.Buffer
	var disasm strings.Builder
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
			err = Allocate(fset, arch, sizes, p, testFunc)
			if err != nil {
				t.Fatalf("Allocate(): unexpected error: %v", err)
			}

			// Lower the instructions.
			err = Lower(fset, arch, sizes, testFunc)
			if err != nil {
				t.Fatalf("Lower(): unexpected error: %v", err)
			}

			// Encode the instructions.
			code.Reset()
			err = EncodeTo(&code, fset, arch, testFunc)
			if err != nil {
				t.Fatalf("EncodeTo(): unexpected error: %v", err)
			}

			// Check the disassembly.
			disasm.Reset()
			src := code.Bytes()
			var pc uint64
			for len(src) > 0 {
				inst, err := x86asm.Decode(src, 64)
				if err != nil {
					t.Fatalf("x86asm.Decode(): unexpected error: %v", err)
				}

				size := inst.Len
				if size == 0 {
					size = 1
				}

				fmt.Fprintf(&disasm, "%06x:\t% -21x\t", pc, src[:size])
				disasm.WriteString(x86asm.IntelSyntax(inst, pc, nil))
				disasm.WriteByte('\n')

				src = src[size:]
				pc += uint64(size)
			}

			got := disasm.String()
			want := strings.Join(test.Disasm, "\n") + "\n"
			if got != want {
				t.Errorf("Lower(): (-want, +got)\n%s", diff.Format(want, got))
			}

			var testValues []*TestValue
			for _, b := range testFunc.Blocks {
				testValues = append(testValues, ConvertTestValues(fset, test.Code, b.Values)...)
			}

			if diff := cmp.Diff(test.Want, testValues, compareOptions...); diff != "" {
				t.Errorf("Lower(): (-want, +got)\n%s", diff)
			}
		})
	}
}
