// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendeps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
)

// Action represents a logical action that should be
// taken to progress the vendoring of a set of software
// dependencies.
//
// An action should contain any context necessary to
// perform its tasks.
type Action interface {
	Do(fs.FS) error
	fmt.Stringer
}

// RemoveAll deletes a directory, along with any child
// nodes that exist. If the path does not exist, there
// is no effect.
type RemoveAll string

var _ Action = RemoveAll("")

func (r RemoveAll) Do(fsys fs.FS) error { return os.RemoveAll(string(r)) }
func (r RemoveAll) String() string      { return fmt.Sprintf("delete %s", string(r)) }

// DownloadModule indicates that the named module should
// be downloaded from the module proxy and extracted into
// the given path.
type DownloadGoModule struct {
	Module *GoModule
	Path   string
}

var _ Action = DownloadGoModule{}

func (c DownloadGoModule) Do(fsys fs.FS) error {
	ctx := context.Background()
	log.Printf("Downloading Go module %s.", c.Module.Name)
	err := FetchGoModule(ctx, c.Module, c.Path)
	if err != nil {
		return err
	}

	if len(c.Module.Patches) == 0 {
		return nil
	}

	return ApplyPatches(c.Path, c.Module.PatchArgs, c.Module.Patches)
}

func (c DownloadGoModule) String() string {
	line := fmt.Sprintf("download module %s to %s", c.Module.Name, c.Path)
	if len(c.Module.Patches) > 0 {
		line += fmt.Sprintf(" with %d patches", len(c.Module.Patches))
	}

	return line
}

// CopyBUILD indicates that the named BUILD file
// should be copied to the given path.
type CopyBUILD struct {
	Source string
	Path   string
}

var _ Action = CopyBUILD{}

func (c CopyBUILD) Do(fsys fs.FS) error {
	src, err := fsys.Open(c.Source)
	if err != nil {
		return fmt.Errorf("failed to open BUILD file %s: %v", c.Source, err)
	}

	// golang.org/x/mod/zip.Unzip, which we use to
	// extract Go modules, creates files that are
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

	dst, err := os.Create(c.Path)
	if err != nil {
		src.Close()
		return fmt.Errorf("failed to create BUILD file %s: %v", c.Path, err)
	}

	_, err = io.Copy(dst, src)
	if err != nil {
		src.Close()
		dst.Close()
		return fmt.Errorf("failed to copy BUILD file %s to %s: %v", c.Source, c.Path, err)
	}

	if err = src.Close(); err != nil {
		dst.Close()
		return fmt.Errorf("failed to close BUILD file %s: %v", c.Source, err)
	}

	if err = dst.Close(); err != nil {
		return fmt.Errorf("failed to close BUILD file %s: %v", c.Path, err)
	}

	return nil
}

func (c CopyBUILD) String() string {
	return fmt.Sprintf("copy BUILD file %s to %s", c.Source, c.Path)
}

// GeneratePackageBUILD indicates that the named package
// should have its BUILD file generated and written to
// the given path.
type GenerateGoPackageBUILD struct {
	Package *GoPackage
	Path    string
}

var _ Action = GenerateGoPackageBUILD{}

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

// BuildCacheManifest indicates that the cache subsystem
// should scan the vendor filesystem, producing the
// information necessary to avoid unnecessary future work,
// writing it to the given path.
type BuildCacheManifest struct {
	Deps *Deps
	Path string
}

var _ Action = BuildCacheManifest{}

func (c BuildCacheManifest) Do(fsys fs.FS) error {
	manifest, err := GenerateCacheManifest(fsys, c.Deps)
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
