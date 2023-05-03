// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command gen-ops generates ssafir data.
//
// This is used to combine abstract and concrete instructions
// for different architectures into a single data set, which
// is then used in package ssafir.
package main

import (
	"bytes"
	"embed"
	"flag"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// OpInfo contains information about an instruction.
//
// The instruction is either an abstract instruction,
// emitted by the Ruse compiler, or an instruction
// specific to an individual architecture.
type OpInfo struct {
	Opcode      string
	Name        string
	Abstract    bool
	Virtual     bool // Not executed on the machine.
	Operands    int  // Number of arguments (or -1 if variadic).
	Commutative bool // The first two arguments can be reordered without effect.
}

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("")
}

//go:embed templates/*.tmpl
var templatesFS embed.FS

var templates = template.Must(template.New("").ParseFS(templatesFS, "templates/*.tmpl"))

func main() {
	opsets := []struct {
		Name string
		Ops  []OpInfo
	}{
		{Name: "abstract", Ops: AbstractOps},
		{Name: "X86", Ops: X86Ops},
	}

	var (
		opName     string
		opInfoName string
	)

	flag.StringVar(&opName, "op", "", "Path to where op list shuold be written")
	flag.StringVar(&opInfoName, "op-info", "", "path to where op info should be written")

	flag.Parse()
	if opName == "" || opInfoName == "" {
		flag.Usage()
		os.Exit(2)
	}

	var n int
	for _, opset := range opsets {
		n += len(opset.Ops)
	}

	// Build the combined data.
	var data struct {
		Command string
		Package string
		Opcodes []string
		OpInfo  []OpInfo
	}

	data.Command = filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ")
	data.Package = "ssafir"
	data.Opcodes = make([]string, 0, n)
	data.OpInfo = make([]OpInfo, 0, n)

	for _, opset := range opsets {
		for i, op := range opset.Ops {
			if op.Name == "" {
				log.Fatalf("%s.Ops[%d] has no name", opset.Name, i)
			}

			var name string
			if opset.Name != "abstract" {
				name = opset.Name
			}

			data.Opcodes = append(data.Opcodes, "Op"+name+op.Name)
			data.OpInfo = append(data.OpInfo, OpInfo{
				Opcode:      "Op" + name + op.Name,
				Name:        op.Name,
				Abstract:    opset.Name == "abstract",
				Virtual:     op.Virtual,
				Operands:    op.Operands,
				Commutative: op.Commutative,
			})
		}
	}

	var opBuf, opInfoBuf bytes.Buffer
	err := templates.ExecuteTemplate(&opBuf, "op.go.tmpl", data)
	if err != nil {
		log.Fatalf("failed to execute op.go.tmpl template: %v", err)
	}

	formatted, err := format.Source(opBuf.Bytes())
	if err != nil {
		log.Println(err)
		formatted = opBuf.Bytes()
	}

	err = os.WriteFile(opName, formatted, 0644)
	if err != nil {
		log.Fatalf("failed to write %s: %v", opName, err)
	}

	err = templates.ExecuteTemplate(&opInfoBuf, "op-info.go.tmpl", data)
	if err != nil {
		log.Fatalf("failed to execute op-info.go.tmpl template: %v", err)
	}

	formatted, err = format.Source(opInfoBuf.Bytes())
	if err != nil {
		log.Println(err)
		formatted = opInfoBuf.Bytes()
	}

	err = os.WriteFile(opInfoName, formatted, 0644)
	if err != nil {
		log.Fatalf("failed to write %s: %v", opInfoName, err)
	}
}
