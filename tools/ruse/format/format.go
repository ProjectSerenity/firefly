// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package format contains functionality to format a Ruse file into the
// canonical style.
package format

import (
	"bytes"
	"fmt"
	"io"
	"sort"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/token"
)

// maxListWidth is the maximum width in bytes for a list.
//
// If a list is wider than this value, it will be split
// into a vertical list with all entries after the first
// indented by one. If the list is narrower than this,
// it will be printed on one line, separated by a space.
const maxListWidth = 80

// containsMultipleLists returns whether a set of expressions
// contains more than one list.
//
// Alongside maxListWidth above, this is used to determine
// whether to split a list onto multiple lines. We don't
// want to have multiple lists on a single line, as it can
// become quite hard to read.
func containsMultipleLists(elts []ast.Expression) bool {
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

// SortAnnotations ensures that the annotations in each
// list are sorted into alphabetical order.
func SortAnnotations(file *ast.File) {
	// Scan the whole file.
	ast.Inspect(file, func(n ast.Node) bool {
		// Look for lists.
		list, ok := n.(*ast.List)
		if !ok {
			return true
		}

		// We must use a stable sort so we don't
		// reorder any parameter annotations.
		sort.SliceStable(list.Annotations, func(i, j int) bool {
			return list.Annotations[i].X.Elements[0].(*ast.Identifier).Name < list.Annotations[j].X.Elements[0].(*ast.Identifier).Name
		})

		return true
	})
}

// Fprint writes the file to w, according to the standard
// style.
func Fprint(w io.Writer, fset *token.FileSet, file *ast.File) error {
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
	lists := make([]*ast.List, len(file.Expressions))
	copy(lists, file.Expressions)

	// First, we check for any comments before
	// the package statement and do those,
	// then the package statement.
	for len(comments) > 0 && comments[0].Pos() < file.Package.ParenOpen {
		comment := comments[0]
		comments = comments[1:]
		fprintCommentGroup(buf, 0, comment)
		buf.WriteByte('\n') // Add a line break.
	}

	// Print the package statement and any
	// line comment after it.
	buf.WriteString("(package ")
	buf.WriteString(file.Name.Name)
	buf.WriteByte(')')
	if len(comments) == 0 || fset.Position(file.Name.NamePos).Line != fset.Position(comments[0].Pos()).Line {
		buf.WriteString("\n\n")
	} else {
		buf.WriteByte(' ')
		comment := comments[0]
		comments = comments[1:]
		fprintCommentGroup(buf, 0, comment)
		buf.WriteByte('\n') // Add a line break.
	}

	nextNode := func() (comment *ast.CommentGroup, list *ast.List, node ast.Node) {
		if len(comments) == 0 {
			// Must be a list.
			list = lists[0]
			lists = lists[1:]
			return nil, list, list
		} else if len(lists) == 0 {
			// Must be a comment.
			comment = comments[0]
			comments = comments[1:]
			return comment, nil, comment
		}

		// Pick the one with the smaller
		// offset.
		nextComment := fset.Position(comments[0].Pos())
		nextList := fset.Position(lists[0].Pos())
		if nextComment.Offset < nextList.Offset {
			comment = comments[0]
			comments = comments[1:]
			return comment, nil, comment
		}

		list = lists[0]
		lists = lists[1:]
		return nil, list, list
	}

	first := true
	prevEnd := file.Name.NamePos
	for len(comments) != 0 || len(lists) != 0 {
		comment, list, node := nextNode()
		pos := node.Pos() // We need to do more work for lists to account for annotations, which may have been reordered.
		if list, ok := node.(*ast.List); ok {
			for _, anno := range list.Annotations {
				if pos > anno.Quote {
					pos = anno.Quote
				}
			}
		}

		if first {
			first = false
		} else if fset.Position(prevEnd).Line+1 < fset.Position(pos).Line {
			// Add a line break between
			// statements.
			fmt.Println("line break")
			buf.WriteByte('\n')
		}

		if comment != nil {
			fprintCommentGroup(buf, 0, comment)
		} else {
			fprintExpr(buf, 0, list)
		}

		buf.WriteByte('\n')
		prevEnd = node.End()
	}

	if allocated {
		_, err := w.Write(buf.Bytes())
		return err
	}

	return nil
}

func listWidth(list *ast.List) int {
	if len(list.Elements) == 0 {
		return 2 // Just the parentheses.
	}

	width := 2
	for i, elt := range list.Elements {
		if i != 0 {
			width++ // The intervening space.
		}

		switch x := elt.(type) {
		case *ast.List:
			width += listWidth(x)
		case *ast.Identifier:
			width += len(x.Name)
		case *ast.Literal:
			width += len(x.Value)
		default:
			panic(fmt.Sprintf("unexpected expression %#v", x))
		}
	}

	return width
}

// 100 tabs.
const tabs = "\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t" +
	"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t" +
	"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t" +
	"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t"

// fprintCommentGroup writes the comment group to buf.
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
// If indentation is negative, the expression is not broken into
// multiple lines.
func fprintExpr(buf *bytes.Buffer, indentation int, expr ast.Expression) {
	switch x := expr.(type) {
	case *ast.Identifier:
		buf.WriteString(x.Name)
	case *ast.Literal:
		buf.WriteString(x.Value)
	case *ast.List:
		width := listWidth(x)
		for _, anno := range x.Annotations {
			buf.WriteByte('\'')
			fprintExpr(buf, indentation, anno.X)
			buf.WriteByte('\n')
		}

		// We handle top-level function declarations
		// slightly differently. We always put the
		// first element (func / asm-func) and the
		// second element (name /signature) on the
		// same line, then all subsequent expressions
		// on a separate line.

		isFunc := indentation == 0 && len(x.Elements) >= 1
		if isFunc {
			ident, ok := x.Elements[0].(*ast.Identifier)
			isFunc = ok && ident.Name == "func" || ident.Name == "asm-func"
		}

		buf.WriteByte('(')
		if indentation < 0 || (!isFunc && width <= maxListWidth && !containsMultipleLists(x.Elements)) {
			// Nice and simple, all on one line.
			for i, elt := range x.Elements {
				if i > 0 {
					buf.WriteByte(' ')
				}

				fprintExpr(buf, -1, elt)
			}
		} else {
			// The opening parenthesis and the first
			// element go straight away, then we add
			// a newline and indent for all subsequent
			// elements.
			for i, elt := range x.Elements {
				if indentation < 0 || (isFunc && i == 1) {
					buf.WriteByte(' ')
					fprintExpr(buf, -1, elt)
					continue
				} else if i > 0 {
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
