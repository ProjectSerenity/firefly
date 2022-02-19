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

func cmdRules(ctx context.Context, w io.Writer, args []string) error {
	const (
		rulesBzl       = "rules.bzl"
		updateTemplate = "https://api.github.com/repos/%s/releases/latest"
	)

	var (
		allRules = []string{
			"RULES_BUILDTOOLS",
			"RULES_CC",
			"RULES_GAZELLE",
			"RULES_GO",
			"RULES_PROTOBUF",
			"RULES_RUST",
			"RULES_SKYLIB",
		}
	)

	const rulesFields = 5
	type RuleData struct {
		Name        string
		Repo        string
		Archive     string
		Version     string
		VersionExpr *build.StringExpr
		SHA256      string
		SHA256Expr  *build.StringExpr
	}

	// Find and parse the rules.bzl file to see
	// what versions we've got currently.
	bzlPath := filepath.Join("bazel", "deps", rulesBzl)
	data, err := os.ReadFile(bzlPath)
	if err != nil {
		return fmt.Errorf("Failed to open %s: %v", bzlPath, err)
	}

	f, err := build.ParseBzl(bzlPath, data)
	if err != nil {
		return fmt.Errorf("Failed to parse %s: %v", rulesBzl, err)
	}

	rulesData := make(map[string]*RuleData)
	for _, stmt := range f.Stmt {
		if stmt == nil {
			continue
		}

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
				return fmt.Errorf("Failed to parse %s: found %s with non-call value %#v", rulesBzl, rule, assign.RHS)
			}

			if fun, ok := call.X.(*build.Ident); !ok || fun.Name != "struct" {
				return fmt.Errorf("Failed to parse %s: found %s with non-struct value %#v", rulesBzl, rule, call.X)
			}

			// Pull out the fields.
			stringField := func(field *string, expr **build.StringExpr, name string, value build.Expr) {
				if err != nil {
					// Don't override the first error we see.
					return
				}

				assign, ok := value.(*build.AssignExpr)
				if !ok {
					err = fmt.Errorf("field %q has non-assign value %#v", name, value)
					return
				}

				lhs, ok := assign.LHS.(*build.Ident)
				if !ok {
					err = fmt.Errorf("field %q has non-ident name %#v", name, assign.LHS)
					return
				}

				if lhs.Name != name {
					err = fmt.Errorf("got field %q, want %q", lhs.Name, name)
					return
				}

				rhs, ok := assign.RHS.(*build.StringExpr)
				if !ok {
					err = fmt.Errorf("field %q has non-string value %#v", name, assign.RHS)
					return
				}

				*field = rhs.Value
				if expr != nil {
					*expr = rhs
				}
			}

			var data RuleData
			if len(call.List) != rulesFields {
				return fmt.Errorf("Failed to parse %s: found %s with %d fields, want %d", rulesBzl, rule, len(call.List), rulesFields)
			}

			stringField(&data.Name, nil, "name", call.List[0])
			stringField(&data.Repo, nil, "repo", call.List[1])
			stringField(&data.Archive, nil, "archive", call.List[2])
			stringField(&data.Version, &data.VersionExpr, "version", call.List[3])
			stringField(&data.SHA256, &data.SHA256Expr, "sha256", call.List[4])
			if err != nil {
				return fmt.Errorf("Failed to parse %s: %v", rulesBzl, err)
			}

			rulesData[rule] = &data
		}
	}

	type ReleaseData struct {
		TagName string `json:"tag_name"`
		// We don't care about the other fields.
	}

	updated := make([]string, 0, len(allRules))
	for _, rule := range allRules {
		data := rulesData[rule]
		if data == nil {
			return fmt.Errorf("Failed to parse %s: found no data for %s", rulesBzl, rule)
		}

		// Rules Rust doesn't do releases yet.
		if data.Name == "rules_rust" {
			continue
		}

		// Check for updates.
		updateURL := fmt.Sprintf(updateTemplate, data.Repo)
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
		current := "v" + data.Version
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
		archiveURL := strings.ReplaceAll(data.Archive, "{v}", version)

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

		data.VersionExpr.Value = version
		data.SHA256Expr.Value = hex.EncodeToString(checksum.Sum(nil))
		updated = append(updated, fmt.Sprintf("%s from %s to %s", data.Name, data.Version, version))
	}

	if len(updated) == 0 {
		fmt.Fprintln(w, "All Bazel rules up to date.")
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
