// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"embed"
	"fmt"
	"path"
	"strings"
	"text/template"

	"github.com/ProjectSerenity/firefly/tools/starlark"
)

// The templates used to render build files and
// dependency cache manifests.
//
//go:embed templates/*.txt
var templatesFS embed.FS

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"crateName":   crateName,
	"packageName": packageName,
}).ParseFS(templatesFS, "templates/*.txt"))

var crateNameReplacer = strings.NewReplacer("-", "_")

// crateName return the name in a form suitable
// for use as a Rust crate name
//
func crateName(name string) string {
	return crateNameReplacer.Replace(name)
}

// packageName return the name in a form suitable
// for use as a Go package name
//
func packageName(name string) string {
	return path.Base(name)
}

// RenderRustCrateBuildFile generates a build file
// for the given Rust crate.
//
func RenderRustCrateBuildFile(name string, crate *RustCrate) ([]byte, error) {
	var buf bytes.Buffer
	err := templates.ExecuteTemplate(&buf, "rust-BUILD.txt", crate)
	if err != nil {
		return nil, fmt.Errorf("failed to render build file: %v", err)
	}

	return starlark.Format(name, buf.Bytes())
}

// RenderGoPackageBuildFile generates a build file
// for the given Go package.
//
func RenderGoPackageBuildFile(name string, pkg *GoPackage) ([]byte, error) {
	var buf bytes.Buffer
	err := templates.ExecuteTemplate(&buf, "go-BUILD.txt", pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to render build file: %v", err)
	}

	return starlark.Format(name, buf.Bytes())
}

// RenderManifest generates a dependency manifest
// from the given set of dependencies.
//
func RenderManifest(name string, manifest *Deps) ([]byte, error) {
	var buf bytes.Buffer
	err := templates.ExecuteTemplate(&buf, "manifest.txt", manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to render cache manifest: %v", err)
	}

	return starlark.Format(name, buf.Bytes())
}
