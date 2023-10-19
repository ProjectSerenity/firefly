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

	// Find the rpkgs segment using the section
	// headers.
	var rpkgsSection *elf.Section
	for _, section := range r.Sections {
		if section.Name == "rpkgs" {
			rpkgsSection = section
			break
		}
	}

	if rpkgsSection == nil {
		return fmt.Errorf("failed to parse %s: no 'rpkgs' section found", filenames[0])
	}

	rpkgs, err := rpkgsSection.Data()
	if err != nil {
		return fmt.Errorf("failed to parse %s: failed to read rpkgs segment: %v", filenames[0], err)
	}

	s := cryptobyte.String(rpkgs)
	var rpkgsLen uint32
	var rpkgsData []byte
	if !s.ReadUint32(&rpkgsLen) ||
		!s.ReadBytes(&rpkgsData, int(rpkgsLen)) {
		return fmt.Errorf("failed to parse %s: failed to read rpkgs data: %v", filenames[0], io.ErrUnexpectedEOF)
	}

	s = cryptobyte.String(rpkgsData)
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
