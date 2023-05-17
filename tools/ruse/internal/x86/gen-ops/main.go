// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command gen-ops uses x86.csv data to
// generate the set of Ruse compiler opcodes
// for x86, supporting each defined
// instruction form.
//
// The resulting Go code is then written out.
package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"firefly-os.dev/tools/ruse/internal/x86"
)

var program = filepath.Base(os.Args[0])

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("")
}

func main() {
	var help bool
	var output string
	flag.BoolVar(&help, "h", false, "Show this message and exit.")
	flag.StringVar(&output, "out", "", "Path to the generated opcodes output")

	flag.Usage = func() {
		log.Printf("Usage:\n  %s OPTIONS\n\n", program)
		flag.PrintDefaults()
		os.Exit(2)
	}

	flag.Parse()
	if help {
		flag.Usage()
	}

	if output == "" {
		flag.Usage()
		os.Exit(2)
	}

	err := genOpcodes(output)
	if err != nil {
		log.Fatal(err)
	}
}

//go:embed templates/*.tmpl
var templatesFS embed.FS

var templates = template.Must(template.New("").ParseFS(templatesFS, "templates/*.tmpl"))

// OpInfo contains information about an instruction.
//
// The instruction is either an abstract instruction,
// emitted by the Ruse compiler, or an instruction
// specific to an individual architecture.
type OpInfo struct {
	Name        string
	Abstract    bool
	Virtual     bool // Not executed on the machine.
	Operands    int  // Number of arguments (or -1 if variadic).
	Commutative bool // The first two arguments can be reordered without effect.
}

// gen4Opcodes reads the x86 architecture
// CSV from input and writes the set of x86
// opcodes as Go code to output.
func genOpcodes(output string) error {
	args := make(map[string]int)
	var opcodes []OpInfo
	for _, inst := range x86.Instructions {
		got := len(inst.Parameters)
		want, ok := args[inst.UID]
		if ok {
			if got != want {
				return fmt.Errorf("operand %q has %d operands, but %q (%d operands) has the same opcode", inst.UID, want, inst.Syntax, got)
			}

			continue
		}

		args[inst.UID] = got
		opcodes = append(opcodes, OpInfo{
			Name:     inst.UID, // Note, the OpX86 prefix is added in ssafir/gen-ops.
			Operands: got,
		})
	}

	// Build the combined data.
	var data struct {
		Command  string
		Package  string
		Variable string
		Opcodes  []OpInfo
	}

	data.Command = filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ")
	data.Package = "main"
	data.Variable = "X86Ops"
	data.Opcodes = opcodes

	var b bytes.Buffer
	err := templates.ExecuteTemplate(&b, "opcodes.go.tmpl", data)
	if err != nil {
		return fmt.Errorf("failed to execute opcodes.go.tmpl template: %v", err)
	}

	fail := false
	formatted, err := format.Source(b.Bytes())
	if err != nil {
		fail = true
		log.Println(err)
		formatted = b.Bytes()
	}

	err = os.WriteFile(output, formatted, 0644)
	if err != nil {
		return fmt.Errorf("failed to write %s: %v", output, err)
	}

	if fail {
		os.Exit(1)
	}

	return nil
}
