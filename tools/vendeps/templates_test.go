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
)

func TestRenderRustCrateBuildFile(t *testing.T) {
	tests := []struct {
		Name  string
		Want  string
		Crate *RustCrate
	}{
		{
			Name: "simple crate",
			Want: "simple_crate_BUILD.txt",
			Crate: &RustCrate{
				Name:    "foo",
				Version: "1.2.3",
				NoTests: true,
			},
		},
		{
			Name: "complex crate",
			Want: "complex_crate_BUILD.txt",
			Crate: &RustCrate{
				Name:    "foo-bar",
				Edition: "2018",
				Features: []string{
					"bar",
					"baz",
					"default",
				},
				Version: "1.2.3",
				Deps: []string{
					"apic-two",
					"serde",
				},
				ProcMacroDeps: []string{
					"proc-macro2",
				},
				BuildScript: "build.rs",
				BuildScriptDeps: []string{
					"apic",
					"serde",
				},
				TestData: []string{
					"@rust_linux_x86_64//:rustc",
				},
				TestDeps: []string{
					"serde",
				},
				TestEnv: map[string]string{
					"RUSTC": "$(location @rust_linux_x86_64//:rustc)",
				},
			},
		},
		{
			Name: "proc macro",
			Want: "proc_macro_crate_BUILD.txt",
			Crate: &RustCrate{
				Name:    "foo-bar",
				Edition: "2018",
				Features: []string{
					"bar",
					"baz",
					"default",
				},
				Version: "1.2.3",
				Deps: []string{
					"apic-two",
					"serde",
				},
				ProcMacro:   true,
				BuildScript: "build.rs",
				BuildScriptDeps: []string{
					"apic",
					"serde",
				},
				TestDeps: []string{
					"serde",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := RenderRustCrateBuildFile(test.Want, test.Crate)
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

func TestRenderGoPackageBuildFile(t *testing.T) {
	tests := []struct {
		Name    string
		Want    string
		Package *GoPackage
	}{
		{
			Name: "simple package",
			Want: "simple_package_BUILD.txt",
			Package: &GoPackage{
				Name:    "rsc.io/quote",
				NoTests: true,
			},
		},
		{
			Name: "complex package",
			Want: "complex_package_BUILD.txt",
			Package: &GoPackage{
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
		Deps *Deps
	}{
		{
			Name: "simple manifest",
			Want: "simple_manifest.txt",
			Deps: &Deps{
				Rust: []*RustCrate{
					{
						Name:    "my_crate",
						Edition: "2018",
						Features: []string{
							"foo",
							"bar",
						},
						Version: "1.2.3",
						Deps: []string{
							"other_crate",
							"serde",
						},
						TestDeps: []string{
							"test",
						},
						Digest: "sha256:deadbeef",
					},
					{
						Name:      "other_crate",
						Version:   "0.3.2",
						Digest:    "sha256:asdf",
						PatchArgs: []string{"foo", "bar"},
						Patches: []string{
							"foo/bar.patch",
							"foo/other.patch",
						},
						PatchDigest: "asdfasdf",
					},
					{
						Name:    "serde",
						Version: "99.88.77",
						Digest:  "sha256:foobar",
					},
				},
				Go: []*GoModule{
					{
						Name:    "golang.org/x/crypto",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "golang.org/x/crypto"},
						},
						Digest: "sha256:deadbeef",
					},
					{
						Name:    "golang.org/x/mod",
						Version: "v1.2.3",
						Packages: []*GoPackage{
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
