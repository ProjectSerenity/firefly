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

func TestGenerateKernelCode(t *testing.T) {
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
			Text: `(enumeration
			           (name colour)
			           (docs "A colour.")
			           (type sint8)
			           (value
			               (name red)
			               (docs "The colour red."))
			           (value
			               (name green)
			               (docs "The colour green.")))

			       (structure
			           (name file info)
			           (docs "Information about a file in a filesystem.")
			           (field
			               (name name pointer)
			               (docs "The pointer to the file's name contents.")
			               (type *constant byte))
			           (field
			               (name padding)
			               (docs "")
			               (padding 4))
			           (field
			               (name name size)
			               (docs "The number of bytes at 'name pointer'.")
			               (type uint32))
			           (field
			               (name permissions)
			               (docs "The actions that can be performed on the file.")
			               (type permissions))
			           (field
			               (name padding2)
			               (docs "")
			               (padding 7)))

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
			           (docs "An I/O error returned by" (reference get file info) ".")
			           (type uint64)
			           (embed error))

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
			           (docs "Denies the process access to the specified syscalls.")
			           (arg1
			               (name syscalls)
			               (docs "The syscalls to deny.")
			               (type syscalls))
			           (result1
			               (name error)
			               (docs "Any error encountered.")
			               (type error)))

			       (syscall
			           (name exit)
			           (docs "Exits everything immediately.")
			           (result1
			               (name error)
			               (docs "Any error encountered while trying to exit.")
			               (type error)))

			       (syscall
			           (name get process id)
			           (docs "Returns the process's unique identifier.")
			           (result1
			               (name process id)
			               (docs "The process identifier")
			               (type uint32))
			           (result2
			               (name error)
			               (docs "Any error encountered.")
			               (type error)))

			       (syscall
			           (name three args two results)
			           (docs "Docs on" "" "two lines")
			           (arg1
			               (name foo)
			               (docs "")
			               (type uint16))
			           (arg2
			               (name bar)
			               (docs "")
			               (type colour))
			           (arg3
			               (name baz)
			               (docs "")
			               (type *constant sint8))
			           (result1
			               (name happiness)
			               (docs "")
			               (type uint64))
			           (result2
			               (name sadness)
			               (docs "")
			               (type error)))

			       (syscall
			           (name get file info)
			           (docs "Returns the information about the" (code "name") "file.")
			           (arg1
			               (name name pointer)
			               (docs "")
			               (type *constant byte))
			           (arg2
			               (name name size)
			               (docs "")
			               (type uint64))
			           (result1
			               (name info)
			               (docs "")
			               (type *constant file info))
			           (result2
			               (name size)
			               (docs "")
			               (type io error)))

			       (syscall
			           (name get colour)
			           (docs "Returns a" (reference colour) ".")
			           (result1
			               (name colour)
			               (docs "")
			               (type colour))
			           (result2
			               (name error)
			               (docs "")
			               (type error)))

			       (integer
			           (name port number)
			           (docs "A network port number.")
			           (type uint16))

			       (syscall
			           (name close port)
			           (docs "Closes a network port if open.")
			           (arg1
			               (name port)
			               (docs "The port number.")
			               (type port number))
			           (result1
			               (name error)
			               (docs "Any error.")
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

			err = GenerateKernelCode(&buf, typed, test.Arch, rustfmt)
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
				t.Fatalf("GenerateKernelCode(): output mismatch:\n%s", diff.Format(string(got), string(want)))
			}
		})
	}
}
