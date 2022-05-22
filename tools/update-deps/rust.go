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
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/bazelbuild/buildtools/build"
)

func init() {
	RegisterCommand("rust", "Update the Rust nightly version and its tooling.", cmdRust)
}

// RustToolData contains the data representing
// a Rust tool's data specified in a struct in
// //bazel/deps/rust.bzl.
//
type RustToolData struct {
	Name StringField `bzl:"name"`
	Sum  StringField `bzl:"sum"`
}

// Parse a rust.bzl, returning the Rust Nightly
// release date and the set of tools, plus the
// *build.File containing the Starlark file's AST.
//
func ParseRustBzl(name string) (file *build.File, date *StringField, tools []*RustToolData, err error) {
	const (
		rustDate = "RUST_ISO_DATE"
	)

	var allTools = []string{
		"LLVM_TOOLS",
		"RUST",
		"RUST_SRC",
		"RUST_STD",
		"RUST_RUSTFMT",
		"RUST_NO_STD",
	}

	data, err := os.ReadFile(name)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to open %s: %v", name, err)
	}

	f, err := build.ParseBzl(name, data)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to parse %s: %v", name, err)
	}

	toolsMap := make(map[string]bool)
	for _, stmt := range f.Stmt {
		assign, ok := stmt.(*build.AssignExpr)
		if !ok {
			continue
		}

		lhs, ok := assign.LHS.(*build.Ident)
		if !ok {
			continue
		}

		if lhs.Name == rustDate {
			rhs, ok := assign.RHS.(*build.StringExpr)
			if !ok {
				return nil, nil, nil, fmt.Errorf("Failed to parse %s: %s has non-string value %#v", name, lhs.Name, assign.RHS)
			}

			if date != nil {
				return nil, nil, nil, fmt.Errorf("Failed to parse %s: %s assigned for the second time", name, lhs.Name)
			}

			date = &StringField{
				Value: rhs.Value,
				Ptr:   &rhs.Value,
			}

			continue
		}

		// Find which tool this is.
		for _, tool := range allTools {
			if lhs.Name != tool {
				continue
			}

			call, ok := assign.RHS.(*build.CallExpr)
			if !ok {
				return nil, nil, nil, fmt.Errorf("Failed to parse %s: found %s with non-call value %#v", name, tool, assign.RHS)
			}

			if fun, ok := call.X.(*build.Ident); !ok || fun.Name != "struct" {
				return nil, nil, nil, fmt.Errorf("Failed to parse %s: found %s with non-struct value %#v", name, tool, call.X)
			}

			var data RustToolData
			err = UnmarshalFields(call, &data)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("Failed to parse %s: invalid data for %s: %v", name, tool, err)
			}

			if toolsMap[tool] {
				return nil, nil, nil, fmt.Errorf("Failed to parse %s: %s assigned for the second time", name, tool)
			}

			toolsMap[tool] = true
			tools = append(tools, &data)
		}
	}

	if date == nil {
		return nil, nil, nil, fmt.Errorf("Failed to parse %s: no data found for %s", name, rustDate)
	}

	for _, tool := range allTools {
		if !toolsMap[tool] {
			return nil, nil, nil, fmt.Errorf("Failed to parse %s: no data found for %s", name, tool)
		}
	}

	return f, date, tools, nil
}

func cmdRust(ctx context.Context, w io.Writer, args []string) error {
	const (
		dateFormat       = "2006-01-02"
		rustBzl          = "rust.bzl"
		manifestTemplate = "https://static.rust-lang.org/dist/%s/channel-rust-nightly.toml"
		toolTemplate     = "https://static.rust-lang.org/dist/%s/%s.tar.gz"
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

	defaultDate := false
	if date.IsZero() {
		defaultDate = true
		now := time.Now().UTC()
		date = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	}

	want := date.Format(dateFormat)

	// Find and parse the rust.bzl file to see
	// what version we've got currently.
	bzlPath := filepath.Join("bazel", "deps", rustBzl)
	f, dateField, tools, err := ParseRustBzl(bzlPath)
	if err != nil {
		return err
	}

	currentDate, err := time.Parse(dateFormat, dateField.Value)
	if err != nil {
		return fmt.Errorf("Invalid current Rust nightly version %s: %v", dateField.Value, err)
	}

	if dateField.Value == want {
		fmt.Fprintf(w, "Rust already up-to-date at nightly %s.\n", dateField.Value)
		return nil
	}

	if defaultDate && currentDate.After(date) {
		fmt.Fprintf(w, "Rust already up-to-date at nightly %s.\n", dateField.Value)
		return nil
	}

	// Fetch the data.
	tomlURL := fmt.Sprintf(manifestTemplate, want)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tomlURL, nil)
	if err != nil {
		return fmt.Errorf("Failed to request manifest for %s: %v", want, err)
	}

	res, err := httpRequest(req)
	if err != nil {
		return fmt.Errorf("Failed to fetch manifest for %s: %v", want, err)
	}

	if res.StatusCode != http.StatusOK {
		log.Printf("Warning: Server returned HTTP status code %d: %s.", res.StatusCode, res.Status)
	}

	data, err := io.ReadAll(res.Body)
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

	wantURLs := make([]string, 0, len(tools))
	wantURLMap := make(map[string]string)
	for _, tool := range tools {
		url := fmt.Sprintf(toolTemplate, want, tool.Name.Value)
		wantURLs = append(wantURLs, url)
		wantURLMap[url] = tool.Name.Value
	}

	foundHashes := make(map[string]string)
	for _, tool := range manifest.Packages {
		for _, release := range tool.Targets {
			if !release.Available {
				continue
			}

			name, ok := wantURLMap[release.URL]
			if !ok {
				continue
			}

			foundHashes[name] = release.Hash
		}
	}

	// Check we've got hashes for all the targets
	// we want.
	for _, tool := range tools {
		hash := foundHashes[tool.Name.Value]
		if hash == "" {
			return fmt.Errorf("Manifest for %s had no hash for %s", want, tool.Name.Value)
		}

		hashBytes, err := hex.DecodeString(hash)
		if err != nil {
			return fmt.Errorf("Manifest for %s had invalid hash for %s: %v", want, tool.Name.Value, err)
		}

		if len(hashBytes) != sha256.Size {
			return fmt.Errorf("Manifest for %s had invalid hash for %s: got %d bytes, want %d", want, tool.Name.Value, len(hashBytes), sha256.Size)
		}

		// Update the hash value in the AST.
		*tool.Sum.Ptr = hash
	}

	*dateField.Ptr = want

	// Pretty-print the updated workspace.
	pretty := build.Format(f)
	err = os.WriteFile(bzlPath, pretty, 0644)
	if err != nil {
		return fmt.Errorf("Failed to write updated %s: %v", bzlPath, err)
	}

	if currentDate.After(date) {
		fmt.Fprintf(w, "Warning: Downgraded Rust from nightly %s to %s.\n", dateField.Value, want)
	} else {
		fmt.Fprintf(w, "Updated Rust from nightly %s to %s.\n", dateField.Value, want)
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
