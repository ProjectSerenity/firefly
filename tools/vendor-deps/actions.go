// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"firefly-os.dev/tools/vendeps"
)

// GeneratePackageBUILD indicates that the named package
// should have its BUILD file generated and written to
// the given path.
type GenerateGoPackageBUILD struct {
	Package *vendeps.GoPackage
	Path    string
}

var _ vendeps.Action = GenerateGoPackageBUILD{}

func (c GenerateGoPackageBUILD) Do(fsys fs.FS) error {
	// Render the build files.
	pretty, err := RenderGoPackageBuildFile(c.Path, c.Package)
	if err != nil {
		return err
	}

	// golang.org/x/mod/zip.Unzip, which we use to
	// extract the module, creates files that are
	// read-only, so if the module already contains
	// a BUILD file, we must make it writable before
	// we overwrite it.
	info, err := fs.Stat(fsys, c.Path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to stat %s: %v", c.Path, err)
	}

	if info != nil && info.Mode().Perm()&0200 == 0 {
		err = os.Chmod(c.Path, 0644)
		if err != nil {
			return fmt.Errorf("failed to make %s writable: %v", c.Path, err)
		}
	}

	err = os.WriteFile(c.Path, pretty, 0644)
	if err != nil {
		return fmt.Errorf("failed to write build file to %s: %v", c.Path, err)
	}

	return nil
}

func (c GenerateGoPackageBUILD) String() string {
	return fmt.Sprintf("generate BUILD file for Go package %s to %s", c.Package.Name, c.Path)
}

// GenerateTextFilesBUILD indicates that the named directory
// should have its BUILD file generated and written to
// the given path.
type GenerateTextFilesBUILD struct {
	Files *vendeps.TextFiles
	Path  string
}

var _ vendeps.Action = GenerateTextFilesBUILD{}

func (c GenerateTextFilesBUILD) Do(fsys fs.FS) error {
	// Render the build files.
	pretty, err := RenderTextFilesBuildFile(c.Path, c.Files)
	if err != nil {
		return err
	}

	err = os.WriteFile(c.Path, pretty, 0644)
	if err != nil {
		return fmt.Errorf("failed to write build file to %s: %v", c.Path, err)
	}

	return nil
}

func (c GenerateTextFilesBUILD) String() string {
	return fmt.Sprintf("generate BUILD file for text files %s to %s", c.Files.Name, c.Path)
}

// BuildCacheManifest indicates that the cache subsystem
// should scan the vendor filesystem, producing the
// information necessary to avoid unnecessary future work,
// writing it to the given path.
type BuildCacheManifest struct {
	Deps *vendeps.Deps
	Path string
}

var _ vendeps.Action = BuildCacheManifest{}

func (c BuildCacheManifest) Do(fsys fs.FS) error {
	manifest, err := vendeps.GenerateCacheManifest(fsys, c.Deps)
	if err != nil {
		return fmt.Errorf("failed to build cache manifest: %v", err)
	}

	pretty, err := RenderManifest(c.Path, manifest)
	if err != nil {
		return err
	}

	err = os.WriteFile(c.Path, pretty, 0644)
	if err != nil {
		return fmt.Errorf("failed to write cache manifest to %s: %v", c.Path, err)
	}

	return nil
}

func (c BuildCacheManifest) String() string {
	return fmt.Sprintf("generate cache manifest to %s", c.Path)
}
