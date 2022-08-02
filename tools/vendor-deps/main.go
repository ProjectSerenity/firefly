// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command vendor-deps uses package vendeps to vendor external dependencies into the repository.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"firefly-os.dev/tools/vendeps"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("")
}

func main() {
	var help, noCache, dryRun bool
	flag.BoolVar(&help, "h", false, "Show this message and exit.")
	flag.BoolVar(&noCache, "no-cache", false, "Ignore any locally cached dependency data.")
	flag.BoolVar(&dryRun, "dry-run", false, "Print the set of actions that would be performed, without performing them.")
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

	// Start by parsing the dependency manifest.
	fsys := os.DirFS(".")
	actions, err := vendeps.Vendor(fsys)
	if err != nil {
		log.Fatalf("Failed to load dependency manifest: %v", err)
	}

	if !noCache {
		actions = vendeps.StripCachedActions(fsys, actions)
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
