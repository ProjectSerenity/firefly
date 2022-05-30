// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package rust

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"rsc.io/diff"

	"firefly-os.dev/tools/plan/parser"
	"firefly-os.dev/tools/plan/types"
)

func TestGenerateUserCode(t *testing.T) {
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
			           (docs "Information about a file in a filesystem, returned by" (reference stat) ".")
			           (field
			               (name name pointer)
			               (docs "The pointer to the file's name contents.")
			               (type *constant byte))
			           (field
			               (name name size)
			               (docs "The number of bytes at" (code "name pointer") ".")
			               (type uint32))
			           (field
			               (name permissions)
			               (docs "The actions that can be performed on the file.")
			               (type permissions)))

			       (enumeration
			           (name error)
			           (docs "A general purpose error.")
			           (type uint64)
			           (value
			               (name no error)
			               (docs "No error occurred."))
			           (value
			               (name bad syscall)
			               (docs "The specified syscall does not exist."))
			           (value
			               (name illegal arg1)
			               (docs "Argument 1 to the syscall is an illegal value."))
			           (value
			               (name illegal arg2)
			               (docs "Argument 2 to the syscall is an illegal value."))
			           (value
			               (name illegal arg3)
			               (docs "Argument 3 to the syscall is an illegal value."))
			           (value
			               (name illegal arg4)
			               (docs "Argument 4 to the syscall is an illegal value."))
			           (value
			               (name illegal arg5)
			               (docs "Argument 5 to the syscall is an illegal value."))
			           (value
			               (name illegal arg6)
			               (docs "Argument 6 to the syscall is an illegal value.")))

			       (enumeration
			           (name io error)
			           (docs "Any error encountered during an I/O operation.")
			           (type uint64)
			           (embed error)
			           (value
			               (name file not found)
			               (docs "The specified file does not exist.")))

			       (bitfield
			           (name permissions)
			           (docs "The set of actions permitted on a resource.")
			           (type uint8)
			           (value
			               (name read)
			               (docs "The data can be read."))
			           (value
			               (name write)
			               (docs "The data can be written."))
			           (value
			               (name execute)
			               (docs "The data can be executed.")))

			       (syscall
			           (name deny syscalls)
			           (docs
			               "Denies the process access to the specified syscalls."
			               ""
			               "Attempts to call denied syscalls will result in the"
			               (reference error)
			               (code "bad syscall")
			               ".")
			           (arg1
			               (name syscalls)
			               (docs "The syscalls to deny.")
			               (type syscalls))
			           (result1
			               (name error)
			               (docs "Any error encountered.")
			               (type error)))

			       (syscall
			           (name stat)
			           (docs "Returns the" (reference file info) "for" (code "name") ".")
			           (arg1
			               (name name pointer)
			               (docs "The pointer to the file name.")
			               (type *constant byte))
			           (arg2
			               (name name size)
			               (docs "The number of bytes at" (code "name pointer") ".")
			               (type uint64))
			           (result1
			               (name file info)
			               (docs "A mutable pointer to the file info.")
			               (type *mutable file info))
			           (result2
			               (name error)
			               (docs "Any error encountered while reading the file.")
			               (type io error)))`,
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

			err = GenerateUserCode(&buf, typed, rustfmt)
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
				t.Fatalf("GenerateUserCode(): output mismatch:\n%s", diff.Format(string(got), string(want)))
			}
		})
	}
}

func TestUserSyscallTemplates(t *testing.T) {
	tests := []struct {
		Name string
		Want string
		Tmpl string
		Type any
	}{
		{
			Name: "simple syscall with no args and enum result",
			Want: "syscall_rs_no_args_enum_result",
			Tmpl: userSyscallTemplate,
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
			Tmpl: userSyscallTemplate,
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
			err := userTemplates.ExecuteTemplate(&buf, test.Tmpl, test.Type)
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
