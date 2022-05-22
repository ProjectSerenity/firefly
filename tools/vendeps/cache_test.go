// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"rsc.io/diff"
)

func TestStripCachedActions(t *testing.T) {
	digestFor := func(t *testing.T, dir string, fsys fs.FS) string {
		digest, err := DigestDirectory(fsys, dir)
		if err != nil {
			t.Helper()
			t.Fatal(err)
		}

		return digest
	}

	tests := []struct {
		Name    string
		Fsys    fs.FS
		Actions []Action
		Want    []Action
	}{
		{
			Name: "No cache",
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
			},
			Actions: []Action{
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
			Name: "Partial cache",
			Fsys: fstest.MapFS{
				"deps.bzl": &fstest.MapFile{
					Mode: 0666,
					Data: []byte(`
						rust = [
							crate(
								name = "acpi",
								version = "4.1.0",
								deps = [
									"bit_field",
								],
							),
							crate(
								name = "bit_field",
								version = "0.10.1",
							),
							crate(
								name = "bootloader",
								version = "0.9.22",
							),
							crate(
								name = "serde",
								version = "1.0.137",
							),
						]
						go = [
							module(
								name = "golang.org/x/crypto",
								version = "v1.2.3",
								packages = [
									package(
										name = "golang.org/x/crypto",
									),
								],
							),
							module(
								name = "golang.org/x/mod",
								version = "v1.2.3",
								packages = [
									package(
										name = "golang.org/x/mod/module",
									),
									package(
										name = "golang.org/x/mod/zip",
									),
								],
							),
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
				"vendor/manifest.bzl": &fstest.MapFile{
					Mode: 0666,
					Data: []byte(fmt.Sprintf(`
						rust = [
							# No cache entry for acpi.

							# Cache entry with wrong version
							# for bit_field.
							crate(
								name = "bit_field",
								version = "0.10.2",
								digest = %q,
							),

							# Cache entry with wrong file
							# contents for bootloader.
							crate(
								name = "bootloader",
								version = "0.9.22",
								digest = "sha256:deadbeef",
							),

							# Valid cache entry for serde.
							crate(
								name = "serde",
								version = "1.0.137",
								digest = %q,
							),
						]
						go = [
							# No cache entry for golang.org/x/crypto.

							# Cache entry with wrong version for
							# golang.org/x/mod.
							module(
								name = "golang.org/x/mod",
								version = "v1.0.0",
								digest = %q,
							),

							# Cache entry with wrong file
							# contents for rsc.io/diff.
							module(
								name = "rsc.io/diff",
								version = "v1.2.3",
								digest = "sha256:deadbeef",
							),

							# Valid cache entry for rsc.io/quote.
							module(
								name = "rsc.io/quote",
								version = "v1.2.3",
								digest = %q,
							),
						]
					`,
						digestFor(t, "bit_field", &fstest.MapFS{
							"bit_field/lib.rs": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{1, 2, 3, 4},
							},
						}),
						digestFor(t, "serde", &fstest.MapFS{
							"serde/lib.rs": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{1, 2, 3},
							},
						}),
						digestFor(t, "vendor/go/rsc.io/diff", &fstest.MapFS{
							"vendor/go/rsc.io/diff/diff.go": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{1, 2, 3, 4},
							},
						}),
						digestFor(t, "vendor/go/rsc.io/quote", &fstest.MapFS{
							"vendor/go/rsc.io/quote/quote.go": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{1, 2, 3},
							},
						}),
					)),
				},
				"vendor/rust/bit_field/lib.rs": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
				"vendor/rust/bootloader/lib.rs": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
				"vendor/rust/serde/lib.rs": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
				"vendor/go/rsc.io/diff/diff.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
				"vendor/go/rsc.io/quote/quote.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
			},
			Actions: []Action{
				DownloadRustCrate{
					Crate: &RustCrate{
						Name:    "acpi",
						Version: "4.1.0",
						Deps: []string{
							"bit_field",
						},
					},
					Path: "vendor/rust/acpi",
				},
				GenerateRustCrateBUILD{
					Crate: &RustCrate{
						Name:    "acpi",
						Version: "4.1.0",
						Deps: []string{
							"bit_field",
						},
					},
					Path: "vendor/rust/acpi/BUILD.bazel",
				},
				DownloadRustCrate{
					Crate: &RustCrate{
						Name:    "bit_field",
						Version: "0.10.1",
					},
					Path: "vendor/rust/bit_field",
				},
				GenerateRustCrateBUILD{
					Crate: &RustCrate{
						Name:    "bit_field",
						Version: "0.10.1",
					},
					Path: "vendor/rust/bit_field/BUILD.bazel",
				},
				DownloadRustCrate{
					Crate: &RustCrate{
						Name:    "bootloader",
						Version: "0.9.22",
					},
					Path: "vendor/rust/bootloader",
				},
				GenerateRustCrateBUILD{
					Crate: &RustCrate{
						Name:    "bootloader",
						Version: "0.9.22",
					},
					Path: "vendor/rust/bootloader/BUILD.bazel",
				},
				DownloadRustCrate{
					Crate: &RustCrate{
						Name:    "serde",
						Version: "1.0.137",
					},
					Path: "vendor/rust/serde",
				},
				GenerateRustCrateBUILD{
					Crate: &RustCrate{
						Name:    "serde",
						Version: "1.0.137",
					},
					Path: "vendor/rust/serde/BUILD.bazel",
				},
				DownloadGoModule{
					Module: &GoModule{
						Name:    "golang.org/x/crypto",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "golang.org/x/crypto"},
						},
					},
					Path: "vendor/go/golang.org/x/crypto",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "golang.org/x/crypto",
					},
					Path: "vendor/go/golang.org/x/crypto/BUILD.bazel",
				},
				DownloadGoModule{
					Module: &GoModule{
						Name:    "golang.org/x/mod",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "golang.org/x/mod/module"},
							{Name: "golang.org/x/mod/zip"},
						},
					},
					Path: "vendor/go/golang.org/x/mod",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "golang.org/x/mod/module",
					},
					Path: "vendor/go/golang.org/x/mod/module/BUILD.bazel",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "golang.org/x/mod/zip",
					},
					Path: "vendor/go/golang.org/x/mod/zip/BUILD.bazel",
				},
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
								Name:    "acpi",
								Version: "4.1.0",
								Deps: []string{
									"bit_field",
								},
							},
							{
								Name:    "bit_field",
								Version: "0.10.1",
							},
							{
								Name:    "bootloader",
								Version: "0.9.22",
							},
							{
								Name:    "serde",
								Version: "1.0.137",
							},
						},
						Go: []*GoModule{
							{
								Name:    "golang.org/x/crypto",
								Version: "v1.2.3",
								Packages: []*GoPackage{
									{Name: "golang.org/x/crypto"},
								},
							},
							{
								Name:    "golang.org/x/mod",
								Version: "v1.2.3",
								Packages: []*GoPackage{
									{Name: "golang.org/x/mod/module"},
									{Name: "golang.org/x/mod/zip"},
								},
							},
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
			Want: []Action{
				DownloadRustCrate{
					Crate: &RustCrate{
						Name:    "acpi",
						Version: "4.1.0",
						Deps: []string{
							"bit_field",
						},
					},
					Path: "vendor/rust/acpi",
				},
				GenerateRustCrateBUILD{
					Crate: &RustCrate{
						Name:    "acpi",
						Version: "4.1.0",
						Deps: []string{
							"bit_field",
						},
					},
					Path: "vendor/rust/acpi/BUILD.bazel",
				},
				DownloadRustCrate{
					Crate: &RustCrate{
						Name:    "bit_field",
						Version: "0.10.1",
					},
					Path: "vendor/rust/bit_field",
				},
				GenerateRustCrateBUILD{
					Crate: &RustCrate{
						Name:    "bit_field",
						Version: "0.10.1",
					},
					Path: "vendor/rust/bit_field/BUILD.bazel",
				},
				DownloadRustCrate{
					Crate: &RustCrate{
						Name:    "bootloader",
						Version: "0.9.22",
					},
					Path: "vendor/rust/bootloader",
				},
				GenerateRustCrateBUILD{
					Crate: &RustCrate{
						Name:    "bootloader",
						Version: "0.9.22",
					},
					Path: "vendor/rust/bootloader/BUILD.bazel",
				},
				// Strip the download for serde, as it's cached.
				GenerateRustCrateBUILD{
					Crate: &RustCrate{
						Name:    "serde",
						Version: "1.0.137",
					},
					Path: "vendor/rust/serde/BUILD.bazel",
				},
				DownloadGoModule{
					Module: &GoModule{
						Name:    "golang.org/x/crypto",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "golang.org/x/crypto"},
						},
					},
					Path: "vendor/go/golang.org/x/crypto",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "golang.org/x/crypto",
					},
					Path: "vendor/go/golang.org/x/crypto/BUILD.bazel",
				},
				DownloadGoModule{
					Module: &GoModule{
						Name:    "golang.org/x/mod",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "golang.org/x/mod/module"},
							{Name: "golang.org/x/mod/zip"},
						},
					},
					Path: "vendor/go/golang.org/x/mod",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "golang.org/x/mod/module",
					},
					Path: "vendor/go/golang.org/x/mod/module/BUILD.bazel",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "golang.org/x/mod/zip",
					},
					Path: "vendor/go/golang.org/x/mod/zip/BUILD.bazel",
				},
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
				// Strip the download for rsc.io/quote, as it's cached.
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
								Name:    "acpi",
								Version: "4.1.0",
								Deps: []string{
									"bit_field",
								},
							},
							{
								Name:    "bit_field",
								Version: "0.10.1",
							},
							{
								Name:    "bootloader",
								Version: "0.9.22",
							},
							{
								Name:    "serde",
								Version: "1.0.137",
							},
						},
						Go: []*GoModule{
							{
								Name:    "golang.org/x/crypto",
								Version: "v1.2.3",
								Packages: []*GoPackage{
									{Name: "golang.org/x/crypto"},
								},
							},
							{
								Name:    "golang.org/x/mod",
								Version: "v1.2.3",
								Packages: []*GoPackage{
									{Name: "golang.org/x/mod/module"},
									{Name: "golang.org/x/mod/zip"},
								},
							},
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
			// Just check that the test actions are
			// correct.
			got, err := Vendor(test.Fsys)
			if err != nil {
				t.Fatalf("Vendor(): %v", err)
			}

			if !reflect.DeepEqual(got, test.Actions) {
				var buf strings.Builder
				for _, action := range got {
					buf.WriteString(action.String())
					buf.WriteByte('\n')
				}

				g := buf.String()

				buf.Reset()
				for _, action := range test.Actions {
					buf.WriteString(action.String())
					buf.WriteByte('\n')
				}

				w := buf.String()
				t.Fatalf("Vendor(): got mismatch:\n%s", diff.Format(g, w))
			}

			// Now test the cache.
			got = StripCachedActions(test.Fsys, test.Actions)
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
				t.Fatalf("StripCachedActions(): got mismatch:\n%s", diff.Format(g, w))
			}
		})
	}
}

func TestGenerateCacheManifest(t *testing.T) {
	digestFor := func(t *testing.T, dir string, fsys fs.FS) string {
		digest, err := DigestDirectory(fsys, dir)
		if err != nil {
			t.Helper()
			t.Fatal(err)
		}

		return digest
	}

	tests := []struct {
		Name string
		Fsys fs.FS
		Deps *Deps
		Want *Deps
	}{
		{
			Name: "Simple deps",
			Fsys: fstest.MapFS{
				"deps.bzl": &fstest.MapFile{
					Mode: 0666,
					Data: []byte(`
						rust = [
							crate(
								name = "acpi",
								version = "4.1.0",
								deps = [
									"bit_field",
								],
							),
							crate(
								name = "bit_field",
								version = "0.10.1",
							),
							crate(
								name = "bootloader",
								version = "0.9.22",
							),
							crate(
								name = "serde",
								version = "1.0.137",
							),
						]
						go = [
							module(
								name = "golang.org/x/crypto",
								version = "v1.2.3",
								packages = [
									package(
										name = "golang.org/x/crypto",
									),
								],
							),
							module(
								name = "golang.org/x/mod",
								version = "v1.2.3",
								packages = [
									package(
										name = "golang.org/x/mod/module",
									),
									package(
										name = "golang.org/x/mod/zip",
									),
								],
							),
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
				"vendor/rust/bit_field/lib.rs": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{4, 5, 6},
				},
				"vendor/rust/bootloader/lib.rs": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{7, 8, 9},
				},
				"vendor/rust/serde/lib.rs": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3, 4, 5},
				},
				"vendor/go/golang.org/x/crypto/crypto.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
				"vendor/go/golang.org/x/mod/module/module.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{4, 5, 6},
				},
				"vendor/go/golang.org/x/mod/zip/zip.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{7, 8, 9},
				},
				"vendor/go/rsc.io/diff/diff.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3, 4, 5},
				},
				"vendor/go/rsc.io/quote/quote.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{6, 7, 8, 9, 0},
				},
			},
			Deps: &Deps{
				Rust: []*RustCrate{
					{
						Name:    "acpi",
						Version: "4.1.0",
						Deps: []string{
							"bit_field",
						},
					},
					{
						Name:    "bit_field",
						Version: "0.10.1",
					},
					{
						Name:    "bootloader",
						Version: "0.9.22",
					},
					{
						Name:    "serde",
						Version: "1.0.137",
					},
				},
				Go: []*GoModule{
					{
						Name:    "golang.org/x/crypto",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "golang.org/x/crypto"},
						},
					},
					{
						Name:    "golang.org/x/mod",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "golang.org/x/mod/module"},
							{Name: "golang.org/x/mod/zip"},
						},
					},
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
			Want: &Deps{
				Rust: []*RustCrate{
					{
						Name:    "acpi",
						Version: "4.1.0",
						Deps: []string{
							"bit_field",
						},
						Digest: digestFor(t, "vendor/rust/acpi", &fstest.MapFS{
							"vendor/rust/acpi/lib.rs": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{1, 2, 3},
							},
						}),
					},
					{
						Name:    "bit_field",
						Version: "0.10.1",
						Digest: digestFor(t, "vendor/rust/bit_field", &fstest.MapFS{
							"vendor/rust/bit_field/lib.rs": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{4, 5, 6},
							},
						}),
					},
					{
						Name:    "bootloader",
						Version: "0.9.22",
						Digest: digestFor(t, "vendor/rust/bootloader", &fstest.MapFS{
							"vendor/rust/bootloader/lib.rs": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{7, 8, 9},
							},
						}),
					},
					{
						Name:    "serde",
						Version: "1.0.137",
						Digest: digestFor(t, "vendor/rust/serde", &fstest.MapFS{
							"vendor/rust/serde/lib.rs": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{1, 2, 3, 4, 5},
							},
						}),
					},
				},
				Go: []*GoModule{
					{
						Name:    "golang.org/x/crypto",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "golang.org/x/crypto"},
						},
						Digest: digestFor(t, "vendor/go/golang.org/x/crypto", &fstest.MapFS{
							"vendor/go/golang.org/x/crypto/crypto.go": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{1, 2, 3},
							},
						}),
					},
					{
						Name:    "golang.org/x/mod",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "golang.org/x/mod/module"},
							{Name: "golang.org/x/mod/zip"},
						},
						Digest: digestFor(t, "vendor/go/golang.org/x/mod", &fstest.MapFS{
							"vendor/go/golang.org/x/mod/module/module.go": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{4, 5, 6},
							},
							"vendor/go/golang.org/x/mod/zip/zip.go": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{7, 8, 9},
							},
						}),
					},
					{
						Name:    "rsc.io/diff",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "rsc.io/diff"},
						},
						Digest: digestFor(t, "vendor/go/rsc.io/diff", &fstest.MapFS{
							"vendor/go/rsc.io/diff/diff.go": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{1, 2, 3, 4, 5},
							},
						}),
					},
					{
						Name:    "rsc.io/quote",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "rsc.io/quote"},
						},
						Digest: digestFor(t, "vendor/go/rsc.io/quote", &fstest.MapFS{
							"vendor/go/rsc.io/quote/quote.go": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{6, 7, 8, 9, 0},
							},
						}),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := GenerateCacheManifest(test.Fsys, test.Deps)
			if err != nil {
				t.Fatalf("GenerateCacheManifest(): %v", err)
			}

			if !reflect.DeepEqual(got, test.Want) {
				g, err := json.MarshalIndent(got, "", "\t")
				if err != nil {
					t.Fatal(err)
				}

				w, err := json.MarshalIndent(test.Want, "", "\t")
				if err != nil {
					t.Fatal(err)
				}

				t.Fatalf("GenerateCacheManifest(): result mismatch:\n%s", diff.Format(string(g), string(w)))
			}
		})
	}
}
