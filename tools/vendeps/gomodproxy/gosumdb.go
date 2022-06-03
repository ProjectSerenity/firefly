// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package gomodproxy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/mod/sumdb"

	"firefly-os.dev/tools/simplehttp"
)

const (
	// Public key for sum.golang.org. See go/src/cmd/go/internal/modfetch/key.go
	goChecksumHost = "sum.golang.org"
	goChecksumKey  = "sum.golang.org+033de0ae+Ac4zctda0e5eza+HJyk9SxEdh+s3Ux18htTTAD8OuAn8"
)

type GoChecksumDatabaseClient struct {
	host      string
	publicKey string
}

func NewGoChecksumDatabaseClient() *GoChecksumDatabaseClient {
	return &GoChecksumDatabaseClient{
		host:      goChecksumHost,
		publicKey: goChecksumKey,
	}
}

func (c *GoChecksumDatabaseClient) ReadRemote(path string) ([]byte, error) {
	fullURL := "https://" + c.host + path
	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}

	res, err := simplehttp.Request(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		return nil, fmt.Errorf("%s returned status code %d", c.host, res.StatusCode)
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
			return []byte(c.publicKey), nil
		} else if file == c.host+"/latest" {
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
	if file == c.host+"/latest" {
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
