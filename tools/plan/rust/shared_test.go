// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package rust

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"rsc.io/diff"

	"firefly-os.dev/tools/plan/parser"
	"firefly-os.dev/tools/plan/types"
)

func TestGenerateSharedCode(t *testing.T) {
	tests := []struct {
		Name string
		Want string
		Arch types.Arch
		Text string
	}{
		{
			Name: "Simple",
			Want: "file_shared_simple_rs.txt",
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

			       (integer
			           (name port number)
			           (docs "The number of a TCP or UDP port.")
			           (type uint16))

			       (array
			           (name ipv4 address)
			           (docs "An IPv4 address.")
			           (size 4)
			           (type byte))

			       (array
			           (name ipv6 address)
			           (docs "An IPv6 address.")
			           (size 16)
			           (type byte))

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

			err = GenerateSharedCode(&buf, typed, rustfmt)
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
				t.Fatalf("GenerateSharedCode(): output mismatch:\n%s", diff.Format(string(got), string(want)))
			}
		})
	}
}

func TestSubTemplates(t *testing.T) {
	tests := []struct {
		Name string
		Want string
		Tmpl string
		Type any
	}{
		{
			Name: "port number integer",
			Want: "integer_rs_port_number",
			Tmpl: integerTemplate,
			Type: &types.NewInteger{
				Name: types.Name{"port", "number"},
				Docs: types.Docs{
					types.Text("The number of a TCP or UDP port."),
				},
				Type: types.Uint16,
			},
		},
		{
			Name: "read error enumeration",
			Want: "enumeration_rs_read_error",
			Tmpl: enumerationTemplate,
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
			Name: "access control bitfield",
			Want: "bitfield_rs_access_control",
			Tmpl: bitfieldTemplate,
			Type: &types.Bitfield{
				Name: types.Name{"access", "control"},
				Docs: types.Docs{
					types.Text("The permissions available on a resource."),
				},
				Type: types.Uint16,
				Values: []*types.Value{
					{
						Name: types.Name{"read", "access"},
						Docs: types.Docs{types.Text("The data can be read.")},
					},
					{
						Name: types.Name{"write", "access"},
						Docs: types.Docs{types.Text("The data can be written.")},
					},
					{
						Name: types.Name{"execute", "access"},
						Docs: types.Docs{types.Text("The data can be executed.")},
					},
				},
			},
		},
		{
			Name: "file info structure",
			Want: "structure_rs_file_info",
			Tmpl: structureTemplate,
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
	}

	var buf bytes.Buffer
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			buf.Reset()
			err := sharedTemplates.ExecuteTemplate(&buf, test.Tmpl, test.Type)
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

	// If we're testing outside Bazel, then the environment
	// variable will be unset. In which case, we just use
	// whatever the system gives us.
	if rustfmt == "" && os.Getenv("RUNFILES_DIR") == "" {
		rustfmt, err := exec.LookPath("rustfmt")
		if err != nil {
			t.Fatalf("rustfmt: failed to find binary in PATH: %v", err)
		}

		t.Logf("WARNING: using system installation of `rustfmt` at %s", rustfmt)

		return rustfmt
	}

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
