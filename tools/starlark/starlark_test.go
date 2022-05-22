// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package starlark

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"rsc.io/diff"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		Name string
		Text string
		Want string
	}{
		{
			Name: "simple expression",
			Text: `foo=True`,
			Want: `foo = True`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := Format("test.bzl", []byte(test.Text))
			if err != nil {
				t.Fatal(err)
			}

			want := test.Want + "\n"
			if string(got) != want {
				t.Fatalf("Format(): output mismatch:\n%s", diff.Format(string(got), want))
			}
		})
	}
}

func TestFormatErrors(t *testing.T) {
	tests := []struct {
		Name string
		Text string
		Want string
	}{
		{
			Name: "syntax error",
			Text: `foo=[`,
			Want: `syntax error`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := Format("test.bzl", []byte(test.Text))
			if err == nil {
				t.Fatalf("Format(): unexpected success: got %q", got)
			}

			if e := err.Error(); !strings.HasSuffix(e, test.Want) {
				t.Fatalf("Format():\nGot:  %s\nWant: %s", e, test.Want)
			}
		})
	}
}

type (
	testBool struct {
		Foo bool   `bzl:"foo"`
		Bar string `bzl:"bar"`
	}
	testString struct {
		Foo string `bzl:"foo"`
	}
	testStringsSlice struct {
		Bar bool
		Foo []string `bzl:"Bar"`
	}
	testStringsMap struct {
		Foo map[string]string `bzl:"foo"`
	}
	testSubStruct struct {
		Name   string `bzl:"name"`
		Test   bool   `bzl:"test"`
		Other  string
		Ignore string `bzl:"Other"`
	}
	testStruct struct {
		Foo testSubStruct `bzl:"foo/sub"`
	}
	testStructPtr struct {
		Foo *testSubStruct `bzl:"foo/sub"`
	}
	testStructsSlice struct {
		Foo []testSubStruct `bzl:"foo/sub"`
	}
	testStructPtrsSlice struct {
		Foo []*testSubStruct `bzl:"foo/sub"`
	}
	testNestedStructsSlice struct {
		Bar []testStructsSlice `bzl:"bar/nest"`
	}
	testNestedStructPtrsSlice struct {
		Bar []*testStructsSlice `bzl:"bar/nest"`
	}
)

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		Name string
		Text string
		In   any
		Want any
	}{
		{
			Name: "bool",
			Text: "# Foo is a bool.\n# Another comment.\n\nfoo = True",
			In:   new(testBool),
			Want: &testBool{Foo: true},
		},
		{
			Name: "string",
			Text: `foo = "foo"`,
			In:   new(testString),
			Want: &testString{Foo: "foo"},
		},
		{
			Name: "strings slice",
			Text: `Bar = ["bar", "baz"]`,
			In:   new(testStringsSlice),
			Want: &testStringsSlice{Foo: []string{"bar", "baz"}},
		},
		{
			Name: "strings map",
			Text: `foo = {"bar": "baz", "one": "1"}`,
			In:   new(testStringsMap),
			Want: &testStringsMap{Foo: map[string]string{"bar": "baz", "one": "1"}},
		},
		{
			Name: "struct",
			Text: `foo = sub(name = "bar", test = True, Other = "baz")`,
			In:   new(testStruct),
			Want: &testStruct{Foo: testSubStruct{Name: "bar", Test: true, Ignore: "baz"}},
		},
		{
			Name: "struct pointer",
			Text: `foo = sub(name = "bar", test = True)`,
			In:   new(testStructPtr),
			Want: &testStructPtr{Foo: &testSubStruct{Name: "bar", Test: true}},
		},
		{
			Name: "structs slice",
			Text: `foo = [
			    sub(name = "bar", test = True),
			    sub(name = "two", test = False),
			]`,
			In: new(testStructsSlice),
			Want: &testStructsSlice{Foo: []testSubStruct{
				{Name: "bar", Test: true},
				{Name: "two", Test: false},
			}},
		},
		{
			Name: "structs pointer slice",
			Text: `foo = [
			    sub(name = "bar", test = True),
			    sub(name = "two", test = False),
			]`,
			In: new(testStructPtrsSlice),
			Want: &testStructPtrsSlice{Foo: []*testSubStruct{
				{Name: "bar", Test: true},
				{Name: "two", Test: false},
			}},
		},
		{
			Name: "nested structs slice",
			Text: `bar = [
				nest(
					foo = [
						sub(name = "bar", test = True),
						sub(name = "two", test = False),
					],
				),
			]`,
			In: new(testNestedStructsSlice),
			Want: &testNestedStructsSlice{
				Bar: []testStructsSlice{
					{
						Foo: []testSubStruct{
							{Name: "bar", Test: true},
							{Name: "two", Test: false},
						},
					},
				},
			},
		},
		{
			Name: "nested structs pointer slice",
			Text: `bar = [
				nest(
					foo = [
						sub(name = "bar", test = True),
						sub(name = "two", test = False),
					],
				),
			]`,
			In: new(testNestedStructPtrsSlice),
			Want: &testNestedStructPtrsSlice{
				Bar: []*testStructsSlice{
					{
						Foo: []testSubStruct{
							{Name: "bar", Test: true},
							{Name: "two", Test: false},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			err := Unmarshal("test.bzl", []byte(test.Text), test.In)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(test.In, test.Want) {
				g, err := json.MarshalIndent(test.In, "", "\t")
				if err != nil {
					t.Fatal(err)
				}

				w, err := json.MarshalIndent(test.Want, "", "\t")
				if err != nil {
					t.Fatal(err)
				}

				t.Fatalf("UnmarshalStarlark(): result mismatch:\n%s", diff.Format(string(g), string(w)))
			}
		})
	}
}

func TestUnmarshalErrors(t *testing.T) {
	tests := []struct {
		Name string
		Text string
		In   any
		Want string
	}{
		// Unmarshal errors.
		{
			Name: "syntax error",
			Text: `foo = [`,
			In:   new(testBool),
			Want: `syntax error`,
		},
		{
			Name: "invalid input type",
			Text: `foo = True`,
			In:   new(int),
			Want: `invalid value type: got int, expected struct`,
		},
		{
			Name: "top level function call",
			Text: `foo()`,
			In:   new(testBool),
			Want: `unexpected statement type: *build.CallExpr`,
		},
		{
			Name: "top level assignment to string",
			Text: `"foo" = "bar"`,
			In:   new(testBool),
			Want: `found assignment to *build.StringExpr, expected identifier`,
		},
		{
			Name: "invalid structure tag",
			Text: `foo = "bar"`,
			In: new(struct {
				Foo string `bzl:"foo/"`
			}),
			Want: `.Foo has an invalid tag: structure name cannot be empty`,
		},
		{
			Name: "field not found",
			Text: `bar = "baz"`,
			In:   new(testString),
			Want: `assignment to bar: not found in *starlark.testString`,
		},
		// unmarshal errors.
		{
			Name: "wrong identifier for bool",
			Text: `foo = bar`,
			In:   new(testBool),
			Want: `found identifier value "bar", want bool`,
		},
		{
			Name: "wrong type for bool",
			Text: `foo = True`,
			In:   new(testString),
			Want: `found bool value for foo, want string`,
		},
		{
			Name: "struct name for bool",
			Text: `foo = True`,
			In: new(struct {
				Foo bool `bzl:"foo/bar"`
			}),
			Want: `found bool value with structure name "bar" in tag`,
		},
		{
			Name: "wrong type for string",
			Text: `foo = "bar"`,
			In:   new(testBool),
			Want: `found string value for foo, want bool`,
		},
		{
			Name: "struct name for string",
			Text: `foo = "bar"`,
			In: new(struct {
				Foo string `bzl:"foo/bar"`
			}),
			Want: `found string value with structure name "bar" in tag`,
		},
		{
			Name: "wrong type for slice",
			Text: `foo = ["bar"]`,
			In:   new(testBool),
			Want: `found list value for foo, want bool`,
		},
		{
			Name: "wrong type for slice element",
			Text: `Bar = ["bar", True]`,
			In:   new(testStringsSlice),
			Want: `found bool value for Bar[1], want string`,
		},
		{
			Name: "wrong type for map",
			Text: `foo = {"bar": "baz"}`,
			In:   new(testBool),
			Want: `found dict value for foo, want bool`,
		},
		{
			Name: "wrong type for map key",
			Text: `foo = {True: "baz"}`,
			In:   new(testStringsMap),
			Want: `found bool value for foo key, want string`,
		},
		{
			Name: "wrong type for map value",
			Text: `foo = {"bar": True}`,
			In:   new(testStringsMap),
			Want: `found bool value for foo value, want string`,
		},
		{
			Name: "wrong type for structure",
			Text: `foo = sub(bar = "baz")`,
			In:   new(testBool),
			Want: `found structure value for foo, want bool`,
		},
		{
			Name: "wrong structure type for structure",
			Text: `foo = a.b(bar = "baz")`,
			In:   new(testStruct),
			Want: `found structure type *build.DotExpr, expected identifier`,
		},
		{
			Name: "wrong structure name for structure",
			Text: `foo = other(bar = "baz")`,
			In:   new(testStruct),
			Want: `found structure type other, want sub`,
		},
		{
			Name: "wrong structure field expression type",
			Text: `foo = sub("baz")`,
			In:   new(testStruct),
			Want: `found structure field with *build.StringExpr type, want assignment`,
		},
		{
			Name: "wrong structure field type",
			Text: `foo = sub(True = "baz")`,
			In:   new(testStruct),
			Want: `found assignment to bool, expected identifier`,
		},
		{
			Name: "invalid structure field tag",
			Text: `foo = [sub(bar = "bar")]`,
			In: new(struct {
				Foo []struct {
					Tests []testSubStruct `bzl:"tests/"`
				} `bzl:"foo/sub"`
			}),
			Want: `.Tests has an invalid tag: structure name cannot be empty`,
		},
		{
			Name: "structure field not found",
			Text: `foo = sub(other = "baz")`,
			In:   new(testStruct),
			Want: `assignment to other: not found in testSubStruct`,
		},
		{
			Name: "wrong nested structure field type",
			Text: `bar = [nest(foo = [sub(True = "baz")])]`,
			In:   new(testNestedStructPtrsSlice),
			Want: `found assignment to bool, expected identifier`,
		},
		{
			Name: "unexpected type",
			Text: `foo = 4`,
			In:   new(testBool),
			Want: `unexpected Starlark value of type *build.LiteralExpr`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			err := Unmarshal("test.bzl", []byte(test.Text), test.In)
			if err == nil {
				g, err := json.MarshalIndent(test.In, "", "\t")
				if err != nil {
					t.Fatal(err)
				}

				t.Fatalf("Unmarshal(): unexpected success: got %s", g)
			}

			if e := err.Error(); !strings.HasSuffix(e, test.Want) {
				t.Fatalf("Unmarshal():\nGot:  %s\nWant: %s", e, test.Want)
			}
		})
	}
}
