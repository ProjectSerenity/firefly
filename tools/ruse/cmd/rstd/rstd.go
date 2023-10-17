// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package rstd prints debug information about a Ruse rstd file.
package rstd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	cmdrpkg "firefly-os.dev/tools/ruse/cmd/rpkg"
	"firefly-os.dev/tools/ruse/rpkg"
)

var program = filepath.Base(os.Args[0])

// Main prints debug information about a Ruse rstd file.
func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("rstd", flag.ExitOnError)

	var help, header, checksums, packages, short, rpkgs bool
	flags.BoolVar(&help, "h", false, "Show this message and exit.")
	flags.BoolVar(&header, "header", true, "Print information about the rstd header.")
	flags.BoolVar(&checksums, "checksums", false, "Print the list of packages and their checksums.")
	flags.BoolVar(&short, "short", false, "Print only the first 8 characters of checksums")
	flags.BoolVar(&packages, "packages", false, "Print the list of packages.")
	flags.BoolVar(&rpkgs, "rpkgs", false, "Pass each package through rpkg printing.")

	flags.Usage = func() {
		log.Printf("Usage:\n  %s %s [OPTIONS] RSTD\n\n", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(2)
	}

	err := flags.Parse(args)
	if err != nil || help {
		flags.Usage()
	}

	if short && !checksums {
		flags.Usage()
	}

	if (checksums && packages) ||
		(checksums && rpkgs) ||
		(packages && rpkgs) {
		log.Println("Only one of -checksums, -packages, and -rpkgs can be chosen.")
		flags.Usage()
	}

	// Process any rpkg flags.
	err = cmdrpkg.Flags.Parse(flags.Args())
	if err != nil {
		cmdrpkg.Flags.Usage()
	}

	filenames := cmdrpkg.Flags.Args()
	if len(filenames) != 1 {
		flags.Usage()
		os.Exit(2)
	}

	name := filenames[0]
	data, err := os.ReadFile(filenames[0])
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", name, err)
	}

	d, err := rpkg.NewStdlibDecoder(data)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", name, err)
	}

	var numSections int
	for _, b := range []bool{header, checksums, packages, rpkgs} {
		if b {
			numSections++
		}
	}

	var printSectionHeadings bool
	switch numSections {
	case 0:
		return nil
	case 1:
		printSectionHeadings = false
	default:
		printSectionHeadings = true
	}

	printSection := func(s string) {
		if printSectionHeadings {
			fmt.Printf("%s:\n", s)
		}
	}

	printText := func(format string, v ...any) {
		if printSectionHeadings {
			fmt.Printf("\t"+format, v...)
		} else {
			fmt.Printf(format, v...)
		}
	}

	if header {
		hdr := d.Header()
		fmt.Printf("architecture: %s\n", hdr.Architecture)
		fmt.Printf("rpkg version: %d\n", hdr.Version)
	}

	if checksums {
		printSection("checksums")
		pkgs := d.Packages()
		for _, pkg := range pkgs {
			data := d.Extract(pkg)
			d, err := rpkg.NewDecoder(data)
			if err != nil {
				return fmt.Errorf("failed to parse rpkg %q: %v", pkg.PackageName, err)
			}

			if short {
				printText("%x  %s\n", d.Header().Checksum[:4], pkg.PackageName)
			} else {
				printText("%x  %s\n", d.Header().Checksum, pkg.PackageName)
			}
		}
	}

	if packages {
		printSection("packages")
		pkgs := d.Packages()
		for _, pkg := range pkgs {
			printText("%s\n", pkg.PackageName)
		}
	}

	if rpkgs {
		printSection("rpkgs")
		pkgs := d.Packages()
		for _, pkg := range pkgs {
			data := d.Extract(pkg)
			printText("%q\n", pkg.PackageName)
			err := cmdrpkg.Print(pkg.PackageName, data)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
