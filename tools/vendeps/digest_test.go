// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendeps

import (
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"rsc.io/diff"
)

func TestDigestDirectoryLines(t *testing.T) {
	line := func(name string, b []byte) string {
		h := digestHash.New()
		h.Write(b)
		return fmt.Sprintf("%x  %s\n", h.Sum(nil), name)
	}

	tests := []struct {
		Name   string
		Dir    string
		Ignore []string
		Fsys   fs.FS
		Want   []string
	}{
		{
			Name: "One empty file",
			Dir:  "foo",
			Fsys: &fstest.MapFS{
				"foo/bar.txt": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{},
				},
			},
			Want: []string{
				line("foo/bar.txt", nil),
			},
		},
		{
			Name: "Subdirectories",
			Dir:  "foo/bar",
			Ignore: []string{
				"foo/bar/ignored.txt",
			},
			Fsys: &fstest.MapFS{
				"foo/bar/baz.txt": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
				"foo/bar/ignored.txt": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
				"foo/bar/a/b/c.txt": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{5, 6, 7},
				},
			},
			Want: []string{
				line("bar/a/b/c.txt", []byte{5, 6, 7}),
				line("bar/baz.txt", []byte{1, 2, 3}),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := digestDirectoryLines(test.Fsys, test.Dir, test.Ignore...)
			if err != nil {
				t.Fatalf("digestDirectoryLines(): %v", err)
			}

			if !reflect.DeepEqual(got, test.Want) {
				g := strings.Join(got, "")
				w := strings.Join(test.Want, "")
				t.Fatalf("digestDirectoryLines(): mismatch:\n%s", diff.Format(g, w))
			}
		})
	}
}
