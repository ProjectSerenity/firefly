// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package cratesio simplifies calling the Crates.io API.
//
package cratesio

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"firefly-os.dev/tools/simplehttp"
)

const cratesIO = "https://crates.io/api/v1/"

// Lookup returns the metadata for a Rust crate using the
// crates.io API.
//
func Lookup(ctx context.Context, crate string) (*Crate, error) {
	return lookup(ctx, cratesIO, crate)
}

func lookup(ctx context.Context, registry, crate string) (*Crate, error) {
	u, err := url.Parse(registry)
	if err != nil {
		return nil, fmt.Errorf("failed to parse crates.io API URL %q: %v", registry, err)
	}

	u.Path = path.Join("/", u.Path, "crates", crate)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare API request for Rust crate %q: %v", crate, err)
	}

	res, err := simplehttp.Request(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request for Rust crate %q: %v", crate, err)
	}

	if res.StatusCode != http.StatusOK {
		log.Printf("WARN: Unexpected status code for API request for Rust crate %q: %v (%s)", crate, res.StatusCode, res.Status)
	}

	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read API response for Rust crate %q: %v", crate, err)
	}

	var data Crate
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse API response for Rust crate %q: %v", crate, err)
	}

	if data.Crate.Name != crate {
		return nil, fmt.Errorf("query to crates.io for Rust crate %s returned data for %q", crate, data.Crate.Name)
	}

	return &data, nil
}

// Download fetches the given Rust crate version and writes
// the contents into the directory dir.
//
func Download(ctx context.Context, crate *CrateVersion, dir string) error {
	return download(ctx, cratesIO, crate, dir)
}

func download(ctx context.Context, registry string, crate *CrateVersion, dir string) error {
	u, err := url.Parse(registry)
	if err != nil {
		return fmt.Errorf("failed to parse crates.io API URL %q: %v", registry, err)
	}

	if crate.DownloadPath == "" {
		return fmt.Errorf("failed to find download path for Rust crate %s %s", crate.Crate, crate.Number)
	}

	// Delete any old version that remains.
	err = os.RemoveAll(dir)
	if err != nil {
		return fmt.Errorf("failed to remove old data for Rust crate %s: %v", crate.Crate, err)
	}

	// Download the crate to dir.
	u.Path = crate.DownloadPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to prepare API request for Rust crate %q: %v", crate.Crate, err)
	}

	res, err := simplehttp.Request(req)
	if err != nil {
		return fmt.Errorf("failed to make API request for Rust crate %q: %v", crate.Crate, err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		log.Printf("WARN: Unexpected status code for API request for Rust crate %q: %v (%s)", crate.Crate, res.StatusCode, res.Status)
	}

	// The response body is a Gzipped tarball containing
	// the crate data.
	gr, err := gzip.NewReader(res.Body)
	if err != nil {
		return fmt.Errorf("failed to read contents for Rust crate %q: %v", crate.Crate, err)
	}

	tr := tar.NewReader(gr)
	created := make(map[string]bool)
	prefix := fmt.Sprintf("%s-%s/", crate.Crate, crate.Number)
	for {
		file, err := tr.Next()
		if err == io.EOF {
			break
		}

		switch file.Typeflag {
		case tar.TypeReg:
		case tar.TypeDir:
			continue
		default:
			log.Printf("WARN: Ignoring %q in Rust crate %s: unexpected file type %q", file.Name, crate.Crate, file.Typeflag)
			continue
		}

		filename := strings.TrimPrefix(file.Name, prefix)

		parent := filepath.Join(dir, filepath.FromSlash(path.Dir(filename)))
		if !created[parent] {
			created[parent] = true
			err = os.MkdirAll(parent, 0777)
			if err != nil {
				return fmt.Errorf("failed to store Rust crate %q: failed to create directory %q: %v", crate.Crate, parent, err)
			}
		}

		name := filepath.Join(parent, path.Base(filename))
		dst, err := os.Create(name)
		if err != nil {
			return fmt.Errorf("failed to store Rust crate %q: failed to create %q: %v", crate.Crate, name, err)
		}

		_, err = io.Copy(dst, tr)
		if err != nil {
			dst.Close()
			return fmt.Errorf("failed to store Rust crate %q: failed to write %q: %v", crate.Crate, name, err)
		}

		if err = dst.Close(); err != nil {
			return fmt.Errorf("failed to store Rust crate %q: failed to close %q: %v", crate.Crate, name, err)
		}
	}

	if err = res.Body.Close(); err != nil {
		return fmt.Errorf("failed to read contents for Rust crate %q: %v", crate.Crate, err)
	}

	return nil
}
