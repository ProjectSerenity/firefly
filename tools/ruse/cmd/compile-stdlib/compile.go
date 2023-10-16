// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package compile compiles a set of Ruse source code.
package compilestdlib

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"firefly-os.dev/tools/ruse/rpkg"
	"firefly-os.dev/tools/ruse/sys"
)

var program = filepath.Base(os.Args[0])

// Main combines a set of Ruse standard library rpkg files
// into a single rpkg file.
func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("compile-stdlib", flag.ExitOnError)

	var help bool
	var out string
	var arch *sys.Arch
	flags.BoolVar(&help, "h", false, "Show this message and exit.")
	flags.Func("arch", "The target architecture (x86-64).", func(s string) error {
		if arch != nil {
			return fmt.Errorf("-arch can only be specified once")
		}

		switch s {
		case "x86-64":
			arch = sys.X86_64
		default:
			return fmt.Errorf("unrecognised -arch: %q", s)
		}

		return nil
	})
	flags.StringVar(&out, "o", "", "The name of the compiled rpkg.")

	flags.Usage = func() {
		log.Printf("Usage:\n  %s %s OPTIONS FILE...\n\n", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(2)
	}

	err := flags.Parse(args)
	if err != nil || help {
		flags.Usage()
	}

	rpkgs := flags.Args()
	if arch == nil || out == "" || len(rpkgs) == 0 {
		flags.Usage()
		os.Exit(2)
	}

	var pkgs [][]byte
	for _, name := range rpkgs {
		data, err := os.ReadFile(name)
		if err != nil {
			return fmt.Errorf("failed to read rpkg %q: %v", name, err)
		}

		pkgs = append(pkgs, data)
	}

	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("failed to create %s: %v", out, err)
	}

	err = rpkg.EncodeStdlib(f, arch, pkgs)
	if err != nil {
		return fmt.Errorf("failed to compile %s: %v", out, err)
	}

	return nil
}
