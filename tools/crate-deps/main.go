// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command crate-deps analyses the set of Rust crate dependencies.
//
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bazelbuild/buildtools/build"
	"github.com/bazelbuild/buildtools/labels"
)

type Manifest struct {
	Checksum string            `json:"checksum"`
	Crates   map[string]*Crate `json:"crates"`
}

type Crate struct {
	Name                  string     `json:"name"`
	Version               string     `json:"version"`
	CommonAttributes      Attributes `json:"common_attrs"`
	BuildScriptAttributes Attributes `json:"build_script_attrs"`
}

type Attributes struct {
	Deps          Deps     `json:"deps"`
	ProcMacroDeps Deps     `json:"proc_macro_deps"`
	ExtraDeps     []string `json:"extra_deps"`
}

type Deps struct {
	Common  []BazelTarget            `json:"common"`
	Selects map[string][]BazelTarget `json:"selects"`
}

type BazelTarget struct {
	Id     string `json:"id"`
	Target string `json:"target"`
	Alias  string `json:"alias"`
}

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("")
}

func main() {
	// If we're being run with `bazel run`, we're in
	// a semi-random build directory, and need to move
	// to the caller's working directory.
	//
	workspace := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if workspace != "" {
		err := os.Chdir(workspace)
		if err != nil {
			log.Printf("Failed to change directory to %q: %v", workspace, err)
		}
	}

	var name string
	if workspace != "" {
		// We know the target within our workspace.
		name = filepath.Join("bazel", "deps", "Cargo.Bazel.lock")
	} else {
		if len(os.Args) != 2 {
			fmt.Fprintf(os.Stderr, "Usage:\n  %s MANIFEST\n", filepath.Base(os.Args[0]))
			os.Exit(2)
		}

		name = os.Args[1]
	}

	// Read and parse the manifest.

	data, err := os.ReadFile(name)
	if err != nil {
		log.Fatalf("Failed to open manifest %q: %v", name, err)
	}

	var manifest Manifest
	err = json.Unmarshal(data, &manifest)
	if err != nil {
		log.Fatalf("Failed to parse manifest %s: %v", name, err)
	}

	// Get the set of directly used crate dependencies
	// from Bazel. This isn't quite as simple as we'd
	// like. First, we query deps(@//..., 1) to get our
	// direct dependencies. Then, we must resolve each
	// dependency alias of the form `@crates//:foo` to
	// its target of the form `@crates__foo-1.2.3//`.
	//
	// We could do this resolution by performing a query
	// for deps(@crates//:foo, 1), but the overhead of
	// calling bazel query that many times is annoying.
	// To optimise this, we instead do one query for
	// @crates//:all with output location to discover
	// the filepath to the BUILD file for @crates. We
	// then parse that using github.com/bazelbuild/buildtools/build
	// to build a mapping for each alias.
	//
	// With this mapping, we can then process each
	// alias target to same format as used in the manifest.

	// Find the BUILD file for @crates//.
	var stdout, both bytes.Buffer
	cmd := exec.Command("bazel", "query", "@crates//:all", "--output=location")
	cmd.Stdout = io.MultiWriter(&stdout, &both)
	cmd.Stderr = &both
	err = cmd.Run()
	if err != nil {
		os.Stderr.Write(both.Bytes())
		log.Fatalf("Failed to run bazel query for the path to @crates//:BUILD: %v", err)
	}

	cratesPath, _, ok := strings.Cut(stdout.String(), ":")
	if !ok {
		os.Stderr.Write(both.Bytes())
		log.Fatalf("Failed to find path to @crates//:BUILD: no line number found")
	}

	aliases, err := ParseCratesBuild(cratesPath)
	if err != nil {
		os.Stderr.Write(both.Bytes())
		log.Fatalf("Failed to parse @crates// aliases: %v", err)
	}

	stdout.Reset()
	both.Reset()
	cmd = exec.Command("bazel", "query", "deps(@//..., 1) + deps(@//:bootloader_binary, 2) + deps(@//:bootloader_build_script, 2)")
	cmd.Stdout = io.MultiWriter(&stdout, &both)
	cmd.Stderr = &both
	err = cmd.Run()
	if err != nil {
		os.Stderr.Write(both.Bytes())
		log.Fatalf("Failed to run bazel query: %v", err)
	}

	directDeps := NewSet()
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		const (
			shortCratesPrefix = "@crates//:"
			fullCratesPrefix  = "@crates__"
			cratesRepository  = "crates__"
		)

		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, shortCratesPrefix) {
			continue
		}

		resolved, ok := aliases[line]
		if !ok {
			log.Fatalf("Found crate dependency on %q, but no alias found for this crate", line)
		}

		if !strings.HasPrefix(resolved, fullCratesPrefix) {
			log.Fatalf("Found crate dependency on %q, which resolves to %q: unexpected label format", line, resolved)
		}

		// Within Bazel, dependencies are expressed as
		// @crates__foo-bar-1.2.3//, whereas in the
		// manifest they're foo-bar 1.2.3, so here we
		// translate from Bazel's format to the manifest's.
		label := labels.Parse(resolved)
		crate := strings.TrimPrefix(label.Repository, cratesRepository)
		if i := strings.LastIndexByte(crate, '-'); i < 0 {
			log.Fatalf("Found malformed direct dependency %q: expected form @crates__foo-bar-1.2.3//", resolved)
		} else {
			crate = crate[:i] + " " + crate[i+1:]
		}

		directDeps.Add(crate)
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Failed to parse bazel query output: %v", err)
	}

	const DirectDependencies = "<direct>"

	// Parse the manifest for the remaining data.
	deps := NewDepSet()
	crates := NewSet()
	direct := NewSet()
	indirect := NewSet()
	versions := make(Mapping)
	for id, crate := range manifest.Crates {
		const buildScriptPrefix = "build_script_"
		if id == "direct-cargo-bazel-deps 0.0.1" {
			id = DirectDependencies
		}

		deps.All.Add(id)
		crates.Add(crate.Name)
		versions.Add(crate.Name, crate.Version)
		for _, target := range crate.CommonAttributes.Deps.Common {
			if !strings.HasPrefix(target.Target, buildScriptPrefix) {
				deps.Add(id, target.Id)
			}
		}
		for _, selects := range crate.CommonAttributes.Deps.Selects {
			for _, target := range selects {
				if !strings.HasPrefix(target.Target, buildScriptPrefix) {
					deps.Add(id, target.Id)
				}
			}
		}
		for _, target := range crate.CommonAttributes.ProcMacroDeps.Common {
			if !strings.HasPrefix(target.Target, buildScriptPrefix) {
				deps.Add(id, target.Id)
			}
		}
		for _, selects := range crate.CommonAttributes.ProcMacroDeps.Selects {
			for _, target := range selects {
				if !strings.HasPrefix(target.Target, buildScriptPrefix) {
					deps.Add(id, target.Id)
				}
			}
		}
		for _, target := range crate.BuildScriptAttributes.Deps.Common {
			if !strings.HasPrefix(target.Target, buildScriptPrefix) {
				deps.Add(id, target.Id)
			}
		}
		for _, selects := range crate.BuildScriptAttributes.Deps.Selects {
			for _, target := range selects {
				if !strings.HasPrefix(target.Target, buildScriptPrefix) {
					deps.Add(id, target.Id)
				}
			}
		}
		for _, target := range crate.BuildScriptAttributes.ProcMacroDeps.Common {
			if !strings.HasPrefix(target.Target, buildScriptPrefix) {
				deps.Add(id, target.Id)
			}
		}
		for _, selects := range crate.BuildScriptAttributes.ProcMacroDeps.Selects {
			for _, target := range selects {
				if !strings.HasPrefix(target.Target, buildScriptPrefix) {
					deps.Add(id, target.Id)
				}
			}
		}
		if id != DirectDependencies {
			if directDeps.Has(id) {
				direct.Add(id)
			} else {
				indirect.Add(id)
			}
		}
	}

	transitive := make(Mapping)
	var addTransitives func(id string, set *Set)
	addTransitives = func(id string, set *Set) {
		if set == nil {
			return
		}

		for _, dep := range set.List() {
			transitive.Add(id, dep)
			addTransitives(id, deps.Forward[dep])
		}
	}

	for _, id := range deps.All.List() {
		if id == DirectDependencies {
			continue
		}

		addTransitives(id, deps.Forward[id])
	}

	// Print out our analysis before we start
	// performing tests. We tend to ignore the
	// fake crate used to represent the set
	// of dependencies declared explicitly.

	fmt.Printf("INFO: Dependencies (%d):\n", len(deps.All.List())-1)
	for _, id := range deps.All.List() {
		if id == DirectDependencies {
			continue
		}

		dependencies := deps.Forward[id].List()
		transitive := transitive[id].List()
		direct := ""
		if len(dependencies) > 0 {
			direct = " (" + strings.Join(dependencies, ", ") + ")"
		}

		fmt.Printf("\t%s: %d direct%s, %d transitive\n", id, len(dependencies), direct, len(transitive))
	}

	fmt.Printf("INFO: Reverse dependencies (%d):\n", len(deps.All.List())-1)
	for _, id := range deps.All.List() {
		if id == DirectDependencies {
			continue
		}

		dependants := deps.Backward[id].List()
		fmt.Printf("\t%s: %s\n", id, strings.Join(dependants, ", "))
	}

	fmt.Printf("INFO: Direct dependencies (%d):\n", len(direct.List()))
	for _, dep := range direct.List() {
		dependencies := deps.Forward[dep].List()
		transitive := transitive[dep].List()

		fmt.Printf("\t%s (%d direct, %d transitive dependencies)\n", dep, len(dependencies), len(transitive))
	}

	fmt.Printf("INFO: Indirect dependencies (%d):\n", len(indirect.List()))
	for _, dep := range indirect.List() {
		fmt.Printf("\t%s\n", dep)
	}

	// Perform tests on the dependency graph
	// to identify issues.

	fail := false
	errorf := func(format string, v ...any) {
		fail = true
		log.Printf(format, v...)
	}

	// Check we only have one version of each dependency.
	multiVersion := NewSet()
	for _, crate := range crates.List() {
		v := versions[crate].List()
		if len(v) > 1 {
			multiVersion.Add(fmt.Sprintf("%s: %s", crate, strings.Join(v, ", ")))
		}
	}

	if all := multiVersion.List(); len(all) != 0 {
		errorf("ERROR: Crates have multiple versions:\n\t%s", strings.Join(all, "\n\t"))
	} else {
		fmt.Println("INFO: All crates use a single version.")
	}

	// Check we have no unnecessary dependencies.
	unnecessary := NewSet()
	for _, id := range deps.All.List() {
		if id == DirectDependencies {
			continue
		}

		dependants := deps.Backward[id].List()

		// Skip our direct dependencies.
		if directDeps.Has(id) {
			continue
		}

		// Ignore the explicit deps list.
		if len(dependants) == 0 || (len(dependants) == 1 && dependants[0] == DirectDependencies) {
			unnecessary.Add(id)
		}
	}

	if all := unnecessary.List(); len(all) != 0 {
		errorf("ERROR: Crates have no direct internal dependants:\n\t%s", strings.Join(all, "\n\t"))
	} else {
		fmt.Println("INFO: All crate dependencies are necessary.")
	}

	if fail {
		os.Exit(1)
	}
}

type DepSet struct {
	All      *Set
	Forward  Mapping
	Backward Mapping
}

func NewDepSet() *DepSet {
	return &DepSet{
		All:      NewSet(),
		Forward:  make(Mapping),
		Backward: make(Mapping),
	}
}

func (s *DepSet) Add(from, to string) {
	s.All.Add(from)
	s.All.Add(to)
	s.Forward.Add(from, to)
	s.Backward.Add(to, from)
}

type Mapping map[string]*Set

func (m Mapping) Add(from, to string) {
	s, ok := m[from]
	if !ok {
		s = NewSet()
		m[from] = s
	}

	s.Add(to)
}

type Set struct {
	m map[string]bool
	l []string
}

func NewSet() *Set {
	return &Set{
		m: make(map[string]bool),
	}
}

func (s *Set) Add(dep string) {
	if !s.m[dep] {
		s.m[dep] = true
		s.l = append(s.l, dep)
		sort.Strings(s.l)
	}
}

func (s *Set) Has(v string) bool {
	return s.m[v]
}

func (s *Set) List() []string {
	if s == nil {
		return nil
	}

	return s.l
}

func ParseCratesBuild(name string) (aliases map[string]string, err error) {
	cratesData, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("failed to open @crates//:BUILD at %q: %v", name, err)
	}

	cratesBuild, err := build.ParseBuild(name, cratesData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse @crates//:BUILD at %q: %v", name, err)
	}

	at := func(v build.Expr) string {
		pos, _ := v.Span()
		return fmt.Sprintf("%s:%d:", name, pos.Line)
	}

	aliases = make(map[string]string)
	for _, stmt := range cratesBuild.Stmt {
		call, ok := stmt.(*build.CallExpr)
		if !ok {
			continue
		}

		if fun, ok := call.X.(*build.Ident); !ok || fun.Name != "alias" {
			continue
		}

		var name, actual string
		for _, expr := range call.List {
			assign, ok := expr.(*build.AssignExpr)
			if !ok {
				return nil, fmt.Errorf("%s found alias with unexpected call parameter: %#v", at(expr), expr)
			}

			lhs, ok := assign.LHS.(*build.Ident)
			if !ok {
				return nil, fmt.Errorf("%s found alias with unexpected call parameter: LHS = %#v", at(assign.LHS), assign.LHS)
			}

			if lhs.Name != "name" && lhs.Name != "actual" {
				continue
			}

			rhs, ok := assign.RHS.(*build.StringExpr)
			if !ok {
				return nil, fmt.Errorf("%s found alias with unexpected call parameter: %s RHS = %#v", at(assign.RHS), lhs.Name, assign.RHS)
			}

			if lhs.Name == "name" {
				if name != "" {
					return nil, fmt.Errorf("%s found alias with multiple %q parameters", at(lhs), lhs.Name)
				}

				name = rhs.Value
			} else {
				if actual != "" {
					return nil, fmt.Errorf("%s found alias with multiple %q parameters", at(lhs), lhs.Name)
				}

				actual = rhs.Value
			}
		}

		if name == "" || actual == "" {
			return nil, fmt.Errorf("%s found alias with missing parameters: name = %q, actual = %q", at(call), name, actual)
		}

		full := labels.Label{
			Repository: "crates",
			Target:     name,
		}

		aliases[full.Format()] = actual
	}

	return aliases, nil
}
