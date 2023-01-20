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

	"firefly-os.dev/tools/plan/parser"
	"firefly-os.dev/tools/plan/types"
)

func init() {
	RegisterCommand("build", "Build a Plan document, optionally translating to another language.", cmdBuild)
}

func cmdBuild(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("build", flag.ExitOnError)

	var help bool
	var arch types.Arch
	flags.BoolVar(&help, "h", false, "Show this message and exit.")
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

	_, err = types.Interpret(filename, syntax, arch)
	if err != nil {
		return fmt.Errorf("failed to interpret %s: %v", filename, err)
	}

	return nil
}
