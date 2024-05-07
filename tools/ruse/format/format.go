// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package format contains functionality to format a Ruse file into the
// canonical style.
package format

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"slices"
	"strconv"
	"unicode/utf8"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/token"
)

// SortAnnotations ensures that the annotations in each
// list are sorted into alphabetical order.
func SortAnnotations(file *ast.File) {
	// Scan the whole file.
	ast.Inspect(file, func(n ast.Node) bool {
		// Look for lists.
		list, ok := n.(*ast.List)
		if !ok || list == nil || list.Annotations == nil {
			return true
		}

		// We must use a stable sort so we don't
		// reorder any parameter annotations.
		slices.SortStableFunc(list.Annotations, func(a, b *ast.QuotedList) int {
			return cmp.Compare(a.X.Elements[0].(*ast.Identifier).Name, b.X.Elements[0].(*ast.Identifier).Name)
		})

		return true
	})
}

type lineComment struct {
	lineNum     int // The output line number we're on.
	lineStart   int // The offset into the output where the line starts.
	offset      int // The offset into the output where the pre-comment text ends.
	textLen     int // The length of pre-comment text, not including indentation.
	indentation int // The number of indentations at the start of the line.
	group       *ast.CommentGroup
}

type formatter struct {
	buf          *bytes.Buffer
	fset         *token.FileSet
	lineNum      int
	lineStart    int
	comments     []*ast.CommentGroup
	lineComments []*lineComment
}

func (f *formatter) Newline() {
	f.buf.WriteByte('\n')
	f.lineNum++
	f.lineStart = f.buf.Len()
}

func (f *formatter) PeekComment() *ast.CommentGroup {
	if f == nil || len(f.comments) == 0 {
		return nil
	}

	comment := f.comments[0]
	return comment
}

func (f *formatter) PopComment() *ast.CommentGroup {
	if f == nil || len(f.comments) == 0 {
		return nil
	}

	comment := f.comments[0]
	f.comments = f.comments[1:]
	return comment
}

func (f *formatter) SaveLineComment(comment *ast.CommentGroup) {
	// Determine the indentation from the current
	// line.
	line := f.buf.Bytes()[f.lineStart:]
	notTab := func(r rune) bool { return r != '\t' }
	indentation := bytes.IndexFunc(line, notTab)
	textWidth := utf8.RuneCount(line) // Handle multi-byte runes carefully.
	f.lineComments = append(f.lineComments, &lineComment{
		lineNum:     f.lineNum,
		lineStart:   f.lineStart,
		offset:      f.buf.Len(),
		textLen:     textWidth - indentation,
		indentation: indentation,
		group:       comment,
	})
}

// line is a helper to determine the line number.
func (f *formatter) line(p token.Pos) int {
	pos := f.fset.Position(p)
	if !pos.IsValid() {
		panic(fmt.Sprintf("got invalid position at position %d", p))
	}

	return pos.Line
}

// AddCommentsBefore prints any comments before the given
// position, using the indentation provided.
//
// prev is the previous node, which is used to handle any
// trailing line comments.
//
// After adding comments, the final node (which is either
// prev or a comment after prev) is returned.
func (f *formatter) AddCommentsBefore(indentation string, prev, node ast.Node, printTrailingNewlines bool) (final ast.Node) {
	nodePos := token.NoPos
	if node != nil {
		nodePos = node.Pos()
		if list, ok := node.(*ast.List); ok && len(list.Annotations) > 0 {
			// Find the first annotation.
			for _, anno := range list.Annotations {
				if nodePos > anno.Quote {
					nodePos = anno.Quote
				}
			}
		}
	}

	prevIsComment := false
	for {
		comment := f.PeekComment()
		if comment == nil {
			break
		}

		commentLine := f.line(comment.Pos())
		thisLine := 0
		if node != nil {
			thisLine = f.line(nodePos)
		}

		prevLine := 0
		if prev != nil {
			prevLine = f.line(prev.End())
		}

		// Handle comments on the same line as
		// the previous element specially, but
		// only if it's not the same line as us,
		// or we'll interrupt this line.
		if prev != nil && commentLine == prevLine && (node == nil || commentLine != thisLine) {
			f.SaveLineComment(f.PopComment())
			continue
		}

		// Stop if we reach a comment on the
		// same line as us or later.
		if (node != nil && commentLine >= thisLine) || (prev != nil && commentLine == prevLine) {
			break
		}

		// Add a line break after the previous
		// element if there's a gap between it
		// and the comment.
		if prev != nil {
			switch commentLine - prevLine {
			case 1:
				f.Newline()
				if indentation == "" {
					f.Newline() // Always add a line break for top-level entries.
				}
			default:
				f.Newline()
				f.Newline()
			}
		}

		f.buf.WriteString(indentation)
		f.FprintCommentGroup(indentation, f.PopComment())

		prev = comment
		prevIsComment = true
	}

	// If the element is on a different
	// line to its predecessor, we insert
	// a single line break. Otherwise, we
	// add a space.
	if printTrailingNewlines && prev != nil {
		switch f.line(nodePos) - f.line(prev.End()) {
		case 0:
		case 1:
			f.Newline()
			if !prevIsComment && indentation == "" {
				f.Newline() // Always add a line break for top-level entries.
			}
		default:
			f.Newline()
			f.Newline() // Add a line break.
		}
	}

	return prev
}

// Fprint writes the file to w, according to the standard
// style.
func Fprint(w io.Writer, fset *token.FileSet, file *ast.File) error {
	// Vertically-aligning successive line comments
	// is surprisingly difficult, as we need to group
	// them together and identify the length of the
	// set of text preceeding each comment, which we
	// will not know until we've printed them all.
	//
	// To solve this, we record the position of each
	// line comment, along with the offset into the
	// output. Once we've finished, we go back and
	// calculate the line lengths and adjust the
	// comments accordingly.

	f := &formatter{
		buf:      new(bytes.Buffer),
		fset:     fset,
		lineNum:  1,
		comments: make([]*ast.CommentGroup, len(file.Comments)),
	}

	// We don't know the order in which comments
	// and expressions are interleaved, so we
	// track the position of the next comment and
	// interleave them.
	//
	// We make a copy of the comments so we can
	// advance them to track our progress without
	// modifying the file.
	copy(f.comments, file.Comments)

	// We build a list of lists, consisting
	// of the package statement, up to one
	// list for an import group, and the top-
	// level lists in the file. Once we have
	// built up the list of lists, we iterate
	// through it, printing each element,
	// along with any intervening comments.
	elements := make([]ast.Expression, 0, 2+len(file.Expressions))
	elements = append(elements, file.Package)

	// We always use import groups, even
	// if the input is one or more individual
	// imports.
	if len(file.Imports) > 0 {
		// Create a list with the right layout
		// then use our normal code to print
		// it.
		n := len(file.Imports) - 1
		imports := &ast.List{
			ParenOpen:  file.Imports[0].List.ParenOpen,
			Elements:   make([]ast.Expression, 1+len(file.Imports)),
			ParenClose: file.Imports[n].List.ParenClose,
		}

		// Update the positions if the input
		// did use groups.
		if file.Imports[0].Group != nil {
			imports.ParenOpen = file.Imports[0].Group.ParenOpen
		}
		if file.Imports[n].Group != nil {
			imports.ParenClose = file.Imports[n].Group.ParenClose
		}

		// Sort the imports by path.
		importPath := func(imp *ast.Import) string {
			s, _ := strconv.Unquote(imp.Path.Value) // Checked by the parser.
			return s
		}

		slices.SortFunc(file.Imports, func(a, b *ast.Import) int { return cmp.Compare(importPath(a), importPath(b)) })

		imports.Elements[0] = &ast.Identifier{NamePos: imports.ParenOpen + 1, Name: "import"}
		for i, imp := range file.Imports {
			// Add the list entry.
			entry := &ast.List{
				ParenOpen:  imp.List.ParenOpen,
				Elements:   make([]ast.Expression, 0, 2),
				ParenClose: imp.List.ParenClose,
			}

			if imp.Name != nil {
				entry.Elements = append(entry.Elements, imp.Name)
			}

			entry.Elements = append(entry.Elements, imp.Path)

			imports.Elements[i+1] = entry
		}

		// Add the imports.

		elements = append(elements, imports)
	}

	// Add the remaining elements.
	for _, elt := range file.Expressions {
		elements = append(elements, elt)
	}

	// Process the events.
	var prev ast.Node
	for _, elt := range elements {
		// Add any leading comments.
		f.AddCommentsBefore("", prev, elt, true)

		// Print the element.
		f.FprintExpr(0, elt)

		prev = elt
	}

	// Print any trailing line comments.
	f.AddCommentsBefore("", prev, nil, false)

	// Add a trailing newline.
	f.Newline()

	// Add line comments with appropriate vertical
	// alignment.
	if len(f.lineComments) > 0 {
		data := f.buf.Bytes()
		f.buf = bytes.NewBuffer(make([]byte, 0, len(data)))
		textStart := 0
		groupStart := 0

		// Print out the comments from groupStart
		// to (but not including) i.
		addGroup := func(i int) error {
			// Start by appending the text
			// between the end of the last
			// group and the start of this
			// group.
			first := f.lineComments[groupStart]
			last := f.lineComments[groupStart]
			if i > 0 {
				last = f.lineComments[i-1]
			}

			f.buf.Write(data[textStart:first.lineStart])
			textStart = last.offset + 1

			// Next, we determine the line
			// spacing for the group. We
			// don't count the indentation
			// here, which will already be
			// the same.
			spacing := first.textLen
			for j := groupStart + 1; j < i; j++ {
				next := f.lineComments[j]
				got := next.textLen
				if spacing < got {
					spacing = got
				}
			}

			spacing += 2 // Add two spaces before the comment.

			// Finally, add each line, its
			// spacing, its comment, and the
			// trailing newline.
			for j := groupStart; j < i; j++ {
				comment := f.lineComments[j]
				f.buf.Write(data[comment.lineStart:comment.offset])
				f.buf.WriteString(spaces[:spacing-comment.textLen])
				indentation := tabs[:comment.indentation] + spaces[:spacing] // Indentation for any fresh lines.
				f.FprintCommentGroup(indentation, comment.group)
				f.Newline()
			}

			groupStart = i
			if i == 0 {
				groupStart++
			}

			return nil
		}

		for i, comment := range f.lineComments {
			// We can skip the first instance,
			// as we start with groupStart=0.
			if i == 0 {
				continue
			}

			prev := f.lineComments[i-1]
			if comment.indentation == prev.indentation &&
				comment.lineNum == prev.lineNum+1 {
				// Continue the group.
				continue
			}

			err := addGroup(i)
			if err != nil {
				return err
			}
		}

		err := addGroup(len(f.lineComments))
		if err != nil {
			return err
		}

		// Add all text after the
		// last comment.
		last := f.lineComments[len(f.lineComments)-1]
		f.buf.Write(data[last.offset+1:])
	}

	_, err := w.Write(f.buf.Bytes())
	return err
}

// 100 tabs.
const tabs = "\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t" +
	"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t" +
	"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t" +
	"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t"

// 100 spaces.
const spaces = "                         " +
	"                         " +
	"                         " +
	"                         " +
	"                         "

// FprintCommentGroup writes the comment group to buf.
func (f *formatter) FprintCommentGroup(indentation string, group *ast.CommentGroup) {
	lines := group.Lines()
	for i, line := range lines {
		if i > 0 {
			f.Newline()
			f.buf.WriteString(indentation)
		}

		if line == "" {
			f.buf.WriteByte(';')
		} else {
			f.buf.WriteString("; ")
			f.buf.WriteString(line)
		}
	}
}

// FprintExpr writes the node to buf, with the given indentation.
//
// FprintExpr does not write any spacing around the node.
func (f *formatter) FprintExpr(indentation int, expr ast.Expression) {
	switch x := expr.(type) {
	case *ast.QuotedIdentifier:
		f.buf.WriteByte('\'')
		f.buf.WriteString(x.X.Name)
	case *ast.ExpressionComment:
		f.buf.WriteByte('#')
		f.buf.WriteByte(';')
		f.FprintExpr(indentation, x.X)
	case *ast.Identifier:
		f.buf.WriteString(x.Name)
	case *ast.Literal:
		f.buf.WriteString(x.Value)
	case *ast.List:
		for _, anno := range x.Annotations {
			f.buf.WriteByte('\'')
			f.FprintExpr(indentation, anno.X)

			// We add a newline after annotations
			// unless there is exactly one and it
			// is on the same line as the list in
			// the source.
			if len(x.Annotations) != 1 || f.line(anno.Quote) != f.line(x.ParenOpen) {
				f.Newline()
			}
		}

		// We handle top-level function declarations
		// slightly differently. We always put the
		// first element (func / asm-func) and the
		// second element (the signature) on the same
		// line, then add a line break before any
		// subsequent expressions.
		//
		// In assembly functions, each instruction
		// and label should be on its own line.

		var isAssembly, isFunc, isIf, isImport, isPackage bool
		if ident, ok := x.Elements[0].(*ast.Identifier); ok {
			switch ident.Name {
			case "asm-func":
				isAssembly = true
			case "func":
				isFunc = true
			case "if":
				isIf = true
			case "import":
				isImport = true
			case "package":
				isPackage = true
			}
		}

		f.buf.WriteByte('(')

		// There is always a first element, which is never indented further.
		f.FprintExpr(indentation, x.Elements[0])

		allOneLine := f.line(x.ParenOpen) == f.line(x.ParenClose)
		for i, elt := range x.Elements[1:] {
			// Handle any block comments before
			// the element.
			var prev ast.Node = x.Elements[i] // As we index from 1 above, i is the index of the previous element.
			prev = f.AddCommentsBefore(tabs[:indentation+1], prev, elt, false)

			// If the element is on a different
			// line to its predecessor, we insert
			// a single line break. Otherwise, we
			// add a space.
			lineBreak := false   // Whether to add a line break.
			doubleBreak := false // Whether to add a blank line.
			switch f.line(elt.Pos()) - f.line(prev.End()) {
			case 0:
			case 1:
				lineBreak = true
			default:
				lineBreak = true
				doubleBreak = true
			}

			switch {
			case isAssembly:
				lineBreak = i != 0 // Always break, except before the signature.
			case isFunc:
				if i == 0 {
					// No line break before the signature.
					lineBreak = false
				} else if i == 1 {
					// Always line break after the signature.
					lineBreak = true
				}
			case isIf:
				// If the whole statement is on
				// one line, we leave it as it
				// is. If not, we put the condition
				// on the same line as the keyword
				// and the remaining expressions on
				// subsequent lines.
				if allOneLine || i == 0 {
					lineBreak = false
				} else {
					lineBreak = true
					doubleBreak = false
				}
			case isImport:
				lineBreak = true
				doubleBreak = false
			case isPackage:
				// The package name is always on the same line.
				lineBreak = false
			}

			if !lineBreak {
				f.buf.WriteByte(' ')

				f.FprintExpr(indentation, elt)
			} else {
				// Check whether we have any trailing line
				// comments to print before the line break.
				if comment := f.PeekComment(); comment != nil && f.line(comment.Pos()) == f.line(prev.Pos()) {
					f.SaveLineComment(f.PopComment())
				}

				f.Newline()
				if doubleBreak {
					f.Newline()
				}

				f.buf.WriteString(tabs[:indentation+1])

				f.FprintExpr(indentation+1, elt)
			}
		}

		f.buf.WriteByte(')') // Close the list.
	case *ast.Qualified:
		f.buf.WriteString(x.X.Name)
		f.buf.WriteByte('.')
		f.buf.WriteString(x.Y.Name)
	default:
		panic(fmt.Sprintf("%s: unexpected expression %#v", f.fset.Position(expr.Pos()), expr))
	}
}
