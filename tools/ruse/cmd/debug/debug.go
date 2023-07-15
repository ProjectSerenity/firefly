// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package debug prints debugging information about a Ruse executable binary.
package debug

import (
	"context"
	"debug/elf"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/crypto/cryptobyte"

	"firefly-os.dev/tools/ruse/rpkg"
)

var program = filepath.Base(os.Args[0])

// Main parses a Ruse executable binary, printing debugging
// information.
func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("debug", flag.ExitOnError)

	var help, short bool
	flags.BoolVar(&short, "short", false, "Print only the first 8 characters of checksums")
	flags.BoolVar(&help, "h", false, "Show this message and exit.")

	flags.Usage = func() {
		log.Printf("Usage:\n  %s %s [OPTIONS] BINARY\n\n", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(2)
	}

	err := flags.Parse(args)
	if err != nil || help {
		flags.Usage()
	}

	filenames := flags.Args()
	if len(filenames) != 1 {
		flags.Usage()
		os.Exit(2)
	}

	// TODO: Add support for other binary formats.
	r, err := elf.Open(filenames[0])
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", filenames[0], err)
	}

	// TODO: Derive the rpkgs segment programmatically,
	// rather than knowing it's (currently) the third
	// segment.
	if len(r.Progs) < 3 {
		return fmt.Errorf("failed to parse %s: found %d segments, expected 3", filenames[0], len(r.Progs))
	}

	rpkgs, err := io.ReadAll(r.Progs[2].Open())
	if err != nil {
		return fmt.Errorf("failed to parse %s: failed to read rpkgs segment: %v", filenames[0], err)
	}

	s := cryptobyte.String(rpkgs)
	for !s.Empty() {
		var pkg, checksum []byte
		var pkgString cryptobyte.String
		if !s.ReadUint16LengthPrefixed(&pkgString) ||
			!pkgString.ReadBytes(&pkg, len(pkgString)) ||
			!pkgString.Empty() ||
			!s.ReadBytes(&checksum, rpkg.ChecksumLength) {
			return fmt.Errorf("failed to parse %s: failed to read package name: %v", filenames[0], io.ErrUnexpectedEOF)
		}

		if short {
			fmt.Printf("%x  %s\n", checksum[:4], pkg)
		} else {
			fmt.Printf("%x  %s\n", checksum, pkg)
		}
	}

	return nil
}
