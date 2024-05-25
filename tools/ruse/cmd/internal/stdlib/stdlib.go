// Copyright 2024 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package stdlib provides a helper for using the Ruse standard library.
package stdlib

import (
	"fmt"
	"os"

	"firefly-os.dev/tools/ruse/compiler"
	"firefly-os.dev/tools/ruse/rpkg"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/types"
)

// Embedded is an optional embedded copy of
// the standard library's rstd data.
var Embedded []byte

// Parse processes the given standard library
// rstd file, returning the list of packages
// decoded or an error encountered during the
// decoding or parsing steps.
func Parse(arch *sys.Arch, data []byte) (packages []*compiler.Package, checksums [][]byte, err error) {
	rstd, err := rpkg.NewStdlibDecoder(data)
	if err != nil {
		return nil, nil, err
	}

	pkgs := rstd.Packages()
	for _, hdr := range pkgs {
		depArch, p, checksum, err := rstd.Decode(new(types.Info), hdr)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse stdlib package %q: %v", hdr.PackageName, err)
		}

		if depArch != arch {
			return nil, nil, fmt.Errorf("cannot import stdlib rpkg %q: compiled for %s, need %s", hdr.PackageName, depArch.Name, arch.Name)
		}

		packages = append(packages, p)
		checksums = append(checksums, checksum)
	}

	return packages, checksums, nil
}

// ParseFile processes the given standard
// library rstd file, returning the list of
// packages decoded or an error encountered
// during the decoding or parsing steps.
func ParseFile(arch *sys.Arch, name string) (packages []*compiler.Package, checksums [][]byte, err error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read rstd %q: %v", name, err)
	}

	packages, checksums, err = Parse(arch, data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse rstd %q: %v", name, err)
	}

	return packages, checksums, nil
}
