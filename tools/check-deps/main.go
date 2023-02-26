// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command check-deps uses package vendeps to check external dependencies for issues.
package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/google/osv-scanner/pkg/models"
	"github.com/google/osv-scanner/pkg/osv"

	"firefly-os.dev/tools/starlark"
	"firefly-os.dev/tools/vendeps"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("")
}

func main() {
	// If we're being run with `bazel run`, we're in
	// a semi-random build directory, and need to move
	// to the workspace root directory.
	//
	workspace := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if workspace != "" {
		err := os.Chdir(workspace)
		if err != nil {
			log.Printf("Failed to change directory to %q: %v", workspace, err)
		}
	}

	fsys := os.DirFS(".")
	err := CheckDependencies(fsys)
	if err != nil {
		log.Fatalf("Failed to check dependencies: %v", err)
	}
}

// CheckDependencies assesses the dependency set for
// unused dependencies.
func CheckDependencies(fsys fs.FS) error {
	data, err := fs.ReadFile(fsys, vendeps.DepsBzl)
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", vendeps.DepsBzl, err)
	}

	var deps vendeps.Deps
	err = starlark.Unmarshal(vendeps.DepsBzl, data, &deps)
	if err != nil {
		return err
	}

	if len(deps.Go) == 0 {
		return nil
	}

	// Loop through the set of dependencies to
	// identify all Bazel packages that would
	// be produced. We then delete entries that
	// we know are used. This will leave us with
	// the dependencies that are unused.
	all := make(map[string][]string)
	directOnly := make(map[string][]string)
	var goModules, goPackages int
	for _, dep := range deps.Go {
		goModules++
		for _, pkg := range dep.Packages {
			goPackages++
			path := "vendor/" + pkg.Name
			directChildren := make([]string, 0, len(pkg.Deps))
			children := make([]string, 0, len(pkg.Deps)+len(pkg.TestDeps))
			for _, child := range pkg.Deps {
				children = append(children, "vendor/"+child)
				directChildren = append(directChildren, "vendor/"+child)
			}
			for _, child := range pkg.TestDeps {
				children = append(children, "vendor/"+child)
			}

			all[path] = children
			directOnly[path] = directChildren
		}
	}

	// Use `bazel query` to identify the set of
	// Bazel packages that are being used in the
	// vendored dependencies.
	var roots = []string{
		"shared",
		"tools",
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
		children, _ := all[pkg]
		directChildren, ok := directOnly[pkg]
		if !ok {
			continue
		}

		for _, child := range children {
			delete(all, child)
		}

		for _, child := range directChildren {
			delete(directOnly, child)
		}

		delete(all, pkg)
		delete(directOnly, pkg)
	}

	// Work out how many dependencies were
	// not used.
	var goUnused int
	unused := make([]string, 0, len(all))
	for pkg := range all {
		if strings.HasPrefix(pkg, "vendor/") {
			goUnused++
		} else {
			return fmt.Errorf("found unexpected Bazel package //%s", pkg)
		}

		unused = append(unused, pkg)
	}

	sort.Strings(unused)

	// Work out how many dependencies were
	// only used in tests.
	var goTestsOnly int
	testsOnly := make([]string, 0, len(directOnly))
	for pkg := range directOnly {
		if strings.HasPrefix(pkg, "vendor/") {
			goTestsOnly++
		} else {
			return fmt.Errorf("found unexpected Bazel package //%s", pkg)
		}

		testsOnly = append(testsOnly, pkg)
	}

	sort.Strings(testsOnly)

	fmt.Printf("Go modules: %d (%d packages, %d unused, %d used only in tests)\n", goModules, goPackages, goUnused, goTestsOnly)

	// Check for known vulnerabilities.
	//
	// Start by building the set of queries.
	batched := osv.BatchedQuery{
		Queries: make([]*osv.Query, len(deps.Go)),
	}

	for i, module := range deps.Go {
		batched.Queries[i] = &osv.Query{
			Package: osv.Package{
				Name:      module.Name,
				Ecosystem: "Go",
			},
			Version: strings.TrimPrefix(module.Version, "v"),
		}
	}

	res, err := osv.MakeRequest(batched)
	if err != nil {
		return err
	}

	hydrated, err := osv.Hydrate(res)
	if err != nil {
		return err
	}

	// Group vulns together.
	var vulns []*models.Vulnerability
	seen := make(map[string]bool)
	for _, result := range hydrated.Results {
		if len(result.Vulns) == 0 {
			continue
		}

		for _, vuln := range result.Vulns {
			if !seen[vuln.ID] {
				seen[vuln.ID] = true
				v := vuln
				vulns = append(vulns, &v)
			}
		}
	}

	sort.Slice(vulns, func(i, j int) bool { return vulns[i].ID < vulns[j].ID })

	var b bytes.Buffer
	for i, vuln := range vulns {
		if i > 0 {
			b.WriteByte('\n')
		}

		b.WriteString(vuln.ID)
		if len(vuln.Aliases) > 0 {
			b.WriteString(" (")
			b.WriteString(strings.Join(vuln.Aliases, ", "))
			b.WriteByte(')')
		}

		b.WriteString("\n\tAffected modules:\n")
		for _, src := range vuln.Affected {
			b.WriteString("\t\t")
			b.WriteString(src.Package.Name)
			b.WriteByte('\n')
		}

		if len(vuln.References) > 0 {
			b.WriteString("\tReferences:\n")
			for _, ref := range vuln.References {
				b.WriteString("\t\t")
				b.WriteString(ref.URL)
				b.WriteByte('\n')
			}
		}
	}

	if len(vulns) > 0 {
		os.Stderr.Write(b.Bytes())
		return fmt.Errorf("Found %d vulnerabilities.", len(vulns))
	}

	if len(directOnly) == 0 {
		// All dependencies are used directly.
		return nil
	}

	fmt.Println("Dependencies used only in tests:")
	for _, pkg := range testsOnly {
		fmt.Printf("  //%s\n", pkg)
	}

	if len(all) == 0 {
		// All dependencies are used.
		return nil
	}

	fmt.Println("Unused dependencies:")
	for _, pkg := range unused {
		fmt.Printf("  //%s\n", pkg)
	}

	return nil
}
