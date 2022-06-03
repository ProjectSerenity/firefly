// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendeps

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// ApplyPatches applies the given set of patch files to the
// directory specified.
//
func ApplyPatches(dir string, patchArgs []string, patches []string) error {
	// TODO: Implement ApplyPatches in Go, rather than shelling out to the patch binary.

	// The patch binary is happiest if we pass the set
	// of patch files to stdin. Since we may have many
	// patch files and we don't want to concatenate
	// them in memory, we open them all as files, use
	// an io.MultiReader to concatenate them on the
	// fly, then pass that to stdin.
	readers := make([]io.Reader, len(patches))
	patchFiles := make([]*os.File, len(patches))
	for i, patch := range patches {
		f, err := os.Open(patch)
		if err != nil {
			return fmt.Errorf("failed to open patch path %q: %v", patch, err)
		}

		for j := 0; j < i; j++ {
			patchFiles[j].Close()
		}

		readers[i] = f
		patchFiles[i] = f
	}

	cmd := exec.Command("patch", patchArgs...)
	cmd.Dir = dir
	cmd.Stdin = io.MultiReader(readers...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stderr.Write(out)
		for _, f := range patchFiles {
			f.Close()
		}

		return fmt.Errorf("failed to run patch: %v", err)
	}

	for i, f := range patchFiles {
		err := f.Close()
		if err != nil {
			return fmt.Errorf("failed to close patch file %s: %v", patches[i], err)
		}
	}

	return nil
}
