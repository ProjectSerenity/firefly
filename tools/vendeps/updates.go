// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendeps

import (
	"context"
	"fmt"
	"os"

	"github.com/bazelbuild/buildtools/build"
	"golang.org/x/mod/semver"
)

// UpdateDependencies parses the given set of
// dependencies and checks each for an update,
// updating the document if possible.
//
// Note that UpdateDependencies does not modify
// the set of vendored dependencies, only the
// dependency specification.
//
func UpdateDependencies(name string) error {
	data, err := os.ReadFile(name)
	if err != nil {
		return err
	}

	f, err := build.ParseBzl(name, data)
	if err != nil {
		return err
	}

	deps, err := ParseUpdateDeps(name, f)
	if err != nil {
		return err
	}

	anyUpdated := false
	ctx := context.Background()
	for _, dep := range deps.Rust {
		updated, err := UpdateRustCrate(ctx, dep)
		if err != nil {
			return err
		}

		anyUpdated = anyUpdated || updated
	}

	for _, dep := range deps.Go {
		updated, err := UpdateGoModule(ctx, dep)
		if err != nil {
			return err
		}

		anyUpdated = anyUpdated || updated
	}

	if !anyUpdated {
		fmt.Println("No dependencies had updates available.")
		return nil
	}

	// We've updated the Starlark file's
	// AST, so we format it and write it
	// back out.
	data = build.Format(f)
	err = os.WriteFile(name, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write updates back to %s: %v", name, err)
	}

	return nil
}

// MajorUpdate returns true if the newer
// version has a different major number,
// or if both have major version 0 and
// the newer version has a different
// minor version.
//
func MajorUpdate(current, next string) bool {
	if semver.Major(current) == "v0" && semver.Major(next) == "v0" {
		return semver.Compare(current, next) == -1 && semver.MajorMinor(current) != semver.MajorMinor(next)
	}

	return semver.Compare(current, next) == -1 && semver.Major(current) != semver.Major(next)
}

// ParseUpdateDeps parses a deps.bzl file for
// the set of dependencies so they can be
// checked for updates.
//
func ParseUpdateDeps(filename string, f *build.File) (*UpdateDeps, error) {
	// pos is a helper for printing file:line prefixes
	// for error messages.
	pos := func(x build.Expr) string {
		start, _ := x.Span()
		return fmt.Sprintf("%s:%d", filename, start.Line)
	}

	var deps UpdateDeps
	for _, stmt := range f.Stmt {
		if _, ok := stmt.(*build.CommentBlock); ok {
			continue
		}

		// At the top level, we only allow assignments,
		// where the identifier being assigned to indicates
		// which field in the structure we populate.
		assign, ok := stmt.(*build.AssignExpr)
		if !ok {
			return nil, fmt.Errorf("%s: unexpected statement type: %T", pos(stmt), stmt)
		}

		lhs, ok := assign.LHS.(*build.Ident)
		if !ok {
			return nil, fmt.Errorf("%s: found assignment to %T, expected identifier", pos(assign.LHS), assign.LHS)
		}

		switch lhs.Name {
		case "rust", "go":
		default:
			return nil, fmt.Errorf("%s: found assignment to unrecognised identifier %q", pos(assign.LHS), lhs.Name)
		}

		list, ok := assign.RHS.(*build.ListExpr)
		if !ok {
			return nil, fmt.Errorf("%s: found assignment of %T to %s, expected list", pos(assign.RHS), assign.RHS, lhs.Name)
		}

		dep := make([]*UpdateDep, len(list.List))
		for i, elt := range list.List {
			call, ok := elt.(*build.CallExpr)
			if !ok {
				return nil, fmt.Errorf("%s: found dependency type %T, expected structure", pos(elt), elt)
			}

			var next UpdateDep
			for _, elt := range call.List {
				assign, ok := elt.(*build.AssignExpr)
				if !ok {
					return nil, fmt.Errorf("%s: found structure entry type %T, expected assignment", pos(elt), elt)
				}

				lhs, ok := assign.LHS.(*build.Ident)
				if !ok || lhs.Name == "True" || lhs.Name == "False" {
					typeName := fmt.Sprintf("%T", assign.LHS)
					if lhs != nil && lhs.Name != "" {
						typeName = "bool"
					}

					return nil, fmt.Errorf("%s: found structure field assignment to %s, expected identifier", pos(assign.LHS), typeName)
				}

				switch lhs.Name {
				case "name", "version":
				default:
					continue
				}

				str, ok := assign.RHS.(*build.StringExpr)
				if !ok {
					return nil, fmt.Errorf("%s: found assignment of %T to %s, expected string", pos(assign.RHS), assign.RHS, lhs.Name)
				}

				switch lhs.Name {
				case "name":
					if next.Name != "" {
						return nil, fmt.Errorf("%s: %s assigned to for the second time", pos(lhs), lhs.Name)
					}

					next.Name = str.Value
				case "version":
					if next.Version != nil {
						return nil, fmt.Errorf("%s: %s assigned to for the second time", pos(lhs), lhs.Name)
					}

					next.Version = &str.Value
				}
			}

			if next.Name == "" {
				return nil, fmt.Errorf("%s: dependency has no name", pos(call))
			}

			if next.Version == nil {
				return nil, fmt.Errorf("%s: dependency has no version", pos(call))
			}

			dep[i] = &next
		}

		switch lhs.Name {
		case "rust":
			if len(deps.Rust) != 0 {
				return nil, fmt.Errorf("%s: found %s for the second time", pos(assign.LHS), lhs.Name)
			}

			deps.Rust = dep
		case "go":
			if len(deps.Go) != 0 {
				return nil, fmt.Errorf("%s: found %s for the second time", pos(assign.LHS), lhs.Name)
			}

			deps.Go = dep
		}
	}

	return &deps, nil
}
