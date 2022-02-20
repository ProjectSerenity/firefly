// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestParseGoBzl(t *testing.T) {
	bzlPath := filepath.Join("testdata", "go_bzl")
	_, modules, err := ParseGoBzl(bzlPath)
	if err != nil {
		t.Fatal(err)
	}

	want := []struct {
		Name    string
		Path    string
		Sum     string
		Version string
	}{
		{
			Name:    "com_github_BurntSushi_toml",
			Path:    "github.com/BurntSushi/toml",
			Sum:     "h1:dtDWrepsVPfW9H/4y7dDgFc2MBUSeJhlaDtK13CxFlU=",
			Version: "v1.0.0",
		},
		{
			Name:    "org_golang_x_mod",
			Path:    "golang.org/x/mod",
			Sum:     "h1:UG21uOlmZabA4fW5i7ZX6bjw1xELEGg/ZLgZq9auk/Q=",
			Version: "v0.5.0",
		},
	}

	if len(modules) != len(want) {
		t.Errorf("found %d modules, want %d:", len(modules), len(want))
		for _, module := range modules {
			t.Logf("{%q, %q, %q, %q}", module.Name, module.Path, module.Sum, module.Version)
		}

		return
	}

	for i := range modules {
		got := modules[i]
		want := want[i]
		context := fmt.Sprintf("module %d", i)
		checkField(t, context, "name", got.Name, want.Name)
		checkField(t, context, "path", got.Path, want.Path)
		checkField(t, context, "sum", got.Sum, want.Sum)
		checkField(t, context, "version", got.Version, want.Version)
	}
}
