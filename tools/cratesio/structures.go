// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package cratesio

import (
	"time"
)

// Crate contains the metadata for a Rust Crate, as provided
// by the crates.io API.
type Crate struct {
	Categories []*CrateCategory `json:"categories"`
	Crate      CrateData        `json:"crate"`
	Keywords   []*CrateKeyword  `json:"keywords"`
	Versions   []*CrateVersion  `json:"versions"`
}

// CrateCategory includes information about a category of Rust
// crates, as provided by the crates.io API.
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
type CrateKeyword struct {
	Id          string    `json:"id"`
	Keyword     string    `json:"keyword"`
	CratesCount uint64    `json:"crates_cnt"`
	CreatedAt   time.Time `json:"created_at"`
}

// CrateVersion includes information about a published
// version of a Rust crate, as provided by the crates.io
// API.
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
type CrateVersionLinks struct {
	Dependencies     string `json:"dependencies"`
	VersionDownloads string `json:"version_downloads"`
}

// CrateUser includes the metadata about a user of crates.io,
// as provided by the API.
type CrateUser struct {
	Avatar string `json:"avatar,omitempty"`
	Email  string `json:"email,omitempty"`
	Id     uint64 `json:"id"`
	Kind   string `json:"kind,omitempty"`
	Login  string `json:"login"`
	Name   string `json:"name,omitempty"`
	URL    string `json:"url"`
}
