// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/fs"
	"path"
	"reflect"
	"strings"

	"firefly-os.dev/tools/starlark"
)

// StripCachedActions processes the action sequence,
// removing any actions that the cache can prove are
// unnecessary, returning the resulting action
// sequence.
//
// If no actions can be cached, or if there is no
// cache, the unmodified action sequence is returned.
//
func StripCachedActions(fsys fs.FS, actions []Action) []Action {
	// Start by loading the cache manifest. If we
	// fail to do so, we just return the unmodified
	// action sequence.
	data, err := fs.ReadFile(fsys, path.Join(vendor, manifestBzl))
	if err != nil {
		return actions
	}

	var deps Deps
	err = starlark.Unmarshal(manifestBzl, data, &deps)
	if err != nil {
		return actions
	}

	if len(deps.Rust) == 0 {
		return actions
	}

	// Copy the crate data into a map to make
	// lookups quicker.
	crates := make(map[string]*RustCrate)
	for _, crate := range deps.Rust {
		crates[crate.Name] = crate
	}

	modules := make(map[string]*GoModule)
	for _, module := range deps.Go {
		modules[module.Name] = module
	}

	// Search through the action sequence for crates
	// being downloaded and check whether the right
	// data is already present.
	out := make([]Action, 0, len(actions))
	for _, action := range actions {
		switch dl := action.(type) {
		default:
			// For other actions, we leave them alone.
			out = append(out, action)
			continue
		case DownloadRustCrate:
			// Check whether we have this crate in the
			// cache. If the version in the cache isn't
			// the same, we ignore it.
			cached := crates[dl.Crate.Name]
			if cached == nil || dl.Crate.Version != cached.Version {
				out = append(out, dl)
				continue
			}

			// We have already downloaded the right
			// version, so now we check that the local
			// copy hasn't been modified. If the digest
			// in the filesystem matches, we can skip
			// this download.
			gotDigest, err := DigestDirectory(fsys, dl.Path, path.Join(dl.Path, buildBazel))
			if err != nil {
				out = append(out, dl)
				continue
			}

			if gotDigest != cached.Digest {
				out = append(out, dl)
				continue
			}

			// We also invalidate the download cache
			// if our patches have changed, as we need
			// to apply the patches to the fresh data.
			if len(dl.Crate.Patches) != 0 || len(cached.Patches) != 0 {
				gotDigest, err := DigestFiles(fsys, dl.Crate.Patches)
				if err != nil {
					out = append(out, dl)
					continue
				}

				if !reflect.DeepEqual(dl.Crate.PatchArgs, cached.PatchArgs) || gotDigest != cached.PatchDigest {
					out = append(out, dl)
					continue
				}
			}

			// We've got the right content, so we
			// drop this action.
			continue
		case DownloadGoModule:
			// Check whether we have this module in the
			// cache. If the version in the cache isn't
			// the same, we ignore it.
			cached := modules[dl.Module.Name]
			if cached == nil || dl.Module.Version != cached.Version {
				out = append(out, dl)
				continue
			}

			// We have already downloaded the right
			// version, so now we check that the local
			// copy hasn't been modified. If the digest
			// in the filesystem matches, we can skip
			// this download.
			//
			// First, build up the list of build files
			// to ignore; one for each package.
			ignore := make([]string, len(dl.Module.Packages))
			root := strings.TrimSuffix(dl.Path, dl.Module.Name)
			for i, pkg := range dl.Module.Packages {
				ignore[i] = path.Join(root, pkg.Name, buildBazel)
			}

			gotDigest, err := DigestDirectory(fsys, dl.Path, ignore...)
			if err != nil {
				out = append(out, dl)
				continue
			}

			if gotDigest != cached.Digest {
				out = append(out, dl)
				continue
			}

			// We also invalidate the download cache
			// if our patches have changed, as we need
			// to apply the patches to the fresh data.
			if len(dl.Module.Patches) != 0 || len(cached.Patches) != 0 {
				gotDigest, err := DigestFiles(fsys, dl.Module.Patches)
				if err != nil {
					out = append(out, dl)
					continue
				}

				if !reflect.DeepEqual(dl.Module.PatchArgs, cached.PatchArgs) || gotDigest != cached.PatchDigest {
					out = append(out, dl)
					continue
				}
			}

			// We've got the right content, so we
			// drop this action.
			continue
		}
	}

	return out
}

// GenerateCacheManifest produces the cache manifest,
// which describes the set of data cached in the
// vendor directly.
//
func GenerateCacheManifest(fsys fs.FS, deps *Deps) (*Deps, error) {
	manifest := &Deps{
		Rust: make([]*RustCrate, len(deps.Rust)),
		Go:   make([]*GoModule, len(deps.Go)),
	}

	// Iterate through the crates, storing the cache
	// digest for each one, ignoring the generated
	// build file.
	for i, crate := range deps.Rust {
		dir := path.Join(vendor, "rust", crate.Name)
		digest, err := DigestDirectory(fsys, dir, path.Join(dir, buildBazel))
		if err != nil {
			return nil, fmt.Errorf("failed to cache Rust crate %s: %v", crate.Name, err)
		}

		var patchDigest string
		if len(crate.Patches) > 0 {
			patchDigest, err = DigestFiles(fsys, crate.Patches)
			if err != nil {
				return nil, fmt.Errorf("failed to cache Rust crate %s's patches: %v", crate.Name, err)
			}
		}

		out := new(RustCrate)
		*out = *crate
		out.Digest = digest
		out.PatchDigest = patchDigest

		manifest.Rust[i] = out
	}

	// Iterate through the modules, storing the cache
	// digest for each one, ignoring the generated
	// build files.
	for i, module := range deps.Go {
		dir := path.Join(vendor, "go", module.Name)
		// First, build up the list of build files
		// to ignore; one for each package.
		ignore := make([]string, len(module.Packages))
		root := path.Join(vendor, "go")
		for i, pkg := range module.Packages {
			ignore[i] = path.Join(root, pkg.Name, buildBazel)
		}

		digest, err := DigestDirectory(fsys, dir, ignore...)
		if err != nil {
			return nil, fmt.Errorf("failed to cache Go module %s: %v", module.Name, err)
		}

		var patchDigest string
		if len(module.Patches) > 0 {
			patchDigest, err = DigestFiles(fsys, module.Patches)
			if err != nil {
				return nil, fmt.Errorf("failed to cache Go module %s's patches: %v", module.Name, err)
			}
		}

		out := new(GoModule)
		*out = *module
		out.Digest = digest
		out.PatchDigest = patchDigest

		manifest.Go[i] = out
	}

	return manifest, nil
}
