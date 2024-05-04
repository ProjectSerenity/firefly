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

	"firefly-os.dev/tools/diff"
	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/constant"
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
				{ID: 2, Op: ssafir.OpParameter, Extra: &Alloc{Dst: x86.RDI, Data: int64(0)}, Code: `(a string)`},
				{ID: 2, Op: ssafir.OpParameter, Extra: &Alloc{Dst: x86.RSI, Data: int64(0)}, Code: `(a string)`},
				{ID: 3, Op: ssafir.OpParameter, Extra: &Alloc{Dst: x86.RDX, Data: int64(1)}, Uses: 1, Code: `(b int)`},
				{ID: 4, Op: ssafir.OpCopy, Extra: &Alloc{Dst: x86.RDX, Src: x86.RDX}, Uses: 1, Code: `(let c b)`},
				{ID: 5, Op: ssafir.OpMakeResult, Extra: &Alloc{Dst: x86.RAX, Src: x86.RDX}, Uses: 1, Code: `c`},
				{ID: 4, Op: ssafir.OpDrop, Extra: &Alloc{Src: x86.RDX}, Code: `(let c b)`},
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

				'(abi
					(params rdi)
					(result rax))
				(asm-func (double (in int) int)
					(mov rax rdi)
					(add rax rax)
					(ret))

				'(abi
					(params rdi rsi)
					(result rax))
				(asm-func (half-string-length (s string) int)
					(mov rax rsi)
					(shr rax 1)
					(ret))

				(func (test int)
					(let length (len "foobar"))
					(double (len "bar"))
					(double length)
					(double 7)
					(half-string-length "foo")
					(let (val int) 17)
					(double val))
			`,
			Want: []*TestValue{
				{ID: 4, Op: ssafir.OpConstantInt64, Extra: &Alloc{Dst: x86.RDI, Data: int64(3)}, Uses: 1, Code: `(len "bar")`},
				{ID: 5, Op: ssafir.OpFunctionCall, Extra: new(types.Function), Code: `(double (len "bar"))`},
				{ID: 4, Op: ssafir.OpDrop, Extra: &Alloc{Src: x86.RDI}, Code: `(len "bar")`},
				{ID: 3, Op: ssafir.OpCopy, Extra: &Alloc{Dst: x86.RDI, Data: int64(6)}, Uses: 1, Code: `(let length (len "foobar"))`},
				{ID: 7, Op: ssafir.OpFunctionCall, Extra: new(types.Function), Code: `(double length)`},
				{ID: 3, Op: ssafir.OpDrop, Extra: &Alloc{Src: x86.RDI}, Code: `(let length (len "foobar"))`},
				{ID: 9, Op: ssafir.OpConstantUntypedInt, Extra: &Alloc{Dst: x86.RDI, Data: constant.MakeInt64(7)}, Uses: 1, Code: `7`},
				{ID: 10, Op: ssafir.OpFunctionCall, Extra: new(types.Function), Code: `(double 7)`},
				{ID: 9, Op: ssafir.OpDrop, Extra: &Alloc{Src: x86.RDI}, Code: `7`},
				{ID: 12, Op: ssafir.OpConstantString, Extra: &Alloc{Dst: x86.RDI, Data: "foo"}, Uses: 1, Code: `"foo"`},
				{ID: 12, Op: ssafir.OpConstantUntypedInt, Extra: &Alloc{Dst: x86.RSI, Data: int64(3)}, Uses: 1, Code: `"foo"`},
				{ID: 13, Op: ssafir.OpFunctionCall, Extra: new(types.Function), Code: `(half-string-length "foo")`},
				{ID: 12, Op: ssafir.OpDrop, Extra: &Alloc{Src: x86.RDI}, Code: `"foo"`},
				{ID: 12, Op: ssafir.OpDrop, Extra: &Alloc{Src: x86.RSI}, Code: `"foo"`},
				{ID: 16, Op: ssafir.OpConstantInt64, Extra: &Alloc{Dst: x86.RDI, Data: int64(17)}, Uses: 1, Code: `val`},
				{ID: 17, Op: ssafir.OpFunctionCall, Extra: new(types.Function), Code: `(double val)`},
				{ID: 16, Op: ssafir.OpDrop, Extra: &Alloc{Src: x86.RDI}, Code: `val`},
				{ID: 18, Op: ssafir.OpFunctionResult, Extra: &Alloc{Dst: x86.RAX, Src: x86.RAX}, Uses: 1, Code: `(double val)`},
				{ID: 19, Op: ssafir.OpMakeResult, Extra: &Alloc{Dst: x86.RAX, Src: x86.RAX}, Uses: 1, Code: `(double val)`},
				{ID: 18, Op: ssafir.OpDrop, Extra: &Alloc{Src: x86.RAX}, Code: `(double val)`},
			},
			Text: []string{
				"allocator for test (func int)",
				"  rax:  v19",
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
	}

	compareOptions := []cmp.Option{
		cmpopts.IgnoreTypes(new(types.Function), new(FunctionResult)),
		cmp.Comparer(func(a, b constant.Value) bool {
			return cmp.Equal(constant.Val(a), constant.Val(b))
		}),
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
			a := newAllocator(fset, arch, sizes, p, testFunc)
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
				t.Fatalf("allocator.Debug():\n%s", diff.Diff("want", []byte(text), "got", []byte(gotText)))
			}
		})
	}
}

func TestAllocator_Snapshot(t *testing.T) {
	v := func(id ssafir.ID) *ssafir.Value { return &ssafir.Value{ID: id} }
	v1, v2, v3, v4, v5 := v(1), v(2), v(3), v(4), v(5)
	rax, rcx, rdx, rbx, rsi, rdi := x86.RAX, x86.RCX, x86.RDX, x86.RBX, x86.RSI, x86.RDI
	arch := sys.X86_64
	fun := &ssafir.Function{
		Name: "test",
		Type: types.NewSignature("test", nil, nil),
	}

	tests := []struct {
		name  string
		start *allocator
		error string
		want  *allocatorSnapshot
	}{
		{
			name: "simple",
			start: &allocator{
				arch:      arch,
				function:  fun,
				registers: []sys.Location{rax, rcx, rdx, rbx, rsi, rdi},
				allocated: map[sys.Location]*ssafir.Value{
					rax: v1,
					rcx: v2,
					rdx: v3,
					rsi: v3,
					rdi: v5,
				},
				locations: map[*ssafir.Value][]sys.Location{
					v1: {rax},
					v2: {rcx},
					v3: {rsi, rdx},
					v4: {},
					v5: {rdi},
				},
			},
			want: &allocatorSnapshot{
				values: []*ssafir.Value{v1, v2, v3, v5},
				locs: [][]sys.Location{
					{rax},
					{rcx},
					{rsi, rdx},
					{rdi},
				},
				index: map[*ssafir.Value]int{
					v1: 0,
					v2: 1,
					v3: 2,
					v5: 3,
				},
			},
		},
		{
			name: "missing locations",
			start: &allocator{
				arch:      arch,
				function:  fun,
				registers: []sys.Location{rax, rcx, rdx, rbx, rsi, rdi},
				allocated: map[sys.Location]*ssafir.Value{
					rax: v1,
					rcx: v2,
					rdx: v3,
					rsi: v3,
					rdi: v5,
				},
				locations: map[*ssafir.Value][]sys.Location{
					v1: {rax},
					v3: {rsi, rdx},
					v4: {},
					v5: {rdi},
				},
			},
			error: "v2 is allocated to rcx, but is absent from a.locations",
		},
		{
			name: "missing allocated",
			start: &allocator{
				arch:      arch,
				function:  fun,
				registers: []sys.Location{rax, rcx, rdx, rbx, rsi, rdi},
				allocated: map[sys.Location]*ssafir.Value{
					rax: v1,
					rdx: v3,
					rsi: v3,
					rdi: v5,
				},
				locations: map[*ssafir.Value][]sys.Location{
					v1: {rax},
					v2: {rcx},
					v3: {rsi, rdx},
					v4: {},
					v5: {rdi},
				},
			},
			error: "v2 is allocated to [rcx], but is absent from a.allocated",
		},
	}

	opts := []cmp.Option{
		cmp.AllowUnexported(allocatorSnapshot{}),
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.start.Snapshot()
			if test.error != "" {
				if err == nil {
					t.Fatalf("a.Snapshot(): expected error %q, got %s", test.error, test.start.Debug())
				}

				e := err.Error()
				if !strings.Contains(e, test.error) {
					t.Fatalf("a.Snapshot(): got %q, want %q", e, test.error)
				}

				return
			}

			if err != nil {
				t.Fatalf("a.Snapshot(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.want, got, opts...); diff != "" {
				t.Fatalf("a.Snapshot(): (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestAllocator_Revert(t *testing.T) {
	v := func(id ssafir.ID) *ssafir.Value { return &ssafir.Value{ID: id} }
	v1, v2, v3, v4, v5 := v(1), v(2), v(3), v(4), v(5)
	rax, rcx, rdx, rbx, rsi, rdi := x86.RAX, x86.RCX, x86.RDX, x86.RBX, x86.RSI, x86.RDI
	arch := sys.X86_64
	fun := &ssafir.Function{
		Name: "test",
		Type: types.NewSignature("test", nil, nil),
	}

	tests := []struct {
		name     string
		snapshot *allocatorSnapshot
		start    *allocator
		error    string
		want     *allocator
	}{
		{
			name: "no-op",
			snapshot: &allocatorSnapshot{
				values: []*ssafir.Value{v1, v2, v3, v4, v5},
				locs: [][]sys.Location{
					{rax},
					{rcx},
					{rsi, rdx},
					{rbx},
					{rdi},
				},
				index: map[*ssafir.Value]int{
					v1: 0,
					v2: 1,
					v3: 2,
					v4: 3,
					v5: 4,
				},
			},
			// No changes after the snapshot.
			start: &allocator{
				arch:      arch,
				function:  fun,
				registers: []sys.Location{rax, rcx, rdx, rbx, rsi, rdi},
				allocated: map[sys.Location]*ssafir.Value{
					rax: v1,
					rcx: v2,
					rdx: v3,
					rbx: v4,
					rsi: v3,
					rdi: v5,
				},
				locations: map[*ssafir.Value][]sys.Location{
					v1: {rax},
					v2: {rcx},
					v3: {rsi, rdx},
					v4: {rbx},
					v5: {rdi},
				},
			},
			want: &allocator{
				arch:      arch,
				function:  fun,
				registers: []sys.Location{rax, rcx, rdx, rbx, rsi, rdi},
				allocated: map[sys.Location]*ssafir.Value{
					rax: v1,
					rcx: v2,
					rdx: v3,
					rbx: v4,
					rsi: v3,
					rdi: v5,
				},
				locations: map[*ssafir.Value][]sys.Location{
					v1: {rax},
					v2: {rcx},
					v3: {rsi, rdx},
					v4: {rbx},
					v5: {rdi},
				},
			},
		},
		{
			name: "shifts",
			snapshot: &allocatorSnapshot{
				values: []*ssafir.Value{v1, v2, v3},
				locs: [][]sys.Location{
					{rax},
					{rcx},
					{rsi, rdx},
				},
				index: map[*ssafir.Value]int{
					v1: 0,
					v2: 1,
					v3: 2,
				},
			},
			// An extra value, plus
			// another that displaced
			// some existing values.
			start: &allocator{
				arch:      arch,
				function:  fun,
				registers: []sys.Location{rax, rcx, rdx, rbx, rsi, rdi},
				allocated: map[sys.Location]*ssafir.Value{
					rax: v2,
					rcx: v3,
					rdx: nil, // Was v4.
					rbx: v3,
					rsi: v1,
					rdi: nil, // Was v5.
				},
				locations: map[*ssafir.Value][]sys.Location{
					v1: {rsi},
					v2: {rax},
					v3: {rbx, rcx},
					v4: nil, // Was rdx.
					v5: nil, // Was rdi.
				},
			},
			want: &allocator{
				arch:      arch,
				function:  fun,
				registers: []sys.Location{rax, rcx, rdx, rbx, rsi, rdi},
				allocated: map[sys.Location]*ssafir.Value{
					rax: v1,
					rcx: v2,
					rdx: v3,
					rbx: nil, // Was v3.
					rsi: v3,
					rdi: nil, // Was v5.
				},
				locations: map[*ssafir.Value][]sys.Location{
					v1: {rax},
					v2: {rcx},
					v3: {rsi, rdx},
					v4: nil,
					v5: nil,
				},
				allocs: []*ssafir.Value{
					{ID: 3, Op: ssafir.OpCopy, Extra: &Alloc{Dst: rdx, Src: rcx}},
					{ID: 2, Op: ssafir.OpCopy, Extra: &Alloc{Dst: rcx, Src: rax}},
					{ID: 1, Op: ssafir.OpCopy, Extra: &Alloc{Dst: rax, Src: rsi}},
					{ID: 3, Op: ssafir.OpCopy, Extra: &Alloc{Dst: rsi, Src: rbx}},
				},
			},
		},
		{
			name: "missing value",
			snapshot: &allocatorSnapshot{
				values: []*ssafir.Value{v1, v2, v3},
				locs: [][]sys.Location{
					{rax},
					{rcx},
					{rsi, rdx},
				},
				index: map[*ssafir.Value]int{
					v1: 0,
					v2: 1,
					v3: 2,
				},
			},
			// v2 is gone somehow.
			start: &allocator{
				arch:      arch,
				function:  fun,
				registers: []sys.Location{rax, rcx, rdx, rbx, rsi, rdi},
				allocated: map[sys.Location]*ssafir.Value{
					rcx: v3,
					rbx: v3,
					rsi: v1,
				},
				locations: map[*ssafir.Value][]sys.Location{
					v1: {rsi},
					v3: {rbx, rcx},
				},
			},
			error: "v2 must be moved to rcx but is absent from a.locations",
		},
		{
			name: "extra location",
			snapshot: &allocatorSnapshot{
				values: []*ssafir.Value{v1, v2, v3},
				locs: [][]sys.Location{
					{rax},
					{rcx},
					{rsi, rdx},
				},
				index: map[*ssafir.Value]int{
					v1: 0,
					v2: 1,
					v3: 2,
				},
			},
			// v1 has more locations somehow.
			start: &allocator{
				arch:      arch,
				function:  fun,
				registers: []sys.Location{rax, rcx, rdx, rbx, rsi, rdi},
				allocated: map[sys.Location]*ssafir.Value{
					rax: v2,
					rcx: v3,
					rbx: v3,
					rsi: v1,
					rdi: v1,
				},
				locations: map[*ssafir.Value][]sys.Location{
					v1: {rsi, rdi},
					v2: {rax},
					v3: {rbx, rcx},
				},
			},
			error: "v1 has 1 snapshot locations and 2 current locations",
		},
		{
			name: "inconsistent allocations",
			snapshot: &allocatorSnapshot{
				values: []*ssafir.Value{v1, v2, v3},
				locs: [][]sys.Location{
					{rax},
					{rcx},
					{rsi, rdx},
				},
				index: map[*ssafir.Value]int{
					v1: 0,
					v2: 1,
					v3: 2,
				},
			},
			// a.allocated and a.locations
			// disagree on v1.
			start: &allocator{
				arch:      arch,
				function:  fun,
				registers: []sys.Location{rax, rcx, rdx, rbx, rsi, rdi},
				allocated: map[sys.Location]*ssafir.Value{
					rax: v1,
					rcx: v3,
					rbx: v3,
					rsi: v2,
				},
				locations: map[*ssafir.Value][]sys.Location{
					v1: {rsi},
					v2: {rax},
					v3: {rbx, rcx},
				},
			},
			error: "moving v1 from rsi to rax, but a.allocated[rsi] = v2",
		},
		{
			name: "unexpected value",
			snapshot: &allocatorSnapshot{
				values: []*ssafir.Value{v1, v2, v3},
				locs: [][]sys.Location{
					{rax},
					{rcx},
					{rsi, rdx},
				},
				index: map[*ssafir.Value]int{
					v1: 0,
					v2: 1,
					v3: 2,
				},
			},
			// An extra value hasn't been
			// dropped.
			start: &allocator{
				arch:      arch,
				function:  fun,
				registers: []sys.Location{rax, rcx, rdx, rbx, rsi, rdi},
				allocated: map[sys.Location]*ssafir.Value{
					rax: v2,
					rcx: v3,
					rdx: v4,
					rbx: v3,
					rsi: v1,
				},
				locations: map[*ssafir.Value][]sys.Location{
					v1: {rsi},
					v2: {rax},
					v3: {rbx, rcx},
					v4: {rdx},
				},
			},
			error: "v4 is allocated to rdx, but is absent from the snapshot",
		},
		{
			name: "extra dependency location",
			snapshot: &allocatorSnapshot{
				values: []*ssafir.Value{v1, v2, v3},
				locs: [][]sys.Location{
					{rax},
					{rcx},
					{rsi, rdx},
				},
				index: map[*ssafir.Value]int{
					v1: 0,
					v2: 1,
					v3: 2,
				},
			},
			// v2 has more locations somehow.
			start: &allocator{
				arch:      arch,
				function:  fun,
				registers: []sys.Location{rax, rcx, rdx, rbx, rsi, rdi},
				allocated: map[sys.Location]*ssafir.Value{
					rax: v2,
					rcx: v3,
					rbx: v3,
					rsi: v1,
					rdi: v2,
				},
				locations: map[*ssafir.Value][]sys.Location{
					v1: {rsi},
					v2: {rdi, rax},
					v3: {rbx, rcx},
				},
			},
			error: "v2 has 1 snapshot locations and 2 current locations",
		},
		{
			name: "missing from locations",
			snapshot: &allocatorSnapshot{
				values: []*ssafir.Value{v1, v2, v3},
				locs: [][]sys.Location{
					{rax},
					{rcx},
					{rsi, rdx},
				},
				index: map[*ssafir.Value]int{
					v1: 0,
					v2: 1,
					v3: 2,
				},
			},
			// v2 is in the wrong place in a.locations somehow.
			start: &allocator{
				arch:      arch,
				function:  fun,
				registers: []sys.Location{rax, rcx, rdx, rbx, rsi, rdi},
				allocated: map[sys.Location]*ssafir.Value{
					rax: v2,
					rcx: v3,
					rbx: v3,
					rsi: v1,
				},
				locations: map[*ssafir.Value][]sys.Location{
					v1: {rsi},
					v2: {rdx},
					v3: {rbx, rcx},
				},
			},
			error: "v2 is allocated to rax, but is absent from a.locations",
		},
	}

	opts := []cmp.Option{
		cmp.AllowUnexported(allocator{}),
		cmpopts.IgnoreTypes(new(sys.Arch), new(ssafir.Function)),
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.start.Revert(nil, test.snapshot)
			if test.error != "" {
				if err == nil {
					t.Fatalf("a.Revert(snapshot): expected error %q, got %s", test.error, test.start.Debug())
				}

				e := err.Error()
				if !strings.Contains(e, test.error) {
					t.Fatalf("a.Revert(snapshot): got error %q, want %q", e, test.error)
				}

				return
			}

			if err != nil {
				t.Fatalf("a.Revert(snapshot): unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.want, test.start, opts...); diff != "" {
				t.Fatalf("a.Revert(snapshot): (-want, +got)\n%s", diff)
			}
		})
	}
}
