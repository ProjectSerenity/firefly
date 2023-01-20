// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendeps

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/bazelbuild/buildtools/build"
	"rsc.io/diff"
)

func stringPointer(s string) *string {
	return &s
}

func TestParseUpdateDeps(t *testing.T) {
	tests := []struct {
		Name string
		Text string
		Want *UpdateDeps
	}{
		{
			Name: "simple",
			Text: `
				go = [
					module(
						name = "rsc.io/quote",
						version = "v1.2.3",
						packages = [
							package(
								name = "rsc.io/quote",
							),
						],
					),
				]
			`,
			Want: &UpdateDeps{
				Go: []*UpdateDep{
					{
						Name:    "rsc.io/quote",
						Version: stringPointer("v1.2.3"),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			f, err := build.ParseBzl(depsBzl, []byte(test.Text))
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseUpdateDeps(depsBzl, f)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, test.Want) {
				g, err := json.MarshalIndent(got, "", "\t")
				if err != nil {
					t.Fatal(err)
				}

				w, err := json.MarshalIndent(test.Want, "", "\t")
				if err != nil {
					t.Fatal(err)
				}

				t.Fatalf("ParseUpdateDeps(): result mismatch:\n%s", diff.Format(string(g), string(w)))
			}
		})
	}
}

func TestMajorUpdate(t *testing.T) {
	tests := []struct {
		Name    string
		Current string
		Next    string
		Want    bool
	}{
		{
			Name:    "unstable older version",
			Current: "v0.1.2",
			Next:    "v0.1.1",
			Want:    false,
		},
		{
			Name:    "unstable equal version",
			Current: "v0.1.2",
			Next:    "v0.1.2",
			Want:    false,
		},
		{
			Name:    "unstable newer patch version",
			Current: "v0.1.2",
			Next:    "v0.1.3",
			Want:    false,
		},
		{
			Name:    "unstable newer minor version",
			Current: "v0.1.2",
			Next:    "v0.2.2",
			Want:    true,
		},
		{
			Name:    "unstable newer major version",
			Current: "v0.1.2",
			Next:    "v1.2.3",
			Want:    true,
		},
		{
			Name:    "stable older version",
			Current: "v1.2.3",
			Next:    "v1.1.3",
			Want:    false,
		},
		{
			Name:    "stable equal version",
			Current: "v1.2.3",
			Next:    "v1.2.3",
			Want:    false,
		},
		{
			Name:    "stable newer patch version",
			Current: "v1.2.3",
			Next:    "v1.2.4",
			Want:    false,
		},
		{
			Name:    "stable newer minor version",
			Current: "v1.2.3",
			Next:    "v1.3.3",
			Want:    false,
		},
		{
			Name:    "stable newer major version",
			Current: "v1.2.3",
			Next:    "v2.2.3",
			Want:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := MajorUpdate(test.Current, test.Next)
			if got != test.Want {
				t.Fatalf("MajorUpdate(%q, %q): got %v, want %v", test.Current, test.Next, got, test.Want)
			}
		})
	}
}

func TestParseUpdateDepsErrors(t *testing.T) {
	tests := []struct {
		Name string
		Text string
		Want string
	}{
		{
			Name: "top level function call",
			Text: `go()`,
			Want: `unexpected statement type: *build.CallExpr`,
		},
		{
			Name: "top level assignment to string",
			Text: `"go" = "bar"`,
			Want: `found assignment to *build.StringExpr, expected identifier`,
		},
		{
			Name: "top level assignment unrecognised identifier",
			Text: `javascript = ["bar"]`,
			Want: `found assignment to unrecognised identifier "javascript"`,
		},
		{
			Name: "invalid input type",
			Text: `go = True`,
			Want: `found assignment of *build.Ident to go, expected list`,
		},
		{
			Name: "invalid list element type",
			Text: `go = ["bar"]`,
			Want: `found dependency type *build.StringExpr, expected structure`,
		},
		{
			Name: "invalid dependency type",
			Text: `go = [foo("bar")]`,
			Want: `found structure entry type *build.StringExpr, expected assignment`,
		},
		{
			Name: "invalid dependency field type",
			Text: `go = [foo(True = "bar")]`,
			Want: `found structure field assignment to bool, expected identifier`,
		},
		{
			Name: "invalid dependency name type",
			Text: `go = [foo(name = True)]`,
			Want: `found assignment of *build.Ident to name, expected string`,
		},
		{
			Name: "dependency with duplicate name",
			Text: `go = [foo(name = "a", name = "b")]`,
			Want: `name assigned to for the second time`,
		},
		{
			Name: "dependency with duplicate version",
			Text: `go = [foo(version = "a", version = "b")]`,
			Want: `version assigned to for the second time`,
		},
		{
			Name: "dependency with no name",
			Text: `go = [foo(version = "a")]`,
			Want: `dependency has no name`,
		},
		{
			Name: "dependency with no version",
			Text: `go = [foo(name = "a")]`,
			Want: `dependency has no version`,
		},
		{
			Name: "file with duplicate Go",
			Text: `go = [foo(name = "a", version = "b")]` + "\n" +
				`go = [bar(name = "c", version = "d")]`,
			Want: `found go for the second time`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			f, err := build.ParseBzl(depsBzl, []byte(test.Text))
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseUpdateDeps(depsBzl, f)
			if err == nil {
				g, err := json.MarshalIndent(got, "", "\t")
				if err != nil {
					t.Fatal(err)
				}

				t.Fatalf("ParseUpdateDeps(): unexpected success: got %s", g)
			}

			if e := err.Error(); !strings.HasSuffix(e, test.Want) {
				t.Fatalf("ParseUpdateDeps():\nGot:  %s\nWant: %s", e, test.Want)
			}
		})
	}
}
