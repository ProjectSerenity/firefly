// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"testing"
)

func TestName(t *testing.T) {
	tests := []struct {
		name   Name
		camel  string
		pascal string
		snake  string
	}{
		{
			name:   Name{"foo"},
			camel:  "foo",
			pascal: "Foo",
			snake:  "foo",
		},
		{
			name:   Name{"FOO"},
			camel:  "foo",
			pascal: "Foo",
			snake:  "foo",
		},
		{
			name:   Name{"foo", "bar"},
			camel:  "fooBar",
			pascal: "FooBar",
			snake:  "foo_bar",
		},
		{
			name:   Name{"FOO", "BAR"},
			camel:  "fooBar",
			pascal: "FooBar",
			snake:  "foo_bar",
		},
		{
			name:   Name{"foo", "bar", "baz"},
			camel:  "fooBarBaz",
			pascal: "FooBarBaz",
			snake:  "foo_bar_baz",
		},
		{
			name:   Name{"FOO", "BAR", "BAZ"},
			camel:  "fooBarBaz",
			pascal: "FooBarBaz",
			snake:  "foo_bar_baz",
		},
	}

	check := func(t *testing.T, name Name, method string, fun func(Name) string, want string) {
		got := fun(name)
		if got != want {
			t.Helper()
			t.Errorf("name %#v.%s(): got %q, want %q", name, method, got, want)
		}
	}

	for _, test := range tests {
		check(t, test.name, "CamelCase", Name.CamelCase, test.camel)
		check(t, test.name, "PascalCase", Name.PascalCase, test.pascal)
		check(t, test.name, "SnakeCase", Name.SnakeCase, test.snake)
	}
}
