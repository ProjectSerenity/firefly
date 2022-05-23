// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"firefly-os.dev/tools/plan/format"
	"firefly-os.dev/tools/plan/parser"
	"firefly-os.dev/tools/plan/types"
)

func init() {
	RegisterCommand("format", "Format a Plan document to the standard style.", cmdFormat)
}

func cmdFormat(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("format", flag.ExitOnError)

	var help, check bool
	var out string
	var arch types.Arch
	flags.BoolVar(&help, "h", false, "Show this message and exit.")
	flags.BoolVar(&check, "check", false, "Exit with an error if the input file is not already formatted.")
	flags.StringVar(&out, "out", "", "Path where the formatted output should be written (default: stdout).")
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
		log.Printf("Usage:\n  %s %s [OPTIONS] [FILE]\n\n", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(2)
	}

	err := flags.Parse(args)
	if err != nil || help {
		flags.Usage()
	}

	args = flags.Args()
	if len(args) > 1 {
		log.Printf("%s %s can only format one file at a time.", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(1)
	}

	if arch == types.InvalidArch {
		log.Printf("%s %s: -arch not specified.", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(1)
	}

	var src []byte
	var filename string
	if len(args) == 0 || args[0] == "-" {
		filename = "stdin"
		src, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read %s: %v", filename, err)
		}
	} else {
		filename = args[0]
		src, err = os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read %s: %v", filename, err)
		}
	}

	var output *os.File
	if out == "" {
		output = os.Stdout
	} else {
		output, err = os.Create(out)
		if err != nil {
			return fmt.Errorf("failed to create %s: %v", out, err)
		}
	}

	file, err := parser.ParseFile(filename, src)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", filename, err)
	}

	err = format.SortFields(file, arch)
	if err != nil {
		return fmt.Errorf("failed to sort fields for %s: %v", filename, err)
	}

	var formatted bytes.Buffer
	err = format.Fprint(io.MultiWriter(output, &formatted), file)
	if err != nil {
		return fmt.Errorf("failed to format %s: %v", filename, err)
	}

	err = output.Close()
	if err != nil {
		return fmt.Errorf("failed to close %s: %v", out, err)
	}

	if check && !bytes.Equal(src, formatted.Bytes()) {
		return fmt.Errorf("Plan was not formatted correctly.")
	}

	return nil
}
