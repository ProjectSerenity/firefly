// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendeps

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/vuln/osv"

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
	directOnly := make(map[string][]string)
	crateVersion := make(map[string]string)
	var rustCrates, goModules, goPackages int
	for _, dep := range deps.Rust {
		rustCrates++
		path := "vendor/rust/" + dep.Name
		directChildren := make([]string, 0, len(dep.Deps)+len(dep.ProcMacroDeps)+len(dep.BuildScriptDeps))
		children := make([]string, 0, len(dep.Deps)+len(dep.ProcMacroDeps)+len(dep.BuildScriptDeps)+len(dep.TestDeps))
		for _, child := range dep.Deps {
			children = append(children, "vendor/rust/"+child)
			directChildren = append(directChildren, "vendor/rust/"+child)
		}
		for _, child := range dep.ProcMacroDeps {
			children = append(children, "vendor/rust/"+child)
			directChildren = append(directChildren, "vendor/rust/"+child)
		}
		for _, child := range dep.BuildScriptDeps {
			children = append(children, "vendor/rust/"+child)
			directChildren = append(directChildren, "vendor/rust/"+child)
		}
		for _, child := range dep.TestDeps {
			children = append(children, "vendor/rust/"+child)
		}

		all[path] = children
		directOnly[path] = directChildren
		crateVersion[dep.Name] = dep.Version
	}

	for _, dep := range deps.Go {
		goModules++
		for _, pkg := range dep.Packages {
			goPackages++
			path := "vendor/go/" + pkg.Name
			directChildren := make([]string, 0, len(pkg.Deps))
			children := make([]string, 0, len(pkg.Deps)+len(pkg.TestDeps))
			for _, child := range pkg.Deps {
				children = append(children, "vendor/go/"+child)
				directChildren = append(directChildren, "vendor/go/"+child)
			}
			for _, child := range pkg.TestDeps {
				children = append(children, "vendor/go/"+child)
			}

			all[path] = children
			directOnly[path] = directChildren
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
	var rustUnused, goUnused int
	unused := make([]string, 0, len(all))
	for pkg := range all {
		if strings.HasPrefix(pkg, "vendor/rust/") {
			rustUnused++
		} else if strings.HasPrefix(pkg, "vendor/go/") {
			goUnused++
		} else {
			return fmt.Errorf("found unexpected Bazel package //%s", pkg)
		}

		unused = append(unused, pkg)
	}

	sort.Strings(unused)

	// Work out how many dependencies were
	// only used in tests.
	var rustTestsOnly, goTestsOnly int
	testsOnly := make([]string, 0, len(directOnly))
	for pkg := range directOnly {
		if strings.HasPrefix(pkg, "vendor/rust/") {
			rustTestsOnly++
		} else if strings.HasPrefix(pkg, "vendor/go/") {
			goTestsOnly++
		} else {
			return fmt.Errorf("found unexpected Bazel package //%s", pkg)
		}

		testsOnly = append(testsOnly, pkg)
	}

	sort.Strings(testsOnly)

	fmt.Printf("Rust crates: %d (%d unused, %d used only in tests)\n", rustCrates, rustUnused, rustTestsOnly)
	fmt.Printf("Go modules: %d (%d packages, %d unused, %d used only in tests)\n", goModules, goPackages, goUnused, goTestsOnly)

	vulns, err := FetchVulns()
	if err != nil {
		return fmt.Errorf("failed to fetch vulnerability data: %v", err)
	}

	now := time.Now()
	vulnsFound := 0
	for _, advisory := range vulns.Rust {
		if advisory.Withdrawn != nil && !advisory.Withdrawn.IsZero() && advisory.Withdrawn.Before(now) {
			continue
		}

		var affected []string
		for _, affects := range advisory.Affected {
			version, ok := crateVersion[affects.Package.Name]
			if !ok {
				continue
			}

			if !affects.Ranges.AffectsSemver(version) {
				continue
			}

			affected = append(affected, affects.Package.Name)
		}

		if len(affected) == 0 {
			continue
		}

		log.Printf("Potential vulnerability found:")
		if len(affected) == 1 {
			log.Printf("  Crate:   %s", affected[0])
		} else {
			log.Printf("  Crates:  %s", strings.Join(affected, ", "))
		}
		if len(advisory.Aliases) == 0 {
			log.Printf("  ID:      %s", advisory.ID)
		} else {
			log.Printf("  ID:      %s (%s)", advisory.ID, strings.Join(advisory.Aliases, ", "))
		}
		log.Printf("  Details: %s", strings.Join(strings.Split(advisory.Details, "\n"), "\n           "))
		vulnsFound++
	}

	if vulnsFound == 0 {
		fmt.Printf("No vulnerabilities found in Rust crates.\n")
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

// Vulns describes the set of vulnerability advisory
// data for a set of software dependencies.
//
type Vulns struct {
	Rust []*osv.Entry
	Go   []*osv.Entry
}

// fetchVulns fetches/updates the set of vulnerability
// advisories, then parses them into structured vuln
// data in OSV format.
//
func FetchVulns() (*Vulns, error) {
	vulns := new(Vulns)

	// Rust.
	rustAdvisories := filepath.Join(os.TempDir(), "rust-advisories")
	err := fetchGitRepo("https://github.com/rustsec/advisory-db", "osv", rustAdvisories)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	fsys := os.DirFS(rustAdvisories)
	err = fs.WalkDir(fsys, "crates", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(name, ".json") {
			return nil
		}

		buf.Reset()
		f, err := fsys.Open(name)
		if err != nil {
			return fmt.Errorf("failed to open %s: %v", name, err)
		}

		_, err = io.Copy(&buf, f)
		if err != nil {
			f.Close()
			return fmt.Errorf("failed to read %s: %v", name, err)
		}

		err = f.Close()
		if err != nil {
			return fmt.Errorf("failed to close %s: %v", name, err)
		}

		var entry osv.Entry
		err = json.Unmarshal(buf.Bytes(), &entry)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %v", name, err)
		}

		vulns.Rust = append(vulns.Rust, &entry)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse Rust advisories: %v", err)
	}

	// TODO: Fetch and parse Go advisories.

	return vulns, nil
}

// fetchGitRepo uses git to clone/update the given Git
// repository to the directory provided.
//
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
