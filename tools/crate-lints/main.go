// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command crate-lints checks that all Rust crates in the repository
// configure a mandatory set of lints.
package main

import (
	"bufio"
	"io/fs"
	"log"
	"os"
	"path"
	"sort"
	"strings"
)

// The set of lints to require.
// Please keep the list sorted alphabetically.
var lints = []string{
	"clippy::float_arithmetic",
	"clippy::inline_asm_x86_att_syntax",
	"clippy::missing_panics_doc",
	"clippy::panic",
	"clippy::return_self_not_must_use",
	"clippy::single_char_lifetime_names",
	"clippy::wildcard_imports",
	"deprecated_in_future",
	"keyword_idents",
	"macro_use_extern_crate",
	"missing_abi",
	"unused_crate_dependencies",
	"unsafe_code",
}

// LintChecker tracks whether a set of Rust crates
// is missing any of the mandatory lints.
type LintChecker struct {
	Missing int
}

// Check parses the given file, printing an error
// for each missing lint and updating the LintChecker's
// count of crates with missing lints.
func (c *LintChecker) Check(fsys fs.FS, name string) error {
	f, err := fsys.Open(name)
	if err != nil {
		return err
	}

	defer f.Close()

	missing := make(map[string]bool, len(lints))
	for _, lint := range lints {
		missing[lint] = true
	}

	found := make([]string, 0, len(lints))
	s := bufio.NewScanner(f)
	for s.Scan() {
		const (
			innerAttributePrefix = "#!["
			innerAttributeAllow  = innerAttributePrefix + "allow("
			innerAttributeWarn   = innerAttributePrefix + "warn("
			innerAttributeDeny   = innerAttributePrefix + "deny("
			innerAttributeForbid = innerAttributePrefix + "forbid("
			innerAttributeSuffix = ")]"
		)

		line := s.Text()
		var attribute string
		switch {
		case strings.HasPrefix(line, innerAttributeAllow):
			attribute = strings.TrimPrefix(line, innerAttributeAllow)
		case strings.HasPrefix(line, innerAttributeWarn):
			attribute = strings.TrimPrefix(line, innerAttributeWarn)
		case strings.HasPrefix(line, innerAttributeDeny):
			attribute = strings.TrimPrefix(line, innerAttributeDeny)
		case strings.HasPrefix(line, innerAttributeForbid):
			attribute = strings.TrimPrefix(line, innerAttributeForbid)
		default:
			continue
		}

		// Drop any trailing line comment.
		if i := strings.Index(attribute, "//"); i > 0 {
			attribute = strings.TrimSpace(attribute[:i])
		}

		if !strings.HasSuffix(attribute, innerAttributeSuffix) {
			log.Printf("WARN:  %s: malformed inner attribute line: %q", name, line)
			continue
		}

		attribute = strings.TrimSuffix(attribute, innerAttributeSuffix)
		found = append(found, attribute)
		if !missing[attribute] {
			log.Printf("WARN:  %s: unexpected inner attribute: %q", name, attribute)
			continue
		}

		delete(missing, attribute)
	}

	if err := s.Err(); err != nil {
		return err
	}

	if !sort.StringsAreSorted(found) {
		log.Printf("WARN:  %s: attributes not sorted", name)
	}

	if len(missing) == 0 {
		return nil
	}

	c.Missing++
	names := make([]string, 0, len(missing))
	for missing := range missing {
		names = append(names, missing)
	}

	sort.Strings(names)

	for _, missing := range names {
		log.Printf("ERROR: %s: missing inner attribute %q", name, missing)
	}

	return nil
}

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

	check := new(LintChecker)
	fsys := os.DirFS(".")
	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if name == "vendor" {
				return fs.SkipDir
			}

			return nil
		}

		if path.Base(name) != "lib.rs" {
			return nil
		}

		return check.Check(fsys, name)
	})

	if err != nil {
		log.Fatalf("Failed to scan repository: %v", err)
	}

	if check.Missing > 0 {
		os.Exit(1)
	}
}
