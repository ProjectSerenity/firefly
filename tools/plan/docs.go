// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"firefly-os.dev/tools/plan/html"
	"firefly-os.dev/tools/plan/parser"
	"firefly-os.dev/tools/plan/types"
)

const dirMode = 0777

func mkdir(dir string) error {
	err := os.MkdirAll(dir, dirMode)
	if err != nil {
		return fmt.Errorf("failed to create directory %q: %v", dir, err)
	}

	return nil
}

func init() {
	RegisterCommand("docs", "Generate documentation for a Plan document.", cmdDocs)
}

func cmdDocs(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("docs", flag.ExitOnError)

	var help bool
	var outname string
	var arch types.Arch
	flags.BoolVar(&help, "h", false, "Show this message and exit.")
	flags.StringVar(&outname, "out", "", "The path where the documentation should be written.")
	flags.Func("arch", "Instruction set architecture to target (options: x86-64).", func(s string) error {
		switch s {
		case "x86-64":
			arch = types.X86_64
		default:
			return fmt.Errorf("unrecognised architecture %q", s)
		}

		return nil
	})

	flags.Usage = func() {
		log.Printf("Usage:\n  %s %s [OPTIONS] FILE\n\n", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(2)
	}

	err := flags.Parse(args)
	if err != nil || help {
		flags.Usage()
	}

	if arch == types.InvalidArch {
		log.Printf("%s %s: -arch not specified.", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(1)
	}

	if outname == "" {
		log.Printf("%s %s: -out not specified.", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(1)
	}

	args = flags.Args()
	if len(args) != 1 {
		log.Printf("%s %s can only build one file at a time.", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(1)
	}

	filename := args[0]
	src, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open %s: %v", filename, err)
	}

	defer src.Close()

	syntax, err := parser.ParseFile(filename, src)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", filename, err)
	}

	file, err := types.Interpret(filename, syntax, arch)
	if err != nil {
		return fmt.Errorf("failed to interpret %s: %v", filename, err)
	}

	// Generate the documentation.
	err = mkdir(outname)
	if err != nil {
		return err
	}

	err = html.GenerateDocs(outname, file)
	if err != nil {
		return fmt.Errorf("failed to generate HTML: %v", err)
	}

	return nil
}
