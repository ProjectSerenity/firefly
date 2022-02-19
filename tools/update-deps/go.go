// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bazelbuild/buildtools/build"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/mod/sumdb"
)

func init() {
	RegisterCommand("go", "Update the Go modules used.", cmdGo)
}

func cmdGo(ctx context.Context, w io.Writer, args []string) error {
	const (
		goBzl          = "go.bzl"
		goRepo         = "go_repository"
		updateTemplate = "https://proxy.golang.org"

		// Public key for sum.golang.org. See go/src/cmd/go/internal/modfetch/key.go
		checksumHost = "sum.golang.org"
		checksumKey  = "sum.golang.org+033de0ae+Ac4zctda0e5eza+HJyk9SxEdh+s3Ux18htTTAD8OuAn8"
	)

	const modulesFields = 4
	type ModuleData struct {
		Name        string
		Path        string
		Sum         string
		SumExpr     *build.StringExpr
		Version     string
		VersionExpr *build.StringExpr
	}

	// Find and parse the go.bzl file to see
	// what versions we've got currently.
	bzlPath := filepath.Join("bazel", "deps", goBzl)
	data, err := os.ReadFile(bzlPath)
	if err != nil {
		return fmt.Errorf("Failed to open %s: %v", bzlPath, err)
	}

	f, err := build.ParseBzl(bzlPath, data)
	if err != nil {
		return fmt.Errorf("Failed to parse %s: %v", goBzl, err)
	}

	var modulesData []*ModuleData
	for _, stmt := range f.Stmt {
		if stmt == nil {
			continue
		}

		fun, ok := stmt.(*build.DefStmt)
		if !ok {
			continue
		}

		for _, expr := range fun.Body {
			call, ok := expr.(*build.CallExpr)
			if !ok {
				continue
			}

			name, ok := call.X.(*build.Ident)
			if !ok {
				continue
			}

			if name.Name != goRepo {
				continue
			}

			// Pull out the fields.
			stringField := func(field *string, expr **build.StringExpr, name string, value build.Expr) {
				if err != nil {
					// Don't override the first error we see.
					return
				}

				assign, ok := value.(*build.AssignExpr)
				if !ok {
					err = fmt.Errorf("field %q has non-assign value %#v", name, value)
					return
				}

				lhs, ok := assign.LHS.(*build.Ident)
				if !ok {
					err = fmt.Errorf("field %q has non-ident name %#v", name, assign.LHS)
					return
				}

				if lhs.Name != name {
					err = fmt.Errorf("got field %q, want %q", lhs.Name, name)
					return
				}

				rhs, ok := assign.RHS.(*build.StringExpr)
				if !ok {
					err = fmt.Errorf("field %q has non-string value %#v", name, assign.RHS)
					return
				}

				*field = rhs.Value
				if expr != nil {
					*expr = rhs
				}
			}

			var data ModuleData
			if len(call.List) != modulesFields {
				return fmt.Errorf("Failed to parse %s: found %s with %d fields, want %d", goBzl, goRepo, len(call.List), modulesFields)
			}

			stringField(&data.Name, nil, "name", call.List[0])
			stringField(&data.Path, nil, "importpath", call.List[1])
			stringField(&data.Sum, &data.SumExpr, "sum", call.List[2])
			stringField(&data.Version, &data.VersionExpr, "version", call.List[3])
			if err != nil {
				return fmt.Errorf("Failed to parse %s: %v", goBzl, err)
			}

			modulesData = append(modulesData, &data)
		}
	}

	// See https://go.dev/ref/mod#goproxy-protocol.
	type Info struct {
		Version string
		Time    time.Time
	}

	clientOps := &GoChecksumDatabaseClient{
		Host:      checksumHost,
		PublicKey: checksumKey,
	}

	client := sumdb.NewClient(clientOps)
	updated := make([]string, 0, len(modulesData))
	for _, data := range modulesData {
		// Work out the latest version.
		escaped, err := module.EscapePath(data.Path)
		if err != nil {
			return fmt.Errorf("Failed to check %s for updates: invalid module path: %v", data.Path, err)
		}

		updateURL := fmt.Sprintf("%s/%s/@latest", updateTemplate, escaped)
		res, err := http.Get(updateURL)
		if err != nil {
			return fmt.Errorf("Failed to check %s for updates: fetching @latest: %v", data.Path, err)
		}

		jsonData, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("Failed to check %s for updates: reading @latest: %v", data.Path, err)
		}

		if err = res.Body.Close(); err != nil {
			return fmt.Errorf("Failed to check %s for updates: closing @latest: %v", data.Path, err)
		}

		var latest Info
		err = json.Unmarshal(jsonData, &latest)
		if err != nil {
			return fmt.Errorf("Failed to check %s for update: parsing @latest: %v (%s)", data.Path, err, updateURL)
		}

		if latest.Version == "" {
			return fmt.Errorf("Failed to check %s for update: failed to find latest version", data.Path)
		}

		// Check whether it's newer than the version
		// we're already using.
		if !semver.IsValid(data.Version) {
			return fmt.Errorf("Failed to check %s for update: current version %q is invalid", data.Path, data.Version)
		}

		if !semver.IsValid(latest.Version) {
			return fmt.Errorf("Failed to check %s for update: latest version %q is invalid", data.Path, latest.Version)
		}

		switch semver.Compare(data.Version, latest.Version) {
		case 0:
			// Current is latest.
			continue
		case -1:
			//  Update to do.
		case +1:
			log.Printf("Warning: %s has current version %s, newer than latest version %s", data.Path, data.Version, latest.Version)
			continue
		}

		// Get the checksum.
		lines, err := client.Lookup(data.Path, latest.Version)
		if err != nil {
			return fmt.Errorf("Failed to get checksum for %s@%s: %v", data.Path, latest.Version, err)
		}

		// Find the line consisting of "importpath version checksum".
		var checksum string
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) == 3 && parts[0] == data.Path && parts[1] == latest.Version {
				checksum = parts[2]
				break
			}
		}

		if checksum == "" {
			content := strings.Join(lines, "\n  ")
			return fmt.Errorf("Failed to get checksum for %s@%s: no checksum in response:\n  %s", data.Path, latest.Version, content)
		}

		data.VersionExpr.Value = latest.Version
		data.SumExpr.Value = checksum
		updated = append(updated, fmt.Sprintf("%s from %s to %s", data.Path, data.Version, latest.Version))
	}

	if len(updated) == 0 {
		fmt.Fprintln(w, "All Go modules are up to date.")
		return nil
	}

	// Pretty-print the updated workspace.
	pretty := build.Format(f)
	err = os.WriteFile(bzlPath, pretty, 0644)
	if err != nil {
		return fmt.Errorf("Failed to write updated %s: %v", bzlPath, err)
	}

	if len(updated) == 0 {
		fmt.Fprintf(w, "Updated %s.\n", updated[0])
	} else {
		fmt.Fprintf(w, "Updated:\n  %s\n", strings.Join(updated, "\n  "))
	}

	return nil
}

type GoChecksumDatabaseClient struct {
	Host      string
	PublicKey string
}

func (c *GoChecksumDatabaseClient) ReadRemote(path string) ([]byte, error) {
	fullURL := "https://" + c.Host + path
	res, err := http.Get(fullURL)
	if err != nil {
		return nil, err
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
