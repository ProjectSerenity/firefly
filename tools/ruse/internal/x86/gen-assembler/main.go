// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command gen-assembler uses x86 instruction data
// to generate an assembler implementation for
// x86, supporting each defined instruction form.
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

var statsOnly bool

func main() {
	var help bool
	var output string
	flag.BoolVar(&help, "h", false, "Show this message and exit.")
	flag.StringVar(&output, "out", "", "Path to the generated assembler data")
	flag.BoolVar(&statsOnly, "stats", false, "Print stats then exit.")

	flag.Usage = func() {
		log.Printf("Usage:\n  %s OPTIONS\n\n", program)
		flag.PrintDefaults()
		os.Exit(2)
	}

	flag.Parse()
	if help {
		flag.Usage()
	}

	if output == "" && !statsOnly {
		flag.Usage()
		os.Exit(2)
	}

	err := genAssembler(output)
	if err != nil {
		log.Fatal(err)
	}
}

//go:embed templates/*.tmpl
var templatesFS embed.FS

var templates = template.Must(template.New("").ParseFS(templatesFS, "templates/*.tmpl"))

// genAssembler reads the x86 architecture
// CSV from input and writes an x86 assembler
// in Go to output.
func genAssembler(output string) error {
	// Build the combined data.
	var data struct {
		Command      string
		Package      string
		UIDs         []string
		Instructions map[string][]*x86.Instruction
	}

	data.Command = filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ")
	data.Package = "compiler"
	data.UIDs = make([]string, len(x86.Instructions))
	data.Instructions = make(map[string][]*x86.Instruction)

	for i, inst := range x86.Instructions {
		mnemonic := strings.ToLower(inst.Mnemonic)
		data.Instructions[mnemonic] = append(data.Instructions[mnemonic], inst)
		data.UIDs[i] = inst.UID
	}

	// Print stats.
	if statsOnly {
		var encodings, parseForms int
		encodingMap := make(map[string]bool)
		for _, instructions := range data.Instructions {
			parseForms += len(instructions)
			for _, instruction := range instructions {
				encodingMap[instruction.Encoding.Syntax] = true
			}
		}

		encodings = len(encodingMap)

		fmt.Printf("Mnemonics:   %d\n", len(data.Instructions))
		fmt.Printf("Encodings:   %d\n", encodings)
		fmt.Printf("Parse forms: %d\n", parseForms)
		return nil
	}

	var b bytes.Buffer
	err := templates.ExecuteTemplate(&b, "assembler.go.tmpl", data)
	if err != nil {
		return fmt.Errorf("failed to execute assembler.go.tmpl template: %v", err)
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
