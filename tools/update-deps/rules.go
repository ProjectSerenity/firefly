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
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/buildtools/build"
	"golang.org/x/mod/semver"

	"firefly-os.dev/tools/simplehttp"
)

func init() {
	RegisterCommand("rules", "Update the Bazel rules used.", cmdRules)
}

// BazelRuleData contains the data representing
// a Bazel rule's data specified in a struct in
// //bazel/deps/rules.bzl.
//
type BazelRuleData struct {
	Name    StringField `bzl:"name"`
	Repo    StringField `bzl:"repo"`
	Branch  StringField `bzl:"branch,optional"`
	Archive StringField `bzl:"archive"`
	Version StringField `bzl:"version"`
	SHA256  StringField `bzl:"sha256"`
}

// Parse a rules.bzl, returning the set of imported
// Bazel rules and the *build.File containing the
// Starlark file's AST.
//
func ParseRulesBzl(name string) (file *build.File, rules []*BazelRuleData, err error) {
	var allRules = []string{
		"RULES_BUILDTOOLS",
		"RULES_CC",
		"RULES_GO",
		"RULES_LICENSE",
		"RULES_PKG",
		"RULES_PROTOBUF",
		"RULES_RUST",
		"RULES_SKYLIB",
	}

	data, err := os.ReadFile(name)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to open %s: %v", name, err)
	}

	f, err := build.ParseBzl(name, data)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to parse %s: %v", name, err)
	}

	rulesMap := make(map[string]bool)
	for _, stmt := range f.Stmt {
		assign, ok := stmt.(*build.AssignExpr)
		if !ok {
			continue
		}

		lhs, ok := assign.LHS.(*build.Ident)
		if !ok {
			continue
		}

		// Find which rule this is.
		for _, rule := range allRules {
			if lhs.Name != rule {
				continue
			}

			call, ok := assign.RHS.(*build.CallExpr)
			if !ok {
				return nil, nil, fmt.Errorf("Failed to parse %s: found %s with non-call value %#v", name, rule, assign.RHS)
			}

			if fun, ok := call.X.(*build.Ident); !ok || fun.Name != "struct" {
				return nil, nil, fmt.Errorf("Failed to parse %s: found %s with non-struct value %#v", name, rule, call.X)
			}

			var data BazelRuleData
			err = UnmarshalFields(call, &data)
			if err != nil {
				return nil, nil, fmt.Errorf("Failed to parse %s: invalid data for %s: %v", name, rule, err)
			}

			if rulesMap[rule] {
				return nil, nil, fmt.Errorf("Failed to parse %s: %s assigned for the second time", name, rule)
			}

			rulesMap[rule] = true
			rules = append(rules, &data)
		}
	}

	for _, rule := range allRules {
		if !rulesMap[rule] {
			return nil, nil, fmt.Errorf("Failed to parse %s: no data found for %s", name, rule)
		}
	}

	return f, rules, nil
}

// githubAPI calls the requested GitHub API, decoding
// the response into dst.
//
func githubAPI(v any, baseAPI string, args ...string) error {
	u, err := url.Parse(baseAPI)
	if err != nil {
		return fmt.Errorf("invalid GitHub API URL: %w", err)
	}

	u.Path = path.Join(args...)
	uri := u.String()
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return fmt.Errorf("Failed to request API %v", err)
	}

	res, err := simplehttp.Request(req)
	if err != nil {
		return fmt.Errorf("failed to call API: %v", err)
	}

	jsonData, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("failed to read API data: %v", err)
	}

	if err = res.Body.Close(); err != nil {
		return fmt.Errorf("failed to close API data: %v", err)
	}

	err = json.Unmarshal(jsonData, v)
	if err != nil {
		return fmt.Errorf("failed to parse API data: %v", err)
	}

	return nil
}

// LatestGitRelease identifies the latest release of
// the given rule, returning the version string.
//
func LatestGitRelease(baseAPI, repository string) (version string, err error) {
	type ReleaseData struct {
		TagName string `json:"tag_name"`
		// We don't care about the other fields.
	}

	var release ReleaseData
	err = githubAPI(&release, baseAPI, "repos", repository, "releases", "latest")
	if err != nil {
		return "", fmt.Errorf("Failed to check %s for latest release: %v", repository, err)
	}

	version = strings.TrimPrefix(release.TagName, "v")
	if version == "" {
		return "", fmt.Errorf("Failed to check %s for latest release: failed to find latest version", repository)
	}

	return version, nil
}

// LatestGitCommit identifies the latest commit hash
// on the given branch.
//
func LatestGitCommit(baseAPI, repository, branch string) (commit string, err error) {
	type BranchData struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
		// We don't care about the other fields.
	}

	var data BranchData
	err = githubAPI(&data, baseAPI, "repos", repository, "branches", branch)
	if err != nil {
		return "", fmt.Errorf("Failed to check %s@%s for latest commit: %v", repository, branch, err)
	}

	commit = data.Commit.SHA
	if commit == "" {
		return "", fmt.Errorf("Failed to check %s@%s for latest commit: failed to find latest commit", repository, branch)
	}

	return commit, nil
}

// UpdateRepo checks a Git repository (fetched using http_archive)
// for updates, returning the new version and its checksum if it's
// updated.
//
func UpdateRepo(data *BazelRuleData) (newVersion, checksum string, err error) {
	const githubAPI = "https://api.github.com"

	// Handle repos that don't use releases separately.
	if data.Branch.Value != "" {
		commit, err := LatestGitCommit(githubAPI, data.Repo.Value, data.Branch.Value)
		if err != nil {
			return "", "", err
		}

		if commit == data.Version.Value {
			// We're already on the latest commit.
			return "", "", nil
		}

		newVersion = commit
	} else {
		version, err := LatestGitRelease(githubAPI, data.Repo.Value)
		if err != nil {
			return "", "", err
		}

		// Check whether it's newer than the version
		// we're already using.
		current := "v" + data.Version.Value
		latest := "v" + version
		if !semver.IsValid(current) {
			return "", "", fmt.Errorf("Failed to check %s for update: current version %q is invalid", data.Name, data.Version)
		}

		if !semver.IsValid(latest) {
			return "", "", fmt.Errorf("Failed to check %s for update: latest version %q is invalid", data.Name, version)
		}

		switch semver.Compare(current, latest) {
		case 0:
			// Current is latest.
			return "", "", nil
		case -1:
			//  Update to do.
			newVersion = version
		case +1:
			log.Printf("Warning: %s has current version %s, newer than latest version %s", data.Name, data.Version, version)
			return "", "", nil
		}
	}

	// Calculate the new checksum.
	archiveURL := strings.ReplaceAll(data.Archive.Value, "{v}", newVersion)

	hash := sha256.New()
	req, err := http.NewRequest(http.MethodGet, archiveURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("Failed to request archive for %s: %v", data.Name, err)
	}

	res, err := simplehttp.Request(req)
	if err != nil {
		return "", "", fmt.Errorf("Failed to update %s: fetching archive: %v", data.Name, err)
	}

	_, err = io.Copy(hash, res.Body)
	if err != nil {
		return "", "", fmt.Errorf("Failed to update %s: hashing archive: %v", data.Name, err)
	}

	if err = res.Body.Close(); err != nil {
		return "", "", fmt.Errorf("Failed to update %s: closing archive: %v", data.Name, err)
	}

	checksum = hex.EncodeToString(hash.Sum(nil))

	return newVersion, checksum, nil
}

func cmdRules(ctx context.Context, w io.Writer, args []string) error {
	const (
		rulesBzl = "rules.bzl"
	)

	// Find and parse the rules.bzl file to see
	// what versions we've got currently.
	bzlPath := filepath.Join("bazel", "deps", rulesBzl)
	f, rulesData, err := ParseRulesBzl(bzlPath)
	if err != nil {
		return err
	}

	updated := make([]string, 0, len(rulesData))
	for _, data := range rulesData {
		newVersion, checksum, err := UpdateRepo(data)
		if err != nil {
			return err
		}

		if newVersion == "" {
			// Nothing to do.
			continue
		}

		*data.Version.Ptr = newVersion
		*data.SHA256.Ptr = checksum
		updated = append(updated, fmt.Sprintf("%s from %s to %s", data.Name, data.Version, newVersion))
	}

	if len(updated) == 0 {
		fmt.Fprintln(w, "All Bazel rules are up to date.")
		return nil
	}

	// Pretty-print the updated workspace.
	pretty := build.Format(f)
	err = os.WriteFile(bzlPath, pretty, 0644)
	if err != nil {
		return fmt.Errorf("Failed to write updated %s: %v", bzlPath, err)
	}

	if len(updated) == 0 {
		fmt.Fprintf(w, "Updated %s.\n", updated[0])
	} else {
		fmt.Fprintf(w, "Updated:\n  %s\n", strings.Join(updated, "\n  "))
	}

	return nil
}
