// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package vendeps provides functionality for managing vendored external dependencies.
//
package vendeps

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const (
	buildBazel  = "BUILD.bazel"
	depsBzl     = "deps.bzl"
	manifestBzl = "manifest.bzl"
	vendor      = "vendor"
)

// Deps describes a set of software dependencies.
//
type Deps struct {
	Rust []*RustCrate `bzl:"rust/crate"`
	Go   []*GoModule  `bzl:"go/module"`
}

// RustCrate contains the dependency information for
// a Rust crate.
//
type RustCrate struct {
	// Dependency details.
	Name    string `bzl:"name"`
	Version string `bzl:"version"`

	// Patches to be applied to the
	// downloaded crate, before the
	// BUILD file is copied/generated.
	PatchArgs []string `bzl:"patch_args"`
	Patches   []string `bzl:"patches"`

	// Manually-managed BUILD file.
	BuildFile string `bzl:"build_file"`

	// Build configuration
	Edition       string            `bzl:"edition"`
	Features      []string          `bzl:"features"`
	Deps          []string          `bzl:"deps"`
	ProcMacroDeps []string          `bzl:"proc_macro_deps"`
	RustcEnv      map[string]string `bzl:"rustc_env"`

	// Whether to create rustdocs.
	NoDocs bool `bzl:"no_docs"`

	// Whether the crate is a library or
	// a procedural macro library.
	ProcMacro bool `bzl:"proc_macro"`

	// Build script configuration.
	BuildScript     string   `bzl:"build_script"`
	BuildScriptDeps []string `bzl:"build_script_deps"`

	// Test configuration.
	NoTests       bool              `bzl:"no_tests"`
	TestData      []string          `bzl:"test_data"`
	TestDataGlobs []string          `bzl:"test_data_globs"`
	TestDeps      []string          `bzl:"test_deps"`
	TestEnv       map[string]string `bzl:"test_env"`

	// Generation details.
	Digest      string `bzl:"digest"`
	PatchDigest string `bzl:"patch_digest"`
}

// GoModule contains the information necessary
// to vendor a Go module, specifying the set
// of packages within the module that are used.
//
type GoModule struct {
	// Dependency details.
	Name    string `bzl:"name"`
	Version string `bzl:"version"`

	// Patches to be applied to the
	// downloaded module, before the
	// BUILD file is copied/generated.
	PatchArgs []string `bzl:"patch_args"`
	Patches   []string `bzl:"patches"`

	// Packages that should be used.
	Packages []*GoPackage `bzl:"packages/package"`

	// Generation details.
	Digest      string `bzl:"digest"`
	PatchDigest string `bzl:"patch_digest"`
}

// GoPackage describes a package within
// a Go module.
//
type GoPackage struct {
	// Dependency details.
	Name string `bzl:"name"`

	// Manually-managed BUILD file.
	BuildFile string `bzl:"build_file"`

	// Build configuration
	Deps       []string `bzl:"deps"`
	Embed      []string `bzl:"embed"`
	EmbedGlobs []string `bzl:"embed_globs"`

	// Test configuration.
	NoTests       bool     `bzl:"no_tests"`
	TestData      []string `bzl:"test_data"`
	TestDataGlobs []string `bzl:"test_data_globs"`
	TestDeps      []string `bzl:"test_deps"`
}

// UpdateDeps includes a set of dependencies
// for the purposes of updating them.
//
type UpdateDeps struct {
	Rust []*UpdateDep
	Go   []*UpdateDep
}

// UpdateDep describes the least information
// necessary to determine a third-party
// software library. This is used when
// determining whether updates are available.
//
type UpdateDep struct {
	Name    string
	Version *string
}

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("")
}

const userAgent = "Firefly-dependency-vendoring/1 (github.com/ProjectSerenity/firefly)"

func main() {
	var help, noCache, dryRun, check, update bool
	flag.BoolVar(&help, "h", false, "Show this message and exit.")
	flag.BoolVar(&noCache, "no-cache", false, "Ignore any locally cached dependency data.")
	flag.BoolVar(&dryRun, "dry-run", false, "Print the set of actions that would be performed, without performing them.")
	flag.BoolVar(&check, "check", false, "Check the dependency set for unused dependencies.")
	flag.BoolVar(&update, "update", false, "Check the dependency specification for dependency updates.")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage\n  %s [OPTIONS]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()

		os.Exit(2)
	}

	flag.Parse()

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

	if check {
		fsys := os.DirFS(".")
		err := CheckDependencies(fsys)
		if err != nil {
			log.Fatalf("Failed to check dependencies: %v", err)
		}

		return
	}

	if update {
		err := UpdateDependencies(depsBzl)
		if err != nil {
			log.Fatalf("Failed to update dependencies: %v", err)
		}

		return
	}

	// Start by parsing the dependency manifest.
	fsys := os.DirFS(".")
	actions, err := Vendor(fsys)
	if err != nil {
		log.Fatalf("Failed to load dependency manifest: %v", err)
	}

	if !noCache {
		actions = StripCachedActions(fsys, actions)
	}

	// Perform/print the actions.
	for _, action := range actions {
		if dryRun {
			fmt.Println(action)
		} else {
			err = action.Do(fsys)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
