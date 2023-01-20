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
	"os"
	"os/exec"
	"sort"
	"strings"

	"golang.org/x/vuln/osv"

	"firefly-os.dev/tools/starlark"
)

// CheckDependencies assesses the dependency set for
// unused dependencies.
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

// Vulns describes the set of vulnerability advisory
// data for a set of software dependencies.
type Vulns struct {
	Go []*osv.Entry
}

// fetchVulns fetches/updates the set of vulnerability
// advisories, then parses them into structured vuln
// data in OSV format.
func FetchVulns() (*Vulns, error) {
	vulns := new(Vulns)

	// TODO: Fetch and parse Go advisories.

	return vulns, nil
}

// fetchGitRepo uses git to clone/update the given Git
// repository to the directory provided.
func fetchGitRepo(repo, branch, dir string) error {
	info, err := os.Stat(dir)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		err = os.MkdirAll(dir, 0777)
		if err != nil {
			return fmt.Errorf("failed to create %s: %v", dir, err)
		}

		// Clone the repo.
		out, err := exec.Command("git", "clone", repo, "--branch", branch, "--single-branch", dir).CombinedOutput()
		if err != nil {
			os.Stderr.Write(out)
			return fmt.Errorf("failed to clone %s: %v", repo, err)
		}

		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to check Git repository destination %s: %v", dir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("cannot fetch Git repository to %s: not a directory", dir)
	}

	// Fetch and fast-forward.
	cmd := exec.Command("git-pull", "origin", branch)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stderr.Write(out)
		return fmt.Errorf("failed to update %s: %v", repo, err)
	}

	return nil
}
