// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendeps

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"rsc.io/diff"
)

func TestVendor(t *testing.T) {
	tests := []struct {
		Name string
		Fsys fs.FS
		Want []Action
	}{
		{
			Name: "No deps",
			Fsys: fstest.MapFS{
				"deps.bzl": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{},
				},
			},
			Want: []Action{
				RemoveAll("vendor"),
			},
		},
		{
			Name: "Starting from scratch",
			Fsys: fstest.MapFS{
				"deps.bzl": &fstest.MapFile{
					Mode: 0666,
					Data: []byte(`
						rust = [
							crate(
								name = "serde",
								version = "1.2.3",
							),
							crate(
								name = "x86_64",
								version = "99.88.77",
								build_file = "third_party/x86_64.BUILD",
							),
						]
						go = [
							module(
								name = "rsc.io/quote",
								version = "v1.2.3",
								packages = [
									package(
										name = "rsc.io/quote",
									),
								],
							),
						]
					`),
				},
			},
			Want: []Action{
				DownloadRustCrate{
					Crate: &RustCrate{
						Name:    "serde",
						Version: "1.2.3",
					},
					Path: "vendor/rust/serde",
				},
				GenerateRustCrateBUILD{
					Crate: &RustCrate{
						Name:    "serde",
						Version: "1.2.3",
					},
					Path: "vendor/rust/serde/BUILD.bazel",
				},
				DownloadRustCrate{
					Crate: &RustCrate{
						Name:      "x86_64",
						Version:   "99.88.77",
						BuildFile: "third_party/x86_64.BUILD",
					},
					Path: "vendor/rust/x86_64",
				},
				CopyBUILD{
					Source: "third_party/x86_64.BUILD",
					Path:   "vendor/rust/x86_64/BUILD.bazel",
				},
				DownloadGoModule{
					Module: &GoModule{
						Name:    "rsc.io/quote",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "rsc.io/quote"},
						},
					},
					Path: "vendor/go/rsc.io/quote",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "rsc.io/quote",
					},
					Path: "vendor/go/rsc.io/quote/BUILD.bazel",
				},
				BuildCacheManifest{
					Deps: &Deps{
						Rust: []*RustCrate{
							{
								Name:    "serde",
								Version: "1.2.3",
							},
							{
								Name:      "x86_64",
								Version:   "99.88.77",
								BuildFile: "third_party/x86_64.BUILD",
							},
						},
						Go: []*GoModule{
							{
								Name:    "rsc.io/quote",
								Version: "v1.2.3",
								Packages: []*GoPackage{
									{Name: "rsc.io/quote"},
								},
							},
						},
					},
					Path: "vendor/manifest.bzl",
				},
			},
		},
		{
			Name: "Clearing detritus",
			Fsys: fstest.MapFS{
				"deps.bzl": &fstest.MapFile{
					Mode: 0666,
					Data: []byte(`
						rust = [
							crate(
								name = "serde",
								version = "1.2.3",
							),
						]
						go = [
							module(
								name = "rsc.io/quote",
								version = "v1.2.3",
								packages = [
									package(
										name = "rsc.io/quote",
									),
								],
							),
						]
					`),
				},
				"vendor/manifest.bzl": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{},
				},
				"vendor/parent/child": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{},
				},
				"vendor/random": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{},
				},
				"vendor/rust": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{},
				},
			},
			Want: []Action{
				RemoveAll("vendor/parent"),
				RemoveAll("vendor/random"),
				RemoveAll("vendor/rust"),
				DownloadRustCrate{
					Crate: &RustCrate{
						Name:    "serde",
						Version: "1.2.3",
					},
					Path: "vendor/rust/serde",
				},
				GenerateRustCrateBUILD{
					Crate: &RustCrate{
						Name:    "serde",
						Version: "1.2.3",
					},
					Path: "vendor/rust/serde/BUILD.bazel",
				},
				DownloadGoModule{
					Module: &GoModule{
						Name:    "rsc.io/quote",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "rsc.io/quote"},
						},
					},
					Path: "vendor/go/rsc.io/quote",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "rsc.io/quote",
					},
					Path: "vendor/go/rsc.io/quote/BUILD.bazel",
				},
				BuildCacheManifest{
					Deps: &Deps{
						Rust: []*RustCrate{
							{
								Name:    "serde",
								Version: "1.2.3",
							},
						},
						Go: []*GoModule{
							{
								Name:    "rsc.io/quote",
								Version: "v1.2.3",
								Packages: []*GoPackage{
									{Name: "rsc.io/quote"},
								},
							},
						},
					},
					Path: "vendor/manifest.bzl",
				},
			},
		},
		{
			Name: "Clearing old crates/modules",
			Fsys: fstest.MapFS{
				"deps.bzl": &fstest.MapFile{
					Mode: 0666,
					Data: []byte(`
						rust = [
							crate(
								name = "serde",
								version = "1.2.3",
							),
						]
						go = [
							module(
								name = "rsc.io/diff",
								version = "v1.2.3",
								packages = [
									package(
										name = "rsc.io/diff",
									),
								],
							),
							module(
								name = "rsc.io/quote",
								version = "v1.2.3",
								packages = [
									package(
										name = "rsc.io/quote",
									),
								],
							),
						]
					`),
				},
				"vendor/rust/acpi/lib.rs": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
				"vendor/rust/serde/lib.rs": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
				"vendor/go/golang.org/x/crypto/crypto.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
				"vendor/go/rsc.io/2fa/main.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
			},
			Want: []Action{
				RemoveAll("vendor/rust/acpi"),
				// Don't remove vendor/rust/serde, as it might be cached.
				DownloadRustCrate{
					Crate: &RustCrate{
						Name:    "serde",
						Version: "1.2.3",
					},
					Path: "vendor/rust/serde",
				},
				GenerateRustCrateBUILD{
					Crate: &RustCrate{
						Name:    "serde",
						Version: "1.2.3",
					},
					Path: "vendor/rust/serde/BUILD.bazel",
				},
				RemoveAll("vendor/go/golang.org"), // Root dir of an old module.
				RemoveAll("vendor/go/rsc.io/2fa"), // Don't remove all of rsc.io.
				DownloadGoModule{
					Module: &GoModule{
						Name:    "rsc.io/diff",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "rsc.io/diff"},
						},
					},
					Path: "vendor/go/rsc.io/diff",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "rsc.io/diff",
					},
					Path: "vendor/go/rsc.io/diff/BUILD.bazel",
				},
				DownloadGoModule{
					Module: &GoModule{
						Name:    "rsc.io/quote",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "rsc.io/quote"},
						},
					},
					Path: "vendor/go/rsc.io/quote",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "rsc.io/quote",
					},
					Path: "vendor/go/rsc.io/quote/BUILD.bazel",
				},
				BuildCacheManifest{
					Deps: &Deps{
						Rust: []*RustCrate{
							{
								Name:    "serde",
								Version: "1.2.3",
							},
						},
						Go: []*GoModule{
							{
								Name:    "rsc.io/diff",
								Version: "v1.2.3",
								Packages: []*GoPackage{
									{Name: "rsc.io/diff"},
								},
							},
							{
								Name:    "rsc.io/quote",
								Version: "v1.2.3",
								Packages: []*GoPackage{
									{Name: "rsc.io/quote"},
								},
							},
						},
					},
					Path: "vendor/manifest.bzl",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := Vendor(test.Fsys)
			if err != nil {
				t.Fatalf("Vendor(): %v", err)
			}

			if !reflect.DeepEqual(got, test.Want) {
				var buf strings.Builder
				for _, action := range got {
					buf.WriteString(action.String())
					buf.WriteByte('\n')
				}

				g := buf.String()

				buf.Reset()
				for _, action := range test.Want {
					buf.WriteString(action.String())
					buf.WriteByte('\n')
				}

				w := buf.String()
				t.Fatalf("Vendor(): got mismatch:\n%s", diff.Format(g, w))
			}
		})
	}
}
