// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command check-deps uses package vendeps to check external dependencies for issues.
package main

import (
	"log"
	"os"

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
	err := vendeps.CheckDependencies(fsys)
	if err != nil {
		log.Fatalf("Failed to check dependencies: %v", err)
	}
}
