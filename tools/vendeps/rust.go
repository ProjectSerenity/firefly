// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendeps

import (
	"context"
	"fmt"
	"log"

	"golang.org/x/mod/semver"

	"firefly-os.dev/tools/vendeps/cratesio"
)

// FetchRustCrate downloads a Rust crate using the
// crates.io API.
//
func FetchRustCrate(ctx context.Context, crate *RustCrate, dir string) error {
	data, err := cratesio.Lookup(ctx, crate.Name)
	if err != nil {
		return err
	}

	for _, version := range data.Versions {
		if version.Number == crate.Version {
			_, ok := AcceptableLicense(version.License)
			if !ok {
				return fmt.Errorf("cannot use Rust crate %s %s: license %q is unacceptable", crate.Name, crate.Version, version.License)
			}

			return cratesio.Download(ctx, version, dir)
		}
	}

	return fmt.Errorf("failed to find download path for Rust crate %s %s", crate.Name, crate.Version)
}

// UpdateRustCrate checks a Rust crate for updates,
// using the crates.io API.
//
func UpdateRustCrate(ctx context.Context, crate *UpdateDep) (updated bool, err error) {
	data, err := cratesio.Lookup(ctx, crate.Name)
	if err != nil {
		return false, err
	}

	current := semver.Canonical("v" + *crate.Version)
	if current == "" {
		return false, fmt.Errorf("Rust crate %s has an invalid version %q", crate.Name, *crate.Version)
	}

	yanked := false
	ignoredMajor := false
	for _, version := range data.Versions {
		// Check that the version is canonical.
		// If not, we log it and continue. We
		// could return an error, but that feels
		// likely to be more annoying than helpful.
		next := semver.Canonical("v" + version.Number)
		if next == "" {
			log.Printf("WARN: Rust crate %s returned invalid version %q", crate.Name, version.Number)
			continue
		}

		cmp := semver.Compare(current, next)
		if cmp == 0 {
			if version.Yanked {
				// We keep going to find which version
				// we should downgrade to.
				yanked = true
				continue
			}

			// We're already up to date.
			return false, nil
		}

		// Ignore yanked versions, other than the
		// current version. If that is yanked, we
		// need to downgrade (see above).
		if version.Yanked {
			continue
		}

		// We often won't want to update to a
		// higher major version if we're already
		// on a stable version (one with a major
		// version larger than zero). However,
		// in case we do, we log the largest
		// version we see with a larger major
		// version.
		if MajorUpdate(current, next) {
			if !ignoredMajor {
				ignoredMajor = true
				fmt.Printf("Ignored Rust crate %s major update from %s to %s.\n", crate.Name, *crate.Version, version.Number)
			}

			continue
		}

		// If we see an older version and the
		// current version has been yanked, we
		// need to downgrade.
		if cmp == +1 {
			if yanked {
				fmt.Printf("Downgraded Rust crate %s from %s (which has been yanked) to %s.\n", crate.Name, *crate.Version, version.Number)
				*crate.Version = version.Number
				return true, nil
			}

			return false, fmt.Errorf("Rust crate %s returned no data for the current version %s", crate.Name, *crate.Version)
		}

		fmt.Printf("Upgraded Rust crate %s from %s to %s.\n", crate.Name, *crate.Version, version.Number)
		*crate.Version = version.Number
		return true, nil
	}

	if yanked {
		return false, fmt.Errorf("Rust crate %s returned no viable versions (current version %s is yanked)", crate.Name, *crate.Version)
	}

	return false, fmt.Errorf("Rust crate %s returned no viable versions (current version %s missing)", crate.Name, *crate.Version)
}
