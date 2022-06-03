// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendeps

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"sort"
	"strings"

	"golang.org/x/mod/semver"

	"firefly-os.dev/tools/vendeps/gomodproxy"
)

// FetchGoModule downloads a Go module using the
// proxy.golang.org Go module proxy API.
//
func FetchGoModule(ctx context.Context, mod *GoModule, dir string) error {
	err := gomodproxy.Download(ctx, mod.Name, mod.Version, dir)
	if err != nil {
		return err
	}

	// Identify the set of directories to keep,
	// based on the packages specified.
	keep := make(map[string]bool)
	for _, pkg := range mod.Packages {
		keep[pkg.Name] = true
	}

	// Delete directories we haven't marked to
	// keep.
	prefix := strings.TrimSuffix(dir, mod.Name)
	remove := make(map[string]bool)
	fsys := os.DirFS(prefix)
	err = fs.WalkDir(fsys, mod.Name, func(name string, d fs.DirEntry, err error) error {
		// Ignore files.
		if !d.IsDir() || name == mod.Name {
			// Drop build files we wouldn't use.
			if path.Base(name) == "BUILD.bazel" {
				remove[name] = true
			}

			return nil
		}

		// Ignore directories we should keep.
		if keep[name] {
			return fs.SkipDir
		}

		remove[name] = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to identify unused Go packages to delete: %v", err)
	}

	// Don't remove any directories that are a
	// parent directory of a package we use.
	//
	// We have to do this separately so that we
	// remove packages that are siblings of
	// packages we want to keep.
	for pkg := range keep {
		name := path.Dir(pkg)
		for name != "" && name != "." && name != mod.Name {
			delete(remove, name)
			name = path.Dir(name)
		}
	}

	// There's no point removing child directories
	// of other directories we will remove.
	for dir := range remove {
		if remove[path.Dir(dir)] {
			delete(remove, dir)
		}
	}

	if len(remove) == 0 {
		return nil
	}

	delete := make([]string, 0, len(remove))
	for remove := range remove {
		delete = append(delete, remove)
	}

	sort.Strings(delete)

	for _, dir := range delete {
		err = os.RemoveAll(prefix + dir)
		if err != nil {
			return fmt.Errorf("failed to delete unused Go package %s: %v", dir, err)
		}
	}

	return nil
}

// UpdateGoModule checks a Go module for updates,
// using the proxy.golang.org Go module proxy API.
//
func UpdateGoModule(ctx context.Context, mod *UpdateDep) (updated bool, err error) {
	latest, err := gomodproxy.Latest(ctx, mod.Name)
	if err != nil {
		return false, err
	}

	switch semver.Compare(*mod.Version, latest) {
	case 0:
		// Current is latest.
		return false, nil
	case -1:
		// There is a newer version.
		fmt.Printf("Updated Go module %s from %s to %s.\n", mod.Name, *mod.Version, latest)
		*mod.Version = latest
		return true, nil
	default:
		log.Printf("WARN: Go module %s has version %s, but latest is %s, which is older", mod.Name, *mod.Version, latest)
		return false, nil
	}
}
