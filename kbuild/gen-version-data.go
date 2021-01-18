// +build ignore

// Generate offset data for the current and previous versions
// of Go.

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/ProjectSerenity/firefly/kbuild/internal/build"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("")
}

func main() {
	// First, work out the current Go version and check
	// we can call the previous version.

	gotool, err := exec.LookPath("go")
	if err != nil {
		log.Fatalf("failed to find path to the Go tool: %v", err)
	}

	version, err := build.GoVersion(gotool)
	if err != nil {
		log.Fatal(err)
	}

	architectures := []string{
		"amd64",
	}

	for _, goarch := range architectures {
		entries, err := build.DeriveOffsets(gotool, version, goarch)
		if err != nil {
			log.Fatalf("failed to derive offsets for %s on %s: %v", version, goarch, err)
		}

		name := fmt.Sprintf("offsets-%s_%s.go", version, goarch)
		err = build.WriteOffsets(name, "main", version, goarch, entries)
		if err != nil {
			log.Fatalf("failed to write offsets for %s on %s: %v", version, goarch, err)
		}
	}
}
