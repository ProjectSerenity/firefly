// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendeps

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
				DownloadGoModule{
					Module: &GoModule{
						Name:    "rsc.io/quote",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "rsc.io/quote"},
						},
					},
					Path: "vendor/rsc.io/quote",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "rsc.io/quote",
					},
					Path: "vendor/rsc.io/quote/BUILD.bazel",
				},
				BuildCacheManifest{
					Deps: &Deps{
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
				DownloadGoModule{
					Module: &GoModule{
						Name:    "rsc.io/quote",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "rsc.io/quote"},
						},
					},
					Path: "vendor/rsc.io/quote",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "rsc.io/quote",
					},
					Path: "vendor/rsc.io/quote/BUILD.bazel",
				},
				BuildCacheManifest{
					Deps: &Deps{
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
						digestFor(t, "vendor/rsc.io/diff", &fstest.MapFS{
							"vendor/rsc.io/diff/diff.go": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{1, 2, 3, 4},
							},
						}),
						digestFor(t, "vendor/rsc.io/quote", &fstest.MapFS{
							"vendor/rsc.io/quote/quote.go": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{1, 2, 3},
							},
						}),
					)),
				},
				"vendor/rsc.io/diff/diff.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
				"vendor/rsc.io/quote/quote.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
			},
			Actions: []Action{
				DownloadGoModule{
					Module: &GoModule{
						Name:    "golang.org/x/crypto",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "golang.org/x/crypto"},
						},
					},
					Path: "vendor/golang.org/x/crypto",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "golang.org/x/crypto",
					},
					Path: "vendor/golang.org/x/crypto/BUILD.bazel",
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
					Path: "vendor/golang.org/x/mod",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "golang.org/x/mod/module",
					},
					Path: "vendor/golang.org/x/mod/module/BUILD.bazel",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "golang.org/x/mod/zip",
					},
					Path: "vendor/golang.org/x/mod/zip/BUILD.bazel",
				},
				DownloadGoModule{
					Module: &GoModule{
						Name:    "rsc.io/diff",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "rsc.io/diff"},
						},
					},
					Path: "vendor/rsc.io/diff",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "rsc.io/diff",
					},
					Path: "vendor/rsc.io/diff/BUILD.bazel",
				},
				DownloadGoModule{
					Module: &GoModule{
						Name:    "rsc.io/quote",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "rsc.io/quote"},
						},
					},
					Path: "vendor/rsc.io/quote",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "rsc.io/quote",
					},
					Path: "vendor/rsc.io/quote/BUILD.bazel",
				},
				BuildCacheManifest{
					Deps: &Deps{
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
				DownloadGoModule{
					Module: &GoModule{
						Name:    "golang.org/x/crypto",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "golang.org/x/crypto"},
						},
					},
					Path: "vendor/golang.org/x/crypto",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "golang.org/x/crypto",
					},
					Path: "vendor/golang.org/x/crypto/BUILD.bazel",
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
					Path: "vendor/golang.org/x/mod",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "golang.org/x/mod/module",
					},
					Path: "vendor/golang.org/x/mod/module/BUILD.bazel",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "golang.org/x/mod/zip",
					},
					Path: "vendor/golang.org/x/mod/zip/BUILD.bazel",
				},
				DownloadGoModule{
					Module: &GoModule{
						Name:    "rsc.io/diff",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "rsc.io/diff"},
						},
					},
					Path: "vendor/rsc.io/diff",
				},
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "rsc.io/diff",
					},
					Path: "vendor/rsc.io/diff/BUILD.bazel",
				},
				// Strip the download for rsc.io/quote, as it's cached.
				GenerateGoPackageBUILD{
					Package: &GoPackage{
						Name: "rsc.io/quote",
					},
					Path: "vendor/rsc.io/quote/BUILD.bazel",
				},
				BuildCacheManifest{
					Deps: &Deps{
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
				"vendor/golang.org/x/crypto/crypto.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
				"vendor/golang.org/x/mod/module/module.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{4, 5, 6},
				},
				"vendor/golang.org/x/mod/zip/zip.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{7, 8, 9},
				},
				"vendor/rsc.io/diff/diff.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3, 4, 5},
				},
				"vendor/rsc.io/quote/quote.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{6, 7, 8, 9, 0},
				},
			},
			Deps: &Deps{
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
				Go: []*GoModule{
					{
						Name:    "golang.org/x/crypto",
						Version: "v1.2.3",
						Packages: []*GoPackage{
							{Name: "golang.org/x/crypto"},
						},
						Digest: digestFor(t, "vendor/golang.org/x/crypto", &fstest.MapFS{
							"vendor/golang.org/x/crypto/crypto.go": &fstest.MapFile{
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
						Digest: digestFor(t, "vendor/golang.org/x/mod", &fstest.MapFS{
							"vendor/golang.org/x/mod/module/module.go": &fstest.MapFile{
								Mode: 0666,
								Data: []byte{4, 5, 6},
							},
							"vendor/golang.org/x/mod/zip/zip.go": &fstest.MapFile{
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
						Digest: digestFor(t, "vendor/rsc.io/diff", &fstest.MapFS{
							"vendor/rsc.io/diff/diff.go": &fstest.MapFile{
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
						Digest: digestFor(t, "vendor/rsc.io/quote", &fstest.MapFS{
							"vendor/rsc.io/quote/quote.go": &fstest.MapFile{
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
