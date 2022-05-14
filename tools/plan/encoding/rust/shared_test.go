// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package rust

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// getRustfmt returns a path to the rustfmt
// tool.
//
func getRustfmt(t *testing.T) string {
	t.Helper()

	// We need to access rustfmt, but for some reason, we
	// can't just use the path we get from the environment
	// variable and instead need to construct an absolute
	// path. This combines the runfiles path, the workspace
	// name, and the second path we receive.
	//
	// As this code is quite fragile, we're unusually strict
	// in checking that we get what we expect.
	rustfmt := os.Getenv("rustfmt")
	parts := strings.Split(rustfmt, " ")
	if len(parts) != 2 {
		t.Fatalf("rustfmt: expected 2 space-separated paths, found %d: %q", len(parts), rustfmt)
	}

	// Make sure we have them the right way around.
	if !strings.HasPrefix(parts[0], "bazel-out") {
		t.Fatalf("rustfmt: expected first path to begin in bazel-out, got %q", parts[0])
	}
	if !strings.HasPrefix(parts[1], "external") {
		t.Fatalf("rustfmt: expected second path to begin in external, got %q", parts[1])
	}

	runfiles := os.Getenv("RUNFILES_DIR")
	if runfiles == "" {
		t.Fatal("rustfmt: got no path in RUNFILES_DIR")
	}

	if !filepath.IsAbs(runfiles) {
		t.Fatalf("rustfmt: unexpected relative path in RUNFILES_DIR: %q", runfiles)
	}

	workspace := os.Getenv("TEST_WORKSPACE")
	if workspace == "" {
		t.Fatal("rustfmt: got no path in TEST_WORKSPACE")
	}

	rustfmt = filepath.Join(runfiles, workspace, parts[1])
	info, err := os.Stat(rustfmt)
	if err != nil {
		t.Fatalf("rustfmt: failed to find rustfmt at %q: %v", rustfmt, err)
	}

	mode := info.Mode()
	if mode.IsDir() {
		t.Fatalf("rustfmt: path at %q is a directory", rustfmt)
	}

	if !mode.IsRegular() {
		t.Fatalf("rustfmt: path at %q is not regular", rustfmt)
	}

	return rustfmt
}
