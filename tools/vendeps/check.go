// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"sort"
	"strings"

	"firefly-os.dev/tools/starlark"
)

// CheckDependencies assesses the dependency set for
// unused dependencies.
//
func CheckDependencies(fsys fs.FS) error {
	data, err := fs.ReadFile(fsys, depsBzl)
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", depsBzl, err)
	}

	var deps Deps
	err = starlark.Unmarshal(depsBzl, data, &deps)
	if err != nil {
		return err
	}

	if len(deps.Rust) == 0 && len(deps.Go) == 0 {
		return nil
	}

	// Loop through the set of dependencies to
	// identify all Bazel packages that would
	// be produced. We then delete entries that
	// we know are used. This will leave us with
	// the dependencies that are unused.
	all := make(map[string][]string)
	for _, dep := range deps.Rust {
		path := "vendor/rust/" + dep.Name
		children := make([]string, 0, len(dep.Deps)+len(dep.ProcMacroDeps)+len(dep.BuildScriptDeps)+len(dep.TestDeps))
		for _, child := range dep.Deps {
			children = append(children, "vendor/rust/"+child)
		}
		for _, child := range dep.ProcMacroDeps {
			children = append(children, "vendor/rust/"+child)
		}
		for _, child := range dep.BuildScriptDeps {
			children = append(children, "vendor/rust/"+child)
		}
		for _, child := range dep.TestDeps {
			children = append(children, "vendor/rust/"+child)
		}

		all[path] = children
	}

	for _, dep := range deps.Go {
		for _, pkg := range dep.Packages {
			path := "vendor/go/" + pkg.Name
			children := make([]string, 0, len(pkg.Deps)+len(pkg.TestDeps))
			for _, child := range pkg.Deps {
				children = append(children, "vendor/go/"+child)
			}
			for _, child := range pkg.TestDeps {
				children = append(children, "vendor/go/"+child)
			}

			all[path] = children
		}
	}

	// Use `bazel query` to identify the set of
	// Bazel packages that are being used in the
	// vendored dependencies.
	var roots = []string{
		"bootloader",
		"kernel",
		"shared",
		"tools",
		"user",
	}

	for i := range roots {
		roots[i] = fmt.Sprintf("deps(//%s/...)", roots[i])
	}

	query := fmt.Sprintf(`(%s) intersect //vendor/...`, strings.Join(roots, " union "))
	args := []string{
		"query",
		"--noshow_progress",
		"--noshow_loading_progress",
		"--ui_event_filters=-info",
		query,
		"--output=package",
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("bazel", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		os.Stderr.Write(stderr.Bytes())
		return err
	}

	used := strings.Fields(stdout.String())

	// Loop through the dependency set,
	// removing them from the set of all
	// dependencies.
	for _, pkg := range used {
		children, ok := all[pkg]
		if !ok {
			continue
		}

		for _, child := range children {
			delete(all, child)
		}

		delete(all, pkg)
	}

	if len(all) == 0 {
		// All dependencies are used.
		return nil
	}

	unused := make([]string, 0, len(all))
	for pkg := range all {
		unused = append(unused, pkg)
	}

	sort.Strings(unused)

	fmt.Println("Unused dependencies:")
	for _, pkg := range unused {
		fmt.Printf("  //%s\n", pkg)
	}

	return nil
}
