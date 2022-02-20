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
	"os"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/buildtools/build"
	"golang.org/x/mod/semver"
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
		"RULES_GAZELLE",
		"RULES_GO",
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

func cmdRules(ctx context.Context, w io.Writer, args []string) error {
	const (
		rulesBzl       = "rules.bzl"
		updateTemplate = "https://api.github.com/repos/%s/releases/latest"
	)

	// Find and parse the rules.bzl file to see
	// what versions we've got currently.
	bzlPath := filepath.Join("bazel", "deps", rulesBzl)
	f, rulesData, err := ParseRulesBzl(bzlPath)
	if err != nil {
		return err
	}

	type ReleaseData struct {
		TagName string `json:"tag_name"`
		// We don't care about the other fields.
	}

	updated := make([]string, 0, len(rulesData))
	for _, data := range rulesData {
		// Rules Rust doesn't do releases yet.
		if data.Name.Value == "rules_rust" {
			continue
		}

		// Check for updates.
		updateURL := fmt.Sprintf(updateTemplate, data.Repo.Value)
		res, err := http.Get(updateURL)
		if err != nil {
			return fmt.Errorf("Failed to check %s for updates: fetching release: %v", data.Name, err)
		}

		jsonData, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("Failed to check %s for updates: reading release: %v", data.Name, err)
		}

		if err = res.Body.Close(); err != nil {
			return fmt.Errorf("Failed to check %s for updates: closing release: %v", data.Name, err)
		}

		var release ReleaseData
		err = json.Unmarshal(jsonData, &release)
		if err != nil {
			return fmt.Errorf("Failed to check %s for update: parsing release: %v", data.Name, err)
		}

		version := strings.TrimPrefix(release.TagName, "v")
		if version == "" {
			return fmt.Errorf("Failed to check %s for update: failed to find latest version", data.Name)
		}

		// Check whether it's newer than the version
		// we're already using.
		current := "v" + data.Version.Value
		latest := "v" + version
		if !semver.IsValid(current) {
			return fmt.Errorf("Failed to check %s for update: current version %q is invalid", data.Name, data.Version)
		}

		if !semver.IsValid(latest) {
			return fmt.Errorf("Failed to check %s for update: latest version %q is invalid", data.Name, version)
		}

		switch semver.Compare(current, latest) {
		case 0:
			// Current is latest.
			continue
		case -1:
			//  Update to do.
		case +1:
			log.Printf("Warning: %s has current version %s, newer than latest version %s", data.Name, data.Version, version)
			continue
		}

		// Calculate the new checksum.
		archiveURL := strings.ReplaceAll(data.Archive.Value, "{v}", version)

		checksum := sha256.New()
		res, err = http.Get(archiveURL)
		if err != nil {
			return fmt.Errorf("Failed to update %s: fetching archive: %v", data.Name, err)
		}

		_, err = io.Copy(checksum, res.Body)
		if err != nil {
			return fmt.Errorf("Failed to update %s: hashing archive: %v", data.Name, err)
		}

		if err = res.Body.Close(); err != nil {
			return fmt.Errorf("Failed to update %s: closing archive: %v", data.Name, err)
		}

		*data.Version.Ptr = version
		*data.SHA256.Ptr = hex.EncodeToString(checksum.Sum(nil))
		updated = append(updated, fmt.Sprintf("%s from %s to %s", data.Name, data.Version, version))
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
