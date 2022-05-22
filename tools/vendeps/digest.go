// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"crypto"
	_ "crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

const (
	digestHash = crypto.SHA256
	digestName = "sha256"
)

// DigestDirectory produces the digest for a directory
// and its contents in a filesystem. This is performed
// by hashing one line of text for each file, with the
// files sorted into lexographical order. Each line
// consists of the hexadecimal digest of the file's
// contents, two spaces (\x20), the relative filename,
// and a newline (\x0a).
//
// Filenames containing a newline (\x0a) are not allowed.
//
// Any filenames listed in ignore are not included in
// the hashing process.
//
// The final digest is formatted as the hash algorithm
// name, a colon (\x3a), and the hexadecimal digest.
//
func DigestDirectory(fsys fs.FS, dir string, ignore ...string) (string, error) {
	lines, err := digestDirectoryLines(fsys, dir, ignore...)
	if err != nil {
		return "", err
	}

	h := digestHash.New()
	for _, line := range lines {
		io.WriteString(h, line)
	}

	return digestName + ":" + hex.EncodeToString(h.Sum(nil)), nil
}

// digestDirectoryLines produces the lines for DigestDirectory.
//
func digestDirectoryLines(fsys fs.FS, dir string, ignore ...string) ([]string, error) {
	// Start by getting the list of filenames
	// so we can sort them, then we get the
	// file hashes.
	prefix := strings.TrimSuffix(dir, path.Base(dir))
	filenames := make([]string, 0, 10)
	err := fs.WalkDir(fsys, dir, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		for _, ignore := range ignore {
			if ignore == name {
				return nil
			}
		}

		if !d.IsDir() {
			filenames = append(filenames, strings.TrimPrefix(name, prefix))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk %s: %v", dir, err)
	}

	return digestLines(fsys, prefix, filenames)
}

// DigestFiles produces the digest for a set of named
// files and their contents in a filesystem. This is
// performed by hashing one line of text for each file,
// with the files sorted into lexographical order. Each
// line consists of the hexadecimal digest of the file's
// contents, two spaces (\x20), the relative filename,
// and a newline (\x0a).
//
// Filenames containing a newline (\x0a) are not allowed.
//
// The final digest is formatted as the hash algorithm
// name, a colon (\x3a), and the hexadecimal digest.
//
func DigestFiles(fsys fs.FS, filenames []string) (string, error) {
	lines, err := digestLines(fsys, "", filenames)
	if err != nil {
		return "", err
	}

	h := digestHash.New()
	for _, line := range lines {
		io.WriteString(h, line)
	}

	return digestName + ":" + hex.EncodeToString(h.Sum(nil)), nil
}

func digestLines(fsys fs.FS, prefix string, filenames []string) ([]string, error) {
	lines := make([]string, len(filenames))
	copy(lines, filenames)
	sort.Strings(lines)

	hashBuf := make([]byte, digestHash.Size())
	for i, filename := range lines {
		if strings.Contains(filename, "\n") {
			return nil, fmt.Errorf("filenames with newlines are not allowed: found %q", prefix+filename)
		}

		f, err := fsys.Open(prefix + filename)
		if err != nil {
			return nil, fmt.Errorf("failed to open %s%s: %v", prefix, filename, err)
		}

		fh := digestHash.New()
		_, err = io.Copy(fh, f)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s%s: %v", prefix, filename, err)
		}

		if err = f.Close(); err != nil {
			return nil, fmt.Errorf("failed to close %s%s: %v", prefix, filename, err)
		}

		lines[i] = fmt.Sprintf("%x  %s\n", fh.Sum(hashBuf[:0]), filename)
	}

	return lines, nil
}
