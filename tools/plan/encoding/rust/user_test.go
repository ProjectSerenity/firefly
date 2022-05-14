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

	"github.com/ProjectSerenity/firefly/tools/plan/parser"
	"github.com/ProjectSerenity/firefly/tools/plan/types"
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
			               (docs "No error occurred."))
			           (value
			               (name bad syscall)
			               (docs "The specified syscall does not exist."))
			           (value
			               (name illegal parameter)
			               (docs "A parameter to the syscall is an illegal value.")))

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
				t.Fatalf("GenerateUserCode(): output mismatch:\nGot:\n%s\nWant:\n%s", got, want)
			}
		})
	}
}
