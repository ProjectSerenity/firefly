// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package starlark provides helper functionality for
// processing Starlark files.
package starlark

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/bazelbuild/buildtools/build"
)

// Format pretty-prints the Starlark content in data.
// The filename is only used to improve any error messages.
func Format(filename string, data []byte) ([]byte, error) {
	// Parse it into a file, so we can auto-format
	// it.
	file, err := build.Parse(filename, data)
	if err != nil {
		return nil, err
	}

	pretty := build.Format(file)
	return pretty, nil
}

// Unmarshal parses a Starlark file into structured Go data.
// The filename is only used to improve any error messages.
func Unmarshal(filename string, data []byte, v any) error {
	f, err := build.ParseBzl(filename, data)
	if err != nil {
		return err
	}

	// pos is a helper for printing file:line prefixes
	// for error messages.
	pos := func(x build.Expr) string {
		start, _ := x.Span()
		return fmt.Sprintf("%s:%d", filename, start.Line)
	}

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return fmt.Errorf("starlark.Unmarshal(): invalid value type: got %v, expected struct", val.Kind())
	}

	fields := val.NumField()
	structType := val.Type()
	for _, stmt := range f.Stmt {
		if _, ok := stmt.(*build.CommentBlock); ok {
			continue
		}

		// At the top level, we only allow assignments,
		// where the identifier being assigned to indicates
		// which field in the structure we populate.
		assign, ok := stmt.(*build.AssignExpr)
		if !ok {
			return fmt.Errorf("%s: unexpected statement type: %T", pos(stmt), stmt)
		}

		lhs, ok := assign.LHS.(*build.Ident)
		if !ok {
			return fmt.Errorf("%s: found assignment to %T, expected identifier", pos(assign.LHS), assign.LHS)
		}

		// Find the structure field with the right tag.
		found := false
		for i := 0; i < fields; i++ {
			fieldType := structType.Field(i)
			tag, ok := fieldType.Tag.Lookup("bzl")
			if !ok {
				// We ignore fields without a tag.
				continue
			}

			// Lists of function calls are tagged
			// as name/type, where name is the name
			// of the identifier being assigned to
			// and type is the name of the function
			// to expect in the list.
			tag, structName, ok := strings.Cut(tag, "/")
			if ok && structName == "" {
				return fmt.Errorf("%T.%s has an invalid tag: structure name cannot be empty", v, fieldType.Name)
			}

			if tag != lhs.Name {
				continue
			}

			found = true
			err = unmarshal(filename, assign.RHS, tag, structName, val.Field(i))
			if err != nil {
				return err
			}

			break
		}

		if !found {
			return fmt.Errorf("%s: assignment to %s: not found in %T", pos(assign.LHS), lhs.Name, v)
		}
	}

	return nil
}

func unmarshal(filename string, x build.Expr, name, structName string, v reflect.Value) error {
	// pos is a helper for printing file:line prefixes
	// for error messages.
	pos := func(x build.Expr) string {
		start, _ := x.Span()
		return fmt.Sprintf("%s:%d", filename, start.Line)
	}

	switch expr := x.(type) {
	case *build.Ident:
		if expr.Name != "True" && expr.Name != "False" {
			return fmt.Errorf("%s: found identifier value %q, want bool", pos(x), expr.Name)
		}

		if v.Kind() != reflect.Bool {
			return fmt.Errorf("%s: found bool value for %s, want %s", pos(x), name, v.Kind())
		}

		if structName != "" {
			return fmt.Errorf("%s: found %s value with structure name %q in tag", pos(x), v.Kind(), structName)
		}

		v.SetBool(expr.Name == "True")
	case *build.StringExpr:
		if v.Kind() != reflect.String {
			return fmt.Errorf("%s: found string value for %s, want %s", pos(x), name, v.Kind())
		}

		if structName != "" {
			return fmt.Errorf("%s: found %s value with structure name %q in tag", pos(x), v.Kind(), structName)
		}

		v.SetString(expr.Value)
	case *build.ListExpr:
		if v.Kind() != reflect.Slice {
			return fmt.Errorf("%s: found list value for %s, want %s", pos(expr), name, v.Kind())
		}

		elemType := v.Type().Elem()
		sliceType := reflect.SliceOf(elemType)
		v.Set(reflect.MakeSlice(sliceType, len(expr.List), len(expr.List)))
		for i, elt := range expr.List {
			err := unmarshal(filename, elt, fmt.Sprintf("%s[%d]", name, i), structName, v.Index(i))
			if err != nil {
				return err
			}
		}
	case *build.DictExpr:
		if v.Kind() != reflect.Map {
			return fmt.Errorf("%s: found dict value for %s, want %s", pos(expr), name, v.Kind())
		}

		keyType := v.Type().Key()
		elemType := v.Type().Elem()
		mapType := reflect.MapOf(keyType, elemType)
		v.Set(reflect.MakeMapWithSize(mapType, len(expr.List)))
		for _, elt := range expr.List {
			key := reflect.New(keyType)
			err := unmarshal(filename, elt.Key, fmt.Sprintf("%s key", name), "", key.Elem())
			if err != nil {
				return err
			}

			val := reflect.New(elemType)
			err = unmarshal(filename, elt.Value, fmt.Sprintf("%s value", name), structName, val.Elem())
			if err != nil {
				return err
			}

			v.SetMapIndex(key.Elem(), val.Elem())
		}
	case *build.CallExpr:
		if v.Kind() != reflect.Struct && (v.Kind() != reflect.Pointer || v.Type().Elem().Kind() != reflect.Struct) {
			return fmt.Errorf("%s: found structure value for %s, want %s", pos(expr), name, v.Kind())
		}

		if fun, ok := expr.X.(*build.Ident); !ok {
			return fmt.Errorf("%s: found structure type %T, expected identifier", pos(expr.X), expr.X)
		} else if fun.Name != structName {
			return fmt.Errorf("%s: found structure type %s, want %s", pos(expr.X), fun.Name, structName)
		}

		structType := v.Type()
		if v.Type().Kind() == reflect.Pointer {
			structType = structType.Elem()
			v.Set(reflect.New(structType))
			v = v.Elem()
		}

		fields := structType.NumField()
		for _, elt := range expr.List {
			assign, ok := elt.(*build.AssignExpr)
			if !ok {
				return fmt.Errorf("%s: found structure field with %T type, want assignment", pos(elt), elt)
			}

			lhs, ok := assign.LHS.(*build.Ident)
			if !ok || lhs.Name == "True" || lhs.Name == "False" {
				typeName := fmt.Sprintf("%T", assign.LHS)
				if lhs != nil && lhs.Name != "" {
					typeName = "bool"
				}

				return fmt.Errorf("%s: found assignment to %s, expected identifier", pos(assign.LHS), typeName)
			}

			// Find the structure field with the right tag.
			found := false
			for i := 0; i < fields; i++ {
				fieldType := structType.Field(i)
				tag, ok := fieldType.Tag.Lookup("bzl")
				if !ok {
					// We ignore fields without a tag.
					continue
				}

				// Lists of function calls are tagged
				// as name/type, where name is the name
				// of the identifier being assigned to
				// and type is the name of the function
				// to expect in the list.
				tag, structName, ok := strings.Cut(tag, "/")
				if ok && structName == "" {
					return fmt.Errorf("%T.%s has an invalid tag: structure name cannot be empty", v, fieldType.Name)
				}

				if tag != lhs.Name {
					continue
				}

				found = true
				err := unmarshal(filename, assign.RHS, tag, structName, v.Field(i))
				if err != nil {
					return err
				}

				break
			}

			if !found {
				return fmt.Errorf("%s: assignment to %s: not found in %s", pos(assign.LHS), lhs.Name, structType.Name())
			}
		}
	default:
		return fmt.Errorf("%s: unexpected Starlark value of type %T", pos(x), x)
	}

	return nil
}
