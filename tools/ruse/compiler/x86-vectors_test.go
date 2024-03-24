// Copyright 2024 The Firefly Authors.
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
	"sort"
	"strconv"
	"strings"
	"testing"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/internal/x86"
	"firefly-os.dev/tools/ruse/parser"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

// This is our very expensive set of test vectors.
// Running it takes a long time, so we use another
// test target so we can skip these tests most of
// the time.

func TestX86GeneratedAssemblyTests(t *testing.T) {
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

						'(mode %s)
						(asm-func (test)
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

					pkg, err := types.Check("test", fset, files, arch, info)
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
						if test.Inst.Page != 0 {
							fmt.Fprintf(&b, "  Page:   %d\n", test.Inst.Page)
						}
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
						if test.Inst.Page != 0 {
							fmt.Fprintf(&b, "  Page:    %d\n", test.Inst.Page)
						}
						fmt.Fprintf(&b, "  UID:     %s\n", test.Inst.UID)
						fmt.Fprintf(&b, "  Mode:    %s\n", test.Mode)
						fmt.Fprintf(&b, "  Data:    %d\n", test.Inst.DataSize)
						for i := 0; i < test.Inst.MinArgs; i++ {
							operand := test.Inst.Operands[i]
							fmt.Fprintf(&b, "  Param %d: %s %s %v\n", i+1, operand.Encoding, operand.Type, data.Args[i])
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
							test.Inst.MinArgs == 2 &&
							(test.Inst.Operands[0].UID == "Sreg" ||
								test.Inst.Operands[1].UID == "Sreg") {
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
						if test.Inst.Page != 0 {
							fmt.Fprintf(&b, "  Page:    %d\n", test.Inst.Page)
						}
						fmt.Fprintf(&b, "  UID:     %s\n", test.Inst.UID)
						fmt.Fprintf(&b, "  Code:    %s\n", test.Inst.Encoding.Syntax)
						fmt.Fprintf(&b, "  Mode:    %s\n", test.Mode)
						fmt.Fprintf(&b, "  Data:    %d\n", test.Inst.DataSize)
						fmt.Fprintf(&b, "  Operand: %v\n", test.Inst.OperandSize)
						fmt.Fprintf(&b, "  Address: %v\n", test.Inst.AddressSize)
						fmt.Fprintf(&b, "  Rich:    %s\n", &code)
						for i := 0; i < test.Inst.MinArgs; i++ {
							operand := test.Inst.Operands[i]
							fmt.Fprintf(&b, "  Param %d: %s %s %v\n", i+1, operand.Encoding, operand.Type, data.Args[i])
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

// splitPrefixes takes x86 machine code in
// hexadecimal format and splits it into
// the set of legacy x86 prefixes and the
// remaining machine code.
//
// If the input is not valid hexadecimal,
// splitPrefixes will panic.
func splitPrefixes(s string) (prefixOpcodes, prefixes []byte, rest string) {
	code, err := hex.DecodeString(s)
	if err != nil {
		panic("invalid hex '" + s + "' passed to SplitPrefixes: " + err.Error())
	}

	for i, b := range code {
		switch b {
		case 0x9b:
			prefixOpcodes = append(prefixOpcodes, b)
		case 0xf0, 0xf2, 0xf3, // Group 1.
			0x2e, 0x36, 0x3e, 0x26, 0x64, 0x65, // Group 2.
			0x66, // Group 3.
			0x67: // Group 4.
			prefixes = append(prefixes, b)
		default:
			// Machine code.
			rest = s[i*2:]
			return prefixOpcodes, prefixes, rest
		}
	}

	return prefixOpcodes, prefixes, rest
}

// sortPrefixes takes x86 machine code in
// hexadecimal format and returns it with
// the x86 prefixes sorted into numerical
// order.
//
// If the input is not valid hexadecimal,
// sortPrefixes will panic.
func sortPrefixes(s string) string {
	prefixOpcodes, prefixes, rest := splitPrefixes(s)
	if len(prefixes) == 0 && len(prefixOpcodes) == 0 {
		return rest
	}

	if len(prefixes) == 0 {
		return hex.EncodeToString(prefixOpcodes) + rest
	}

	sort.Slice(prefixes, func(i, j int) bool { return prefixes[i] < prefixes[j] })

	return hex.EncodeToString(prefixOpcodes) + hex.EncodeToString(prefixes) + rest
}
