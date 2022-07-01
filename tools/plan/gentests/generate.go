// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command gentests is used to test the generate code from sample Plan documents.
//
// For each language that Plan can generate code, Gentests generates code from a
// sample Plan document, appending tests to verify the runtime behaviour of the
// generated code. This is then tested using the usual test rule for that language.
//
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"firefly-os.dev/tools/plan/parser"
	"firefly-os.dev/tools/plan/rust"
	"firefly-os.dev/tools/plan/types"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("")
}

func main() {
	var help bool
	var arch types.Arch
	var rustfmtPath, rustPlanPath, rustTestsPath, rustOutPath string
	flag.BoolVar(&help, "h", false, "Show this message and exit.")
	flag.StringVar(&rustfmtPath, "rustfmt", "rustfmt", "The path to the `rustfmt` binary.")
	flag.StringVar(&rustPlanPath, "rust-path", "", "The path to the Rust Plan document.")
	flag.StringVar(&rustTestsPath, "rust-tests", "", "The path to the Rust tests that should be appended.")
	flag.StringVar(&rustOutPath, "rust-out", "", "The path where the generated Rust code should be written.")
	flag.Func("arch", "Instruction set architecture to target (options: x86-64).", func(s string) error {
		switch s {
		case "x86-64":
			arch = types.X86_64
		default:
			return fmt.Errorf("unrecognised architecture %q", s)
		}

		return nil
	})

	flag.Usage = func() {
		log.Printf("Usage:\n  %s [OPTIONS] FILE\n\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
		os.Exit(2)
	}

	flag.Parse()
	if help {
		flag.Usage()
	}

	if arch == types.InvalidArch {
		log.Printf("%s: -arch not specified.", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
		os.Exit(1)
	}

	src, err := os.Open(rustPlanPath)
	if err != nil {
		log.Fatalf("failed to open %s: %v", rustPlanPath, err)
	}

	defer src.Close()

	syntax, err := parser.ParseFile(rustPlanPath, src)
	if err != nil {
		log.Fatalf("failed to parse %s: %v", rustPlanPath, err)
	}

	file, err := types.Interpret(rustPlanPath, syntax, arch)
	if err != nil {
		log.Fatalf("failed to interpret %s: %v", rustPlanPath, err)
	}

	// Do the translations.

	out, err := os.Create(rustOutPath)
	if err != nil {
		log.Fatalf("failed to create Rust file %s: %v", rustOutPath, err)
	}

	err = rust.GenerateSharedCode(out, file, arch, rustfmtPath)
	if err != nil {
		out.Close()
		log.Fatalf("failed to write Rust file %s: %v", rustOutPath, err)
	}

	tests, err := os.Open(rustTestsPath)
	if err != nil {
		out.Close()
		log.Fatalf("failed to open Rust tests file %s: %v", rustTestsPath, err)
	}

	_, err = io.Copy(out, tests)
	if err != nil {
		out.Close()
		tests.Close()
		log.Fatalf("failed to append Rust tests file %s: %v", rustTestsPath, err)
	}

	err = tests.Close()
	if err != nil {
		out.Close()
		log.Fatalf("failed to close Rust tests file %s: %v", rustTestsPath, err)
	}

	err = out.Close()
	if err != nil {
		log.Fatalf("failed to close Rust file %s: %v", rustOutPath, err)
	}
}
