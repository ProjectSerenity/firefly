// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb"
	"golang.org/x/mod/sumdb/dirhash"
	"golang.org/x/mod/zip"
)

// FetchGoModule downloads a Go module using the
// proxy.golang.org Go module proxy API.
//
func FetchGoModule(ctx context.Context, mod *GoModule, dir string) error {
	const (
		moduleProxyBase = "https://proxy.golang.org"

		// Public key for sum.golang.org. See go/src/cmd/go/internal/modfetch/key.go
		checksumHost = "sum.golang.org"
		checksumKey  = "sum.golang.org+033de0ae+Ac4zctda0e5eza+HJyk9SxEdh+s3Ux18htTTAD8OuAn8"
	)

	// We start by fetching the checksum we should
	// expect for the module data.

	clientOps := &GoChecksumDatabaseClient{
		Host:      checksumHost,
		PublicKey: checksumKey,
	}

	// Get the checksum.
	client := sumdb.NewClient(clientOps)
	lines, err := client.Lookup(mod.Name, mod.Version)
	if err != nil {
		return fmt.Errorf("failed to get checksum: %v (you may want to delete $TEMPDIR/config)", err)
	}

	// Find the line consisting of "importpath version checksum".
	var checksum string
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) == 3 && parts[0] == mod.Name && parts[1] == mod.Version {
			checksum = parts[2]
			break
		}
	}

	if checksum == "" {
		content := strings.Join(lines, "\n  ")
		return fmt.Errorf("failed to get checksum for %s@%s: no checksum in response:\n  %s", mod.Name, mod.Version, content)
	}

	// Delete any old version that remains.
	err = os.RemoveAll(dir)
	if err != nil {
		return fmt.Errorf("failed to remove old data for Go module %s: %v", mod.Name, err)
	}

	// Now we fetch the module contents.
	escaped, err := module.EscapePath(mod.Name)
	if err != nil {
		return fmt.Errorf("failed to download Go module %s: invalid module path: %v", mod.Name, err)
	}

	zipURL := fmt.Sprintf("%s/%s/@v/%s.zip", moduleProxyBase, escaped, mod.Version)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, zipURL, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch Go module %s: %v", mod.Name, err)
	}

	res, err := httpRequest(req)
	if err != nil {
		return fmt.Errorf("failed to fetch Go module %s: %v", mod.Name, err)
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
		return fmt.Errorf("failed to create temporary file for Go module %s's zip: %v", mod.Name, err)
	}

	defer os.RemoveAll(tmp.Name())

	_, err = io.Copy(tmp, res.Body)
	if err != nil {
		tmp.Close()
		res.Body.Close()
		return fmt.Errorf("failed to download Go module %s: %v", mod.Name, err)
	}

	if err = res.Body.Close(); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to close response body for Go module %s: %v", mod.Name, err)
	}

	if err = tmp.Close(); err != nil {
		return fmt.Errorf("failed to close Go module %s's zip: %v", mod.Name, err)
	}

	// Check the checksum.
	gotChecksum, err := dirhash.HashZip(tmp.Name(), dirhash.Hash1)
	if err != nil {
		return fmt.Errorf("failed to verify Go module %s: %v", mod.Name, err)
	}

	if gotChecksum != checksum {
		return fmt.Errorf("failed to verify Go module %s: got checksum %s, want %s", mod.Name, gotChecksum, checksum)
	}

	// Extract the ZIP.
	version := module.Version{
		Path:    mod.Name,
		Version: mod.Version,
	}

	err = zip.Unzip(dir, version, tmp.Name())
	if err != nil {
		return fmt.Errorf("failed to unzip Go module %s: %v", mod.Name, err)
	}

	return nil
}

type GoChecksumDatabaseClient struct {
	Host      string
	PublicKey string
}

func (c *GoChecksumDatabaseClient) ReadRemote(path string) ([]byte, error) {
	fullURL := "https://" + c.Host + path
	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}

	res, err := httpRequest(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		return nil, fmt.Errorf("%s returned status code %d", c.Host, res.StatusCode)
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if err = res.Body.Close(); err != nil {
		return nil, err
	}

	return data, nil
}

func (c *GoChecksumDatabaseClient) configDir(file string) (string, error) {
	fullPath := filepath.Join(os.TempDir(), "config", file)
	dir := filepath.Dir(fullPath)
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		return "", err
	}

	return fullPath, nil
}

func (c *GoChecksumDatabaseClient) ReadConfig(file string) ([]byte, error) {
	fullPath, err := c.configDir(file)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		if file == "key" {
			return []byte(c.PublicKey), nil
		} else if file == c.Host+"/latest" {
			return []byte{}, nil
		}

		return nil, err
	}

	return data, nil
}

func (c *GoChecksumDatabaseClient) WriteConfig(file string, old, new []byte) error {
	// There are some transient issues with
	// the checksum state that gets stored.
	// See github.com/golang.go/issues/38348.
	// For now, we just don't save the state.
	if file == c.Host+"/latest" {
		return nil
	}

	fullPath, err := c.configDir(file)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}

	current, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	if !bytes.Equal(current, old) {
		return sumdb.ErrWriteConflict
	}

	if err = f.Truncate(0); err != nil {
		return err
	}

	_, err = f.Write(new)
	if err != nil {
		return err
	}

	return f.Close()
}

func (c *GoChecksumDatabaseClient) cacheDir(file string) (string, error) {
	fullPath := filepath.Join(os.TempDir(), "cache", file)
	dir := filepath.Dir(fullPath)
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		return "", err
	}

	return fullPath, nil
}

func (c *GoChecksumDatabaseClient) ReadCache(file string) ([]byte, error) {
	fullPath, err := c.cacheDir(file)
	if err != nil {
		return nil, err
	}

	return os.ReadFile(fullPath)
}

func (c *GoChecksumDatabaseClient) WriteCache(file string, data []byte) {
	fullPath, err := c.cacheDir(file)
	if err != nil {
		return
	}

	os.WriteFile(fullPath, data, 0666)
}

func (c *GoChecksumDatabaseClient) Log(msg string) {
	log.Print(msg)
}

func (c *GoChecksumDatabaseClient) SecurityError(msg string) {
	log.Print(msg)
}
