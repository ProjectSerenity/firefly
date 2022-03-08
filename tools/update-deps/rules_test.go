// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestParseRulesBzl(t *testing.T) {
	bzlPath := filepath.Join("testdata", "rules_bzl")
	_, rules, err := ParseRulesBzl(bzlPath)
	if err != nil {
		t.Fatal(err)
	}

	want := []struct {
		Name    string
		Repo    string
		Branch  string
		Archive string
		Version string
		SHA256  string
	}{
		{
			Name:    "com_github_bazelbuild_buildtools",
			Repo:    "bazelbuild/buildtools",
			Archive: "https://github.com/bazelbuild/buildtools/archive/{v}.zip",
			Version: "5.0.1",
			SHA256:  "518b2ce90b1f8ad7c9a319ca84fd7de9a0979dd91e6d21648906ea68faa4f37a",
		},
		{
			Name:    "rules_cc",
			Repo:    "bazelbuild/rules_cc",
			Archive: "https://github.com/bazelbuild/rules_cc/releases/download/{v}/rules_cc-{v}.tar.gz",
			Version: "0.0.1",
			SHA256:  "4dccbfd22c0def164c8f47458bd50e0c7148f3d92002cdb459c2a96a68498241",
		},
		{
			Name:    "bazel_gazelle",
			Repo:    "bazelbuild/bazel-gazelle",
			Archive: "https://github.com/bazelbuild/bazel-gazelle/releases/download/v{v}/bazel-gazelle-v{v}.tar.gz",
			Version: "0.24.0",
			SHA256:  "de69a09dc70417580aabf20a28619bb3ef60d038470c7cf8442fafcf627c21cb",
		},
		{
			Name:    "io_bazel_rules_go",
			Repo:    "bazelbuild/rules_go",
			Archive: "https://github.com/bazelbuild/rules_go/releases/download/v{v}/rules_go-v{v}.zip",
			Version: "0.30.0",
			SHA256:  "d6b2513456fe2229811da7eb67a444be7785f5323c6708b38d851d2b51e54d83",
		},
		{
			Name:    "rules_pkg",
			Repo:    "bazelbuild/rules_pkg",
			Archive: "https://github.com/bazelbuild/rules_pkg/releases/download/{v}/rules_pkg-{v}.tar.gz",
			Version: "0.4.0",
			SHA256:  "038f1caa773a7e35b3663865ffb003169c6a71dc995e39bf4815792f385d837d",
		},
		{
			Name:    "com_google_protobuf",
			Repo:    "protocolbuffers/protobuf",
			Archive: "https://github.com/protocolbuffers/protobuf/archive/v{v}.zip",
			Version: "3.19.4",
			SHA256:  "25680843adf0c3302648d35f744e38cc3b6b05a6c77a927de5aea3e1c2e36106",
		},
		{
			Name:    "rules_rust",
			Repo:    "bazelbuild/rules_rust",
			Branch:  "main",
			Archive: "https://github.com/bazelbuild/rules_rust/archive/{v}.tar.gz",
			Version: "f569827113d0f1058f33da4a449ddd34be35a011",
			SHA256:  "391d6a7f34c89d475e03e02f71957663c9aff6dbd8b8c974945e604828eebe11",
		},
		{
			Name:    "bazel_skylib",
			Repo:    "bazelbuild/bazel-skylib",
			Archive: "https://github.com/bazelbuild/bazel-skylib/releases/download/{v}/bazel-skylib-{v}.tar.gz",
			Version: "1.2.0",
			SHA256:  "af87959afe497dc8dfd4c6cb66e1279cb98ccc84284619ebfec27d9c09a903de",
		},
	}

	if len(rules) != len(want) {
		t.Errorf("found %d rules, want %d:", len(rules), len(want))
		for _, rule := range rules {
			t.Logf("{%q, %q, %q, %q, %q}", rule.Name, rule.Repo, rule.Archive, rule.Version, rule.SHA256)
		}

		return
	}

	for i := range rules {
		got := rules[i]
		want := want[i]
		context := fmt.Sprintf("rule %d", i)
		checkField(t, context, "name", got.Name, want.Name)
		checkField(t, context, "repo", got.Repo, want.Repo)
		checkField(t, context, "branch", got.Branch, want.Branch)
		checkField(t, context, "archive", got.Archive, want.Archive)
		checkField(t, context, "version", got.Version, want.Version)
		checkField(t, context, "sha256", got.SHA256, want.SHA256)
	}
}

func TestGitHubAPI(t *testing.T) {
	// Start an HTTP server, serving a
	// captured copy of an actual response.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/bazelbuild/buildtools/releases/latest":
			http.ServeFile(w, r, filepath.Join("testdata", "buildtools.json"))
		case "/repos/bazelbuild/rules_rust/branches/main":
			http.ServeFile(w, r, filepath.Join("testdata", "rules_rust.json"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Run("release", func(t *testing.T) {
		got, err := LatestGitRelease(srv.URL, "bazelbuild/buildtools")
		if err != nil {
			t.Fatal(err)
		}

		want := "5.0.1"
		if got != want {
			t.Fatalf("Got unexpected latest version %q, want %q", got, want)
		}
	})

	t.Run("commit", func(t *testing.T) {
		got, err := LatestGitCommit(srv.URL, "bazelbuild/rules_rust", "main")
		if err != nil {
			t.Fatal(err)
		}

		want := "b9469a0a22fe36eecf85820fafba7e901662f900"
		if got != want {
			t.Fatalf("Got unexpected latest commit %q, want %q", got, want)
		}
	})
}
