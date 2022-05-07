// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package format contains functionality to format a Plan document into the
// canonical style.
//
package format

import (
	"bytes"
	"fmt"
	"io"
	"sort"

	"github.com/ProjectSerenity/firefly/tools/plan/ast"
	"github.com/ProjectSerenity/firefly/tools/plan/types"
)

// maxListWidth is the maximum width in bytes for a list.
//
// If a list is wider than this value, it will be split
// into a vertical list with all entries after the first
// indented by one. If the list is narrower than this,
// it will be printed on one line, separated by a space.
//
const maxListWidth = 80

// containsMultipleLists returns whether a set of expressions
// contains more than one list.
//
// Alongside maxListWidth above, this is used to determine
// whether to split a list onto multiple lines. We don't
// want to have multiple lists on a single line, as it can
// become quite hard to read.
//
func containsMultipleLists(elts []ast.Expr) bool {
	lists := 0
	for _, elt := range elts {
		if _, ok := elt.(*ast.List); ok {
			lists++
			if lists > 1 {
				return true
			}
		}
	}

	return false
}

// SortFields ensures that the fields in each declaration
// are sorted into the standard order.
//
// The list of fields, in order, includes:
//
// - name (all)
// - docs (all)
// - field (structure)
// - type (enumeration, field, parameter)
// - padding (field)
// - value (enumeration)
// - argN (syscall)
// - resultN (syscall)
//
func SortFields(file *ast.File, arch types.Arch) error {
	// Sort the fields before pretty-printing.
	//
	// This is easiest done by interpreting the
	// file first, so that we can only sort the
	// fields we care about and makes it easier
	// to find those fields.
	//
	// It does mean we'll refuse to format files
	// that are syntactically valid but have
	// semantic errors, but this shouldn't matter
	// much.
	prog, err := types.Interpret("", file, arch)
	if err != nil {
		return err
	}

	// This is the order for fields.
	order := map[string]int{
		// These fields are always present and
		// always comes first.
		"name": 1,
		"docs": 2,
		// These fields are present in some
		// types but not others.
		"field":   3,
		"type":    4,
		"padding": 5,
		"value":   6,
		// Leave some space before these, which
		// always come last.
		"arg1":    20,
		"arg2":    21,
		"arg3":    22,
		"arg4":    23,
		"arg5":    24,
		"arg6":    25,
		"result1": 26,
		"result2": 27,
	}

	sortList := func(typ string, list *ast.List) {
		sort.SliceStable(list.Elements[1:], func(i, j int) bool {
			namei := list.Elements[i+1].(*ast.List).Elements[0].(*ast.Identifier).Name
			namej := list.Elements[j+1].(*ast.List).Elements[0].(*ast.Identifier).Name
			priorityi := order[namei]
			priorityj := order[namej]
			if priorityi == 0 {
				panic("unrecognised " + typ + " field: " + namei)
			}
			if priorityj == 0 {
				panic("unrecognised " + typ + " field: " + namej)
			}

			return priorityi < priorityj
		})
	}

	for _, enumeration := range prog.Enumerations {
		sortList("enumeration", enumeration.Node)
	}

	for _, structure := range prog.Structures {
		sortList("structure", structure.Node)
		for _, field := range structure.Fields {
			sortList("field", field.Node)
		}
	}

	for _, syscall := range prog.Syscalls {
		sortList("syscall", syscall.Node)
		for _, arg := range syscall.Args {
			sortList("arg", arg.Node)
		}
		for _, result := range syscall.Results {
			sortList("result", result.Node)
		}
	}

	return nil
}

// Fprint writes the file to w, according to the standard
// style.
//
func Fprint(w io.Writer, file *ast.File) error {
	allocated := false
	var buf *bytes.Buffer
	if b, ok := w.(*bytes.Buffer); ok {
		buf = b
	} else {
		allocated = true
		buf = new(bytes.Buffer)
	}

	// We don't know the order in which comments
	// and expressions are interleaved, so we
	// track the position of the next node and
	// of each type and print the earlier of the
	// two.
	//
	// We make a copy of the two slices so we
	// can advance them to track our progress
	// without modifying the file.
	comments := make([]*ast.CommentGroup, len(file.Comments))
	copy(comments, file.Comments)
	lists := make([]*ast.List, len(file.Lists))
	copy(lists, file.Lists)

	nextNode := func() (comment *ast.CommentGroup, list *ast.List) {
		if len(comments) == 0 {
			// Must be a list.
			list = lists[0]
			lists = lists[1:]
			return nil, list
		} else if len(lists) == 0 {
			// Must be a comment.
			comment = comments[0]
			comments = comments[1:]
			return comment, nil
		}

		// Pick the one with the smaller
		// offset.
		nextComment := comments[0].Pos()
		nextList := lists[0].Pos()
		if nextComment.Offset() < nextList.Offset() {
			comment = comments[0]
			comments = comments[1:]
			return comment, nil
		}

		list = lists[0]
		lists = lists[1:]
		return nil, list
	}

	first := true
	for len(comments) != 0 || len(lists) != 0 {
		comment, list := nextNode()
		if first {
			first = false
		} else {
			// Add a two-line gap between
			// statements.
			buf.WriteString("\n\n")
		}

		if comment != nil {
			fprintCommentGroup(buf, 0, comment)
		} else {
			fprintExpr(buf, 0, list)
		}

		buf.WriteByte('\n')
	}

	if allocated {
		_, err := w.Write(buf.Bytes())
		return err
	}

	return nil
}

// 100 tabs.
const tabs = "\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t" +
	"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t" +
	"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t" +
	"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t"

// fprintCommentGroup writes the comment group to buf, using
// the simplify method to style the comments.
//
func fprintCommentGroup(buf *bytes.Buffer, indentation int, group *ast.CommentGroup) {
	lines := group.Lines()
	for i, line := range lines {
		if i > 0 {
			buf.WriteByte('\n')
			buf.WriteString(tabs[:indentation])
		}

		if line == "" {
			buf.WriteByte(';')
		} else {
			buf.WriteString("; ")
			buf.WriteString(line)
		}
	}
}

// fprintExpr writes the node to buf, with the given indentation.
//
// fprintExpr does not write any spacing around the node.
//
func fprintExpr(buf *bytes.Buffer, indentation int, expr ast.Expr) {
	switch x := expr.(type) {
	case *ast.Identifier:
		buf.WriteString(x.Name)
	case *ast.String:
		buf.WriteString(x.Text)
	case *ast.Number:
		buf.WriteString(x.Value)
	case *ast.Pointer:
		buf.WriteByte('*')
		buf.WriteString(x.Note)
	case *ast.List:
		width := x.Width()
		buf.WriteByte('(')
		if width <= maxListWidth && !containsMultipleLists(x.Elements) {
			// Nice and simple, all on one line.
			for i, elt := range x.Elements {
				if i > 0 {
					buf.WriteByte(' ')
				}

				fprintExpr(buf, indentation+1, elt)
			}
		} else {
			// The opening parenthesis and the first
			// element go straight away, then we add
			// a newline and indent for all subsequent
			// elements.
			for i, elt := range x.Elements {
				if i > 0 {
					buf.WriteByte('\n')
					buf.WriteString(tabs[:indentation+1])
				}

				fprintExpr(buf, indentation+1, elt)
			}
		}

		buf.WriteByte(')')
	default:
		panic(fmt.Sprintf("unexpected expression %#v", expr))
	}
}
