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
				"f3 0f 1e fa          	endbr64",
				"c3                   	ret",
			},
			Want: []*TestValue{
				{ID: 0, Op: ssafir.OpX86_ENDBR64, Extra: &x86InstructionData{Length: 4}, Uses: 0, Code: `func (test (a string) (b int))`},
				{ID: 4, Op: ssafir.OpX86_RET, Extra: &x86InstructionData{Length: 1}, Uses: 0, Code: `(let _ a)`},
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
				"f3 0f 1e fa          	endbr64",
				"48 8b c2             	mov rax, rdx",
				"c3                   	ret",
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
				"f3 0f 1e fa          	endbr64",
				"bf 03 00 00 00       	mov edi, 0x3",    // Prepare arg (len "bar")
				"e8 3f 33 22 11       	call 0x1122334d", // Call func   (double (len "bar"))
				"bf 06 00 00 00       	mov edi, 0x6",    // Prepare arg (let length (len "foobar")))
				"e8 3f 33 22 11       	call 0x11223357", // Call func   (double length)
				"bf 07 00 00 00       	mov edi, 0x7",    // Prepare arg 7
				"e8 3f 33 22 11       	call 0x11223361", // Call func   (double 7)
				"bf 11 00 00 00       	mov edi, 0x11",   // Prepare arg (let (val int) 17)
				"e8 3f 33 22 11       	call 0x1122336b", // Call func   (double val)
				"c3                   	ret",             // Return      (double val)
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
				"f3 0f 1e fa          	endbr64",
				"b8 07 00 00 00       	mov eax, 0x7",    // Prepare arg 7
				"e8 3f 33 22 11       	call 0x1122334d", // Call func   (copy-n 7)
				"48 8b c8             	mov rcx, rax",    // Save result (let a (copy-n 7))
				"b8 03 00 00 00       	mov eax, 0x3",    // Prepare arg 3
				"e8 3f 33 22 11       	call 0x1122335a", // Call func   (copy-n 3)
				"48 8b d0             	mov rdx, rax",    // Save result (let b (copy-n 3))
				"48 8b f1             	mov rsi, rcx",    // Prepare arg a
				"48 03 f0             	add rsi, rax",    // Arithmetic  (+ a b)
				"48 8b f9             	mov rdi, rcx",    // Prepare arg a
				"48 2b f8             	sub rdi, rax",    // Arithmetic  (- a b)
				"4c 8b c2             	mov r8, rdx",     // Save result (let ub (int->uint b))
				"4c 8b c8             	mov r9, rax",     // Prepare arg b
				"48 8b c1             	mov rax, rcx",    // Prepare arg a
				"49 f7 e1             	mul r9",          // Arithmetic  (× a b)
				"4c 8b d0             	mov r10, rax",    // Save result (let mul (* a b))
				"48 8b c1             	mov rax, rcx",    // Prepare arg a
				"48 33 d2             	xor rdx, rdx",    // Clear RDX
				"49 f7 f1             	div r9",          // Arithmetic  (÷ a b)
				"48 8b d1             	mov rdx, rcx",    // Prepare arg a
				"49 23 d1             	and rdx, r9",     // Arithmetic  (and a b)
				"4c 8b c9             	mov r9, rcx",     // Prepare arg a
				"4d 8b d9             	mov r11, r9",     // Save arg    a
				"49 8b c8             	mov rcx, r8",     // Prepare arg ub
				"49 d3 e3             	shl r11, cl",     // Arithmetic  (<< a ub)
				"4d 8b ca             	mov r9, r10",     // Prepare arg mul
				"49 8b c8             	mov rcx, r8",     // Prepare arg ub
				"49 d3 f9             	sar r9, cl",      // Arithmetic  (>> mul ub)
				"48 8b ce             	mov rcx, rsi",    // Prepare arg sum
				"48 0b cf             	or rcx, rdi",     // Arithmetic  (or sum dif)
				"49 0b ca             	or rcx, r10",     // Arithmetic  (or sum dif mul)
				"48 0b c8             	or rcx, rax",     // Arithmetic  (or sum ... mul div)
				"48 0b ca             	or rcx, rdx",     // Arithmetic  (or sum ... div bnd)
				"49 0b cb             	or rcx, r11",     // Arithmetic  (or sum ... bnd big)
				"49 0b c9             	or rcx, r9",      // Arithmetic  (or sum ... big sml)
				"48 8b c1             	mov rax, rcx",    // Save result (or sum ... sml)
				"c3                   	ret",
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
					(let b (>> a 1))      ; 3
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
				"f3 0f 1e fa          	endbr64",
				"b8 07 00 00 00       	mov eax, 0x7",    // Prepare arg 7
				"e8 3f 33 22 11       	call 0x1122334d", // Call func   (copy-n 7)
				"48 8b c8             	mov rcx, rax",    // Save result (let a (copy-n 7))
				"48 d1 f9             	sar rcx, 0x1",    // Perform     (>> a 1)
				"48 3b c1             	cmp rax, rcx",    // Compare     (> a b)
				"0f 9f c2             	setnle dl",       // Perform     (> a b)
				"48 3b c1             	cmp rax, rcx",    // Compare     (>= a b)
				"40 0f 9d c6          	setnl sil",       // Perform     (>= a b)
				"48 3b c1             	cmp rax, rcx",    // Compare     (< a b)
				"40 0f 9c c7          	setl dil",        // Perform     (< a b)
				"48 3b c1             	cmp rax, rcx",    // Compare     (<= a b)
				"41 0f 9e c0          	setle r8b",       // Perform     (<= a b)
				"48 3b c1             	cmp rax, rcx",    // Compare     (= a b)
				"41 0f 94 c1          	setz r9b",        // Perform     (= a b)
				"48 3b c1             	cmp rax, rcx",    // Compare     (!= a b)
				"41 0f 95 c2          	setnz r10b",      // Perform     (!= a b)
				"40 22 d6             	and dl, sil",     // Perform     (and gtr geq)
				"0f 95 c1             	setnz cl",        // Save result (and gtr geq)
				"40 22 cf             	and cl, dil",     // Perform     (and gtr geq lss)
				"0f 95 c1             	setnz cl",        // Save result (and gtr geq lss)
				"41 22 c8             	and cl, r8b",     // Perform     (and gtr ... lss leq)
				"0f 95 c1             	setnz cl",        // Save result (and gtr ... lss leq)
				"41 22 c9             	and cl, r9b",     // Perform     (and gtr ... leq eql)
				"0f 95 c1             	setnz cl",        // Save result (and gtr ... leq eql)
				"41 22 ca             	and cl, r10b",    // Perform     (and gtr ... eql neq)
				"0f 95 c1             	setnz cl",        // Save result (and gtr ... eql neq)
				"40 0a d6             	or dl, sil",      // Perform     (or gtr geq)
				"41 0f 95 c3          	setnz r11b",      // Save result (or gtr geq)
				"44 0a df             	or r11b, dil",    // Perform     (or gtr geq lss)
				"41 0f 95 c3          	setnz r11b",      // Save result (or gtr geq lss)
				"45 0a d8             	or r11b, r8b",    // Perform     (or gtr ... lss leq)
				"41 0f 95 c3          	setnz r11b",      // Save result (or gtr ... lss leq)
				"45 0a d9             	or r11b, r9b",    // Perform     (or gtr ... leq eql)
				"41 0f 95 c3          	setnz r11b",      // Save result (or gtr ... leq eql)
				"45 0a da             	or r11b, r10b",   // Perform     (or gtr ... eql neq)
				"41 0f 95 c3          	setnz r11b",      // Save result (or gtr ... eql neq)
				"48 3b c0             	cmp rax, rax",    // Compare     (= a a)
				"0f 94 c2             	setz dl",         // Perform     (= a a)
				"41 0a cb             	or cl, r11b",     // Perform     (or all any)
				"0f 95 c0             	setnz al",        // Save result (or all any)
				"0a c2                	or al, dl",       // Perform     (or all any (= a a))
				"0f 95 c0             	setnz al",        // Save result (or all any (= a a))
				"c3                   	ret",
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
				"f3 0f 1e fa          	endbr64",
				"53                   	push rbx",        // Preserve    rbx
				"55                   	push rbp",        // Preserve    rbp
				"bf 03 00 00 00       	mov edi, 0x3",    // Prepare arg 3
				"e8 3f 33 22 11       	call 0x1122334f", // Call func   (x2-and-x4 3)
				"48 8b c8             	mov rcx, rax",    // Save result x2
				"4c 8b da             	mov r11, rdx",    // Save result x4
				"bf 04 00 00 00       	mov edi, 0x4",    // Prepare arg 4
				"e8 3f 33 22 11       	call 0x1122335f", // Call func   (x2-and-x4 4)
				"48 8b d8             	mov rbx, rax",    // Save result x2
				"48 8b ea             	mov rbp, rdx",    // Save result x4
				"48 8b fb             	mov rdi, rbx",    // Prepare arg x2
				"48 8b f5             	mov rsi, rbp",    // Prepare arg x4
				"e8 3f 33 22 11       	call 0x11223370", // Call func   (pick-smaller (x2-and-x4 4))
				"48 8b d1             	mov rdx, rcx",    // Prepare arg x2
				"48 03 d0             	add rdx, rax",    // Perform     (+ x2 less)
				"49 03 d3             	add rdx, r11",    // Perform     (+ x2 less x4)
				"48 8b ca             	mov rcx, rdx",    // Save result (+ x2 less x4)
				"48 8b f9             	mov rdi, rcx",    // Prepare arg (+ x2 less x4)
				"e8 3f 33 22 11       	call 0x11223384", // Call func   (exit (pick-smaller x2 x4))
				"5d                   	pop rbp",         // Restore     rbp
				"5b                   	pop rbx",         // Restore     rbx
				"c3                   	ret",
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

				fmt.Fprintf(&disasm, "% -21x\t", src[:size])
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

			// The value data is optional.
			if test.Want == nil {
				return
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
