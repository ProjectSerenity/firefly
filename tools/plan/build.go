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
	"firefly-os.dev/tools/plan/rust"
	"firefly-os.dev/tools/plan/types"
)

func init() {
	RegisterCommand("build", "Build a Plan document, optionally translating to another language.", cmdBuild)
}

func cmdBuild(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("build", flag.ExitOnError)

	var help bool
	var arch types.Arch
	var rustfmtPath, rustUserPath, rustKernelPath, rustSharedPath string
	flags.BoolVar(&help, "h", false, "Show this message and exit.")
	flags.StringVar(&rustfmtPath, "rustfmt", "rustfmt", "The path to the `rustfmt` binary.")
	flags.StringVar(&rustUserPath, "rust-user", "", "The path where the Rust userspace module should be written.")
	flags.StringVar(&rustKernelPath, "rust-kernel", "", "The path where the Rust kernelspace module should be written.")
	flags.StringVar(&rustSharedPath, "rust-shared", "", "The path where the Rust shared module should be written.")
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

	file, err := types.Interpret(filename, syntax, arch)
	if err != nil {
		return fmt.Errorf("failed to interpret %s: %v", filename, err)
	}

	// Do any necessary translations.

	if rustUserPath != "" {
		out, err := os.Create(rustUserPath)
		if err != nil {
			return fmt.Errorf("failed to create Rust userspace file %s: %v", rustUserPath, err)
		}

		err = rust.GenerateUserCode(out, file, arch, rustfmtPath)
		if err != nil {
			out.Close()
			return fmt.Errorf("failed to write Rust userspace file %s: %v", rustUserPath, err)
		}

		err = out.Close()
		if err != nil {
			return fmt.Errorf("failed to close Rust userspace file %s: %v", rustUserPath, err)
		}
	}

	if rustKernelPath != "" {
		out, err := os.Create(rustKernelPath)
		if err != nil {
			return fmt.Errorf("failed to create Rust kernelspace file %s: %v", rustKernelPath, err)
		}

		err = rust.GenerateKernelCode(out, file, arch, rustfmtPath)
		if err != nil {
			out.Close()
			return fmt.Errorf("failed to write Rust kernelspace file %s: %v", rustKernelPath, err)
		}

		err = out.Close()
		if err != nil {
			return fmt.Errorf("failed to close Rust kernelspace file %s: %v", rustKernelPath, err)
		}
	}

	if rustSharedPath != "" {
		out, err := os.Create(rustSharedPath)
		if err != nil {
			return fmt.Errorf("failed to create Rust shared file %s: %v", rustSharedPath, err)
		}

		err = rust.GenerateSharedCode(out, file, arch, rustfmtPath)
		if err != nil {
			out.Close()
			return fmt.Errorf("failed to write Rust shared file %s: %v", rustSharedPath, err)
		}

		err = out.Close()
		if err != nil {
			return fmt.Errorf("failed to close Rust shared file %s: %v", rustSharedPath, err)
		}
	}

	return nil
}
