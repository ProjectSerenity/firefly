// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendeps

import (
	"bytes"
	"embed"
	"fmt"
	"path"
	"text/template"

	"firefly-os.dev/tools/starlark"
)

// The templates used to render build files and
// dependency cache manifests.
//
//go:embed templates/*.txt
var templatesFS embed.FS

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"packageName": packageName,
}).ParseFS(templatesFS, "templates/*.txt"))

// packageName return the name in a form suitable
// for use as a Go package name
func packageName(name string) string {
	return path.Base(name)
}

// RenderGoPackageBuildFile generates a build file
// for the given Go package.
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
func RenderManifest(name string, manifest *Deps) ([]byte, error) {
	var buf bytes.Buffer
	err := templates.ExecuteTemplate(&buf, "manifest.txt", manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to render cache manifest: %v", err)
	}

	return starlark.Format(name, buf.Bytes())
}
