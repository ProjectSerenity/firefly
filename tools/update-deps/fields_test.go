// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"strings"
	"testing"

	"github.com/bazelbuild/buildtools/build"
)

func checkField(t *testing.T, context, name string, got StringField, want string) {
	if got.Value != want {
		t.Helper()
		t.Errorf("%s has %s %q, want %q", context, name, got.Value, want)
	}
}

func TestUnmarshalFields(t *testing.T) {
	type SampleData struct {
		Foo StringField
		Bar StringField `bzl:"bar"`
		Baz StringField `bzl:"baz,optional"`
	}

	tests := []struct {
		Name     string
		Starlark string
		Want     SampleData
		Updated  string
	}{
		{
			Name:     "full",
			Starlark: `foo(Foo = "foo", bar = "bar", baz = "baz")`,
			Want: SampleData{
				Foo: StringField{Value: "foo"},
				Bar: StringField{Value: "bar"},
				Baz: StringField{Value: "baz"},
			},
			Updated: `foo(Foo = "foo1", bar = "bar2", baz = "baz3")`,
		},
		{
			Name:     "required",
			Starlark: `foo(Foo = "foo", bar = "bar")`,
			Want: SampleData{
				Foo: StringField{Value: "foo"},
				Bar: StringField{Value: "bar"},
				Baz: StringField{Value: ""},
			},
			Updated: `foo(Foo = "foo1", bar = "bar2")`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			f, err := build.ParseBzl("test.bzl", []byte(test.Starlark))
			if err != nil {
				t.Fatalf("bad Starlark: %v", err)
			}

			if len(f.Stmt) != 1 {
				t.Fatalf("bad Starlark: got %d statements, want 1", len(f.Stmt))
			}

			call, ok := f.Stmt[0].(*build.CallExpr)
			if !ok {
				t.Fatalf("bad Starlark: got %T, want *build.CallExpr", f.Stmt[0])
			}

			var got SampleData
			err = UnmarshalFields(call, &got)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Foo.Value != test.Want.Foo.Value || got.Bar.Value != test.Want.Bar.Value || got.Baz.Value != test.Want.Baz.Value {
				t.Fatalf("bad result:\nGot:  %#v\nWant: %#v", got, test.Want)
			}

			if got.Foo.Ptr != nil {
				*got.Foo.Ptr += "1"
			}
			if got.Bar.Ptr != nil {
				*got.Bar.Ptr += "2"
			}
			if got.Baz.Ptr != nil {
				*got.Baz.Ptr += "3"
			}

			pretty := build.Format(f)
			updated := strings.TrimSpace(string(pretty))
			if updated != test.Updated {
				t.Fatalf("bad updated:\nGot:  %s\nWant: %s", updated, test.Updated)
			}
		})
	}
}

func TestUnmarshalFieldsError(t *testing.T) {
	tests := []struct {
		Name     string
		Starlark string
		Data     interface{}
		Error    string
	}{
		{
			Name:     "bad input type",
			Starlark: `foo()`,
			Data:     1,
			Error:    "expected struct",
		},
		{
			Name:     "bad field type",
			Starlark: `foo()`,
			Data: &struct {
				Foo int
			}{},
			Error: "field Foo has unexpected type int",
		},
		{
			Name:     "unnamed field",
			Starlark: `foo()`,
			Data: &struct {
				Foo StringField `bzl:""`
			}{},
			Error: "field Foo has no field name",
		},
		{
			Name:     "unnamed optional field",
			Starlark: `foo()`,
			Data: &struct {
				Foo StringField `bzl:",optional"`
			}{},
			Error: "field Foo has no field name",
		},
		{
			Name:     "field name clash",
			Starlark: `foo()`,
			Data: &struct {
				Foo StringField
				Bar StringField `bzl:"Foo"`
			}{},
			Error: "multiple fields have the name \"Foo\"",
		},
		{
			Name:     "non assignment",
			Starlark: `foo(1)`,
			Data: &struct {
				Foo StringField
			}{},
			Error: "field 0 in the call is not an assignment",
		},
		{
			Name:     "unexpected field",
			Starlark: `foo(bar = 1)`,
			Data: &struct {
				Foo StringField
			}{},
			Error: "field 0 in the call has unexpected field \"bar\"",
		},
		{
			Name:     "duplicate field",
			Starlark: `foo(bar = "1", bar = "2")`,
			Data: &struct {
				Bar StringField `bzl:"bar"`
			}{},
			Error: "field 1 in the call assigns to bar for the second time",
		},
		{
			Name:     "non-string field value",
			Starlark: `foo(bar = 1)`,
			Data: &struct {
				Bar StringField `bzl:"bar"`
			}{},
			Error: "field 0 in the call (bar) has non-string value",
		},
		{
			Name:     "required field missing",
			Starlark: `foo(bar = "1")`,
			Data: &struct {
				Foo StringField
				Bar StringField `bzl:"bar"`
			}{},
			Error: "function call had no value for required field Foo",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			f, err := build.ParseBzl("test.bzl", []byte(test.Starlark))
			if err != nil {
				t.Fatalf("bad Starlark: %v", err)
			}

			if len(f.Stmt) != 1 {
				t.Fatalf("bad Starlark: got %d statements, want 1", len(f.Stmt))
			}

			call, ok := f.Stmt[0].(*build.CallExpr)
			if !ok {
				t.Fatalf("bad Starlark: got %T, want *build.CallExpr", f.Stmt[0])
			}

			err = UnmarshalFields(call, test.Data)
			if err == nil {
				t.Fatalf("no error: got data %#v", test.Data)
			}

			errString := err.Error()
			if !strings.Contains(errString, test.Error) {
				t.Fatalf("bad error: got %q, want %q", errString, test.Error)
			}
		})
	}
}
