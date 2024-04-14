// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package format

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/tools/txtar"

	"firefly-os.dev/tools/diff"
	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/parser"
	"firefly-os.dev/tools/ruse/token"
)

func TestFormatFile(t *testing.T) {
	files, _ := filepath.Glob("testdata/*.txtar")
	if len(files) == 0 {
		t.Fatalf("no testdata")
	}

	var buf, buf2 bytes.Buffer
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			buf.Reset()
			fset := token.NewFileSet()
			a, err := txtar.ParseFile(file)
			if err != nil {
				t.Fatalf("failed to open %s: %v", file, err)
			}

			if len(a.Files) != 2 {
				t.Fatalf("invalid archive: got %d files, need %d", len(a.Files), 2)
			}

			file, err := parser.ParseFile(fset, a.Files[0].Name, a.Files[0].Data, parser.ParseComments)
			if err != nil {
				t.Fatalf("ParseFile(): %v", err)
			}

			SortAnnotations(file)

			err = Fprint(&buf, fset, file)
			if err != nil {
				t.Fatalf("Fprint(): %v", err)
			}

			got := buf.Bytes()
			if !bytes.Equal(got, a.Files[1].Data) {
				t.Fatalf("Fprint():\n%s", diff.Diff(a.Files[1].Name, a.Files[1].Data, "got", got))
			}

			// Check that interpreting the original source and
			// the formatted source gives the same result, as
			// formatting should not result in semantic changes.

			origParsed, err := parser.ParseFile(fset, "reparsed.ruse", a.Files[0].Data, parser.ParseComments)
			if err != nil {
				t.Fatalf("ParseFile(test.Source): %v", err)
			}

			formattedParsed, err := parser.ParseFile(fset, "formatted.ruse", got, parser.ParseComments)
			if err != nil {
				t.Fatalf("ParseFile(formatted): %v", err)
			}

			// Repeat sorting so we're consistent.
			SortAnnotations(origParsed)

			// Ignore positions and comments, as these are changed.
			if diff := cmp.Diff(origParsed, formattedParsed, cmpopts.IgnoreTypes(token.Pos(0), new(ast.Comment))); diff != "" {
				t.Fatalf("Fprintf(): (+got, -want)\n%s", diff)
			}

			// Check that formatting the formatted code results
			// in exactly the same sequence of bytes.

			format1 := got

			buf2.Reset()
			err = Fprint(&buf2, fset, formattedParsed)
			if err != nil {
				t.Fatalf("Fprint(formatted): %v", err)
			}

			format2 := buf2.Bytes()
			if !bytes.Equal(format2, format1) {
				t.Fatalf("Fprint(formatted):\n%s", diff.Diff("first", format1, "second", format2))
			}
		})
	}
}
