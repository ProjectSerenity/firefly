// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"rsc.io/diff"

	"firefly-os.dev/tools/vendeps"
)

func TestRenderGoPackageBuildFile(t *testing.T) {
	tests := []struct {
		Name    string
		Want    string
		Package *vendeps.GoPackage
	}{
		{
			Name: "simple package",
			Want: "simple_package_BUILD.txt",
			Package: &vendeps.GoPackage{
				Name:    "rsc.io/quote",
				NoTests: true,
			},
		},
		{
			Name: "complex package",
			Want: "complex_package_BUILD.txt",
			Package: &vendeps.GoPackage{
				Name: "golang.org/x/mod/zip",
				Deps: []string{
					"golang.org/x/mod/module",
					"rsc.io/quote",
				},
				TestData: []string{
					"@rust_linux_x86_64//:rustc",
				},
				TestDeps: []string{
					"rsc.io/diff",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := RenderGoPackageBuildFile(test.Want, test.Package)
			if err != nil {
				t.Fatal(err)
			}

			wantName := filepath.Join("testdata", test.Want)
			want, err := os.ReadFile(wantName)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(got, want) {
				t.Fatalf("RenderCrateBuildFile(): mismatch:\n%s", diff.Format(string(got), string(want)))
			}
		})
	}
}

func TestRenderManifest(t *testing.T) {
	tests := []struct {
		Name string
		Want string
		Deps *vendeps.Deps
	}{
		{
			Name: "simple manifest",
			Want: "simple_manifest.txt",
			Deps: &vendeps.Deps{
				Go: []*vendeps.GoModule{
					{
						Name:    "golang.org/x/crypto",
						Version: "v1.2.3",
						Packages: []*vendeps.GoPackage{
							{Name: "golang.org/x/crypto"},
						},
						Digest: "sha256:deadbeef",
					},
					{
						Name:    "golang.org/x/mod",
						Version: "v1.2.3",
						Packages: []*vendeps.GoPackage{
							{Name: "golang.org/x/mod/module"},
							{Name: "golang.org/x/mod/zip"},
						},
						Digest: "sha256:foobar",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := RenderManifest(test.Want, test.Deps)
			if err != nil {
				t.Fatal(err)
			}

			wantName := filepath.Join("testdata", test.Want)
			want, err := os.ReadFile(wantName)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(got, want) {
				t.Fatalf("RenderManifest(): mismatch:\n%s", diff.Format(string(got), string(want)))
			}
		})
	}
}
