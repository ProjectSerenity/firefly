// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/bazelbuild/buildtools/build"
)

func init() {
	RegisterCommand("rust", "Update the Rust nightly version and its tooling.", cmdRust)
}

func cmdRust(ctx context.Context, w io.Writer, args []string) error {
	const (
		dateFormat       = "2006-01-02"
		workspace        = "WORKSPACE"
		rustDate         = "RUST_ISO_DATE"
		toolchains       = "rust_register_toolchains"
		sha256s          = "sha256s"
		manifestTemplate = "https://static.rust-lang.org/dist/%s/channel-rust-nightly.toml"
		toolTemplate     = "https://static.rust-lang.org/dist/%s%s.tar.gz"
		manifestVersion  = "2"
	)

	var (
		date time.Time
	)

	flags := flag.NewFlagSet("rust", flag.ExitOnError)
	flags.Func("version", "Rust nightly date to use (eg 2006-01-02). Defaults to the first of the current month.", func(s string) error {
		var err error
		date, err = time.Parse(dateFormat, s)
		if err != nil {
			return fmt.Errorf("failed to parse date %q: %w", s, err)
		}

		return nil
	})

	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  %s %s [OPTIONS]\n\n", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(2)
	}

	err := flags.Parse(args)
	if err != nil {
		flags.Usage()
	}

	if date.IsZero() {
		now := time.Now().UTC()
		date = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	}

	want := date.Format(dateFormat)

	// Find and parse the workspace file to see
	// what version we've got currently.
	data, err := os.ReadFile(workspace)
	if err != nil {
		return fmt.Errorf("Failed to open %s: %v", workspace, err)
	}

	f, err := build.ParseWorkspace(workspace, data)
	if err != nil {
		return fmt.Errorf("Failed to parse %s: %v", workspace, err)
	}

	var currentVersion string
	var resourcePaths []string
	var rhsElement *build.StringExpr
	resourceMap := make(map[string]*build.StringExpr)
	for _, stmt := range f.Stmt {
		if stmt == nil {
			continue
		}

		switch stmt := stmt.(type) {
		case *build.AssignExpr:
			lhs, ok := stmt.LHS.(*build.Ident)
			if !ok || lhs.Name != rustDate {
				continue
			}

			rhsElement, ok = stmt.RHS.(*build.StringExpr)
			if !ok {
				return fmt.Errorf("Failed to parse %s: found %s with non-string value %#v", workspace, rustDate, stmt.RHS)
			}

			currentVersion = rhsElement.Value

			continue
		case *build.CallExpr:
			name, ok := stmt.X.(*build.Ident)
			if !ok || name.Name != toolchains {
				continue
			}

			for _, arg := range stmt.List {
				assign, ok := arg.(*build.AssignExpr)
				if !ok {
					continue
				}

				lhs, ok := assign.LHS.(*build.Ident)
				if !ok || lhs.Name != sha256s {
					continue
				}

				rhs, ok := assign.RHS.(*build.DictExpr)
				if !ok {
					return fmt.Errorf("Failed to parse %s: %s.%s had unexpected type %T", workspace, toolchains, sha256s, assign.RHS)
				}

				for _, entry := range rhs.List {
					key, ok := entry.Key.(*build.BinaryExpr)
					if !ok {
						return fmt.Errorf("Failed to parse %s: %s.%s has bad key %T", workspace, toolchains, sha256s, entry.Key)
					}

					// X is the date variable. Y is the URL path.
					path, ok := key.Y.(*build.StringExpr)
					if !ok {
						return fmt.Errorf("Failed to parse %s: %s.%s has bad key %T", workspace, toolchains, sha256s, key.Y)
					}

					val, ok := entry.Value.(*build.StringExpr)
					if !ok {
						return fmt.Errorf("Failed to parse %s: %s.%s has bad hash %T", workspace, toolchains, sha256s, entry.Value)
					}

					pathFragment := path.Value
					resourcePaths = append(resourcePaths, pathFragment)
					resourceMap[pathFragment] = val
				}
			}
		}
	}

	if len(resourcePaths) < 4 {
		log.Printf("Warning: Only found %d resources to update.", len(resourcePaths))
	}

	currentDate, err := time.Parse(dateFormat, currentVersion)
	if err != nil {
		return fmt.Errorf("Invalid current Rust nightly version %s: %v", currentVersion, err)
	}

	if currentVersion == want {
		fmt.Fprintf(w, "Rust already up-to-date at nightly %s.\n", currentVersion)
		return nil
	}

	// Fetch the data.
	tomlURL := fmt.Sprintf(manifestTemplate, want)
	res, err := http.Get(tomlURL)
	if err != nil {
		return fmt.Errorf("Failed to fetch manifest for %s: %v", want, err)
	}

	if res.StatusCode != http.StatusOK {
		log.Printf("Warning: Server returned HTTP status code %d: %s.", res.StatusCode, res.Status)
	}

	data, err = io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return fmt.Errorf("Failed to fetch manifest for %s: %v", want, err)
	}

	var manifest ReleaseManifest
	err = toml.Unmarshal(data, &manifest)
	if err != nil {
		return fmt.Errorf("Failed to parse manifest for %s: %v", want, err)
	}

	if manifest.Version != manifestVersion {
		log.Printf("Warning: Manifest for %s has version %q, expected %q.", want, manifest.Version, manifestVersion)
	}

	if manifest.Date != want {
		return fmt.Errorf("Manifest for %s has date %q", manifest.Date, want)
	}

	wantURLs := make([]string, 0, len(resourcePaths))
	wantURLMap := make(map[string]string)
	for _, path := range resourcePaths {
		url := fmt.Sprintf(toolTemplate, want, path)
		wantURLs = append(wantURLs, url)
		wantURLMap[url] = path
	}

	foundHashes := make(map[string]string)
	for _, tool := range manifest.Packages {
		for _, release := range tool.Targets {
			if !release.Available {
				continue
			}

			path, ok := wantURLMap[release.URL]
			if !ok {
				continue
			}

			foundHashes[path] = release.Hash
			break
		}
	}

	// Check we've got hashes for all the targets
	// we want.
	for _, path := range resourcePaths {
		hash := foundHashes[path]
		if hash == "" {
			return fmt.Errorf("Manifest for %s had no hash for %s", want, path)
		}

		hashBytes, err := hex.DecodeString(hash)
		if err != nil {
			return fmt.Errorf("Manifest for %s had invalid hash for %s: %v", want, path, err)
		}

		if len(hashBytes) != sha256.Size {
			return fmt.Errorf("Manifest for %s had invalid hash for %s: got %d bytes, want %d", want, path, len(hashBytes), sha256.Size)
		}

		// Update the hash value in the AST.
		resourceMap[path].Value = hash

	}

	rhsElement.Value = want

	// Pretty-print the updated workspace.
	pretty := build.Format(f)
	err = os.WriteFile(workspace, pretty, 0644)
	if err != nil {
		return fmt.Errorf("Failed to write updated %s: %v", workspace, err)
	}

	if currentDate.After(date) {
		fmt.Fprintf(w, "Warning: Downgraded Rust from nightly %s to %s.\n", currentVersion, want)
	} else {
		fmt.Fprintf(w, "Updated Rust from nightly %s to %s.\n", currentVersion, want)
	}

	return nil
}

type ReleaseManifest struct {
	Version  string                   `toml:"manifest-version"`
	Date     string                   `toml:"date"`
	Packages map[string]*ToolMetadata `toml:"pkg"`
}

type ToolMetadata struct {
	Targets map[string]*ToolRelease `toml:"target"`
}

type ToolRelease struct {
	Available bool   `toml:"available"`
	URL       string `toml:"url"`
	Hash      string `toml:"hash"`
}
