// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package compiler

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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

func TestAssembleX86(t *testing.T) {
	tests := []struct {
		Name     string
		Mode     x86.Mode
		Assembly string
		Err      string
		Want     *ssafir.Value
	}{
		{
			Name:     "return",
			Mode:     x86.Mode64,
			Assembly: "(ret)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86RET,
				Extra: &x86InstructionData{
					Length: 1,
				},
			},
		},
		{
			Name:     "shift right",
			Mode:     x86.Mode64,
			Assembly: "(shr eax 3)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86SHR_Rmr32_Imm8,
				Extra: &x86InstructionData{
					Args:   [4]any{x86.EAX, uint64(3)},
					Length: 3,
				},
			},
		},
		{
			Name:     "small displaced adc register pair",
			Mode:     x86.Mode64,
			Assembly: "(adc '(*byte)(+ bx si) cl)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86ADC_M8_R8,
				Extra: &x86InstructionData{
					Args:   [4]any{&x86.Memory{Base: x86.BX_SI}, x86.CL},
					Length: 3,
				},
			},
		},
		{
			Name:     "small displaced adc segment offset",
			Mode:     x86.Mode64,
			Assembly: "(adc '(*byte)(+ es bp 0x7) cl)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86ADC_M8_R8,
				Extra: &x86InstructionData{
					Args:   [4]any{&x86.Memory{Segment: x86.ES, Base: x86.BP, Displacement: 7}, x86.CL},
					Length: 5,
				},
			},
		},
		{
			Name:     "large displaced add",
			Mode:     x86.Mode64,
			Assembly: "(add r8 (+ rdi 7))",
			Want: &ssafir.Value{
				Op: ssafir.OpX86ADD_R64_M64_REX,
				Extra: &x86InstructionData{
					Args:   [4]any{x86.R8, &x86.Memory{Base: x86.RDI, Displacement: 7}},
					Length: 4,
				},
			},
		},
		{
			Name:     "move to register",
			Mode:     x86.Mode16,
			Assembly: "(mov ah 0)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86MOV_R8op_Imm8u,
				Extra: &x86InstructionData{
					Args:   [4]any{x86.AH, uint64(0)},
					Length: 2,
				},
			},
		},
		{
			Name:     "sysret to 32-bit mode",
			Mode:     x86.Mode64,
			Assembly: "(sysret)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86SYSRET,
				Extra: &x86InstructionData{
					Length: 2,
				},
			},
		},
		{
			Name:     "sysret to 64-bit mode",
			Mode:     x86.Mode64,
			Assembly: "(rex.w sysret)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86SYSRET,
				Extra: &x86InstructionData{
					Length: 3,
					REX_W:  true,
				},
			},
		},
		{
			Name:     "stosb",
			Mode:     x86.Mode64,
			Assembly: "(stosb)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86STOSB,
				Extra: &x86InstructionData{
					Length: 1,
				},
			},
		},
		{
			Name:     "rep stosb",
			Mode:     x86.Mode64,
			Assembly: "(rep stosb)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86STOSB,
				Extra: &x86InstructionData{
					Length:    2,
					Prefixes:  [5]x86.Prefix{x86.PrefixRepeat},
					PrefixLen: 1,
				},
			},
		},
		{
			Name:     "extended register",
			Mode:     x86.Mode64,
			Assembly: "(vaddpd ymm3 ymm2 ymm8)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_VEX,
				Extra: &x86InstructionData{
					Args:   [4]any{x86.YMM3, x86.YMM2, x86.YMM8},
					Length: 5,
				},
			},
		},
		{
			Name:     "EVEX extended register",
			Mode:     x86.Mode64,
			Assembly: "(vaddpd ymm14 ymm3 ymm31)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_EVEX,
				Extra: &x86InstructionData{
					Args:   [4]any{x86.YMM14, x86.YMM3, x86.YMM31},
					Length: 6,
				},
			},
		},
		{
			Name:     "EVEX implicit opmask",
			Mode:     x86.Mode64,
			Assembly: "(vaddpd ymm14 ymm3 ymm31)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_EVEX,
				Extra: &x86InstructionData{
					Args:   [4]any{x86.YMM14, x86.YMM3, x86.YMM31},
					Length: 6,
					Mask:   0,
				},
			},
		},
		{
			Name:     "EVEX explicit opmask",
			Mode:     x86.Mode64,
			Assembly: "'(mask k1)(vaddpd ymm14 ymm3 ymm31)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_EVEX,
				Extra: &x86InstructionData{
					Args:   [4]any{x86.YMM14, x86.YMM3, x86.YMM31},
					Length: 6,
					Mask:   1,
				},
			},
		},
		{
			Name:     "EVEX implicit zeroing",
			Mode:     x86.Mode64,
			Assembly: "'(zero false)(vaddpd ymm14 ymm3 ymm31)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_EVEX,
				Extra: &x86InstructionData{
					Args:   [4]any{x86.YMM14, x86.YMM3, x86.YMM31},
					Length: 6,
					Zero:   false,
				},
			},
		},
		{
			Name:     "EVEX explicit zeroing",
			Mode:     x86.Mode64,
			Assembly: "'(zero true)(vaddpd ymm14 ymm3 ymm31)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_EVEX,
				Extra: &x86InstructionData{
					Args:   [4]any{x86.YMM14, x86.YMM3, x86.YMM31},
					Length: 6,
					Zero:   true,
				},
			},
		},
		{
			Name:     "force selection of a longer encoding",
			Mode:     x86.Mode64,
			Assembly: "'(match ADD_Rmr8_Imm8)(add al 1)",
			Want: &ssafir.Value{
				Op: ssafir.OpX86ADD_Rmr8_Imm8,
				Extra: &x86InstructionData{
					Args:   [4]any{x86.AL, uint64(1)},
					Length: 3,
				},
			},
		},
		{
			Name:     "illegal prefix",
			Mode:     x86.Mode32,
			Assembly: "(rep rdrand eax)",
			Err:      "mnemonic \"rdrand\" cannot be used with repeat prefixes",
		},
		{
			Name:     "illegal register",
			Mode:     x86.Mode32,
			Assembly: "(vaddpd ymm3 ymm2 ymm8)",
			Err:      "register ymm8 cannot be used in 32-bit mode",
		},
	}

	compareOptions := []cmp.Option{
		cmp.Exporter(func(t reflect.Type) bool { return true }),            // Allow unexported types to be compared.
		cmpopts.IgnoreTypes(token.Pos(0), ssafir.ID(0), new(ssafir.Block)), // Ignore token.Pos, ssafir.ID, *ssafir.Block values.
	}

	// Use x86-64.
	arch := sys.X86_64
	sizes := types.SizesFor(arch)

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fset := token.NewFileSet()
			mode := test.Mode.Int
			if mode == 0 {
				mode = 64
			}

			text := fmt.Sprintf("(package test)\n\n'(arch x86-64)\n'(mode %d)\n(asm-func test %s)", mode, test.Assembly)
			file, err := parser.ParseFile(fset, "test.ruse", text, 0)
			if err != nil {
				t.Fatalf("failed to parse text: %v", err)
			}

			files := []*ast.File{file}
			info := &types.Info{
				Types:       make(map[ast.Expression]types.TypeAndValue),
				Definitions: make(map[*ast.Identifier]types.Object),
				Uses:        make(map[*ast.Identifier]types.Object),
			}

			var config types.Config
			pkg, err := config.Check("test", fset, files, arch, info)
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

			// The package should have one function with
			// two values; a memory state and an instruction,
			// which we compare with test.Want.
			if len(p.Functions) != 1 {
				t.Fatalf("got %d functions, want 1: %#v", len(p.Functions), p.Functions)
			}

			fun := p.Functions[0]
			if len(fun.Entry.Values) != 2 {
				t.Fatalf("got %d values, want 1: %#v", len(fun.Entry.Values), fun.Entry.Values)
			}

			v := fun.Entry.Values[1]

			if diff := cmp.Diff(test.Want, v, compareOptions...); diff != "" {
				t.Fatalf("Compile(): (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestX86GeneratedAssemblyTests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping bulk test vector tests")
	}

	name := filepath.Join("testdata", "x86-tests.csv.gz")
	f, err := os.Open(name)
	if err != nil {
		t.Fatalf("failed to open %s: %v", name, err)
	}

	defer f.Close()

	// Use x86-64.
	arch := sys.X86_64
	sizes := types.SizesFor(arch)

	x86TestEntryHeader := []string{"uid", "mode", "code", "ruse", "intel"}
	type x86TestEntry struct {
		Inst  *x86.Instruction // Instruction mnemonic.
		Mode  string           // CPU mode.
		Code  string           // Hex-encoded machine code.
		Ruse  string           // Ruse assembly.
		Intel string           // Intel assembly.
	}

	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("failed to read gzip header: %v", err)
	}

	cr := csv.NewReader(r)
	cr.Comment = '#'
	header, err := cr.Read()
	if err != nil {
		t.Fatalf("failed to read header line: %v", err)
	}

	if len(header) != len(x86TestEntryHeader) {
		t.Fatalf("incorrect header line:\n  Got:  %q\n  Want: %q", header, x86TestEntryHeader)
	}

	for i := range header {
		if header[i] != x86TestEntryHeader[i] {
			t.Fatalf("incorrect header line:\n  Got:  %q\n  Want: %q", header, x86TestEntryHeader)
		}
	}

	type testGroup struct {
		Mode  x86.Mode
		Tests []*x86TestEntry
		Want  map[string]map[string]bool // Map assembly to map of machine code sequences to validity.
	}

	tests16 := &testGroup{Mode: x86.Mode16}
	tests32 := &testGroup{Mode: x86.Mode32}
	tests64 := &testGroup{Mode: x86.Mode64}
	tests := []*testGroup{tests16, tests32, tests64}
	for {
		line, err := cr.Read()
		if err != nil && errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			t.Fatal(err)
		}

		uid := line[0]
		inst := x86.InstructionsByUID[uid]
		if inst == nil {
			t.Fatalf("no instruction with UID %q", uid)
		}

		test := &x86TestEntry{
			Inst:  inst,
			Mode:  line[1],
			Code:  line[2],
			Ruse:  line[3],
			Intel: line[4],
		}

		switch test.Mode {
		case "16":
			tests16.Tests = append(tests16.Tests, test)
		case "32":
			tests32.Tests = append(tests32.Tests, test)
		case "64":
			tests64.Tests = append(tests64.Tests, test)
		default:
			t.Fatalf("found test with unexpected mode %q: %s", test.Mode, line)
		}
	}

	err = r.Close()
	if err != nil {
		t.Fatalf("failed to close GZIP reader: %v", err)
	}

	// The tests map each instruction form
	// to the correct machine code for that
	// instruction form. However, there may
	// be other instruction forms that would
	// be a valid selection for the given
	// assembly, meaning our assembler may
	// generate different (but still valid)
	// machine code.
	//
	// To account for this, we gather up
	// the set of mappings from unique
	// assembly to corresponding machine
	// code so that we can accept any of the
	// machine code selections.
	for _, group := range tests {
		group.Want = make(map[string]map[string]bool)
		for _, test := range group.Tests {
			accept, ok := group.Want[test.Ruse]
			if !ok {
				accept = make(map[string]bool)
				group.Want[test.Ruse] = accept
			}

			accept[sortPrefixes(test.Code)] = true
		}
	}

	// Stats gathering.
	all := &TestAssemblyGroup{Name: "all:"}
	modes := map[uint8]*TestAssemblyGroup{
		16: {Name: "x86-16:"},
		32: {Name: "x86-32:"},
		64: {Name: "x86-64:"},
	}

	spaceHex := func(s string) string {
		var b strings.Builder
		b.Grow(len(s) + len(s)/2)
		for i := 0; i < len(s); i++ {
			if i > 0 && i%2 == 0 {
				b.WriteByte(' ')
			}

			b.WriteByte(s[i])
		}

		return b.String()
	}

	prettyMachineCode := func(s string) string {
		prefixOpcodes, prefixes, rest := splitPrefixes(s)
		if len(prefixes) == 0 && len(prefixOpcodes) == 0 {
			return spaceHex(rest)
		}

		if len(prefixes) == 0 {
			return fmt.Sprintf("% x|%s", prefixOpcodes, spaceHex(rest))
		}

		return fmt.Sprintf("% x|% x|%s", prefixOpcodes, prefixes, spaceHex(rest))
	}

	var b bytes.Buffer
	var code x86.Code
	for _, tests := range tests {
		local := modes[tests.Mode.Int]
		t.Run(strconv.Itoa(int(tests.Mode.Int)), func(t *testing.T) {
			for _, test := range tests.Tests {
				t.Run(test.Code, func(t *testing.T) {
					all.Start()
					local.Start()

					options := tests.Want[test.Ruse]
					if len(options) == 0 {
						t.Fatalf("found no expected results:\n  Ruse: %s\n  Code: %s", test.Ruse, test.Code)
					}

					want := make([]string, 0, len(options))
					for option := range options {
						want = append(want, prettyMachineCode(option))
					}

					sort.Strings(want)

					fset := token.NewFileSet()
					text := fmt.Sprintf(`
						(package test)

						'(arch x86-64)
						'(mode %s)
						(asm-func test
							'(match %s)
							%s)`, test.Mode, test.Inst.UID, test.Ruse)
					file, err := parser.ParseFile(fset, "test.ruse", text, 0)
					if err != nil {
						all.Fail()
						local.Fail()
						if all.Ok() {
							t.Errorf("failed to parse:\n  Ruse: %s\n  Intel: %s\n    %v", test.Ruse, test.Intel, err)
						}

						return
					}

					files := []*ast.File{file}
					info := &types.Info{
						Types:       make(map[ast.Expression]types.TypeAndValue),
						Definitions: make(map[*ast.Identifier]types.Object),
						Uses:        make(map[*ast.Identifier]types.Object),
					}

					var config types.Config
					pkg, err := config.Check("test", fset, files, arch, info)
					if err != nil {
						all.Fail()
						local.Fail()
						if all.Ok() {
							t.Errorf("failed to type-check:\n  Ruse: %s\n  Intel: %s\n    %v", test.Ruse, test.Intel, err)
						}

						return
					}

					defer func() {
						v := recover()
						if v != nil {
							all.Fail()
							local.Fail()
							if !all.Ok() {
								t.Skip()
							}

							var b strings.Builder
							fmt.Fprintf(&b, "failed to compile:\n")
							fmt.Fprintf(&b, "  Ruse:  %s\n", test.Ruse)
							fmt.Fprintf(&b, "  Intel:  %s\n", test.Intel)
							fmt.Fprintf(&b, "  Syntax: %s\n", test.Inst.Syntax)
							fmt.Fprintf(&b, "  UID:    %s\n", test.Inst.UID)
							fmt.Fprintf(&b, "  Code:   %s\n", test.Inst.Encoding.Syntax)
							fmt.Fprintf(&b, "  Mode:   %s\n", test.Mode)
							fmt.Fprintf(&b, "    panic: %v\n", v)
							fmt.Fprintf(&b, "    Want: %s", strings.Join(want, "\n          "))
							t.Error(b.String())

							return
						}
					}()

					p, err := Compile(fset, arch, pkg, files, info, sizes)
					if err != nil {
						all.Fail()
						local.Fail()
						if !all.Ok() {
							t.Skip()
						}

						var b strings.Builder
						fmt.Fprintf(&b, "failed to compile:\n")
						fmt.Fprintf(&b, "  Ruse:  %s\n", test.Ruse)
						fmt.Fprintf(&b, "  Intel:  %s\n", test.Intel)
						fmt.Fprintf(&b, "  Syntax: %s\n", test.Inst.Syntax)
						fmt.Fprintf(&b, "  UID:    %s\n", test.Inst.UID)
						fmt.Fprintf(&b, "  Code:   %s\n", test.Inst.Encoding.Syntax)
						fmt.Fprintf(&b, "  Mode:   %s\n", test.Mode)
						fmt.Fprintf(&b, "    %v\n", err)
						fmt.Fprintf(&b, "    Want: %s", strings.Join(want, "\n          "))
						t.Error(b.String())

						return
					}

					// The package should have one function with
					// two values; a memory state and an instruction,
					// which we compare with test.Want.
					if len(p.Functions) != 1 {
						t.Errorf("bad compile of %s   (%s): got %d functions, want 1: %#v", test.Ruse, test.Intel, len(p.Functions), p.Functions)
					}

					fun := p.Functions[0]
					if len(fun.Entry.Values) != 2 {
						t.Fatalf("bad compile of %s   (%s): got %d values, want 1: %#v", test.Ruse, test.Intel, len(fun.Entry.Values), fun.Entry.Values)
					}

					v := fun.Entry.Values[1]

					data, ok := v.Extra.(*x86InstructionData)
					if !ok {
						t.Fatalf("bad compile of %s   (%s): got value with bad extra: %#v", test.Ruse, test.Intel, v.Extra)
					}

					var mode x86.Mode
					switch test.Mode {
					case "16":
						mode = x86.Mode16
					case "32":
						mode = x86.Mode32
					case "64":
						mode = x86.Mode64
					}

					err = x86EncodeInstruction(&code, mode, v.Op, data)
					if err != nil {
						all.Wrong()
						local.Wrong()
						if !all.Right() {
							t.Skip()
						}

						var b strings.Builder
						fmt.Fprintf(&b, "wrong encoding:\n")
						fmt.Fprintf(&b, "  Ruse:   %s\n", test.Ruse)
						fmt.Fprintf(&b, "  Intel:   %s\n", test.Intel)
						fmt.Fprintf(&b, "  Syntax:  %s\n", test.Inst.Syntax)
						fmt.Fprintf(&b, "  UID:     %s\n", test.Inst.UID)
						fmt.Fprintf(&b, "  Mode:    %s\n", test.Mode)
						fmt.Fprintf(&b, "  Data:    %d\n", test.Inst.DataSize)
						for i, param := range test.Inst.Parameters {
							fmt.Fprintf(&b, "  Param %d: %s %s %v\n", i+1, param.Encoding, param.Type, data.Args[i])
						}
						fmt.Fprintf(&b, "    Code: %s\n", test.Inst.Encoding.Syntax)
						fmt.Fprintf(&b, "    %v\n", err)
						fmt.Fprintf(&b, "    Want: %v\n", strings.Join(want, "\n          "))
						fmt.Fprintf(&b, "    Code: %s", test.Inst.Encoding.Syntax)
						t.Error(b.String())

						return
					}

					b.Reset()
					code.EncodeTo(&b)
					got := hex.EncodeToString(b.Bytes())
					ok = options[got]
					if !ok {
						ok = options[sortPrefixes(got)]
					}
					if !ok {
						// MOV on a segment register is
						// always a 16-bit operation, even
						// when a 32-bit register is used.
						// The Intel manual says that "In
						// 32-bit mode, the assembler may
						// insert the 16-bit operand-size
						// prefix". We don't, but Clang
						// does, so we check whether that
						// is the only difference.
						//
						// For the purpose of this test,
						// we treat 64-bit mode the same
						// as 32-bit mode.
						if (test.Mode == "32" || test.Mode == "64") &&
							test.Inst.Mnemonic == "mov" &&
							len(test.Inst.Parameters) == 2 &&
							(test.Inst.Parameters[0].UID == "Sreg" ||
								test.Inst.Parameters[1].UID == "Sreg") {
							ok = options[sortPrefixes("66"+got)]
						}
					}

					if !ok {
						all.Wrong()
						local.Wrong()
						if !all.Right() {
							t.Skip()
						}

						var b strings.Builder
						fmt.Fprintf(&b, "wrong encoding:\n")
						fmt.Fprintf(&b, "  Ruse:   %s\n", test.Ruse)
						fmt.Fprintf(&b, "  Intel:   %s\n", test.Intel)
						fmt.Fprintf(&b, "  Syntax:  %s\n", test.Inst.Syntax)
						fmt.Fprintf(&b, "  UID:     %s\n", test.Inst.UID)
						fmt.Fprintf(&b, "  Code:    %s\n", test.Inst.Encoding.Syntax)
						fmt.Fprintf(&b, "  Mode:    %s\n", test.Mode)
						fmt.Fprintf(&b, "  Data:    %d\n", test.Inst.DataSize)
						fmt.Fprintf(&b, "  Operand: %v\n", test.Inst.OperandSize)
						fmt.Fprintf(&b, "  Address: %v\n", test.Inst.AddressSize)
						fmt.Fprintf(&b, "  Rich:    %s\n", &code)
						for i, param := range test.Inst.Parameters {
							fmt.Fprintf(&b, "  Param %d: %s %s %v\n", i+1, param.Encoding, param.Type, data.Args[i])
						}
						fmt.Fprintf(&b, "    Got:  %v\n", prettyMachineCode(got))
						fmt.Fprintf(&b, "    Want: %s", strings.Join(want, "\n          "))
						t.Error(b.String())

						return
					}
				})
			}
		})
	}

	modes[16].Print()
	modes[32].Print()
	modes[64].Print()
	all.Print()
}

func TestEncodeInstructionX86(t *testing.T) {
	rex := func(s string) x86.REX {
		var out x86.REX
		out.SetOn()
		for _, r := range s {
			switch r {
			case 'W':
				out.SetW(true)
			case 'R':
				out.SetR(true)
			case 'X':
				out.SetX(true)
			case 'B':
				out.SetB(true)
			default:
				t.Helper()
				t.Fatalf("invalid REX value %c", r)
			}
		}

		return out
	}

	tests := []struct {
		Name     string
		Mode     x86.Mode
		Assembly string
		Op       ssafir.Op
		Data     *x86InstructionData
		Want     *x86.Code
	}{
		{
			Name:     "ret",
			Mode:     x86.Mode64,
			Assembly: "(ret)",
			Op:       ssafir.OpX86RET,
			Data:     &x86InstructionData{},
			Want: &x86.Code{
				Opcode:    [3]byte{0xc3},
				OpcodeLen: 1,
			},
		},
		{
			Name:     "shift right",
			Mode:     x86.Mode64,
			Assembly: "(shr ecx 18)",
			Op:       ssafir.OpX86SHR_Rmr32_Imm8,
			Data: &x86InstructionData{
				Args: [4]any{
					x86.ECX,
					uint64(18),
				},
			},
			Want: &x86.Code{
				Opcode:       [3]byte{0xc1},
				OpcodeLen:    1,
				UseModRM:     true,
				ModRM:        0b11_101_001,
				Immediate:    [8]byte{0x12},
				ImmediateLen: 1,
			},
		},
		{
			Name:     "large add",
			Mode:     x86.Mode64,
			Assembly: "(add r8 (rdi))",
			Op:       ssafir.OpX86ADD_R64_M64_REX,
			Data: &x86InstructionData{
				Args: [4]any{x86.R8, &x86.Memory{Base: x86.RDI}},
			},
			Want: &x86.Code{
				REX:       rex("WR"),
				Opcode:    [3]byte{0x03},
				OpcodeLen: 1,
				UseModRM:  true,
				ModRM:     0b00_000_111,
			},
		},
		{
			Name:     "large displaced add",
			Mode:     x86.Mode64,
			Assembly: "(add r8 (+ rdi 7))",
			Op:       ssafir.OpX86ADD_R64_M64_REX,
			Data: &x86InstructionData{
				Args: [4]any{x86.R8, &x86.Memory{Base: x86.RDI, Displacement: 7}},
			},
			Want: &x86.Code{
				REX:             rex("WR"),
				Opcode:          [3]byte{0x03},
				OpcodeLen:       1,
				UseModRM:        true,
				ModRM:           0b01_000_111,
				Displacement:    [8]byte{7},
				DisplacementLen: 1,
			},
		},
		{
			Name:     "move to from ES segment",
			Mode:     x86.Mode32,
			Assembly: "(mov ah (es eax)",
			Op:       ssafir.OpX86MOV_R8_M8,
			Data: &x86InstructionData{
				Args: [4]any{x86.AH, &x86.Memory{Segment: x86.ES, Base: x86.EAX}},
			},
			Want: &x86.Code{
				Prefixes:  [14]x86.Prefix{x86.PrefixES},
				Opcode:    [3]byte{0x8a},
				OpcodeLen: 1,
				UseModRM:  true,
				ModRM:     0b00_100_000,
			},
		},
		{
			Name:     "move to from CS segment",
			Mode:     x86.Mode32,
			Assembly: "(mov ah (cs eax)",
			Op:       ssafir.OpX86MOV_R8_M8,
			Data: &x86InstructionData{
				Args: [4]any{x86.AH, &x86.Memory{Segment: x86.CS, Base: x86.EAX}},
			},
			Want: &x86.Code{
				Prefixes:  [14]x86.Prefix{x86.PrefixCS},
				Opcode:    [3]byte{0x8a},
				OpcodeLen: 1,
				UseModRM:  true,
				ModRM:     0b00_100_000,
			},
		},
		{
			Name:     "move to from SS segment",
			Mode:     x86.Mode32,
			Assembly: "(mov ah (ss eax)",
			Op:       ssafir.OpX86MOV_R8_M8,
			Data: &x86InstructionData{
				Args: [4]any{x86.AH, &x86.Memory{Segment: x86.SS, Base: x86.EAX}},
			},
			Want: &x86.Code{
				Prefixes:  [14]x86.Prefix{x86.PrefixSS},
				Opcode:    [3]byte{0x8a},
				OpcodeLen: 1,
				UseModRM:  true,
				ModRM:     0b00_100_000,
			},
		},
		{
			Name:     "move to from DS segment",
			Mode:     x86.Mode32,
			Assembly: "(mov ah (ds eax)",
			Op:       ssafir.OpX86MOV_R8_M8,
			Data: &x86InstructionData{
				Args: [4]any{x86.AH, &x86.Memory{Segment: x86.DS, Base: x86.EAX}},
			},
			Want: &x86.Code{
				Prefixes:  [14]x86.Prefix{x86.PrefixDS},
				Opcode:    [3]byte{0x8a},
				OpcodeLen: 1,
				UseModRM:  true,
				ModRM:     0b00_100_000,
			},
		},
		{
			Name:     "move to from FS segment",
			Mode:     x86.Mode32,
			Assembly: "(mov ah (fs eax)",
			Op:       ssafir.OpX86MOV_R8_M8,
			Data: &x86InstructionData{
				Args: [4]any{x86.AH, &x86.Memory{Segment: x86.FS, Base: x86.EAX}},
			},
			Want: &x86.Code{
				Prefixes:  [14]x86.Prefix{x86.PrefixFS},
				Opcode:    [3]byte{0x8a},
				OpcodeLen: 1,
				UseModRM:  true,
				ModRM:     0b00_100_000,
			},
		},
		{
			Name:     "move to from GS segment",
			Mode:     x86.Mode32,
			Assembly: "(mov ah (gs eax)",
			Op:       ssafir.OpX86MOV_R8_M8,
			Data: &x86InstructionData{
				Args: [4]any{x86.AH, &x86.Memory{Segment: x86.GS, Base: x86.EAX}},
			},
			Want: &x86.Code{
				Prefixes:  [14]x86.Prefix{x86.PrefixGS},
				Opcode:    [3]byte{0x8a},
				OpcodeLen: 1,
				UseModRM:  true,
				ModRM:     0b00_100_000,
			},
		},
		{
			Name:     "size override mov",
			Mode:     x86.Mode64,
			Assembly: "(mov eax (edx))",
			Op:       ssafir.OpX86MOV_R32_M32,
			Data: &x86InstructionData{
				Args: [4]any{x86.EAX, &x86.Memory{Base: x86.EDX}},
			},
			Want: &x86.Code{
				Prefixes:  [14]x86.Prefix{x86.PrefixAddressSize},
				Opcode:    [3]byte{0x8b},
				OpcodeLen: 1,
				UseModRM:  true,
				ModRM:     0b00_000_010,
			},
		},
		{
			Name:     "specialised cmppd",
			Mode:     x86.Mode16,
			Assembly: "(cmpeqpd xmm0 (0xb))",
			Op:       ssafir.OpX86CMPEQPD_XMM1_M128,
			Data: &x86InstructionData{
				Args: [4]any{x86.XMM0, &x86.Memory{Displacement: 0xb}},
			},
			Want: &x86.Code{
				Prefixes:        [14]x86.Prefix{x86.PrefixOperandSize},
				Opcode:          [3]byte{0x0f, 0xc2},
				OpcodeLen:       2,
				UseModRM:        true,
				ModRM:           0b00_000_110,
				Displacement:    [8]byte{0x0b, 0x00},
				DisplacementLen: 2,
				Immediate:       [8]byte{0x00},
				ImmediateLen:    1,
			},
		},
		{
			Name:     "old fsave",
			Mode:     x86.Mode32,
			Assembly: "(fsave (ecx))",
			Op:       ssafir.OpX86FSAVE_M94l108byte,
			Data: &x86InstructionData{
				Args: [4]any{&x86.Memory{Base: x86.ECX}},
			},
			Want: &x86.Code{
				PrefixOpcodes: [5]byte{0x9b},
				Opcode:        [3]byte{0xdd},
				OpcodeLen:     1,
				UseModRM:      true,
				ModRM:         0b00_110_001,
			},
		},
		{
			Name:     "sysret to 32-bit mode",
			Mode:     x86.Mode64,
			Assembly: "(sysret)",
			Op:       ssafir.OpX86SYSRET,
			Data:     &x86InstructionData{},
			Want: &x86.Code{
				Opcode:    [3]byte{0x0f, 0x07},
				OpcodeLen: 2,
			},
		},
		{
			Name:     "sysret to 64-bit mode",
			Mode:     x86.Mode64,
			Assembly: "(rex.w sysret)",
			Op:       ssafir.OpX86SYSRET,
			Data: &x86InstructionData{
				REX_W: true,
			},
			Want: &x86.Code{
				REX:       rex("W"),
				Opcode:    [3]byte{0x0f, 0x07},
				OpcodeLen: 2,
			},
		},
		{
			Name:     "stosb",
			Mode:     x86.Mode64,
			Assembly: "(stosb)",
			Op:       ssafir.OpX86STOSB,
			Data:     &x86InstructionData{},
			Want: &x86.Code{
				Opcode:    [3]byte{0xaa},
				OpcodeLen: 1,
			},
		},
		{
			Name:     "rep stosb",
			Mode:     x86.Mode64,
			Assembly: "(rep stosb)",
			Op:       ssafir.OpX86STOSB,
			Data: &x86InstructionData{
				Prefixes:  [5]x86.Prefix{x86.PrefixRepeat},
				PrefixLen: 1,
			},
			Want: &x86.Code{
				Prefixes:  [14]x86.Prefix{x86.PrefixRepeat},
				Opcode:    [3]byte{0xaa},
				OpcodeLen: 1,
			},
		},
		{
			Name:     "EVEX extended register",
			Mode:     x86.Mode64,
			Assembly: "(vaddpd ymm14 ymm3 ymm31)",
			Op:       ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_EVEX,
			Data: &x86InstructionData{
				Args: [4]any{x86.YMM14, x86.YMM3, x86.YMM31},
			},
			Want: &x86.Code{
				EVEX: x86.EVEX{
					0b0001_0001, // 0x11: R:0, X:0, B:0, R':1, mm:01.
					0b1110_0101, // 0xe5: W:0, vvvv:1100, pp:01.
					0b0010_1000, // 0x28: z:0, L':0, L:1, b:0, V':1, aaa:000.
				},
				Opcode:    [3]byte{0x58},
				OpcodeLen: 1,
				ModRM:     0b11_110_111,
				UseModRM:  true,
			},
		},
		{
			Name:     "EVEX uncompressed displacement",
			Mode:     x86.Mode64,
			Assembly: "(vaddpd ymm19 ymm3 (+ rax 513))",
			Op:       ssafir.OpX86VADDPD_YMM1_YMMV_M256_EVEX,
			Data: &x86InstructionData{
				Args: [4]any{x86.YMM19, x86.YMM3, &x86.Memory{Base: x86.RAX, Displacement: 513}},
			},
			Want: &x86.Code{
				EVEX: x86.EVEX{
					0b1110_0001, // 0xe1: R:1, X:1, B:1, R':0, mm:01.
					0b1110_0101, // 0xe5: W:0, vvvv:1100, pp:01.
					0b0010_1000, // 0x28: z:0, L':0, L:1, b:0, V':1, aaa:000.
				},
				Opcode:          [3]byte{0x58},
				OpcodeLen:       1,
				ModRM:           0b10_011_000,
				UseModRM:        true,
				Displacement:    [8]byte{0x01, 0x02, 0x00, 0x00},
				DisplacementLen: 4,
			},
		},
		{
			Name:     "EVEX compressed displacement",
			Mode:     x86.Mode64,
			Assembly: "(vaddpd ymm19 ymm3 (+ rax 512))",
			Op:       ssafir.OpX86VADDPD_YMM1_YMMV_M256_EVEX,
			Data: &x86InstructionData{
				Args: [4]any{x86.YMM19, x86.YMM3, &x86.Memory{Base: x86.RAX, Displacement: 512}},
			},
			Want: &x86.Code{
				EVEX: x86.EVEX{
					0b1110_0001, // 0xe1: R:1, X:1, B:1, R':0, mm:01.
					0b1110_0101, // 0xe5: W:0, vvvv:1100, pp:01.
					0b0010_1000, // 0x28: z:0, L':0, L:1, b:0, V':1, aaa:000.
				},
				Opcode:          [3]byte{0x58},
				OpcodeLen:       1,
				ModRM:           0b01_011_000,
				UseModRM:        true,
				Displacement:    [8]byte{0x10},
				DisplacementLen: 1,
			},
		},
		{
			Name:     "EVEX implicit opmask",
			Mode:     x86.Mode64,
			Assembly: "'(mask k0)(vaddpd ymm14 ymm3 ymm31)",
			Op:       ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_EVEX,
			Data: &x86InstructionData{
				Args: [4]any{x86.YMM14, x86.YMM3, x86.YMM31},
				Mask: 0,
			},
			Want: &x86.Code{
				EVEX: x86.EVEX{
					0b0001_0001, // 0x11: R:0, X:0, B:0, R':1, mm:01.
					0b1110_0101, // 0xe5: W:0, vvvv:1100, pp:01.
					0b0010_1000, // 0x28: z:0, L':0, L:1, b:0, V':1, aaa:000.
				},
				Opcode:    [3]byte{0x58},
				OpcodeLen: 1,
				ModRM:     0b11_110_111,
				UseModRM:  true,
			},
		},
		{
			Name:     "EVEX explicit opmask",
			Mode:     x86.Mode64,
			Assembly: "'(mask k7)(vaddpd ymm14 ymm3 ymm31)",
			Op:       ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_EVEX,
			Data: &x86InstructionData{
				Args: [4]any{x86.YMM14, x86.YMM3, x86.YMM31},
				Mask: 7,
			},
			Want: &x86.Code{
				EVEX: x86.EVEX{
					0b0001_0001, // 0x11: R:0, X:0, B:0, R':1, mm:01.
					0b1110_0101, // 0xe5: W:0, vvvv:1100, pp:01.
					0b0010_1111, // 0x2f: z:0, L':0, L:1, b:0, V':1, aaa:111.
				},
				Opcode:    [3]byte{0x58},
				OpcodeLen: 1,
				ModRM:     0b11_110_111,
				UseModRM:  true,
			},
		},
		{
			Name:     "EVEX implicit zeroing",
			Mode:     x86.Mode64,
			Assembly: "'(zero false)(vaddpd ymm14 ymm3 ymm31)",
			Op:       ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_EVEX,
			Data: &x86InstructionData{
				Args: [4]any{x86.YMM14, x86.YMM3, x86.YMM31},
				Zero: false,
			},
			Want: &x86.Code{
				EVEX: x86.EVEX{
					0b0001_0001, // 0x11: R:0, X:0, B:0, R':1, mm:01.
					0b1110_0101, // 0xe5: W:0, vvvv:1100, pp:01.
					0b0010_1000, // 0x28: z:0, L':0, L:1, b:0, V':1, aaa:000.
				},
				Opcode:    [3]byte{0x58},
				OpcodeLen: 1,
				ModRM:     0b11_110_111,
				UseModRM:  true,
			},
		},
		{
			Name:     "EVEX explicit zeroing",
			Mode:     x86.Mode64,
			Assembly: "'(zero true)(vaddpd ymm14 ymm3 ymm31)",
			Op:       ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_EVEX,
			Data: &x86InstructionData{
				Args: [4]any{x86.YMM14, x86.YMM3, x86.YMM31},
				Zero: true,
			},
			Want: &x86.Code{
				EVEX: x86.EVEX{
					0b0001_0001, // 0x11: R:0, X:0, B:0, R':1, mm:01.
					0b1110_0101, // 0xe5: W:0, vvvv:1100, pp:01.
					0b1010_1000, // 0xa8: z:1, L':0, L:1, b:0, V':1, aaa:000.
				},
				Opcode:    [3]byte{0x58},
				OpcodeLen: 1,
				ModRM:     0b11_110_111,
				UseModRM:  true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var got x86.Code
			err := x86EncodeInstruction(&got, test.Mode, test.Op, test.Data)
			if err != nil {
				t.Fatalf("%s.Encode(): %v", test.Assembly, err)
			}

			if diff := cmp.Diff(test.Want, &got); diff != "" {
				t.Fatalf("%s.Encode(): (-want, +got)\n%s", test.Assembly, diff)
			}
		})
	}
}

func TestEncodeX86(t *testing.T) {
	tests := []struct {
		Name  string
		Ruse  string
		Want  []byte
		Links []*ssafir.Link
	}{
		{
			Name: "simple",
			Ruse: `
				'(arch x86-64)
				'(mode 64)
				(asm-func test
					(mov cl 1)
					(xchg rax rax)
					(syscall))
			`,
			Want: []byte{
				0xb1, 0x01, // MOV cl, 1
				0x48, 0x90, // XCHG rax, rax
				0x0f, 0x05, // SYSCALL
			},
		},
		{
			Name: "backwards jumps",
			Ruse: `
				'(arch x86-64)
				'(mode 64)
				(asm-func test
					'bar
					(mov cl 1)
					'foo
					(xchg rax rax)
					(je 'foo)
					(jmp 'bar))
			`,
			Want: []byte{
				0xb1, 0x01, // MOV cl, 1
				0x48, 0x90, // XCHG rax, rax
				0x74, 0xfc, // JE -4
				0xeb, 0xf8, // JMP -8
			},
		},
		{
			Name: "forwards jumps",
			Ruse: `
				'(arch x86-64)
				'(mode 64)
				(asm-func test
					(je 'foo)
					(jmp 'bar)
					(mov cl 1)
					'bar
					(xchg rax rax)
					'foo)
			`,
			Want: []byte{
				0x74, 0x06, // JE +6
				0xeb, 0x02, // JMP +2
				0xb1, 0x01, // MOV cl, 1
				0x48, 0x90, // XCHG rax, rax
			},
		},
		{
			Name: "string constant length",
			Ruse: `
				(let hello-world "Hello, world!")

				'(arch x86-64)
				(asm-func test
					(mov ecx (len hello-world)))
			`,
			Want: []byte{
				0xb9, 0x0d, 0x00, 0x00, 0x00, // MOV ecx, 13.
			},
		},
		{
			Name: "64 bit string constant link",
			Ruse: `
				(let hello-world "Hello, world!")

				'(arch x86-64)
				(asm-func test
					(nop)
					(mov rcx (string-pointer hello-world))
					(nop))
			`,
			Want: []byte{
				0x90,                                                       // NOP.
				0x48, 0xb9, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, // MOV rcx, 0x1122334455667788.
				0x90, // NOP.
			},
			Links: []*ssafir.Link{
				{
					Name:    "test.hello-world",
					Type:    ssafir.LinkFullAddress,
					Size:    64,
					Offset:  3,
					Address: 11,
				},
			},
		},
		{
			Name: "32 bit string constant link",
			Ruse: `
				(let hello-world "Hello, world!")

				'(arch x86-64)
				'(mode 32)
				(asm-func test
					(nop)
					(mov ecx (string-pointer hello-world))
					(nop))
			`,
			Want: []byte{
				0x90,                         // NOP.
				0xb9, 0x44, 0x33, 0x22, 0x11, // MOV rcx, 0x11223344.
				0x90, // NOP.
			},
			Links: []*ssafir.Link{
				{
					Name:    "test.hello-world",
					Type:    ssafir.LinkFullAddress,
					Size:    32,
					Offset:  2,
					Address: 6,
				},
			},
		},
		{
			Name: "64 bit relative function link",
			Ruse: `
				(let hello-world "Hello, world!") ; This should be a function, but we've set up the test to expect only one function.

				'(arch x86-64)
				(asm-func test
					(nop)
					(call (string-pointer hello-world))
					(nop))
			`,
			Want: []byte{
				0x90,                         // NOP.
				0xe8, 0x3f, 0x33, 0x22, 0x11, // CALL +0x11223344.
				0x90, // NOP.
			},
			Links: []*ssafir.Link{
				{
					Name:    "test.hello-world",
					Type:    ssafir.LinkRelativeAddress,
					Size:    32,
					Offset:  2,
					Address: 6,
				},
			},
		},
	}

	compareOptions := []cmp.Option{
		cmpopts.IgnoreTypes(token.Pos(0)), // Ignore token.Pos.
	}

	// Use x86-64.
	arch := sys.X86_64
	sizes := types.SizesFor(arch)

	var b bytes.Buffer
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.ruse", "(package test)\n\n"+test.Ruse, 0)
			if err != nil {
				t.Fatalf("failed to parse:\n  Ruse: %s\n    %v", test.Ruse, err)
			}

			files := []*ast.File{file}
			info := &types.Info{
				Types:       make(map[ast.Expression]types.TypeAndValue),
				Definitions: make(map[*ast.Identifier]types.Object),
				Uses:        make(map[*ast.Identifier]types.Object),
			}

			var config types.Config
			pkg, err := config.Check("test", fset, files, arch, info)
			if err != nil {
				t.Fatalf("failed to type-check:\n  Ruse: %s\n    %v", test.Ruse, err)
			}

			defer func() {
				v := recover()
				if v != nil {
					var b strings.Builder
					fmt.Fprintf(&b, "failed to compile:\n")
					fmt.Fprintf(&b, "  Ruse:  %s\n", test.Ruse)
					fmt.Fprintf(&b, "    panic: %v\n", v)
					fmt.Fprintf(&b, "    Want: % x", test.Want)
					t.Fatal(b.String())
				}
			}()

			p, err := Compile(fset, arch, pkg, files, info, sizes)
			if err != nil {
				var b strings.Builder
				fmt.Fprintf(&b, "failed to compile:\n")
				fmt.Fprintf(&b, "  Ruse:  %s\n", test.Ruse)
				fmt.Fprintf(&b, "    %v\n", err)
				fmt.Fprintf(&b, "    Want: % x", test.Want)
				t.Fatal(b.String())
			}

			// The package should have one function with
			// two values; a memory state and an instruction,
			// which we compare with test.Want.
			if len(p.Functions) != 1 {
				t.Fatalf("bad compile of %s: got %d functions, want 1: %#v", test.Ruse, len(p.Functions), p.Functions)
			}

			fun := p.Functions[0]

			b.Reset()
			err = EncodeTo(&b, fset, arch, fun)
			if err != nil {
				var b strings.Builder
				fmt.Fprintf(&b, "wrong encoding:\n")
				fmt.Fprintf(&b, "  Ruse:   %s\n", test.Ruse)
				fmt.Fprintf(&b, "    %v\n", err)
				fmt.Fprintf(&b, "    Want: % x", test.Want)
				t.Fatal(b.String())
			}

			got := b.Bytes()
			if !bytes.Equal(got, test.Want) {
				var b strings.Builder
				fmt.Fprintf(&b, "wrong encoding:\n")
				fmt.Fprintf(&b, "  Ruse:   %s\n", test.Ruse)
				fmt.Fprintf(&b, "    Got:  % x\n", got)
				fmt.Fprintf(&b, "    Want: % x", test.Want)
				t.Fatal(b.String())
			}

			if diff := cmp.Diff(test.Links, fun.Links, compareOptions...); diff != "" {
				t.Fatalf("Compile(): (-want, +got)\n%s", diff)
			}
		})
	}
}
