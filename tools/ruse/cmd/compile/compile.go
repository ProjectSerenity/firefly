// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package compile compiles a set of Ruse source code.
package compile

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/compiler"
	"firefly-os.dev/tools/ruse/parser"
	"firefly-os.dev/tools/ruse/rpkg"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

var program = filepath.Base(os.Args[0])

// Main compiles a set of Ruse files into an executable
// binary.
func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("compile", flag.ExitOnError)

	var help bool
	var out, pkgName, stdlib string
	var rpkgs, debugFunctions []string
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
	flags.Func("rpkg", "One or more dependency rpkg files.", func(s string) error {
		rpkgs = append(rpkgs, s)
		return nil
	})
	flags.Func("debug", "One or more function names to debug.", func(s string) error {
		debugFunctions = append(debugFunctions, s)
		return nil
	})
	flags.StringVar(&pkgName, "package", "", "The full package name.")
	flags.StringVar(&stdlib, "stdlib", "", "The standard library rpkg file.")
	flags.StringVar(&out, "o", "", "The name of the compiled rpkg.")

	flags.Usage = func() {
		log.Printf("Usage:\n  %s %s OPTIONS FILE...\n\n", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(2)
	}

	debug := func(s string) bool {
		for _, want := range debugFunctions {
			if want == s {
				return true
			}
		}

		return false
	}

	err := flags.Parse(args)
	if err != nil || help {
		flags.Usage()
	}

	if arch == nil || out == "" || pkgName == "" {
		flags.Usage()
		os.Exit(2)
	}

	// Get the set of packages available
	// to import.
	rpkgFiles := make(map[string]string)
	availableImports := make(map[string]*types.Package)
	for _, name := range rpkgs {
		data, err := os.ReadFile(name)
		if err != nil {
			return fmt.Errorf("failed to read rpkg %q: %v", name, err)
		}

		info := new(types.Info)
		depArch, pkg, _, err := rpkg.Decode(info, data)
		if err != nil {
			return fmt.Errorf("failed to parse rpkg %q: %v", name, err)
		}

		if depArch != arch {
			return fmt.Errorf("cannot import rpkg %q: compiled for %s: need %s", name, depArch.Name, arch.Name)
		}

		rpkgFiles[pkg.Path] = name
		availableImports[pkg.Path] = pkg.Types
	}

	isStdlib := make(map[string]bool)
	if stdlib != "" {
		data, err := os.ReadFile(stdlib)
		if err != nil {
			return fmt.Errorf("failed to read stdlib rpkg %q: %v", stdlib, err)
		}

		rstd, err := rpkg.NewStdlibDecoder(data)
		if err != nil {
			return fmt.Errorf("failed to parse stdlib rstd %q: %v", stdlib, err)
		}

		pkgs := rstd.Packages()
		for _, hdr := range pkgs {
			info := new(types.Info)
			depArch, p, _, err := rstd.Decode(info, hdr)
			if err != nil {
				return fmt.Errorf("failed to parse stdlib rpkg %q from %q: %v", hdr.PackageName, stdlib, err)
			}

			if depArch != arch {
				return fmt.Errorf("cannot import stdlib rpkg %q: compiled for %s: need %s", hdr.PackageName, depArch.Name, arch.Name)
			}

			isStdlib[hdr.PackageName] = true
			availableImports[hdr.PackageName] = p.Types
		}
	}

	filenames := flags.Args()
	if len(filenames) == 0 {
		flags.Usage()
		os.Exit(2)
	}

	fset := token.NewFileSet()
	files := make([]*ast.File, len(filenames))
	for i, filename := range filenames {
		files[i], err = parser.ParseFile(fset, filename, nil, 0)
		if err != nil {
			return err
		}
	}

	// Accumulate the set of imports.
	seenImport := make(map[string]bool)
	for _, file := range files {
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return fmt.Errorf("%s: found malformed import path %q: %v", fset.Position(imp.Path.ValuePos), imp.Path.Value, err)
			}

			if _, ok := availableImports[path]; !ok {
				return fmt.Errorf("%s: no package found for import %s", fset.Position(imp.Path.ValuePos), imp.Path.Value)
			}

			seenImport[path] = true
		}
	}

	// Check that we have no redundant rpkg files.
	for imp := range availableImports {
		if !seenImport[imp] && !isStdlib[imp] {
			return fmt.Errorf("rpkg file %s (package %q) was provided but not imported", rpkgFiles[imp], imp)
		}
	}

	info := &types.Info{
		Types:       make(map[ast.Expression]types.TypeAndValue),
		Definitions: make(map[*ast.Identifier]types.Object),
		Uses:        make(map[*ast.Identifier]types.Object),
		Packages:    availableImports,
	}

	pkg, err := types.Check(pkgName, fset, files, arch, info)
	if err != nil {
		return err
	}

	sizes := types.SizesFor(arch)
	p, err := compiler.Compile(fset, arch, pkg, files, info, sizes)
	if err != nil {
		return err
	}

	// Allocate registers and lower the instructions
	// for Ruse functions.
	for _, fun := range p.Functions {
		// If we've been asked to debug this
		// function, we print its name once
		// and each phase description. We trim
		// The first line from the debug output,
		// which contains its name.
		shouldDebug := debug(fun.Name)
		if shouldDebug {
			_, text, _ := strings.Cut(fun.Print(), "\n")
			fmt.Fprintf(os.Stderr, "debug: %s %s\ncompile/assemble:\n%s\n", fun.Name, fun.Type, text)
		}

		// Skip assembly functions.
		if fun.Code.Elements[0].(*ast.Identifier).Name != "func" {
			continue
		}

		err = compiler.Allocate(fset, arch, sizes, p, fun)
		if err != nil {
			return err
		}

		if shouldDebug {
			_, text, _ := strings.Cut(fun.Print(), "\n")
			fmt.Fprintf(os.Stderr, "allocate:\n%s\n", text)
		}

		err = compiler.Lower(fset, arch, sizes, fun)
		if err != nil {
			return err
		}

		if shouldDebug {
			_, text, _ := strings.Cut(fun.Print(), "\n")
			fmt.Fprintf(os.Stderr, "lower:\n%s\n", text)
		}
	}

	// Put the main function first.
	sort.Slice(p.Functions, func(i, j int) bool {
		switch {
		case p.Functions[i].Name == "main":
			return true
		case p.Functions[j].Name == "main":
			return false
		default:
			return p.Functions[i].Name < p.Functions[j].Name
		}
	})

	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("failed to create %s: %v", out, err)
	}

	err = rpkg.Encode(f, fset, arch, p, info)
	if err != nil {
		return fmt.Errorf("failed to compile %s: %v", out, err)
	}

	return nil
}
