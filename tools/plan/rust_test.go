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

	"github.com/ProjectSerenity/firefly/tools/plan/types"
)

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
