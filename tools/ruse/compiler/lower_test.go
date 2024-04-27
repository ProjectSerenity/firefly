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

	"firefly-os.dev/tools/diff"
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
		Arch   *sys.Arch
		Code   string
		Disasm []string
		Want   []*TestValue
	}{
		{
			Name: "no-op",
			Arch: sys.X86_64,
			Code: `
				(package test)

				(func (test (a string) (b int))
					(let _ a))
			`,
			Disasm: []string{
				"000000:	f3 0f 1e fa          	endbr64",
				"000004:	c3                   	ret",
			},
			Want: []*TestValue{
				{ID: 0, Op: ssafir.OpX86_ENDBR64, Extra: &x86InstructionData{Length: 4}, Uses: 0, Code: `func (test (a string) (b int))`},
				{ID: 0, Op: ssafir.OpX86_RET, Extra: &x86InstructionData{Length: 1}, Uses: 0, Code: `)`},
			},
		},
		{
			Name: "passthrough",
			Arch: sys.X86_64,
			Code: `
				(package test)

				(func (test (a string) (b int) int)
					(let c b)
					c)
			`,
			Disasm: []string{
				"000000:	f3 0f 1e fa          	endbr64",
				"000004:	48 8b c2             	mov rax, rdx",
				"000007:	c3                   	ret",
			},
			Want: []*TestValue{
				{ID: 0, Op: ssafir.OpX86_ENDBR64, Extra: &x86InstructionData{Length: 4}, Uses: 0, Code: `func (test (a string) (b int) int)`},
				{ID: 5, Op: ssafir.OpX86_MOV_R64_Rmr64_REX, Extra: &x86InstructionData{Args: [4]any{x86.RAX, x86.RDX}, Length: 3}, Uses: 1, Code: `c`},
				{ID: 5, Op: ssafir.OpX86_RET, Extra: &x86InstructionData{Length: 1}, Uses: 1, Code: `c`},
			},
		},
		{
			Name: "call",
			Arch: sys.X86_64,
			Code: `
				(package test)

				'(abi
					(params rdi)
					(result rax))
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
				"000000:	f3 0f 1e fa          	endbr64",
				"000004:	bf 03 00 00 00       	mov edi, 0x3",    // Prepare arg (len "bar")
				"000009:	e8 3f 33 22 11       	call 0x1122334d", // Call func   (double (len "bar"))
				"00000e:	bf 06 00 00 00       	mov edi, 0x6",    // Prepare arg (let length (len "foobar")))
				"000013:	e8 3f 33 22 11       	call 0x11223357", // Call func   (double length)
				"000018:	bf 07 00 00 00       	mov edi, 0x7",    // Prepare arg 7
				"00001d:	e8 3f 33 22 11       	call 0x11223361", // Call func   (double 7)
				"000022:	bf 11 00 00 00       	mov edi, 0x11",   // Prepare arg (let (val int) 17)
				"000027:	e8 3f 33 22 11       	call 0x1122336b", // Call func   (double val)
				"00002c:	c3                   	ret",             // Return      (double val)
			},
			Want: []*TestValue{
				{ID: 0, Op: ssafir.OpX86_ENDBR64, Extra: &x86InstructionData{Length: 4}, Uses: 0, Code: `func (test int)`},
				{
					ID:    4,
					Op:    ssafir.OpX86_MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EDI, uint64(3)}, Length: 5},
					Uses:  1,
					Code:  `(len "bar")`,
				},
				{
					ID: 5,
					Op: ssafir.OpX86_CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     215,
								Name:    "tests/test.double",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  10,
								Address: 0x0e,
							},
						},
						Length: 5,
					},
					Uses: 1,
					Code: `(double (len "bar"))`,
				},
				{
					ID:    3,
					Op:    ssafir.OpX86_MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EDI, uint64(6)}, Length: 5},
					Uses:  1,
					Code:  `(let length (len "foobar"))`,
				},
				{
					ID: 7,
					Op: ssafir.OpX86_CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     241,
								Name:    "tests/test.double",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  20,
								Address: 0x18,
							},
						},
						Length: 5,
					},
					Uses: 1,
					Code: `(double length)`,
				},
				{
					ID:    9,
					Op:    ssafir.OpX86_MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EDI, uint64(7)}, Length: 5},
					Uses:  1,
					Code:  `7`,
				},
				{
					ID: 10,
					Op: ssafir.OpX86_CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     262,
								Name:    "tests/test.double",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  30,
								Address: 0x22,
							},
						},
						Length: 5,
					},
					Uses: 1,
					Code: `(double 7)`,
				},
				{
					ID:    13,
					Op:    ssafir.OpX86_MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EDI, uint64(17)}, Length: 5},
					Uses:  1,
					Code:  `val`,
				},
				{
					ID: 14,
					Op: ssafir.OpX86_CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     302,
								Name:    "tests/test.double",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  40,
								Address: 0x2c,
							},
						},
						Length: 5,
					},
					Uses: 1,
					Code: `(double val)`,
				},
				{
					ID:    16,
					Op:    ssafir.OpX86_RET,
					Extra: &x86InstructionData{Length: 1},
					Uses:  1,
					Code:  `(double val)`,
				},
			},
		},
		{
			Name: "arithmetic",
			Arch: sys.X86_64,
			Code: `
				(package test)

				'(abi
					(params rax)
					(result rax))
				(asm-func (copy-n (n int) int)
					(ret))

				(func (test int)
					(let a (copy-n 7))
					(let b (copy-n 3))
					(let ub (int->uint b))
					(let sum (+ a b))     ; 10
					(let dif (- a b))     ;  4
					(let mul (× a b))     ; 21
					(let div (÷ a b))     ;  2
					(let bnd (and a b))   ;  3
					(let big (<< a ub))   ; 56
					(let sml (>> mul ub)) ;  2
					(or sum dif mul div bnd big sml)) ; 63
			`,
			Disasm: []string{
				"000000:	f3 0f 1e fa          	endbr64",
				"000004:	53                   	push rbx",        // Save rbx
				"000005:	b8 07 00 00 00       	mov eax, 0x7",    // Prepare arg 7
				"00000a:	e8 3f 33 22 11       	call 0x1122334e", // Call func   (copy-n 7)
				"00000f:	48 8b c8             	mov rcx, rax",    // Save result (let a (copy-n 7))
				"000012:	b8 03 00 00 00       	mov eax, 0x3",    // Prepare arg 3
				"000017:	e8 3f 33 22 11       	call 0x1122335b", // Call func   (copy-n 3)
				"00001c:	48 8b d0             	mov rdx, rax",    // Save result (let b (copy-n 3))
				"00001f:	48 8b f1             	mov rsi, rcx",    // Prepare arg a
				"000022:	48 03 f0             	add rsi, rax",    // Arithmetic  (+ a b)
				"000025:	48 8b f9             	mov rdi, rcx",    // Prepare arg a
				"000028:	48 2b f8             	sub rdi, rax",    // Arithmetic  (- a b)
				"00002b:	4c 8b c2             	mov r8, rdx",     // Save result (let ub (int->uint b))
				"00002e:	4c 8b c8             	mov r9, rax",     // Prepare arg b
				"000031:	48 8b c1             	mov rax, rcx",    // Prepare arg a
				"000034:	49 f7 e1             	mul r9",          // Arithmetic  (× a b)
				"000037:	4c 8b d0             	mov r10, rax",    // Save result (let mul (* a b))
				"00003a:	48 8b c1             	mov rax, rcx",    // Prepare arg a
				"00003d:	48 33 d2             	xor rdx, rdx",    // Clear RDX
				"000040:	49 f7 f1             	div r9",          // Arithmetic  (÷ a b)
				"000043:	48 8b d1             	mov rdx, rcx",    // Prepare arg a
				"000046:	49 23 d1             	and rdx, r9",     // Arithmetic  (and a b)
				"000049:	4c 8b c9             	mov r9, rcx",     // Prepare arg a
				"00004c:	4d 8b d9             	mov r11, r9",     // Save arg    a
				"00004f:	49 8b c8             	mov rcx, r8",     // Prepare arg ub
				"000052:	49 d3 e3             	shl r11, cl",     // Arithmetic  (<< a ub)
				"000055:	4c 8b c9             	mov r9, rcx",     // Save arg    b
				"000058:	49 8b da             	mov rbx, r10",    // Prepare arg mul
				"00005b:	49 8b c8             	mov rcx, r8",     // Prepare arg ub
				"00005e:	48 d3 fb             	sar rbx, cl",     // Arithmetic  (>> mul ub)
				"000061:	4c 8b c6             	mov r8, rsi",     // Prepare arg sum
				"000064:	4c 0b c7             	or r8, rdi",      // Arithmetic  (or sum dif)
				"000067:	4d 0b c2             	or r8, r10",      // Arithmetic  (or sum dif mul)
				"00006a:	4c 0b c0             	or r8, rax",      // Arithmetic  (or sum ... mul div)
				"00006d:	4c 0b c2             	or r8, rdx",      // Arithmetic  (or sum ... div bnd)
				"000070:	4d 0b c3             	or r8, r11",      // Arithmetic  (or sum ... bnd big)
				"000073:	4c 0b c3             	or r8, rbx",      // Arithmetic  (or sum ... big sml)
				"000076:	49 8b c0             	mov rax, r8",     // Save result (or sum ... sml)
				"000079:	5b                   	pop rbx",         // Restore rbx
				"00007a:	c3                   	ret",
			},
			Want: []*TestValue{
				{ID: 0, Op: ssafir.OpX86_ENDBR64, Extra: &x86InstructionData{Length: 4}, Uses: 0, Code: `func (test int)`},
				{
					ID:    0,
					Op:    ssafir.OpX86_PUSH_R64op,
					Extra: &x86InstructionData{Args: [4]any{x86.RBX}, Length: 1},
					Uses:  0,
					Code:  `func (test int)`,
				},
				{
					ID:    2,
					Op:    ssafir.OpX86_MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EAX, uint64(7)}, Length: 5},
					Uses:  1,
					Code:  `7`,
				},
				{
					ID: 3,
					Op: ssafir.OpX86_CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     150,
								Name:    "tests/test.copy-n",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  11,
								Address: 0x0f,
							},
						},
						Length: 5,
					},
					Uses: 1,
					Code: `(copy-n 7)`,
				},
				{
					ID:    5,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RCX, x86.RAX}, Length: 3},
					Uses:  6,
					Code:  `(let a (copy-n 7))`,
				},
				{
					ID:    6,
					Op:    ssafir.OpX86_MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EAX, uint64(3)}, Length: 5},
					Uses:  1,
					Code:  `3`,
				},
				{
					ID: 7,
					Op: ssafir.OpX86_CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     174,
								Name:    "tests/test.copy-n",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  24,
								Address: 0x1c,
							},
						},
						Length: 5,
					},
					Uses: 1,
					Code: `(copy-n 3)`,
				},
				{
					ID:    10,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDX, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  `(int->uint b)`,
				},
				{
					ID:    12,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RSI, x86.RCX}, Length: 3},
					Uses:  1,
					Code:  `a b`,
				},
				{
					ID:    12,
					Op:    ssafir.OpX86_ADD_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RSI, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  `a b`,
				},
				{
					ID:    14,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDI, x86.RCX}, Length: 3},
					Uses:  1,
					Code:  `a b`,
				},
				{
					ID:    14,
					Op:    ssafir.OpX86_SUB_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDI, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    11,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.RDX}, Length: 3},
					Uses:  2,
					Code:  "(let ub (int->uint b))",
				},
				{
					ID:    9,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R9, x86.RAX}, Length: 3},
					Uses:  6,
					Code:  "(let b (copy-n 3))",
				},
				{
					ID:    16,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RAX, x86.RCX}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    16,
					Op:    ssafir.OpX86_MUL_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R9}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    17,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R10, x86.RAX}, Length: 3},
					Uses:  2,
					Code:  "(let mul (× a b))",
				},
				{
					ID:    18,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RAX, x86.RCX}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    18,
					Op:    ssafir.OpX86_XOR_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDX, x86.RDX}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    18,
					Op:    ssafir.OpX86_DIV_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R9}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    20,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDX, x86.RCX}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    20,
					Op:    ssafir.OpX86_AND_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDX, x86.R9}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    5,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R9, x86.RCX}, Length: 3},
					Uses:  6,
					Code:  "(let a (copy-n 7))",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R11, x86.R9}, Length: 3},
					Uses:  1,
					Code:  "a ub",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RCX, x86.R8}, Length: 3},
					Uses:  1,
					Code:  "a ub",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86_SAL_Rmr64_CL_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R11, x86.CL}, Length: 3},
					Uses:  1,
					Code:  "a ub",
				},
				{
					ID:    11,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R9, x86.RCX}, Length: 3},
					Uses:  2,
					Code:  "(let ub (int->uint b))",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RBX, x86.R10}, Length: 3},
					Uses:  1,
					Code:  "mul ub",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RCX, x86.R8}, Length: 3},
					Uses:  1,
					Code:  "mul ub",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86_SAR_Rmr64_CL_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RBX, x86.CL}, Length: 3},
					Uses:  1,
					Code:  "mul ub",
				},
				{
					ID:    26,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.RSI}, Length: 3},
					Uses:  1,
					Code:  "sum dif",
				},
				{
					ID:    26,
					Op:    ssafir.OpX86_OR_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.RDI}, Length: 3},
					Uses:  1,
					Code:  "sum dif",
				},
				{
					ID:    26,
					Op:    ssafir.OpX86_OR_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.R10}, Length: 3},
					Uses:  1,
					Code:  "dif mul",
				},
				{
					ID:    26,
					Op:    ssafir.OpX86_OR_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  "mul div",
				},
				{
					ID:    26,
					Op:    ssafir.OpX86_OR_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.RDX}, Length: 3},
					Uses:  1,
					Code:  "div bnd",
				},
				{
					ID:    26,
					Op:    ssafir.OpX86_OR_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.R11}, Length: 3},
					Uses:  1,
					Code:  "bnd big",
				},
				{
					ID:    26,
					Op:    ssafir.OpX86_OR_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R8, x86.RBX}, Length: 3},
					Uses:  1,
					Code:  "big sml",
				},
				{
					ID:    27,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RAX, x86.R8}, Length: 3},
					Uses:  1,
					Code:  "(or sum dif mul div bnd big sml)",
				},
				{
					ID:    0,
					Op:    ssafir.OpX86_POP_R64op,
					Extra: &x86InstructionData{Args: [4]any{x86.RBX}, Length: 1},
					Uses:  0,
					Code:  `func (test int)`,
				},
				{
					ID:    27,
					Op:    ssafir.OpX86_RET,
					Extra: &x86InstructionData{Length: 1},
					Uses:  1,
					Code:  "(or sum dif mul div bnd big sml)",
				},
			},
		},
		{
			Name: "booleans",
			Arch: sys.X86_64,
			Code: `
				(package test)

				'(abi
					(params rax)
					(result rax))
				(asm-func (copy-n (n int) int)
					(ret))

				(func (test bool)
					(let a (copy-n 7))
					(let b (copy-n 3))
					(let gtr (> a b))     ; true
					(let geq (>= a b))    ; true
					(let lss (< a b))     ; false
					(let leq (<= a b))    ; false
					(let eql (= a b))     ; false
					(let neq (!= a b))    ; true
					(let all (and gtr geq lss leq eql neq)) ; false
					(let any (or gtr geq lss leq eql neq))  ; true
					(or all any (= a a))) ; true
			`,
			Disasm: []string{
				"000000:	f3 0f 1e fa          	endbr64",
				"000004:	b8 07 00 00 00       	mov eax, 0x7",    // Prepare arg 7
				"000009:	e8 3f 33 22 11       	call 0x1122334d", // Call func   (copy-n 7)
				"00000e:	48 8b c8             	mov rcx, rax",    // Save result (let a (copy-n 7))
				"000011:	b8 03 00 00 00       	mov eax, 0x3",    // Prepare arg 3
				"000016:	e8 3f 33 22 11       	call 0x1122335a", // Call func   (copy-n 3)
				"00001b:	48 3b c8             	cmp rcx, rax",    // Compare     (> a b)
				"00001e:	0f 9f c2             	setnle dl",       // Perform     (> a b)
				"000021:	48 3b c8             	cmp rcx, rax",    // Compare     (>= a b)
				"000024:	40 0f 9d c6          	setnl sil",       // Perform     (>= a b)
				"000028:	48 3b c8             	cmp rcx, rax",    // Compare     (< a b)
				"00002b:	40 0f 9c c7          	setl dil",        // Perform     (< a b)
				"00002f:	48 3b c8             	cmp rcx, rax",    // Compare     (<= a b)
				"000032:	41 0f 9e c0          	setle r8b",       // Perform     (<= a b)
				"000036:	48 3b c8             	cmp rcx, rax",    // Compare     (= a b)
				"000039:	41 0f 94 c1          	setz r9b",        // Perform     (= a b)
				"00003d:	48 3b c8             	cmp rcx, rax",    // Compare     (!= a b)
				"000040:	41 0f 95 c2          	setnz r10b",      // Perform     (!= a b)
				"000044:	40 22 d6             	and dl, sil",     // Perform     (and gtr geq)
				"000047:	0f 95 c0             	setnz al",        // Save result (and gtr geq)
				"00004a:	40 22 c7             	and al, dil",     // Perform     (and gtr geq lss)
				"00004d:	0f 95 c0             	setnz al",        // Save result (and gtr geq lss)
				"000050:	41 22 c0             	and al, r8b",     // Perform     (and gtr ... lss leq)
				"000053:	0f 95 c0             	setnz al",        // Save result (and gtr ... lss leq)
				"000056:	41 22 c1             	and al, r9b",     // Perform     (and gtr ... leq eql)
				"000059:	0f 95 c0             	setnz al",        // Save result (and gtr ... leq eql)
				"00005c:	41 22 c2             	and al, r10b",    // Perform     (and gtr ... eql neq)
				"00005f:	0f 95 c0             	setnz al",        // Save result (and gtr ... eql neq)
				"000062:	40 0a d6             	or dl, sil",      // Perform     (or gtr geq)
				"000065:	41 0f 95 c3          	setnz r11b",      // Save result (or gtr geq)
				"000069:	44 0a df             	or r11b, dil",    // Perform     (or gtr geq lss)
				"00006c:	41 0f 95 c3          	setnz r11b",      // Save result (or gtr geq lss)
				"000070:	45 0a d8             	or r11b, r8b",    // Perform     (or gtr ... lss leq)
				"000073:	41 0f 95 c3          	setnz r11b",      // Save result (or gtr ... lss leq)
				"000077:	45 0a d9             	or r11b, r9b",    // Perform     (or gtr ... leq eql)
				"00007a:	41 0f 95 c3          	setnz r11b",      // Save result (or gtr ... leq eql)
				"00007e:	45 0a da             	or r11b, r10b",   // Perform     (or gtr ... eql neq)
				"000081:	41 0f 95 c3          	setnz r11b",      // Save result (or gtr ... eql neq)
				"000085:	48 3b c9             	cmp rcx, rcx",    // Compare     (= a a)
				"000088:	0f 94 c2             	setz dl",         // Perform     (= a a)
				"00008b:	41 0a c3             	or al, r11b",     // Perform     (or all any)
				"00008e:	0f 95 c1             	setnz cl",        // Save result (or all any)
				"000091:	0a ca                	or cl, dl",       // Perform     (or all any (= a a))
				"000093:	0f 95 c1             	setnz cl",        // Save result (or all any (= a a))
				"000096:	48 8b c1             	mov rax, rcx",    // Return      (or all any)
				"000099:	c3                   	ret",
			},
			Want: []*TestValue{
				{ID: 0, Op: ssafir.OpX86_ENDBR64, Extra: &x86InstructionData{Length: 4}, Uses: 0, Code: `func (test bool)`},
				{
					ID:    2,
					Op:    ssafir.OpX86_MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EAX, uint64(7)}, Length: 5},
					Uses:  1,
					Code:  `7`,
				},
				{
					ID: 3,
					Op: ssafir.OpX86_CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     151,
								Name:    "tests/test.copy-n",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  10,
								Address: 0x0e,
							},
						},
						Length: 5,
					},
					Uses: 1,
					Code: `(copy-n 7)`,
				},
				{
					ID:    5,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RCX, x86.RAX}, Length: 3},
					Uses:  8,
					Code:  `(let a (copy-n 7))`,
				},
				{
					ID:    6,
					Op:    ssafir.OpX86_MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EAX, uint64(3)}, Length: 5},
					Uses:  1,
					Code:  `3`,
				},
				{
					ID: 7,
					Op: ssafir.OpX86_CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     175,
								Name:    "tests/test.copy-n",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  23,
								Address: 0x1b,
							},
						},
						Length: 5,
					},
					Uses: 1,
					Code: `(copy-n 3)`,
				},
				{
					ID:    10,
					Op:    ssafir.OpX86_CMP_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RCX, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    10,
					Op:    ssafir.OpX86_SETG_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.DL}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    12,
					Op:    ssafir.OpX86_CMP_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RCX, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    12,
					Op:    ssafir.OpX86_SETGE_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.SIL}, Length: 4},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    14,
					Op:    ssafir.OpX86_CMP_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RCX, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    14,
					Op:    ssafir.OpX86_SETL_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.DIL}, Length: 4},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    16,
					Op:    ssafir.OpX86_CMP_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RCX, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    16,
					Op:    ssafir.OpX86_SETLE_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.R8L}, Length: 4},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    18,
					Op:    ssafir.OpX86_CMP_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RCX, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    18,
					Op:    ssafir.OpX86_SETE_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.R9L}, Length: 4},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    20,
					Op:    ssafir.OpX86_CMP_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RCX, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    20,
					Op:    ssafir.OpX86_SETNE_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.R10L}, Length: 4},
					Uses:  1,
					Code:  "a b",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86_AND_R8_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.DL, x86.SIL}, Length: 3},
					Uses:  1,
					Code:  "gtr geq",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86_SETNZ_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.AL}, Length: 3},
					Uses:  1,
					Code:  "gtr geq",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86_AND_R8_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.AL, x86.DIL}, Length: 3},
					Uses:  1,
					Code:  "geq lss",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86_SETNZ_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.AL}, Length: 3},
					Uses:  1,
					Code:  "geq lss",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86_AND_R8_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.AL, x86.R8L}, Length: 3},
					Uses:  1,
					Code:  "lss leq",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86_SETNZ_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.AL}, Length: 3},
					Uses:  1,
					Code:  "lss leq",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86_AND_R8_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.AL, x86.R9L}, Length: 3},
					Uses:  1,
					Code:  "leq eql",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86_SETNZ_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.AL}, Length: 3},
					Uses:  1,
					Code:  "leq eql",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86_AND_R8_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.AL, x86.R10L}, Length: 3},
					Uses:  1,
					Code:  "eql neq",
				},
				{
					ID:    22,
					Op:    ssafir.OpX86_SETNZ_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.AL}, Length: 3},
					Uses:  1,
					Code:  "eql neq",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86_OR_R8_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.DL, x86.SIL}, Length: 3},
					Uses:  1,
					Code:  "gtr geq",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86_SETNZ_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.R11L}, Length: 4},
					Uses:  1,
					Code:  "gtr geq",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86_OR_R8_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.R11L, x86.DIL}, Length: 3},
					Uses:  1,
					Code:  "geq lss",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86_SETNZ_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.R11L}, Length: 4},
					Uses:  1,
					Code:  "geq lss",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86_OR_R8_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.R11L, x86.R8L}, Length: 3},
					Uses:  1,
					Code:  "lss leq",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86_SETNZ_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.R11L}, Length: 4},
					Uses:  1,
					Code:  "lss leq",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86_OR_R8_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.R11L, x86.R9L}, Length: 3},
					Uses:  1,
					Code:  "leq eql",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86_SETNZ_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.R11L}, Length: 4},
					Uses:  1,
					Code:  "leq eql",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86_OR_R8_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.R11L, x86.R10L}, Length: 3},
					Uses:  1,
					Code:  "eql neq",
				},
				{
					ID:    24,
					Op:    ssafir.OpX86_SETNZ_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.R11L}, Length: 4},
					Uses:  1,
					Code:  "eql neq",
				},
				{
					ID:    26,
					Op:    ssafir.OpX86_CMP_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RCX, x86.RCX}, Length: 3},
					Uses:  1,
					Code:  "a a",
				},
				{
					ID:    26,
					Op:    ssafir.OpX86_SETE_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.DL}, Length: 3},
					Uses:  1,
					Code:  "a a",
				},
				{
					ID:    27,
					Op:    ssafir.OpX86_OR_R8_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.AL, x86.R11L}, Length: 3},
					Uses:  1,
					Code:  "all any",
				},
				{
					ID:    27,
					Op:    ssafir.OpX86_SETNZ_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.CL}, Length: 3},
					Uses:  1,
					Code:  "all any",
				},
				{
					ID:    27,
					Op:    ssafir.OpX86_OR_R8_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.CL, x86.DL}, Length: 2},
					Uses:  1,
					Code:  "any (= a a)",
				},
				{
					ID:    27,
					Op:    ssafir.OpX86_SETNZ_Rmr8,
					Extra: &x86InstructionData{Args: [4]any{x86.CL}, Length: 3},
					Uses:  1,
					Code:  "any (= a a)",
				},
				{
					ID:    28,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RAX, x86.RCX}, Length: 3},
					Uses:  1,
					Code:  "(or all any (= a a))",
				},
				{
					ID:    28,
					Op:    ssafir.OpX86_RET,
					Extra: &x86InstructionData{Length: 1},
					Uses:  1,
					Code:  "(or all any (= a a))",
				},
			},
		},
		{
			Name: "multiple-returns",
			Arch: sys.X86_64,
			Code: `
				(package test)

				(let System-V-x86-64 (abi
					(params rdi rsi rdx r10 r8 r9)
					(result rax rdx)))

				; Return double n and quadruple n.
				'(abi System-V-x86-64)
				(func (x2-and-x4 (n int) int int)
					(let x2 (<< n 1))
					(let x4 (<< n 2))
					(return x2 x4))

				; Returns the smaller argument.
				'(abi System-V-x86-64)
				(asm-func (pick-smaller (a int) (b int) int)
					(cmp rdi rsi)
					(jg 'second)
					(mov rax rdi)
					(ret)
					'second
					(mov rax rsi)
					(ret))

				; Exit with the given status code.
				'(abi System-V-x86-64)
				(asm-func (exit (code int))
					(mov eax 60)  ; sys_exit
					(syscall))    ; exit(code)

				(func (test)
					(let x2 x4 (x2-and-x4 3))                ; Multi-parameter 'let'.
					(let less (pick-smaller (x2-and-x4 4)))  ; Multi-parameter function call, passing two results from (x2-and-x4 4) to (pick-smaller).
					(exit (+ x2 less x4)))
			`,
			Disasm: []string{
				"000000:	f3 0f 1e fa          	endbr64",
				"000004:	53                   	push rbx",        // Preserve    rbx
				"000005:	55                   	push rbp",        // Preserve    rbp
				"000006:	bf 03 00 00 00       	mov edi, 0x3",    // Prepare arg 3
				"00000b:	e8 3f 33 22 11       	call 0x1122334f", // Call func   (x2-and-x4 3)
				"000010:	48 8b c8             	mov rcx, rax",    // Save result x2
				"000013:	4c 8b da             	mov r11, rdx",    // Save result x4
				"000016:	bf 04 00 00 00       	mov edi, 0x4",    // Prepare arg 4
				"00001b:	e8 3f 33 22 11       	call 0x1122335f", // Call func   (x2-and-x4 4)
				"000020:	48 8b d8             	mov rbx, rax",    // Save result x2
				"000023:	48 8b ea             	mov rbp, rdx",    // Save result x4
				"000026:	48 8b fb             	mov rdi, rbx",    // Prepare arg x2
				"000029:	48 8b f5             	mov rsi, rbp",    // Prepare arg x4
				"00002c:	e8 3f 33 22 11       	call 0x11223370", // Call func   (pick-smaller (x2-and-x4 4))
				"000031:	48 8b d1             	mov rdx, rcx",    // Prepare arg x2
				"000034:	48 03 d0             	add rdx, rax",    // Perform     (+ x2 less)
				"000037:	49 03 d3             	add rdx, r11",    // Perform     (+ x2 less x4)
				"00003a:	48 8b fa             	mov rdi, rdx",    // Prepare arg (+ x2 less x4)
				"00003d:	e8 3f 33 22 11       	call 0x11223381", // Call func   (exit (pick-smaller x2 x4))
				"000042:	5d                   	pop rbp",         // Restore     rbp
				"000043:	5b                   	pop rbx",         // Restore     rbx
				"000044:	c3                   	ret",
			},
			Want: []*TestValue{
				{ID: 0, Op: ssafir.OpX86_ENDBR64, Extra: &x86InstructionData{Length: 4}, Uses: 0, Code: `func (test)`},
				{ID: 0, Op: ssafir.OpX86_PUSH_R64op, Extra: &x86InstructionData{Args: [4]any{x86.RBX}, Length: 1}, Uses: 0, Code: `func (test)`},
				{ID: 0, Op: ssafir.OpX86_PUSH_R64op, Extra: &x86InstructionData{Args: [4]any{x86.RBP}, Length: 1}, Uses: 0, Code: `func (test)`},
				{
					ID:    2,
					Op:    ssafir.OpX86_MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EDI, uint64(3)}, Length: 5},
					Uses:  1,
					Code:  `3`,
				},
				{
					ID: 3,
					Op: ssafir.OpX86_CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     703,
								Name:    "tests/test.x2-and-x4",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  12,
								Address: 0x10,
							},
						},
						Length: 5,
					},
					Uses: 2,
					Code: `(x2-and-x4 3)`,
				},
				{
					ID:    6,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RCX, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  `(let x2 x4 (x2-and-x4 3))`,
				},
				{
					ID:    7,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.R11, x86.RDX}, Length: 3},
					Uses:  1,
					Code:  `(let x2 x4 (x2-and-x4 3))`,
				},
				{
					ID:    8,
					Op:    ssafir.OpX86_MOV_R32op_Imm32,
					Extra: &x86InstructionData{Args: [4]any{x86.EDI, uint64(4)}, Length: 5},
					Uses:  1,
					Code:  `4`,
				},
				{
					ID: 9,
					Op: ssafir.OpX86_CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     787,
								Name:    "tests/test.x2-and-x4",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  28,
								Address: 0x20,
							},
						},
						Length: 5,
					},
					Uses: 2,
					Code: `(x2-and-x4 4)`,
				},
				{
					ID:    10,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RBX, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  `(x2-and-x4 4)`,
				},
				{
					ID:    11,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RBP, x86.RDX}, Length: 3},
					Uses:  1,
					Code:  `(x2-and-x4 4)`,
				},
				{
					ID:    10,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDI, x86.RBX}, Length: 3},
					Uses:  1,
					Code:  `(x2-and-x4 4)`,
				},
				{
					ID:    11,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RSI, x86.RBP}, Length: 3},
					Uses:  1,
					Code:  `(x2-and-x4 4)`,
				},
				{
					ID: 12,
					Op: ssafir.OpX86_CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     773,
								Name:    "tests/test.pick-smaller",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  45,
								Address: 0x31,
							},
						},
						Length: 5,
					},
					Uses: 1,
					Code: `(pick-smaller (x2-and-x4 4))`,
				},
				{
					ID:    15,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDX, x86.RCX}, Length: 3},
					Uses:  1,
					Code:  "x2 less",
				},
				{
					ID:    15,
					Op:    ssafir.OpX86_ADD_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDX, x86.RAX}, Length: 3},
					Uses:  1,
					Code:  "x2 less",
				},
				{
					ID:    15,
					Op:    ssafir.OpX86_ADD_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDX, x86.R11}, Length: 3},
					Uses:  1,
					Code:  "less x4",
				},
				{
					ID:    15,
					Op:    ssafir.OpX86_MOV_R64_Rmr64_REX,
					Extra: &x86InstructionData{Args: [4]any{x86.RDI, x86.RDX}, Length: 3},
					Uses:  1,
					Code:  "less x4",
				},
				{
					ID: 16,
					Op: ssafir.OpX86_CALL_Rel32,
					Extra: &x86InstructionData{
						Args: [4]any{
							&ssafir.Link{
								Pos:     900,
								Name:    "tests/test.exit",
								Type:    ssafir.LinkRelativeAddress,
								Size:    32,
								Offset:  62,
								Address: 0x42,
							},
						},
						Length: 5,
					},
					Uses: 0,
					Code: `(exit (+ x2 less x4))`,
				},
				{ID: 0, Op: ssafir.OpX86_POP_R64op, Extra: &x86InstructionData{Args: [4]any{x86.RBP}, Length: 1}, Uses: 0, Code: `func (test)`},
				{ID: 0, Op: ssafir.OpX86_POP_R64op, Extra: &x86InstructionData{Args: [4]any{x86.RBX}, Length: 1}, Uses: 0, Code: `func (test)`},
				{
					ID:    0,
					Op:    ssafir.OpX86_RET,
					Extra: &x86InstructionData{Length: 1},
					Uses:  0,
					Code:  ")",
				},
			},
		},
	}

	compareOptions := []cmp.Option{
		cmpopts.IgnoreTypes(new(types.Function)),
	}

	var code bytes.Buffer
	var disasm strings.Builder
	for _, test := range tests {
		t.Run(test.Arch.Name+"/"+test.Name, func(t *testing.T) {
			sizes := types.SizesFor(test.Arch)
			if err := test.Arch.Validate(&test.Arch.DefaultABI); err != nil {
				t.Fatalf("invalid test ABI: %v", err)
			}

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
			pkg, err := types.Check(testPath, fset, files, test.Arch, info)
			if err != nil {
				t.Fatalf("failed to type-check package: %v", err)
			}

			p, err := Compile(fset, test.Arch, pkg, files, info, sizes)
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
			err = Allocate(fset, test.Arch, sizes, p, testFunc)
			if err != nil {
				t.Fatalf("Allocate(): unexpected error: %v", err)
			}

			// Lower the instructions.
			err = Lower(fset, test.Arch, sizes, testFunc)
			if err != nil {
				t.Fatalf("Lower(): unexpected error: %v", err)
			}

			// Encode the instructions.
			code.Reset()
			err = EncodeTo(&code, fset, test.Arch, testFunc)
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
				t.Errorf("Lower(): (-want, +got)\n%s", diff.Diff("want", []byte(want), "got", []byte(got)))
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
