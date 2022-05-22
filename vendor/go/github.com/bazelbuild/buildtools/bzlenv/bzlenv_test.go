/*
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bzlenv

import (
	"strconv"
	"strings"
	"testing"

	"github.com/bazelbuild/buildtools/build"
)

func TestWalkEnvironment(t *testing.T) {
	input := `
a, b = 2, 3

def bar(x, y = a):
    b = 4
    c = a
    [a for a in [b, c]]
    if True:
        return foo()

def foo():
    pass
`

	expected := `
a0, b1 = 2, 3

def bar2(x4, y5 = a0):
    b6 = 4
    c7 = a0
    [a8 for a8 in [b6, c7]]
    if True:
        return foo3()

def foo3():
    pass
`

	var buildFile build.Expr
	buildFile, _ = build.Parse("test_file.bzl", []byte(input))

	var walk func(e *build.Expr, env *Environment)
	walk = func(e *build.Expr, env *Environment) {
		switch e := (*e).(type) {
		case *build.DefStmt:
			binding := env.Get(e.Name)
			if binding != nil {
				e.Name += strconv.Itoa(binding.ID)
			}
		case *build.Ident:
			binding := env.Get(e.Name)
			if binding != nil {
				e.Name += strconv.Itoa(binding.ID)
			}
		}
		WalkOnceWithEnvironment(*e, env, walk)
	}
	walk(&buildFile, NewEnvironment())

	output := strings.Trim(build.FormatString(buildFile), "\n")
	expected = strings.Trim(expected, "\n")
	if output != expected {
		t.Errorf("\nexpected:\n%s\ngot:\n%s", expected, output)
	}
}
