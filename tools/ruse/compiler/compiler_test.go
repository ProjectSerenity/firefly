// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package compiler

import (
	"fmt"
	"go/constant"
	gotoken "go/token"
	"path/filepath"
	"reflect"
	"strconv"
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

func humaniseNumber(v int) string {
	prefix, suffix := strconv.Itoa(v), ""
	for len(prefix) > 3 {
		suffix = "," + prefix[len(prefix)-3:] + suffix
		prefix = prefix[:len(prefix)-3]
	}

	return prefix + suffix
}

type TestAssemblyGroup struct {
	Name  string
	fail  int
	wrong int
	skip  int
	total int
}

func (g *TestAssemblyGroup) Start()      { g.total++ }
func (g *TestAssemblyGroup) Fail()       { g.fail++ }
func (g *TestAssemblyGroup) Wrong()      { g.wrong++ }
func (g *TestAssemblyGroup) Skip()       { g.skip++ }
func (g *TestAssemblyGroup) Ok() bool    { return g.fail <= 10 }
func (g *TestAssemblyGroup) Right() bool { return g.wrong <= 10 }

func (g *TestAssemblyGroup) Print() {
	if g.skip > 0 {
		pc := (100 * g.skip) / g.total
		n := humaniseNumber(g.skip)
		max := humaniseNumber(g.total)
		println(fmt.Sprintf("%-8s skipped           %3d%% (%9s of %9s) test instructions", g.Name, pc, n, max))
	}

	if g.fail > 0 {
		pc := (100 * g.fail) / (g.total - g.skip)
		n := humaniseNumber(g.fail)
		max := humaniseNumber(g.total - g.skip)
		println(fmt.Sprintf("%-8s failed to compile %3d%% (%9s of %9s) test instructions", g.Name, pc, n, max))
	}

	if g.wrong > 0 {
		pc := (100 * g.wrong) / (g.total - g.skip - g.fail)
		n := humaniseNumber(g.wrong)
		max := humaniseNumber(g.total - g.skip - g.fail)
		println(fmt.Sprintf("%-8s failed to encode  %3d%% (%9s of %9s) test instructions", g.Name, pc, n, max))
	}

	if g.fail == 0 && g.wrong == 0 && g.skip == 0 {
		n := humaniseNumber(g.total)
		println(fmt.Sprintf("%-8s passed all %s test instructions", g.Name, n))
	}
}

func TestCompile(t *testing.T) {
	tests := []struct {
		Name  string
		Path  string
		Text  string
		Files []string
		Err   string
		Want  *Package
		Print [][]string
	}{
		// Valid packages.
		{
			Name:  "minimal",
			Path:  "tests/minimal",
			Files: []string{"minimal"},
			Want: &Package{
				Name: "minimal",
				Path: "tests/minimal",
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
					Constants: []*types.Constant{
						types.NewConstant(nil, 79, 110, nil, "INFERRED-STRING", types.UntypedString, constant.MakeString("string 1")),
						types.NewConstant(nil, 113, 133, nil, "InferredInt", types.UntypedInt, constant.MakeInt64(123)),
						types.NewConstant(nil, 136, 155, nil, "big", types.Uint64, constant.MakeUint64(1)),
						types.NewConstant(nil, 158, 188, nil, "NAMED", types.String, constant.MakeString("string 2")),
						types.NewConstant(nil, 191, 213, nil, "SMALL", types.Int8, constant.MakeInt64(-127)),
						types.NewConstant(nil, 216, 240, nil, "derived", types.Int, constant.MakeInt64(3)),
						types.NewConstant(nil, 243, 356, nil, "complex-constant-op", types.Int, constant.MakeInt64(
							int64(len("foo"))+
								3+
								int64(len("other"))+
								((0xff-250)-2)+
								((2*3)*4)+
								((12/6)/2))),
						types.NewConstant(nil, 359, 395, nil, "compound-string", types.String, constant.BinaryOp(constant.MakeString("string 2"), gotoken.ADD, constant.MakeString("foo"))),
					},
				}

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
				}

				f1 := &ssafir.Function{
					Name:        "nullary-function",
					Type:        types.NewSignature("(func)", []*types.Variable{}, nil),
					Params:      [][]sys.Location{},
					Result:      []sys.Location{},
					NamedValues: make(map[*types.Variable][]*ssafir.Value),
				}
				b11 := f1.NewBlock(93, ssafir.BlockReturn)
				v111 := b11.NewValue(67, 110, ssafir.OpMakeMemoryState, ssafir.MemoryState{})
				b11.NewValueInt(93, 109, ssafir.OpConstantInt8, types.Int8, 0)
				v113 := b11.NewValue(93, 109, ssafir.OpMakeResult, ssafir.Result{}, v111)
				b11.Control = v113
				b11.End = 109
				f1.Entry = b11

				p21 := types.NewParameter(nil, 134, 142, nil, "x", types.Byte)
				f2 := &ssafir.Function{
					Name:        "unary-function",
					Type:        types.NewSignature("(func (byte))", []*types.Variable{p21}, nil),
					Params:      [][]sys.Location{{x86.RDI}},
					Result:      []sys.Location{},
					NamedValues: make(map[*types.Variable][]*ssafir.Value),
				}
				b21 := f2.NewBlock(145, ssafir.BlockReturn)
				v211 := b21.NewValue(112, 155, ssafir.OpMakeMemoryState, ssafir.MemoryState{})
				v212 := b21.NewValueInt(134, 142, ssafir.OpParameter, types.Byte, 0)
				b21.NewValue(145, 154, ssafir.OpCopy, types.Byte, v212)
				v214 := b21.NewValue(145, 154, ssafir.OpMakeResult, ssafir.Result{}, v211)
				b21.Control = v214
				b21.End = 154
				f2.NamedValues[p21] = []*ssafir.Value{v212}
				f2.Entry = b21

				p31 := types.NewParameter(nil, 180, 189, nil, "x", types.Int32)
				p32 := types.NewParameter(nil, 190, 200, nil, "y", types.String)
				f3 := &ssafir.Function{
					Name:        "binary-function",
					Type:        types.NewSignature("(func (int32) (string))", []*types.Variable{p31, p32}, nil),
					Params:      [][]sys.Location{{x86.RDI}, {x86.RSI, x86.RDX}},
					Result:      []sys.Location{},
					NamedValues: make(map[*types.Variable][]*ssafir.Value),
				}
				b31 := f3.NewBlock(203, ssafir.BlockReturn)
				v311 := b31.NewValue(157, 238, ssafir.OpMakeMemoryState, ssafir.MemoryState{})
				v312 := b31.NewValueInt(180, 189, ssafir.OpParameter, types.Int32, 0)
				v313 := b31.NewValueInt(190, 200, ssafir.OpParameter, types.String, 1)
				v314 := b31.NewValue(227, 234, ssafir.OpStringLen, types.Int, v313)
				v315 := b31.NewValue(215, 235, ssafir.OpCastInt64ToInt32, types.Int32, v314)
				v316 := b31.NewValue(213, 235, ssafir.OpAddInt32, types.Int32, v312, v315)
				b31.NewValue(203, 237, ssafir.OpCopy, types.Int32, v316)
				v317 := b31.NewValue(203, 237, ssafir.OpMakeResult, ssafir.Result{}, v311)
				b31.Control = v317
				b31.End = 237
				f3.NamedValues[p31] = []*ssafir.Value{v312}
				f3.NamedValues[p32] = []*ssafir.Value{v313}
				f3.Entry = b31

				p41 := types.NewParameter(nil, 252, 260, nil, "x", types.Int8)
				f4 := &ssafir.Function{
					Name:        "add1",
					Type:        types.NewSignature("(func (int8) int8)", []*types.Variable{p41}, types.Int8),
					Params:      [][]sys.Location{{x86.RDI}},
					Result:      []sys.Location{x86.RAX},
					NamedValues: make(map[*types.Variable][]*ssafir.Value),
				}
				b41 := f4.NewBlock(268, ssafir.BlockReturn)
				v411 := b41.NewValue(240, 276, ssafir.OpMakeMemoryState, ssafir.MemoryState{})
				v412 := b41.NewValueInt(252, 260, ssafir.OpParameter, types.Int8, 0)
				v413 := b41.NewValueInt(273, 274, ssafir.OpConstantInt8, types.Int8, 1)
				v414 := b41.NewValue(271, 274, ssafir.OpAddInt8, types.Int8, v412, v413)
				v415 := b41.NewValue(268, 275, ssafir.OpMakeResult, ssafir.Result{Value: types.Int8}, v414, v411)
				b41.Control = v415
				b41.End = 275
				b41.Control.Uses++ // The return uses the value.
				f4.NamedValues[p41] = []*ssafir.Value{v412}
				f4.Entry = b41

				invertedStack := (*ast.Identifier)(nil)
				params := []*ast.Identifier{
					{NamePos: 0, Name: "rcx"},
					{NamePos: 0, Name: "rdx"},
					{NamePos: 0, Name: "r8"},
					{NamePos: 0, Name: "r9"},
				}
				result := []*ast.Identifier{
					{NamePos: 0, Name: "rax"},
				}
				scratch := []*ast.Identifier{
					{NamePos: 0, Name: "rax"},
					{NamePos: 0, Name: "rcx"},
					{NamePos: 0, Name: "rdx"},
					{NamePos: 0, Name: "r8"},
					{NamePos: 0, Name: "r9"},
					{NamePos: 0, Name: "r10"},
					{NamePos: 0, Name: "r11"},
				}
				unused := []*ast.Identifier(nil)
				abi, err := types.NewRawABI(sys.X86_64, invertedStack, params, result, scratch, unused)
				if err != nil {
					panic(err.Error())
				}
				pkg.Constants = append(pkg.Constants, types.NewConstant(nil, 293, 390, nil, "windows-x64", abi, nil))

				p51 := types.NewParameter(nil, 427, 440, nil, "base", types.Uint64)
				p52 := types.NewParameter(nil, 441, 456, nil, "scalar", types.Uint64)
				f5 := &ssafir.Function{
					Name:        "product",
					Type:        types.NewSignature("(func (uint64) (uint64) uint64)", []*types.Variable{p51, p52}, types.Uint64),
					Params:      [][]sys.Location{{x86.RCX}, {x86.RDX}},
					Result:      []sys.Location{x86.RAX},
					NamedValues: make(map[*types.Variable][]*ssafir.Value),
				}
				o5 := types.NewFunction(nil, 466, 481, nil, "product", f5.Type)
				b51 := f5.NewBlock(466, ssafir.BlockReturn)
				v511 := b51.NewValue(412, 482, ssafir.OpMakeMemoryState, ssafir.MemoryState{})
				v512 := b51.NewValueInt(427, 440, ssafir.OpParameter, types.Uint64, 0)
				v513 := b51.NewValueInt(441, 456, ssafir.OpParameter, types.Uint64, 1)
				v514 := b51.NewValue(469, 480, ssafir.OpMultiplyUint64, types.Uint64, v512, v513)
				v515 := b51.NewValue(466, 481, ssafir.OpMakeResult, ssafir.Result{Value: types.Uint64}, v514, v511)
				b51.Control = v515
				b51.End = 481
				b51.Control.Uses++ // The return uses the value.
				f5.NamedValues[p51] = []*ssafir.Value{v512}
				f5.NamedValues[p52] = []*ssafir.Value{v513}
				f5.Entry = b51

				f6 := &ssafir.Function{
					Name:        "maths-examples",
					Type:        types.NewSignature("(func)", []*types.Variable{}, nil),
					Params:      [][]sys.Location{},
					Result:      []sys.Location{},
					NamedValues: make(map[*types.Variable][]*ssafir.Value),
				}
				b61 := f6.NewBlock(508, ssafir.BlockReturn)
				v611 := b61.NewValue(484, 606, ssafir.OpMakeMemoryState, ssafir.MemoryState{})
				v613 := b61.NewValueInt(520, 531, ssafir.OpConstantInt64, types.Int, 3)
				b61.NewValue(508, 532, ssafir.OpCopy, types.Int, v613)
				v615 := b61.NewValue(543, 563, ssafir.OpCastInt64ToUint64, types.Uint64, v613)
				v616 := b61.NewValueExtra(564, 565, ssafir.OpConstantUntypedInt, types.UntypedInt, constant.MakeInt64(2))
				b61.NewValueExtra(534, 566, ssafir.OpFunctionCall, types.Uint64, o5, v615, v616)
				v618 := b61.NewValueInt(590, 601, ssafir.OpConstantInt64, types.Int, 3)
				v619 := b61.NewValue(577, 602, ssafir.OpCastInt64ToUint64, types.Uint64, v618)
				v620 := b61.NewValueExtra(603, 604, ssafir.OpConstantUntypedInt, types.UntypedInt, constant.MakeInt64(2))
				b61.NewValueExtra(568, 605, ssafir.OpFunctionCall, types.Uint64, o5, v619, v620)
				v622 := b61.NewValue(568, 605, ssafir.OpMakeResult, ssafir.Result{}, v611)
				b61.Control = v622
				b61.End = 605
				f6.Entry = b61

				pkg.Functions = []*ssafir.Function{f1, f2, f3, f4, f5, f6}

				return pkg
			})(),
			Print: [][]string{
				{
					"nullary-function (func)",
					"b1:",
					"	v1 := (MakeMemoryState) memory state",
					"	v2 := (ConstantInt8 (extra 0)) int8",
					"	v3 := (MakeResult v1) result",
					"	(Return v3)",
					"",
				},
				{
					"unary-function (func (byte))",
					"b1:",
					"	v1 := (MakeMemoryState) memory state",
					"	v2 := (Parameter (extra 0)) byte (x)",
					"	v3 := (Copy v2) byte",
					"	v4 := (MakeResult v1) result",
					"	(Return v4)",
					"",
				},
				{
					"binary-function (func (int32) (string))",
					"b1:",
					"	v1 := (MakeMemoryState) memory state",
					"	v2 := (Parameter (extra 0)) int32 (x)",
					"	v3 := (Parameter (extra 1)) string (y)",
					"	v4 := (StringLen v3) int",
					"	v5 := (CastInt64ToInt32 v4) int32",
					"	v6 := (AddInt32 v2 v5) int32",
					"	v7 := (Copy v6) int32",
					"	v8 := (MakeResult v1) result",
					"	(Return v8)",
					"",
				},
				// add1
				{
					"add1 (func (int8) int8)",
					"b1:",
					"	v1 := (MakeMemoryState) memory state",
					"	v2 := (Parameter (extra 0)) int8 (x)",
					"	v3 := (ConstantInt8 (extra 1)) int8",
					"	v4 := (AddInt8 v2 v3) int8",
					"	v5 := (MakeResult v4 v1) result",
					"	(Return v5)",
					"",
				},
				// product
				{
					"product (func (uint64) (uint64) uint64)",
					"b1:",
					"	v1 := (MakeMemoryState) memory state",
					"	v2 := (Parameter (extra 0)) uint64 (base)",
					"	v3 := (Parameter (extra 1)) uint64 (scalar)",
					"	v4 := (MultiplyUint64 v2 v3) uint64",
					"	v5 := (MakeResult v4 v1) result",
					"	(Return v5)",
					"",
				},
				// maths-examples
				{
					"maths-examples (func)",
					"b1:",
					"	v1  := (MakeMemoryState) memory state",
					"	v2  := (ConstantInt64 (extra 3)) int",
					"	v3  := (Copy v2) int",
					"	v4  := (CastInt64ToUint64 v2) uint64",
					"	v5  := (ConstantUntypedInt (extra 2)) untyped integer",
					"	v6  := (FunctionCall v4 v5 (extra function product ((func (uint64) (uint64) uint64)))) uint64",
					"	v7  := (ConstantInt64 (extra 3)) int",
					"	v8  := (CastInt64ToUint64 v7) uint64",
					"	v9  := (ConstantUntypedInt (extra 2)) untyped integer",
					"	v10 := (FunctionCall v8 v9 (extra function product ((func (uint64) (uint64) uint64)))) uint64",
					"	v11 := (MakeResult v1) result",
					"	(Return v11)",
					"",
				},
			},
		},
		{
			Name:  "assembly",
			Path:  "tests/assembly",
			Files: []string{"assembly"},
			Want: (func() *Package {
				pkg := &Package{
					Name: "assembly",
					Path: "tests/assembly",
				}

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
				abi, err := types.NewRawABI(sys.X86_64, invertedStack, params, result, scratch, unused)
				if err != nil {
					panic(err.Error())
				}
				pkg.Constants = append(pkg.Constants, types.NewConstant(nil, 273, 360, nil, "syscall", abi, nil))

				funcScope := types.NewScope(nil, 246, 255, "function syscall6")
				syscall := types.NewParameter(funcScope, 120, 133, nil, "sys", types.Uintptr)
				arg1 := types.NewParameter(funcScope, 136, 150, nil, "arg1", types.Uintptr)
				arg2 := types.NewParameter(funcScope, 153, 167, nil, "arg2", types.Uintptr)
				arg3 := types.NewParameter(funcScope, 170, 184, nil, "arg3", types.Uintptr)
				arg4 := types.NewParameter(funcScope, 187, 201, nil, "arg4", types.Uintptr)
				arg5 := types.NewParameter(funcScope, 204, 218, nil, "arg5", types.Uintptr)
				arg6 := types.NewParameter(funcScope, 221, 235, nil, "arg6", types.Uintptr)
				f1 := &ssafir.Function{
					Name: "syscall6",
					Type: types.NewSignature(
						"(func (uintptr) (uintptr) (uintptr) (uintptr) (uintptr) (uintptr) (uintptr) uintptr)",
						[]*types.Variable{syscall, arg1, arg2, arg3, arg4, arg5, arg6},
						types.Uintptr,
					),
					Params: [][]sys.Location{
						{x86.RAX},
						{x86.RDI},
						{x86.RSI},
						{x86.RDX},
						{x86.R10},
						{x86.R8},
						{x86.R9},
					},
					Result:      []sys.Location{x86.RAX},
					Extra:       x86.Mode64,
					NamedValues: make(map[*types.Variable][]*ssafir.Value),
				}
				b11 := f1.NewBlock(246, ssafir.BlockNormal)
				b11.NewValueExtra(246, 255, ssafir.OpX86SYSCALL, nil, &x86InstructionData{Length: 2})
				b11.End = 246
				f1.Entry = b11

				pkg.Functions = []*ssafir.Function{f1}

				return pkg
			})(),
			Print: [][]string{
				{
					"syscall6 (func (uintptr) (uintptr) (uintptr) (uintptr) (uintptr) (uintptr) (uintptr) uintptr)",
					"b1:",
					"	v1 := (SYSCALL (extra (x86-instruction-data)))",
					"	(Normal)",
					"",
				},
			},
		},
		// Invalid packages.
	}

	compareOptions := []cmp.Option{
		cmp.Exporter(func(t reflect.Type) bool { return true }),                            // Allow unexported types to be compared.
		cmpopts.IgnoreTypes(new(types.Package), new(types.Scope), new(types.Function)),     // Ignore *types.Package and *types.Scope values.
		cmpopts.SortMaps(func(v1, v2 *types.Variable) bool { return v1.Pos() < v2.Pos() }), // Sort NamedValues to improve comparisons.
	}

	// Use x86-64.
	arch := sys.X86_64
	sizes := types.SizesFor(arch)

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

			info := &types.Info{
				Types:       make(map[ast.Expression]types.TypeAndValue),
				Definitions: make(map[*ast.Identifier]types.Object),
				Uses:        make(map[*ast.Identifier]types.Object),
			}

			pkg, err := types.Check(test.Path, fset, files, arch, info)
			if err != nil {
				t.Fatalf("failed to type-check package: %v", err)
			}

			p, err := Compile(fset, arch, pkg, files, info, sizes)
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

			if diff := cmp.Diff(test.Want, p, compareOptions...); diff != "" {
				t.Fatalf("Compile(): (-want, +got)\n%s", diff)
			}

			got := make([]string, len(p.Functions))
			for i, fun := range p.Functions {
				got[i] = fun.Print()
			}

			want := make([]string, len(test.Print))
			for i, text := range test.Print {
				want[i] = strings.Join(text, "\n")
			}

			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("Compile(): (-want, +got)\n%s", diff)
			}
		})
	}
}
