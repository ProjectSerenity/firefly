// Copyright 2024 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Code to drop unused symbols from a package to reduce
// the size of the resulting binary.

package link

import (
	"fmt"
	"log"
	"strings"

	"firefly-os.dev/tools/ruse/compiler"
	"firefly-os.dev/tools/ruse/constant"
	"firefly-os.dev/tools/ruse/types"
)

// shakeTree processes a set of Ruse packages, identifying
// symbols that are not referenced at runtime and removes
// them. Note that constants that have already been inlined
// are safely removed if their address is not referenced.
func shakeTree(debug bool, pkgs []*compiler.Package, main *compiler.Package) error {
	// We iterate through the packages, starting with the deepest
	// dependencies in a depth-first search. Otherwise, we run the
	// risk of marking a symbol as used only because of another
	// unused symbol.

	// TODO: remove
	if !debug {
		//return nil
	}

	debugf := func(format string, v ...any) {
		if !debug {
			return
		}

		log.Printf(format, v...)
	}

	// First, we build up a mapping of package path to the
	// full package data.
	pkgPath := make(map[string]*compiler.Package, len(pkgs))
	for _, pkg := range pkgs {
		if pkgPath[pkg.Path] != nil {
			return fmt.Errorf("found second implementation of package %q", pkg.Path)
		}

		pkgPath[pkg.Path] = pkg
	}

	type Symbol struct {
		Package string // The full package path.
		Name    string // The symbol name within the package.
	}

	type SymbolSet map[Symbol]struct{} // Map of symbol names, indicating presence.

	symDeps := make(map[Symbol]SymbolSet)            // Map symbol name to transitive dependencies.
	donePkg := make(map[string]bool, len(pkgs))      // Map package name to whether it's been processed.
	keepSym := make(map[string]SymbolSet, len(pkgs)) // Map package name to symbols to keep.

	keep := func(pkg, name string) {
		syms, ok := keepSym[pkg]
		if !ok {
			syms = make(SymbolSet)
			keepSym[pkg] = syms
		}

		// Add the direct dependency.
		symbol := Symbol{
			Package: pkg,
			Name:    name,
		}

		syms[symbol] = struct{}{}
	}

	var scanPackage func(pkg *compiler.Package) error
	scanPackage = func(pkg *compiler.Package) error {
		if donePkg[pkg.Path] {
			return nil
		}

		// Mark that we've started processing this package.
		// We shouldn't get dependency loops, as they should
		// already have been rejected by the compiler, but
		// we check here just in case.
		donePkg[pkg.Path] = false

		// Do its dependencies first.
		for _, imp := range pkg.Imports {
			dep, ok := pkgPath[imp]
			if !ok {
				return fmt.Errorf("no rpkg provided for package %q", imp)
			}

			err := scanPackage(dep)
			if err != nil {
				return err
			}
		}

		// Iterate through all our symbols, identifying
		// other symbols we depend on.
		//
		// Note that for now, only functions can reference
		// other symbols.
		for _, fun := range pkg.Functions {
			if len(fun.Links) == 0 {
				continue
			}

			deps := make(SymbolSet)
			for _, link := range fun.Links {
				// Split the linked symbol name
				// into the package path and
				// symbol.
				i := strings.LastIndexByte(link.Name, '.')
				if i < 0 {
					return fmt.Errorf("failed to parse symbol name %q in link from %s.%s", link.Name, pkg.Path, fun.Name)
				}

				if link.Name[0] == '.' {
					// This is a literal, which may contain
					// misleading dots.
					i = 0
				}

				// Add the direct dependency.
				symbol := Symbol{
					Package: link.Name[:i],
					Name:    link.Name[i+1:],
				}

				// Literals have no package path, so
				// we have to insert it here.
				if symbol.Package == "" {
					symbol.Package = pkg.Path
				}

				deps[symbol] = struct{}{}

				// Add any transitive dependencies.
				for dep := range symDeps[symbol] {
					deps[dep] = struct{}{}
				}
			}

			symDeps[Symbol{Package: pkg.Path, Name: fun.Name}] = deps
		}

		// We also keep any constants with a specified
		// alignment, as these may be needed only for
		// their location in memory.
		for _, con := range pkg.Constants {
			if con.Alignment() <= 1 {
				continue
			}

			keep(pkg.Path, con.Name())
			debugf("Keeping constant %s.%s with alignment %d.", pkg.Path, con.Name(), con.Alignment())
		}

		// All done.
		donePkg[pkg.Path] = true

		return nil
	}

	// Build up symbol dependency relationships data.
	for _, pkg := range pkgs {
		err := scanPackage(pkg)
		if err != nil {
			return err
		}
	}

	// At this point, the data for main
	// should contain every symbol we
	// care about.
	//
	// We decompose it into symbols per
	// package to make the data easier
	// to manage.

	var addSymbols func(sym Symbol)
	addSymbols = func(sym Symbol) {
		for dep := range symDeps[sym] {
			syms, ok := keepSym[dep.Package]
			if !ok {
				syms = make(SymbolSet)
				keepSym[dep.Package] = syms
			}

			syms[dep] = struct{}{}
			addSymbols(dep) // Follow its dependency graph.
		}
	}

	mainSymbol := Symbol{
		Package: main.Path,
		Name:    "main",
	}

	keep(main.Path, "main") // Add the main function as our starting point.
	addSymbols(mainSymbol)  // Then do its dependency graph.

	// Finally, iterate through each package, removing
	// any symbols we don't want to keep.

	for _, pkg := range pkgs {
		syms := keepSym[pkg.Path]

		// Constants.
		next := 0
		for i, con := range pkg.Constants {
			_, keep := syms[Symbol{Package: pkg.Path, Name: con.Name()}]
			if _, ok := con.Type().(types.Section); ok {
				// Tracking use of sections is surprisingly tricky and they're
				// not encoded into the binary if unused, so it's simpler to
				// treat them all as used.
				keep = true
			}

			if _, ok := con.Type().(types.ABI); ok {
				// Same with ABIs.
				keep = true
			}

			if !keep {
				// Drop this constant.
				debugf("Dropping unused constant %s.%s", pkg.Path, con.Name())
				continue
			}

			// Store it at the next free slot.
			if i != next {
				pkg.Constants[next] = con
			}

			next++ // Advance the next free slot.
		}

		pkg.Constants = pkg.Constants[:next]

		// Literals.
		next = 0
		for i, lit := range pkg.Literals {
			val := lit.Value()
			var name string
			switch val.Kind() {
			case constant.String:
				name = constant.StringVal(val)
			default:
				return fmt.Errorf("internal error: unsure how to reference literal %s.%s (%s)", pkg.Path, lit.Name(), val.Kind())
			}

			_, keep := syms[Symbol{Package: pkg.Path, Name: name}]
			if _, ok := lit.Type().(types.Section); ok {
				// Tracking use of sections is surprisingly tricky and they're
				// not encoded into the binary if unused, so it's simpler to
				// treat them all as used.
				keep = true
			}

			if _, ok := lit.Type().(types.ABI); ok {
				// Same with ABIs.
				keep = true
			}

			if !keep {
				// Drop this literal.
				debugf("Dropping unused literal .%q", name)
				continue
			}

			// Store it at the next free slot.
			if i != next {
				pkg.Literals[next] = lit
			}

			next++ // Advance the next free slot.
		}

		pkg.Literals = pkg.Literals[:next]

		// Functions.
		next = 0
		for i, fun := range pkg.Functions {
			_, keep := syms[Symbol{Package: pkg.Path, Name: fun.Name}]
			if !keep {
				// Drop this function.
				debugf("Dropping unused function %s.%s", pkg.Path, fun.Name)
				continue
			}

			// Store it at the next free slot.
			if i != next {
				pkg.Functions[next] = fun
			}

			next++ // Advance the next free slot.
		}

		pkg.Functions = pkg.Functions[:next]
	}

	// All done!

	return nil
}
