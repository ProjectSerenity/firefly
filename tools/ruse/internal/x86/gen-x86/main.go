// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command x86json generates structured data about
// x86 instructions.
//
// We process the official Go x86.csv, adding some
// additional instructions and mnemonics, making
// some changes that make validating assembly easier,
// and adding 16-bit mode compatibility data.
//
// This is then written out in a JSON format.
package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"firefly-os.dev/tools/ruse/internal/x86"
	"firefly-os.dev/tools/ruse/internal/x86/x86csv"
)

var (
	program  = filepath.Base(os.Args[0])
	data_go  = filepath.Join("tools", "ruse", "internal", "x86", "data.go")
	x86_json = filepath.Join("tools", "ruse", "internal", "x86", "x86.json")
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix(program + ": ")
}

//go:embed x86.csv
var x86CSV []byte

//go:embed templates/*.tmpl
var templatesFS embed.FS

var templates = template.Must(template.New("").ParseFS(templatesFS, "templates/*.tmpl"))

func main() {
	err := GenerateJSON()
	if err != nil {
		log.Fatal(err)
	}
}

// GenerateJSON parses the x86 CSV to gather
// the set of x86 instructions.
func GenerateJSON() error {
	// Start by changing to the workspace
	// root directory so we can write the
	// data.go and x86.json data files into
	// their destination.
	if workspace := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); workspace != "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to determine current directory: %v", err)
		}

		defer os.Chdir(cwd) // Move back to where we started once we're finished.

		err = os.Chdir(workspace)
		if err != nil {
			return fmt.Errorf("failed to change directory to %q: %v", workspace, err)
		}
	}

	// Parse in the existing instructions.
	insts, err := ParseGoX86CSV(bytes.NewReader(x86CSV))
	if err != nil {
		return err
	}

	// Add instructions missing from x86.csv.
	insts = append(insts, Extras...)

	SortInstructions(insts)

	// Check that the results are sane, make
	// the unique IDs, and check they really
	// are unique.
	seenUIDs := make(map[string]string)
	for _, inst := range insts {
		err = CheckInstruction(inst)
		if err != nil {
			var b strings.Builder
			fmt.Fprintf(&b, "  Mnemonic:   %s\n", inst.Mnemonic)
			fmt.Fprintf(&b, "  Syntax:     %s\n", inst.Syntax)
			fmt.Fprintf(&b, "  Encoding:   %s\n", inst.Encoding.Syntax)
			if len(inst.Parameters) > 0 {
				fmt.Fprintf(&b, "  Parameters:\n")
				for _, param := range inst.Parameters {
					fmt.Fprintf(&b, "    %s %s (%s)\n", param.Type, param.Syntax, param.Encoding)
				}
			}

			return fmt.Errorf("generated invalid instruction: %v\n%s", err, b.String())
		}

		mnemonic := inst.Mnemonic

		// Make sure the mnemonic suitable to
		// be a Go identifier
		switch mnemonic {
		case "call-far":
			mnemonic = "call_far"
		case "jmp-far":
			mnemonic = "jmp_far"
		case "ret-far":
			mnemonic = "ret_far"
		}

		var b strings.Builder
		b.WriteString(strings.ToUpper(mnemonic))
		for _, param := range inst.Parameters {
			b.WriteByte('_')
			b.WriteString(param.UID)
			if param == x86.ParamM32bcst || param == x86.ParamM64bcst {
				// These forms can be used for different
				// vector size instructions. We append
				// the vector size to disambiguate them.
				fmt.Fprintf(&b, "%d", inst.Encoding.VectorSize())
			}
		}

		if inst.Encoding.REX {
			b.WriteString("_REX")
		}

		if inst.Encoding.VEX {
			b.WriteString("_VEX")
		}

		if inst.Encoding.EVEX {
			b.WriteString("_EVEX")
			b.WriteString(strconv.Itoa(inst.Encoding.VectorSize()))
		}

		uid := b.String()
		if prev, ok := seenUIDs[uid]; ok {
			return fmt.Errorf("two instructions have the same UID:\n  %s\n  %s\n  UID: %s", prev, inst.Syntax, uid)
		}

		// Check the UID is a valid Go
		// identifier.
		x, err := parser.ParseExpr(uid)
		if _, ok := x.(*ast.Ident); err != nil || !ok {
			return fmt.Errorf("instruction %s has invalid UID %q: not a valid identifier", inst.Syntax, uid)
		}

		seenUIDs[uid] = inst.Syntax
		inst.UID = uid
	}

	// Write out the data.
	f, err := os.Create(x86_json)
	if err != nil {
		return fmt.Errorf("failed to create %q: %v", x86_json, err)
	}

	defer f.Close()

	err = WriteJSON(f, insts)
	if err != nil {
		return fmt.Errorf("failed to write instructions to %q: %v", x86_json, err)
	}

	if err = f.Close(); err != nil {
		return fmt.Errorf("failed to close %q: %v", x86_json, err)
	}

	var data struct {
		Command      string
		Instructions []*x86.Instruction
	}

	data.Command = "//tools/ruse/internal/x86/gen-x86"
	data.Instructions = insts

	var b bytes.Buffer
	err = templates.ExecuteTemplate(&b, "data.go.tmpl", data)
	if err != nil {
		return fmt.Errorf("failed to write instructions to %q: %v", data_go, err)
	}

	fail := false
	formatted, err := format.Source(b.Bytes())
	if err != nil {
		fail = true
		log.Println(err)
		formatted = b.Bytes()
	}

	err = os.WriteFile(data_go, formatted, 0644)
	if err != nil {
		return fmt.Errorf("failed to write %s: %v", data_go, err)
	}

	if fail {
		os.Exit(1)
	}

	return nil
}

// ParseGoX86CSV parses the Go x86.csv
// and converts the instructions into
// our format.
func ParseGoX86CSV(r io.Reader) (insts []*x86.Instruction, err error) {
	crr := x86csv.NewReader(r)
	for {
		inst, err := crr.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("failed to read instruction: %v", err)
		}

		if inst.HasTag("pseudo64") {
			continue
		}

		mnemonic := inst.IntelOpcode()

		// First, we decide which instructions
		// to skip altogether.

		if SkipInstruction(mnemonic, inst) {
			continue
		}

		// Then we make changes.

		mnemonic = Fix(mnemonic, inst)

		encoding, err := x86.ParseEncoding(inst.Encoding)
		if err != nil {
			return nil, fmt.Errorf("failed to parse instruction %s: %v", inst.Intel, err)
		}

		var dataSize int
		if inst.DataSize != "" {
			dataSize, err = strconv.Atoi(inst.DataSize)
			if err != nil {
				return nil, err
			}
		}

		// We instantiate the instruction first,
		// then sort out its parameters, as a single
		// instruction form may result in multiple
		// sets of parameters. To handle this, we
		// prepare the instruction with everything
		// else and then make copies as needed.

		var cpuid []string
		if inst.CPUID != "" {
			cpuid = strings.Split(inst.CPUID, "+")
			for i := range cpuid {
				cpuid[i] = strings.TrimSpace(cpuid[i])
			}
		}

		tuple, ok := x86.TupleTypes[inst.TupleType]
		if !ok {
			return nil, fmt.Errorf("instruction %s has invalid tuple type %q", inst.Intel, inst.TupleType)
		}

		parsed := &x86.Instruction{
			Mnemonic:    strings.ToLower(mnemonic),
			Syntax:      inst.Intel,
			Encoding:    encoding,
			Tuple:       tuple,
			Mode16:      inst.Mode32 == "V", // Baseline with 32-bit compatibility.
			Mode32:      inst.Mode32 == "V",
			Mode64:      inst.Mode64 == "V",
			CPUID:       cpuid,
			OperandSize: inst.HasTag("operand16") || inst.HasTag("operand32") || inst.HasTag("operand64"),
			AddressSize: inst.HasTag("address16") || inst.HasTag("address32") || inst.HasTag("address64"),
			DataSize:    dataSize,
		}

		// Identify instructions not supported
		// in 16-bit mode.
		if strings.Contains(inst.Encoding, "VEX.") {
			parsed.Mode16 = false
		}
		switch mnemonic {
		case "CLRSSBSY",
			"INVPCID",
			"JECXZ",
			"LAR",
			"LSL",
			"RSTORSSP":
			parsed.Mode16 = false
		}

		args := inst.IntelArgs()
		for i := range args {
			if i == 0 {
				args[i], parsed.Encoding.Zero = strings.CutSuffix(args[i], "{z}")
				args[i], parsed.Encoding.Mask = strings.CutSuffix(args[i], " {k1}")
				args[i], _ = strings.CutSuffix(args[i], " {k2}")
			}

			var rounding, suppress bool
			args[i], rounding = strings.CutSuffix(args[i], "{er}")
			args[i], suppress = strings.CutSuffix(args[i], "{sae}")
			args[i] = strings.TrimSpace(args[i])

			if rounding {
				parsed.Encoding.Rounding = true
				parsed.Encoding.Suppress = true
			}

			if suppress {
				parsed.Encoding.Suppress = true
			}
		}

		paramSets, err := ParameterCombinations(args)
		if err != nil {
			return nil, fmt.Errorf("failed to parse syntax %q: %v", inst.Intel, err)
		}

		if len(paramSets) == 0 {
			insts = append(insts, parsed)
			continue
		}

		for i, set := range paramSets {
			if i == 0 {
				// This is the simple case.
				parsed.Parameters = paramSets[0]
				insts = append(insts, parsed)
				continue
			}

			// A shallow copy for everything
			// except the parameter set is
			// fine.
			dup := new(x86.Instruction)
			*dup = *parsed
			dup.Parameters = set
			insts = append(insts, dup)
		}
	}

	return insts, nil
}

// WriteJSON writes out a set of x86
// instructions in JSON format.
func WriteJSON(w io.Writer, insts []*x86.Instruction) error {
	jw := json.NewEncoder(w)
	jw.SetEscapeHTML(false)
	for _, inst := range insts {
		err := jw.Encode(inst)
		if err != nil {
			return err
		}
	}

	return nil
}
