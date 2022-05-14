// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package rust

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rsc.io/diff"

	"github.com/ProjectSerenity/firefly/tools/plan/types"
)

// getRustfmt returns a path to the rustfmt
// tool.
//
func getRustfmt(t *testing.T) string {
	t.Helper()

	// We need to access rustfmt, but for some reason, we
	// can't just use the path we get from the environment
	// variable and instead need to construct an absolute
	// path. This combines the runfiles path, the workspace
	// name, and the second path we receive.
	//
	// As this code is quite fragile, we're unusually strict
	// in checking that we get what we expect.
	rustfmt := os.Getenv("rustfmt")
	parts := strings.Split(rustfmt, " ")
	if len(parts) != 2 {
		t.Fatalf("rustfmt: expected 2 space-separated paths, found %d: %q", len(parts), rustfmt)
	}

	// Make sure we have them the right way around.
	if !strings.HasPrefix(parts[0], "bazel-out") {
		t.Fatalf("rustfmt: expected first path to begin in bazel-out, got %q", parts[0])
	}
	if !strings.HasPrefix(parts[1], "external") {
		t.Fatalf("rustfmt: expected second path to begin in external, got %q", parts[1])
	}

	runfiles := os.Getenv("RUNFILES_DIR")
	if runfiles == "" {
		t.Fatal("rustfmt: got no path in RUNFILES_DIR")
	}

	if !filepath.IsAbs(runfiles) {
		t.Fatalf("rustfmt: unexpected relative path in RUNFILES_DIR: %q", runfiles)
	}

	workspace := os.Getenv("TEST_WORKSPACE")
	if workspace == "" {
		t.Fatal("rustfmt: got no path in TEST_WORKSPACE")
	}

	rustfmt = filepath.Join(runfiles, workspace, parts[1])
	info, err := os.Stat(rustfmt)
	if err != nil {
		t.Fatalf("rustfmt: failed to find rustfmt at %q: %v", rustfmt, err)
	}

	mode := info.Mode()
	if mode.IsDir() {
		t.Fatalf("rustfmt: path at %q is a directory", rustfmt)
	}

	if !mode.IsRegular() {
		t.Fatalf("rustfmt: path at %q is not regular", rustfmt)
	}

	return rustfmt
}

func TestTemplates(t *testing.T) {
	tests := []struct {
		Name string
		Want string
		Tmpl string
		Type any
	}{
		{
			Name: "read error enumeration",
			Want: "enumeration_rs_read_error",
			Tmpl: "enumeration_rs.txt",
			Type: &types.Enumeration{
				Name: types.Name{"read", "error"},
				Docs: types.Docs{
					types.Text("An error returned after a failed attempt to read from"),
					types.Newline{},
					types.Text("a file in a filesystem."),
				},
				Type: types.Uint8,
				Values: []*types.Value{
					{
						Name: types.Name{"no", "error"},
						Docs: types.Docs{types.Text("The file read completed successfully.")},
					},
					{
						Name: types.Name{"end", "of", "file"},
						Docs: types.Docs{types.Text("There is no more data available in the file.")},
					},
					{
						Name: types.Name{"access", "denied"},
						Docs: types.Docs{types.Text("Read operations on this file are not permitted.")},
					},
				},
			},
		},
		{
			Name: "file info structure",
			Want: "structure_rs_file_info",
			Tmpl: "structure_rs.txt",
			Type: &types.Structure{
				Name: types.Name{"file", "info"},
				Docs: types.Docs{
					types.Text("The file info structure is used to represent information about"),
					types.Newline{},
					types.Text("one file in a filesystem."),
				},
				Fields: []*types.Field{
					{
						Name: types.Name{"name"},
						Docs: types.Docs{types.Text("The name of the file.")},
						Type: &types.Pointer{
							Mutable: false,
							Underlying: &types.Reference{
								Name: types.Name{"constant", "string"},
								Underlying: &types.Structure{
									Name: types.Name{"constant", "string"},
									Docs: types.Docs{types.Text("A read-only sequence of UTF-8 encoded text.")},
									Fields: []*types.Field{
										{
											Name: types.Name{"pointer"},
											Docs: types.Docs{types.Text("A pointer to the string's text.")},
											Type: &types.Pointer{
												Mutable:    false,
												Underlying: types.Byte,
											},
										},
										{
											Name: types.Name{"size"},
											Docs: types.Docs{types.Text("The number of bytes in the string's text.")},
											Type: types.Uint64,
										},
									},
								},
							},
						},
					},
					{
						Name: types.Name{"permissions"},
						Docs: types.Docs{types.Text("The permitted actions that can be performed on the file.")},
						Type: types.Uint8,
					},
					{
						Name: types.Name{"padding1"},
						Docs: types.Docs{types.Text("Padding to align the structure.")},
						Type: types.Padding(7),
					},
					{
						Name: types.Name{"file", "size"},
						Docs: types.Docs{types.Text("The size of the file in bytes.")},
						Type: types.Uint64,
					},
				},
			},
		},
		{
			Name: "simple syscall with no args and one result",
			Want: "syscall_rs_no_args_one_result",
			Tmpl: "syscall_rs.txt",
			Type: &types.Syscall{
				Name: types.Name{"simple", "syscall"},
				Docs: types.Docs{
					types.Text("A simple function that takes no arguments and"),
					types.Newline{},
					types.Text("returns no results."),
				},
				Results: []*types.Parameter{
					{
						Name: types.Name{"the", "first"},
						Type: types.Uint16,
					},
				},
			},
		},
		{
			Name: "simple syscall with no args and enum result",
			Want: "syscall_rs_no_args_enum_result",
			Tmpl: "syscall_rs.txt",
			Type: &types.Syscall{
				Name: types.Name{"simple", "syscall"},
				Docs: types.Docs{
					types.Text("A simple function that takes no arguments and"),
					types.Newline{},
					types.Text("returns no results."),
				},
				Results: []*types.Parameter{
					{
						Name: types.Name{"the", "first"},
						Type: &types.Reference{
							Name: types.Name{"message", "type"},
							Underlying: &types.Enumeration{
								Name: types.Name{"message", "type"},
								Type: types.Uint16,
							},
						},
					},
				},
			},
		},
		{
			Name: "simple syscall with no args and both results",
			Want: "syscall_rs_no_args_both_results",
			Tmpl: "syscall_rs.txt",
			Type: &types.Syscall{
				Name: types.Name{"simple", "syscall"},
				Docs: types.Docs{
					types.Text("A simple function that takes no arguments and"),
					types.Newline{},
					types.Text("returns no results."),
				},
				Results: []*types.Parameter{
					{
						Name: types.Name{"bytes", "read"},
						Type: types.Sint64,
					},
					{
						Name: types.Name{"read", "error"},
						Type: &types.Reference{
							Name: types.Name{"read", "error"},
							Underlying: &types.Enumeration{
								Name: types.Name{"read", "error"},
								Type: types.Uint8,
								Values: []*types.Value{
									{Name: types.Name{"no", "error"}},
									{Name: types.Name{"end", "of", "file"}},
								},
							},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			buf.Reset()
			err := templates.ExecuteTemplate(&buf, test.Tmpl, test.Type)
			if err != nil {
				t.Fatal(err)
			}

			got := buf.Bytes()
			name := filepath.Join("testdata", test.Want+".txt")
			want, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("failed to read %s: %v", name, err)
			}

			if !bytes.Equal(got, want) {
				t.Fatalf("templating %s:\n%s", test.Type, diff.Format(string(got), string(want)))
			}
		})
	}
}
