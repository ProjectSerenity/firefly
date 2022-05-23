// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
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

	"golang.org/x/mod/semver"
)

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

const cratesIO = "https://crates.io/api/v1/"

// FetchRustCrate downloads a Rust crate using the
// crates.io API.
//
func FetchRustCrate(ctx context.Context, crate *RustCrate, dir string) error {
	u, err := url.Parse(cratesIO)
	if err != nil {
		return fmt.Errorf("failed to parse crates.io API URL %q: %v", cratesIO, err)
	}

	u.Path = path.Join("/", u.Path, "crates", crate.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to prepare API request for Rust crate %q: %v", crate.Name, err)
	}

	res, err := httpRequest(req)
	if err != nil {
		return fmt.Errorf("failed to make API request for Rust crate %q: %v", crate.Name, err)
	}

	if res.StatusCode != http.StatusOK {
		log.Printf("WARN: Unexpected status code for API request for Rust crate %q: %v (%s)", crate.Name, res.StatusCode, res.Status)
	}

	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to read API response for Rust crate %q: %v", crate.Name, err)
	}

	var data Crate
	err = json.Unmarshal(body, &data)
	if err != nil {
		return fmt.Errorf("failed to parse API response for Rust crate %q: %v", crate.Name, err)
	}

	var downloadPath string
	for _, version := range data.Versions {
		if version.Number == crate.Version {
			downloadPath = version.DownloadPath
			break
		}
	}

	if downloadPath == "" {
		return fmt.Errorf("failed to find download path for Rust crate %s %s", crate.Name, crate.Version)
	}

	// Delete any old version that remains.
	err = os.RemoveAll(dir)
	if err != nil {
		return fmt.Errorf("failed to remove old data for Rust crate %s: %v", crate.Name, err)
	}

	// Download the crate to dir.
	u.Path = downloadPath
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to prepare API request for Rust crate %q: %v", crate.Name, err)
	}

	res, err = httpRequest(req)
	if err != nil {
		return fmt.Errorf("failed to make API request for Rust crate %q: %v", crate.Name, err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		log.Printf("WARN: Unexpected status code for API request for Rust crate %q: %v (%s)", crate.Name, res.StatusCode, res.Status)
	}

	// The response body is a Gzipped tarball containing
	// the crate data.
	gr, err := gzip.NewReader(res.Body)
	if err != nil {
		return fmt.Errorf("failed to read contents for Rust crate %q: %v", crate.Name, err)
	}

	tr := tar.NewReader(gr)
	created := make(map[string]bool)
	prefix := fmt.Sprintf("%s-%s/", crate.Name, crate.Version)
	for {
		file, err := tr.Next()
		if err == io.EOF {
			break
		}

		switch file.Typeflag {
		case tar.TypeReg:
		case tar.TypeDir:
			continue
		default:
			log.Printf("WARN: Ignoring %q in Rust crate %s: unexpected file type %q", file.Name, crate.Name, file.Typeflag)
			continue
		}

		filename := strings.TrimPrefix(file.Name, prefix)

		parent := filepath.Join(dir, filepath.FromSlash(path.Dir(filename)))
		if !created[parent] {
			created[parent] = true
			err = os.MkdirAll(parent, 0777)
			if err != nil {
				return fmt.Errorf("failed to store Rust crate %q: failed to create directory %q: %v", crate.Name, parent, err)
			}
		}

		name := filepath.Join(parent, path.Base(filename))
		dst, err := os.Create(name)
		if err != nil {
			return fmt.Errorf("failed to store Rust crate %q: failed to create %q: %v", crate.Name, name, err)
		}

		_, err = io.Copy(dst, tr)
		if err != nil {
			dst.Close()
			return fmt.Errorf("failed to store Rust crate %q: failed to write %q: %v", crate.Name, name, err)
		}

		if err = dst.Close(); err != nil {
			return fmt.Errorf("failed to store Rust crate %q: failed to close %q: %v", crate.Name, name, err)
		}
	}

	if err = res.Body.Close(); err != nil {
		return fmt.Errorf("failed to read contents for Rust crate %q: %v", crate.Name, err)
	}

	return nil
}

// UpdateRustCrate checks a Rust crate for updates,
// using the crates.io API.
//
func UpdateRustCrate(ctx context.Context, crate *UpdateDep) (updated bool, err error) {
	u, err := url.Parse(cratesIO)
	if err != nil {
		return false, fmt.Errorf("failed to parse crates.io API URL %q: %v", cratesIO, err)
	}

	u.Path = path.Join("/", u.Path, "crates", crate.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return false, fmt.Errorf("failed to prepare API request for Rust crate %q: %v", crate.Name, err)
	}

	res, err := httpRequest(req)
	if err != nil {
		return false, fmt.Errorf("failed to make API request for Rust crate %q: %v", crate.Name, err)
	}

	if res.StatusCode != http.StatusOK {
		log.Printf("WARN: Unexpected status code for API request for Rust crate %q: %v (%s)", crate.Name, res.StatusCode, res.Status)
	}

	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return false, fmt.Errorf("failed to read API response for Rust crate %q: %v", crate.Name, err)
	}

	var data Crate
	err = json.Unmarshal(body, &data)
	if err != nil {
		return false, fmt.Errorf("failed to parse API response for Rust crate %q: %v", crate.Name, err)
	}

	if data.Crate.Name != crate.Name {
		return false, fmt.Errorf("query to crates.io for Rust crate %s returned data for %q", crate.Name, data.Crate.Name)
	}

	current := semver.Canonical("v" + *crate.Version)
	if current == "" {
		return false, fmt.Errorf("Rust crate %s has an invalid version %q", crate.Name, *crate.Version)
	}

	yanked := false
	ignoredMajor := false
	for _, version := range data.Versions {
		// Check that the version is canonical.
		// If not, we log it and continue. We
		// could return an error, but that feels
		// likely to be more annoying than helpful.
		next := semver.Canonical("v" + version.Number)
		if next == "" {
			log.Printf("WARN: Rust crate %s returned invalid version %q", crate.Name, version.Number)
			continue
		}

		cmp := semver.Compare(current, next)
		if cmp == 0 {
			if version.Yanked {
				// We keep going to find which version
				// we should downgrade to.
				yanked = true
				continue
			}

			// We're already up to date.
			return false, nil
		}

		// Ignore yanked versions, other than the
		// current version. If that is yanked, we
		// need to downgrade (see above).
		if version.Yanked {
			continue
		}

		// We often won't want to update to a
		// higher major version if we're already
		// on a stable version (one with a major
		// version larger than zero). However,
		// in case we do, we log the largest
		// version we see with a larger major
		// version.
		if MajorUpdate(current, next) {
			if !ignoredMajor {
				ignoredMajor = true
				fmt.Printf("Ignored Rust crate %s major update from %s to %s.\n", crate.Name, *crate.Version, version.Number)
			}

			continue
		}

		// If we see an older version and the
		// current version has been yanked, we
		// need to downgrade.
		if cmp == +1 {
			if yanked {
				fmt.Printf("Downgraded Rust crate %s from %s (which has been yanked) to %s.\n", crate.Name, *crate.Version, version.Number)
				*crate.Version = version.Number
				return true, nil
			}

			return false, fmt.Errorf("Rust crate %s returned no data for the current version %s", crate.Name, *crate.Version)
		}

		fmt.Printf("Upgraded Rust crate %s from %s to %s.\n", crate.Name, *crate.Version, version.Number)
		*crate.Version = version.Number
		return true, nil
	}

	if yanked {
		return false, fmt.Errorf("Rust crate %s returned no viable versions (current version %s is yanked)", crate.Name, *crate.Version)
	}

	return false, fmt.Errorf("Rust crate %s returned no viable versions (current version %s missing)", crate.Name, *crate.Version)
}
