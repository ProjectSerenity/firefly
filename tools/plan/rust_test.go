// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ProjectSerenity/firefly/tools/plan/parser"
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

func TestRustUserspace(t *testing.T) {
	tests := []struct {
		Name string
		Want string
		Arch types.Arch
		Text string
	}{
		{
			Name: "Simple",
			Want: "file_user_simple_rs.txt",
			Arch: types.X86_64,
			Text: `(structure
			           (name file info)
			           (docs "Information about a file in a filesystem.")
			           (field
			               (name name pointer)
			               (docs "The pointer to the file's name contents.")
			               (type *constant byte))
			           (field
			               (name name size)
			               (docs "The number of bytes at 'name pointer'.")
			               (type uint32)))

			       (enumeration
			           (name error)
			           (docs "A general purpose error.")
			           (type uint64)
			           (value
			               (name no error)
			               (docs "No error occurred.")))

			       (syscall
			           (name deny syscalls)
			           (docs "Denies the process access to the specified syscalls.")
			           (arg1
			               (name syscalls)
			               (docs "The syscalls to deny.")
			               (type syscalls))
			           (result1
			               (name error)
			               (docs "Any error encountered.")
			               (type error)))`,
		},
	}

	rustfmt := getRustfmt(t)

	var buf bytes.Buffer
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			buf.Reset()
			parsed, err := parser.ParseFile("test.plan", test.Text)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			typed, err := types.Interpret("test.plan", parsed, test.Arch)
			if err != nil {
				t.Fatalf("failed to interpret: %v", err)
			}

			err = RustUserspace(&buf, typed, rustfmt)
			if err != nil {
				t.Fatalf("failed to translate: %v", err)
			}

			wantName := filepath.Join("testdata", test.Want)
			want, err := os.ReadFile(wantName)
			if err != nil {
				t.Fatalf("failed to read %q: %v", wantName, err)
			}

			got := buf.Bytes()
			if !bytes.Equal(got, want) {
				t.Fatalf("RustUserspace(): output mismatch:\nGot:\n%s\nWant:\n%s", got, want)
			}
		})
	}
}

func TestRustKernelspace(t *testing.T) {
	tests := []struct {
		Name string
		Want string
		Arch types.Arch
		Text string
	}{
		{
			Name: "Simple",
			Want: "file_kernel_simple_rs.txt",
			Arch: types.X86_64,
			Text: `(structure
			           (name file info)
			           (docs "Information about a file in a filesystem.")
			           (field
			               (name name pointer)
			               (docs "The pointer to the file's name contents.")
			               (type *constant byte))
			           (field
			               (name name size)
			               (docs "The number of bytes at 'name pointer'.")
			               (type uint32)))

			       (enumeration
			           (name error)
			           (docs "A general purpose error.")
			           (type uint64)
			           (value
			               (name no error)
			               (docs "No error occurred.")))

			       (syscall
			           (name deny syscalls)
			           (docs "Denies the process access to the specified syscalls.")
			           (arg1
			               (name syscalls)
			               (docs "The syscalls to deny.")
			               (type syscalls))
			           (result1
			               (name error)
			               (docs "Any error encountered.")
			               (type error)))`,
		},
	}

	rustfmt := getRustfmt(t)

	var buf bytes.Buffer
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			buf.Reset()
			parsed, err := parser.ParseFile("test.plan", test.Text)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			typed, err := types.Interpret("test.plan", parsed, test.Arch)
			if err != nil {
				t.Fatalf("failed to interpret: %v", err)
			}

			err = RustKernelspace(&buf, typed, rustfmt)
			if err != nil {
				t.Fatalf("failed to translate: %v", err)
			}

			wantName := filepath.Join("testdata", test.Want)
			want, err := os.ReadFile(wantName)
			if err != nil {
				t.Fatalf("failed to read %q: %v", wantName, err)
			}

			got := buf.Bytes()
			if !bytes.Equal(got, want) {
				t.Fatalf("RustKernelspace(): output mismatch:\nGot:\n%s\nWant:\n%s", got, want)
			}
		})
	}
}

func TestDefineRust(t *testing.T) {
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
					"An error returned after a failed attempt to read from",
					"a file in a filesystem.",
				},
				Type: types.Uint8,
				Values: []*types.Value{
					{
						Name: types.Name{"no", "error"},
						Docs: types.Docs{"The file read completed successfully."},
					},
					{
						Name: types.Name{"end", "of", "file"},
						Docs: types.Docs{"There is no more data available in the file."},
					},
					{
						Name: types.Name{"access", "denied"},
						Docs: types.Docs{"Read operations on this file are not permitted."},
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
					"The file info structure is used to represent information about",
					"one file in a filesystem.",
				},
				Fields: []*types.Field{
					{
						Name: types.Name{"name"},
						Docs: types.Docs{"The name of the file."},
						Type: &types.Pointer{
							Mutable: false,
							Underlying: &types.Reference{
								Name: types.Name{"constant", "string"},
								Underlying: &types.Structure{
									Name: types.Name{"constant", "string"},
									Docs: types.Docs{"A read-only sequence of UTF-8 encoded text."},
									Fields: []*types.Field{
										{
											Name: types.Name{"pointer"},
											Docs: types.Docs{"A pointer to the string's text."},
											Type: &types.Pointer{
												Mutable:    false,
												Underlying: types.Byte,
											},
										},
										{
											Name: types.Name{"size"},
											Docs: types.Docs{"The number of bytes in the string's text."},
											Type: types.Uint64,
										},
									},
								},
							},
						},
					},
					{
						Name: types.Name{"permissions"},
						Docs: types.Docs{"The permitted actions that can be performed on the file."},
						Type: types.Uint8,
					},
					{
						Name: types.Name{"padding1"},
						Docs: types.Docs{"Padding to align the structure."},
						Type: types.Padding(7),
					},
					{
						Name: types.Name{"file", "size"},
						Docs: types.Docs{"The size of the file in bytes."},
						Type: types.Uint64,
					},
				},
			},
		},
		{
			Name: "simple syscall with no args or results",
			Want: "syscall_rs_no_args_or_results",
			Tmpl: "syscall_rs.txt",
			Type: &types.Syscall{
				Name: types.Name{"simple", "syscall"},
				Docs: types.Docs{
					"A simple function that takes no arguments and",
					"returns no results.",
				},
			},
		},
		{
			Name: "simple syscall with one arg and no results",
			Want: "syscall_rs_one_arg_no_results",
			Tmpl: "syscall_rs.txt",
			Type: &types.Syscall{
				Name: types.Name{"simple", "syscall"},
				Docs: types.Docs{
					"A simple function that takes no arguments and",
					"returns no results.",
				},
				Args: []*types.Parameter{
					{
						Name: types.Name{"the", "first"},
						Type: types.Uint16,
					},
				},
			},
		},
		{
			Name: "simple syscall with two args and no results",
			Want: "syscall_rs_two_args_no_results",
			Tmpl: "syscall_rs.txt",
			Type: &types.Syscall{
				Name: types.Name{"simple", "syscall"},
				Docs: types.Docs{
					"A simple function that takes no arguments and",
					"returns no results.",
				},
				Args: []*types.Parameter{
					{
						Name: types.Name{"the", "first"},
						Type: types.Uint16,
					},
					{
						Name: types.Name{"second"},
						Type: &types.Reference{
							Name: types.Name{"message", "type"},
							Underlying: &types.Enumeration{
								Name: types.Name{"message", "type"},
								Type: types.Sint64,
							},
						},
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
					"A simple function that takes no arguments and",
					"returns no results.",
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
					"A simple function that takes no arguments and",
					"returns no results.",
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
					"A simple function that takes no arguments and",
					"returns no results.",
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
			err := rustTemplates.ExecuteTemplate(&buf, test.Tmpl, test.Type)
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
				t.Fatalf("(%s).ToRust():\nGot:\n%s\nWant:\n%s", test.Type, got, want)
			}
		})
	}
}
