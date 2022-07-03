// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command crate-features analyses the Rust crates in the
// repository for unstable features, printing a summary.
//
package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"sort"
	"strings"
	"text/tabwriter"
)

// Features parses the given file, returning the set
// of unstable Rust features the crate requires.
//
func Features(fsys fs.FS, name string) ([]string, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	found := make(map[string]bool)
	s := bufio.NewScanner(f)
	for s.Scan() {
		const (
			featurePrefix = "#![feature("
			featureSuffix = ")]"
		)

		line := s.Text()
		if !strings.HasPrefix(line, featurePrefix) {
			continue
		}

		features := strings.TrimPrefix(line, featurePrefix)

		// Drop any trailing line comment.
		if i := strings.Index(features, "//"); i > 0 {
			features = strings.TrimSpace(features[:i])
		}

		if !strings.HasSuffix(features, featureSuffix) {
			log.Printf("WARN:  %s: malformed feature inner attribute line: %q", name, line)
			continue
		}

		features = strings.TrimSuffix(features, featureSuffix)
		for _, feature := range strings.Split(features, ",") {
			found[strings.TrimSpace(feature)] = true
		}
	}

	if err := s.Err(); err != nil {
		return nil, err
	}

	if len(found) == 0 {
		return nil, nil
	}

	features := make([]string, 0, len(found))
	for feature := range found {
		features = append(features, feature)
	}

	sort.Strings(features)

	return features, nil
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

	features := make(map[string][]string)
	fsys := os.DirFS(".")
	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if path.Base(name) != "lib.rs" {
			return nil
		}

		found, err := Features(fsys, name)
		if err != nil {
			return err
		}

		crate := path.Dir(name)
		if path.Base(crate) == "src" {
			crate = path.Dir(crate)
		}

		for _, feature := range found {
			features[feature] = append(features[feature], crate)
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Failed to scan repository: %v", err)
	}

	sorted := make([]string, 0, len(features))
	for feature, crates := range features {
		sorted = append(sorted, feature)
		sort.Strings(crates)
	}

	sort.Strings(sorted)

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, ' ', 0)
	for _, feature := range sorted {
		fmt.Fprintf(w, "%s\t%s\n", feature, strings.Join(features[feature], ", "))
	}

	err = w.Flush()
	if err != nil {
		log.Fatalf("Failed to print summary: %v", err)
	}
}
