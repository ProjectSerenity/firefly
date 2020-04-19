package cc

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestParseSpan(t *testing.T) {
	// Check that the parser correctly tracks
	// the position within each file parsed.

	name := filepath.Join("testdata", "obj.c")
	f, err := os.Open(name)
	if err != nil {
		t.Fatalf("failed to open %s: %v", name, err)
	}

	defer f.Close()

	prog, err := Read(name, f)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", name, err)
	}

	want := []string{
		"char d;",
		"char c;",
		"char e;",
	}

	for i, decl := range prog.Decls {
		t.Run(fmt.Sprint(i+1), func(t *testing.T) {
			text, err := decl.Span.Text()
			if err != nil {
				t.Errorf("decl.Span(): got %v", err)
				return
			}

			if text != want[i] {
				t.Errorf("decl.Span(): got %q, want %q", text, want[i])
				return
			}
		})
	}
}
