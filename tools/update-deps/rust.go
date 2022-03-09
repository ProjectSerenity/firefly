// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/bazelbuild/buildtools/build"
	"golang.org/x/mod/semver"
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

// RustCrateData contains the data representing
// a Rust crate as specified in a struct in
// //bazel/deps/rust.bzl.
//
type RustCrateData struct {
	Name    string
	Version StringField `bzl:"semver"`
}

// Parse a rust.bzl, returning the Rust Nightly
// release date and the set of tools, plus the
// *build.File containing the Starlark file's AST.
//
func ParseRustBzl(name string) (file *build.File, date *StringField, tools []*RustToolData, crates []*RustCrateData, err error) {
	const (
		rustDate   = "RUST_ISO_DATE"
		rustCrates = "RUST_CRATES"
	)

	var allTools = []string{
		"LLVM_TOOLS",
		"RUST",
		"RUST_SRC",
		"RUST_STD",
		"RUST_RUSTFMT",
	}

	data, err := os.ReadFile(name)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("Failed to open %s: %v", name, err)
	}

	f, err := build.ParseBzl(name, data)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("Failed to parse %s: %v", name, err)
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
				return nil, nil, nil, nil, fmt.Errorf("Failed to parse %s: %s has non-string value %#v", name, lhs.Name, assign.RHS)
			}

			if date != nil {
				return nil, nil, nil, nil, fmt.Errorf("Failed to parse %s: %s assigned for the second time", name, lhs.Name)
			}

			date = &StringField{
				Value: rhs.Value,
				Ptr:   &rhs.Value,
			}

			continue
		}

		if lhs.Name == rustCrates {
			rhs, ok := assign.RHS.(*build.DictExpr)
			if !ok {
				return nil, nil, nil, nil, fmt.Errorf("Failed to parse %s: %s has non-list value %#v", name, lhs.Name, assign.RHS)
			}

			if crates != nil {
				return nil, nil, nil, nil, fmt.Errorf("Failed to parse %s: %s is assigned for the second time", name, lhs.Name)
			}

			for _, expr := range rhs.List {
				crateName, ok := expr.Key.(*build.StringExpr)
				if !ok {
					return nil, nil, nil, nil, fmt.Errorf("Failed to parse %s: found non-string %#v, expected crate name", name, expr.Key)
				}

				crate := crateName.Value

				call, ok := expr.Value.(*build.CallExpr)
				if !ok {
					return nil, nil, nil, nil, fmt.Errorf("Failed to parse %s: %s has non-call crate value %#v", name, lhs.Name, expr.Value)
				}

				lhs, ok := call.X.(*build.DotExpr)
				if !ok {
					return nil, nil, nil, nil, fmt.Errorf("Failed to parse %s: found crate with non-crate.spec value %#v", name, call.X)
				}

				if fun, ok := lhs.X.(*build.Ident); !ok || fun.Name != "crate" || lhs.Name != "spec" {
					err = fmt.Errorf("Failed to parse %s: found crate with non-crate.spec value %#v.%s", name, lhs.X, lhs.Name)
					return nil, nil, nil, nil, err
				}

				var version string
				var versionPtr *string
				for i, expr := range call.List {
					assign, ok := expr.(*build.AssignExpr)
					if !ok {
						err = fmt.Errorf("Failed to parse %s: bad crate %q: field %d is not an assignment", name, crate, i)
						return nil, nil, nil, nil, err
					}

					lhs, ok := assign.LHS.(*build.Ident)
					if !ok {
						err = fmt.Errorf("Failed to parse %s: bad crate %q: field %d assigns to a non-identifier value %#v", name, crate, i, assign.LHS)
						return nil, nil, nil, nil, err
					}

					if lhs.Name != "version" {
						continue
					}

					rhs, ok := assign.RHS.(*build.StringExpr)
					if !ok {
						err = fmt.Errorf("Failed to parse %s: bad crate %q: %q has non-string value %#v", name, crate, lhs.Name, assign.RHS)
						return nil, nil, nil, nil, err
					}

					version = rhs.Value
					versionPtr = &rhs.Value
					break
				}

				if versionPtr == nil {
					return nil, nil, nil, nil, fmt.Errorf("Failed to parse %s: bad crate %q: no version found", name, crate)
				}

				data := RustCrateData{
					Name: crate,
					Version: StringField{
						Value: version,
						Ptr:   versionPtr,
					},
				}

				crates = append(crates, &data)
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
				return nil, nil, nil, nil, fmt.Errorf("Failed to parse %s: found %s with non-call value %#v", name, tool, assign.RHS)
			}

			if fun, ok := call.X.(*build.Ident); !ok || fun.Name != "struct" {
				return nil, nil, nil, nil, fmt.Errorf("Failed to parse %s: found %s with non-struct value %#v", name, tool, call.X)
			}

			var data RustToolData
			err = UnmarshalFields(call, &data)
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("Failed to parse %s: invalid data for %s: %v", name, tool, err)
			}

			if toolsMap[tool] {
				return nil, nil, nil, nil, fmt.Errorf("Failed to parse %s: %s assigned for the second time", name, tool)
			}

			toolsMap[tool] = true
			tools = append(tools, &data)
		}
	}

	if date == nil {
		return nil, nil, nil, nil, fmt.Errorf("Failed to parse %s: no data found for %s", name, rustDate)
	}

	for _, tool := range allTools {
		if !toolsMap[tool] {
			return nil, nil, nil, nil, fmt.Errorf("Failed to parse %s: no data found for %s", name, tool)
		}
	}

	return f, date, tools, crates, nil
}

// Crate contains the metadata for a Rust Crate, as provided
// by the crates.io API.
//
type Crate struct {
	Categories []*CrateCategory `json:"categories"`
	Crate      CrateData        `json:"crate"`
	Keywords   []*CrateKeyword  `json:"keywords"`
	Versions   []*CrateVersion  `json:"versions"`
}

// CrateCategory includes information about a category of Rust
// crates, as provided by the crates.io API.
//
type CrateCategory struct {
	Category    string    `json:"category"`
	CratesCount uint64    `json:"crates_cnt"`
	CreatedAt   time.Time `json:"created_at"`
	Description string    `json:"description"`
	Id          string    `json:"id"`
	Slug        string    `json:"slug"`
}

// CrateData includes the metadata about a Rust crate, as
// provided by the crates.io API.
//
type CrateData struct {
	Id              string     `json:"id"`
	Name            string     `json:"name"`
	Description     string     `json:"description,omitempty"`
	License         string     `json:"license,omitempty"`
	Documentation   string     `json:"documentation,omitempty"`
	Homepage        string     `json:"homepage,omitempty"`
	Repository      string     `json:"repository,omitempty"`
	Downloads       uint64     `json:"downloads"`
	RecentDownloads uint64     `json:"recent_downloads,omitempty"`
	Categories      []string   `json:"categories,omitempty"`
	Keywords        []string   `json:"keywords,omitempty"`
	Versions        []uint64   `json:"versions,omitempty"`
	MaxVersion      string     `json:"max_version"`
	Links           CrateLinks `json:"links"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	ExactMatch      bool       `json:"exact_match"`
}

// CrateLinks includes the standard set of hyperlinks for
// a Rust crate, as provided by the crates.io API.
//
type CrateLinks struct {
	OwnerTeam           string   `json:"owner_team"`
	OwnerUser           string   `json:"owner_user"`
	Owners              string   `json:"owners"`
	ReverseDependencies string   `json:"reverse_dependencies"`
	VersionDownloads    string   `json:"version_downloads"`
	Versions            []string `json:"versions,omitempty"`
}

// CrateKeyword includes information about a keyword that
// describes a set of Rust crates, as provided by the
// crates.io API.
//
type CrateKeyword struct {
	Id          string    `json:"id"`
	Keyword     string    `json:"keyword"`
	CratesCount uint64    `json:"crates_cnt"`
	CreatedAt   time.Time `json:"created_at"`
}

// CrateVersion includes information about a published
// version of a Rust crate, as provided by the crates.io
// API.
//
type CrateVersion struct {
	Crate        string              `json:"crate"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
	DownloadPath string              `json:"dl_path"`
	Downloads    uint64              `json:"downloads"`
	Features     map[string][]string `json:"features"`
	Id           uint64              `json:"id"`
	Number       string              `json:"num"`
	Yanked       bool                `json:"yanked"`
	License      string              `json:"license,omitempty"`
	ReadmePath   string              `json:"readme_path,omitempty"`
	Links        CrateVersionLinks   `json:"links"`
	CrateSize    uint64              `json:"crate_size,omitempty"`
	PublishedBy  *CrateUser          `json:"published_by,omitempty"`
}

// CrateVersionLinks includes the standard set of hyperlinks
// for a published version of a Rust crate, as provided by
// the crates.io API.
//
type CrateVersionLinks struct {
	Dependencies     string `json:"dependencies"`
	VersionDownloads string `json:"version_downloads"`
}

// CrateUser includes the metadata about a user of crates.io,
// as provided by the API.
//
type CrateUser struct {
	Avatar string `json:"avatar,omitempty"`
	Email  string `json:"email,omitempty"`
	Id     uint64 `json:"id"`
	Kind   string `json:"kind,omitempty"`
	Login  string `json:"login"`
	Name   string `json:"name,omitempty"`
	URL    string `json:"url"`
}

// FetchCrate returns the metadata about a Rust crate using
// the crates.io API at the given base address.
//
func FetchCrate(ctx context.Context, api, crate string) (*Crate, error) {
	u, err := url.Parse(api)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse crates.io API URL %q: %v", api, err)
	}

	u.Path = path.Join("/", u.Path, "crates", crate)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to prepare API request for crate %q: %v", crate, err)
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to make API request for crate %q: %v", crate, err)
	}

	if res.StatusCode != http.StatusOK {
		log.Printf("warn: unexpected status code for API request for crate %q: %v (%s)", crate, res.StatusCode, res.Status)
	}

	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("Failed to read API response for crate %q: %v", crate, err)
	}

	var data Crate
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse API response for crate %q: %v", crate, err)
	}

	return &data, nil
}

func cmdRust(ctx context.Context, w io.Writer, args []string) error {
	const (
		dateFormat       = "2006-01-02"
		rustBzl          = "rust.bzl"
		cratesIO         = "https://crates.io/api/v1/"
		manifestTemplate = "https://static.rust-lang.org/dist/%s/channel-rust-nightly.toml"
		toolTemplate     = "https://static.rust-lang.org/dist/%s/%s.tar.gz"
		manifestVersion  = "2"
	)

	var (
		date       time.Time
		skipCrates bool
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
	flags.BoolVar(&skipCrates, "skip-crates", false, "Skip checking for crate updates")

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
	f, dateField, tools, crates, err := ParseRustBzl(bzlPath)
	if err != nil {
		return err
	}

	// Start by checking for updates for the crates.
	if !skipCrates {
	crates:
		for _, crate := range crates {
			name := crate.Name
			current := crate.Version.Value
			if !strings.HasPrefix(current, "=") {
				return fmt.Errorf("Rust crate %q is specified with non-exact version %q", name, current)
			}

			current = current[1:]
			currentSemver := semver.Canonical("v" + current)
			if current == "" {
				return fmt.Errorf("Rust crate %q is specified with invalid version %q", name, current)
			}

			data, err := FetchCrate(ctx, cratesIO, name)
			if err != nil {
				return err
			}

			if data.Crate.Name != name {
				return fmt.Errorf("Query for Rust crate %q returned data for %q", name, data.Crate.Name)
			}

			yanked := false
			for _, version := range data.Versions {
				// Check that the version is canonical.
				// If not, we log it and continue. We
				// could return an error, but that feels
				// likely to be more annoying than helpful.
				thisSemver := semver.Canonical("v" + version.Number)
				if thisSemver == "" {
					fmt.Fprintf(w, "Warning: Rust crate %q returned invalid version %q\n", name, version.Number)
					continue
				}

				// Compare this with the current version.
				cmp := semver.Compare(currentSemver, thisSemver)

				// If we see the current version, we're
				// either already up to date or our version
				// has been yanked and we need to downgrade.
				if cmp == 0 {
					if version.Yanked {
						yanked = true
						continue
					}

					// This crate is already up to date.
					continue crates
				}

				// We can just ignore a yanked version
				// that isn't the version we're already
				// using.
				if version.Yanked {
					continue
				}

				// If we see an older version and our
				// version has been yanked, we must
				// downgrade.
				if cmp == +1 {
					if !yanked {
						return fmt.Errorf("Rust crate %q is missing version data for current version %q", name, current)
					}

					if version.Number != data.Crate.MaxVersion {
						fmt.Fprintf(w, "Warning: Rust crate %q using %q, but %q is latest.\n", name, version.Number, data.Crate.MaxVersion)
					}

					*crate.Version.Ptr = "=" + version.Number
					fmt.Fprintf(w, "Warning: Downgraded Rust crate %s from %s to %s.\n", name, current, version.Number)
					continue crates
				}

				// We've found a newer version than
				// our current, so we upgrade.
				if version.Number != data.Crate.MaxVersion {
					fmt.Fprintf(w, "Warning: Rust crate %q using %q, but %q is latest.\n", name, version.Number, data.Crate.MaxVersion)
				}

				*crate.Version.Ptr = "=" + version.Number
				fmt.Fprintf(w, "Updated Rust crate %s from %s to %s.\n", name, current, version.Number)
				continue crates
			}
		}
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
	res, err := httpClient.Get(tomlURL)
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
			break
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
