// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendeps

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"firefly-os.dev/tools/starlark"
)

// Vendor takes a filesystem, parses the set of software
// dependencies in deps.bzl, then produces the sequence of
// actions necessary to vendor those dependencies into the
// vendor directory.
//
// Note that Vendor does not perform any of these actions;
// it only reads data from fsys.
func Vendor(fsys fs.FS) (actions []Action, err error) {
	data, err := fs.ReadFile(fsys, depsBzl)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %v", depsBzl, err)
	}

	var deps Deps
	err = starlark.Unmarshal(depsBzl, data, &deps)
	if err != nil {
		return nil, err
	}

	if len(deps.Rust) == 0 && len(deps.Go) == 0 {
		actions = []Action{RemoveAll(vendor)}
		return actions, nil
	}

	// Check that the dependency graph is complete. Start
	// by making a mapping for crates to make them easier
	// to look up.
	crates := make(map[string]*RustCrate)
	for _, crate := range deps.Rust {
		crates[crate.Name] = crate
	}

	packages := make(map[string]*GoPackage)
	for _, module := range deps.Go {
		for _, pkg := range module.Packages {
			packages[pkg.Name] = pkg
		}
	}

	var missingDeps bytes.Buffer
	for _, crate := range deps.Rust {
		for _, dep := range crate.Deps {
			if crates[dep] == nil {
				fmt.Fprintf(&missingDeps, "Rust crate %s depends on %s, which is not specified.\n", crate.Name, dep)
			}
		}
		for _, dep := range crate.BuildScriptDeps {
			if crates[dep] == nil {
				fmt.Fprintf(&missingDeps, "Rust crate %s's build script depends on %s, which is not specified.\n", crate.Name, dep)
			}
		}
		for _, dep := range crate.TestDeps {
			if crates[dep] == nil {
				fmt.Fprintf(&missingDeps, "Rust crate %s's tests depends on %s, which is not specified.\n", crate.Name, dep)
			}
		}
	}

	for _, module := range deps.Go {
		for _, pkg := range module.Packages {
			for _, dep := range pkg.Deps {
				if packages[dep] == nil {
					fmt.Fprintf(&missingDeps, "Go package %s depends on %s, which is not specified.\n", pkg.Name, dep)
				}
			}
		}
	}

	if missingDeps.Len() > 0 {
		return nil, fmt.Errorf("missing dependencies:\n%s", missingDeps.String())
	}

	// Start by checking whether the vendor folder exists.
	// If it does, we will need to check the cache later.
	info, err := fs.Stat(fsys, vendor)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to stat %q: %v", vendor, err)
	}

	if info != nil && !info.IsDir() {
		return nil, fmt.Errorf("failed to vendor dependencies: %q exists and is not a directory", vendor)
	}

	// We proceed on the basis that the vendor directory
	// is dirty, so we start by removing any directories
	// that exist but wouldn't be created if we were to
	// start from scratch. These actions are never affected
	// by the cache.
	entries, err := fs.ReadDir(fsys, vendor)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to read files in %q: %v", vendor, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		full := path.Join(vendor, name)
		switch name {
		case manifestBzl:
			// Never remove the cache manifest.
		case "go", "rust":
			if !entry.IsDir() {
				actions = append(actions, RemoveAll(full))
			}
		default:
			actions = append(actions, RemoveAll(full))
		}
	}

	// Now, we iterate through the sets of dependencies,
	// assuming each dependency is dirty and should be
	// fully replaced. The caching layer may later strip
	// some of these actions out if it can prove that
	// they are unnecessary.
	if len(deps.Rust) > 0 {
		actions, err = vendorRust(fsys, actions, deps.Rust)
		if err != nil {
			return nil, err
		}
	}

	if len(deps.Go) > 0 {
		actions, err = vendorGo(fsys, actions, deps.Go)
		if err != nil {
			return nil, err
		}
	}

	actions = append(actions, BuildCacheManifest{Deps: &deps, Path: path.Join(vendor, manifestBzl)})

	return actions, nil
}

func vendorRust(fsys fs.FS, actions []Action, crates []*RustCrate) ([]Action, error) {
	// Sanity-check each crate.
	for i, crate := range crates {
		if crate.Name == "" {
			return nil, fmt.Errorf("Rust crate %d has no name", i)
		}

		if crate.Version == "" {
			return nil, fmt.Errorf("Rust crate %s has no version", crate.Name)
		}
	}

	// Make the rust directory if it does not already
	// exist.
	rustDir := path.Join(vendor, "rust")
	_, err := fs.Stat(fsys, rustDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to stat %q: %v", rustDir, err)
	}

	// Delete any crates we no longer include.
	entries, _ := fs.ReadDir(fsys, rustDir)
	for _, entry := range entries {
		name := entry.Name()
		full := path.Join(rustDir, name)
		ok := false
		for _, crate := range crates {
			if name == crate.Name {
				ok = true
				break
			}
		}

		if !ok {
			actions = append(actions, RemoveAll(full))
		}
	}

	// Now, we download each crate, which will include
	// deleting any contents previously there. The
	// cache may strip out the download action if it
	// can prove that the right data is already there.
	for _, crate := range crates {
		full := path.Join(rustDir, crate.Name)
		actions = append(actions, DownloadRustCrate{Crate: crate, Path: full})
		if crate.BuildFile != "" {
			actions = append(actions, CopyBUILD{Source: crate.BuildFile, Path: path.Join(full, buildBazel)})
		} else {
			actions = append(actions, GenerateRustCrateBUILD{Crate: crate, Path: path.Join(full, buildBazel)})
		}
	}

	return actions, nil
}

func vendorGo(fsys fs.FS, actions []Action, modules []*GoModule) ([]Action, error) {
	// Sanity-check each module and make
	// a mapping of module names to modules
	// to simplify looking up module paths.
	modulePaths := make(map[string]*GoModule)
	for i, module := range modules {
		modulePaths[module.Name] = module
		if module.Name == "" {
			return nil, fmt.Errorf("Go module %d has no name", i)
		}

		if module.Version == "" {
			return nil, fmt.Errorf("Go module %s has no version", module.Name)
		}

		if len(module.Packages) == 0 {
			return nil, fmt.Errorf("Go module %s has no packages", module.Name)
		}

		for i, pkg := range module.Packages {
			if pkg.Name == "" {
				return nil, fmt.Errorf("Go module %s has package %d with no import path", module.Name, i)
			}
		}
	}

	// Make the go directory if it does not already
	// exist.
	goDir := path.Join(vendor, "go")
	_, err := fs.Stat(fsys, goDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to stat %q: %v", goDir, err)
	}

	// Delete any modules we no longer include.
	// Sadly, this is more involved a process than
	// with Rust crates, as each module may have a
	// multi-part path segment, such as golang.org/x/crypto.
	// This makes detecting unwanted directories
	// more complex.
	//
	// First, we collect the set of all file paths
	// under that segment of the file tree.
	filepaths := make(map[string]bool)
	err = fs.WalkDir(fsys, goDir, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Don't delete folders containing a module
			// we're including, as we may want to retain
			// it as a cache.
			if modulePaths[strings.TrimPrefix(name, goDir+"/")] != nil {
				return fs.SkipDir
			}

			filepaths[name] = true
		}

		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to walk %s: %v", goDir, err)
	}

	// Now, we eliminate any filepaths that are
	// a parent directory of, a module we'll be
	// creating.
	for _, module := range modules {
		modname := path.Join(goDir, module.Name)
		for filepath := range filepaths {
			if strings.HasPrefix(modname, filepath+"/") {
				delete(filepaths, filepath)
			}
		}
	}

	// Finally, we reduce the remaining set of
	// filepaths (which should all be deleted)
	// to as small a set as possible by iterating
	// through them, ignoring any whose parent
	// directories also exist in the map.
	sortedFilepaths := make([]string, 0, len(filepaths))
	for filepath := range filepaths {
		if !filepaths[path.Dir(filepath)] {
			sortedFilepaths = append(sortedFilepaths, filepath)
		}
	}

	sort.Strings(sortedFilepaths)

	for _, filepath := range sortedFilepaths {
		actions = append(actions, RemoveAll(filepath))
	}

	// Now, we download each module, which will include
	// deleting any contents previously there. The
	// cache may strip out the download action if it
	// can prove that the right data is already there.
	for _, module := range modules {
		full := path.Join(goDir, module.Name)
		actions = append(actions, DownloadGoModule{Module: module, Path: full})
		for _, pkg := range module.Packages {
			full = path.Join(goDir, pkg.Name)
			if pkg.BuildFile != "" {
				actions = append(actions, CopyBUILD{Source: pkg.BuildFile, Path: path.Join(full, buildBazel)})
			} else {
				actions = append(actions, GenerateGoPackageBUILD{Package: pkg, Path: path.Join(full, buildBazel)})
			}
		}
	}

	return actions, nil
}
