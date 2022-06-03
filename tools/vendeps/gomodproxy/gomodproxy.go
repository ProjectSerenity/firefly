// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package gomodproxy simplifies fetching data from a Go module proxy.
//
package gomodproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/mod/sumdb"
	"golang.org/x/mod/sumdb/dirhash"
	"golang.org/x/mod/zip"

	"firefly-os.dev/tools/simplehttp"
)

const goModuleProxy = "https://proxy.golang.org"

// Latest returns the latest version of a Go module,
// using the proxy.golang.org Go module proxy API.
//
func Latest(ctx context.Context, modName string) (latest string, err error) {
	return getLatest(ctx, goModuleProxy, modName)
}

func getLatest(ctx context.Context, proxy, modName string) (latest string, err error) {
	// Fetch the module's latest version.
	escaped, err := module.EscapePath(modName)
	if err != nil {
		return "", fmt.Errorf("failed to look up Go module %s: invalid module path: %v", modName, err)
	}

	latestURL := fmt.Sprintf("%s/%s/@latest", proxy, escaped)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to look up Go module %s: %v", modName, err)
	}

	res, err := simplehttp.Request(req)
	if err != nil {
		return "", fmt.Errorf("failed to look up Go module %s: %v", modName, err)
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		res.Body.Close()
		return "", fmt.Errorf("failed to read response for Go module %s: %v", modName, err)
	}

	if err = res.Body.Close(); err != nil {
		return "", fmt.Errorf("failed to close response for Go module %s: %v", modName, err)
	}

	// See https://go.dev/ref/mod#goproxy-protocol.
	var info struct {
		Version string
		Time    time.Time
	}

	err = json.Unmarshal(data, &info)
	if err != nil {
		return "", fmt.Errorf("failed to parse response for Go module %s: %v", modName, err)
	}

	if info.Version == "" || !semver.IsValid(info.Version) {
		return "", fmt.Errorf("failed to check Go module %s for updates: latest version %q is invalid", modName, info.Version)
	}

	return semver.Canonical(info.Version), nil
}

// Download fetches a Go module using the
// proxy.golang.org Go module proxy API.
//
func Download(ctx context.Context, modName, version, dir string) error {
	return download(ctx, goModuleProxy, modName, version, dir)
}

func download(ctx context.Context, proxy, modName, version, dir string) error {
	// We start by fetching the checksum we should
	// expect for the module data.
	clientOps := NewGoChecksumDatabaseClient()

	// Get the checksum.
	client := sumdb.NewClient(clientOps)
	lines, err := client.Lookup(modName, version)
	if err != nil {
		return fmt.Errorf("failed to get checksum: %v (you may want to delete $TEMPDIR/config)", err)
	}

	// Find the line consisting of "importpath version checksum".
	var checksum string
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) == 3 && parts[0] == modName && parts[1] == version {
			checksum = parts[2]
			break
		}
	}

	if checksum == "" {
		content := strings.Join(lines, "\n  ")
		return fmt.Errorf("failed to get checksum for %s %s: no checksum in response:\n  %s", modName, version, content)
	}

	// Delete any old version that remains.
	err = os.RemoveAll(dir)
	if err != nil {
		return fmt.Errorf("failed to remove old data for Go module %s: %v", modName, err)
	}

	// Now we fetch the module contents.
	escaped, err := module.EscapePath(modName)
	if err != nil {
		return fmt.Errorf("failed to download Go module %s: invalid module path: %v", modName, err)
	}

	zipURL := fmt.Sprintf("%s/%s/@v/%s.zip", proxy, escaped, version)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, zipURL, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch Go module %s: %v", modName, err)
	}

	res, err := simplehttp.Request(req)
	if err != nil {
		return fmt.Errorf("failed to fetch Go module %s: %v", modName, err)
	}

	// We download the module ZIP to an arbitrary
	// temporary directory so we can check its
	// checksum before extracting it and then use
	// the extraction functionality in golang.org/x/mod/zip,
	// which is battle tested and does some extra
	// safety checks.
	tmp, err := os.CreateTemp("", "*.zip")
	if err != nil {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		return fmt.Errorf("failed to create temporary file for Go module %s's zip: %v", modName, err)
	}

	defer os.RemoveAll(tmp.Name())

	_, err = io.Copy(tmp, res.Body)
	if err != nil {
		tmp.Close()
		res.Body.Close()
		return fmt.Errorf("failed to download Go module %s: %v", modName, err)
	}

	if err = res.Body.Close(); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to close response body for Go module %s: %v", modName, err)
	}

	if err = tmp.Close(); err != nil {
		return fmt.Errorf("failed to close Go module %s's zip: %v", modName, err)
	}

	// Check the checksum.
	gotChecksum, err := dirhash.HashZip(tmp.Name(), dirhash.Hash1)
	if err != nil {
		return fmt.Errorf("failed to verify Go module %s: %v", modName, err)
	}

	if gotChecksum != checksum {
		return fmt.Errorf("failed to verify Go module %s: got checksum %s, want %s", modName, gotChecksum, checksum)
	}

	// Extract the ZIP.
	modVersion := module.Version{
		Path:    modName,
		Version: version,
	}

	err = zip.Unzip(dir, modVersion, tmp.Name())
	if err != nil {
		return fmt.Errorf("failed to unzip Go module %s: %v", modName, err)
	}

	return nil
}
