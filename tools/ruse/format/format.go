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
		if !ok {
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
	lists        []*ast.List
}

func (f *formatter) Newline() {
	f.buf.WriteByte('\n')
	f.lineNum++
	f.lineStart = f.buf.Len()
}

func (f *formatter) Next() (comment *ast.CommentGroup, list *ast.List, node ast.Node) {
	if len(f.comments) == 0 {
		// Must be a list.
		list = f.PopList()
		return nil, list, list
	} else if len(f.lists) == 0 {
		// Must be a comment.
		comment = f.PopComment()
		return comment, nil, comment
	}

	// Pick the one with the smaller
	// offset.
	nextComment := f.fset.Position(f.comments[0].Pos())
	nextList := f.fset.Position(f.lists[0].Pos())
	if nextComment.Offset < nextList.Offset {
		comment = f.PopComment()
		return comment, nil, comment
	}

	list = f.PopList()
	return nil, list, list
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

func (f *formatter) PeekList() *ast.List {
	if f == nil || len(f.lists) == 0 {
		return nil
	}

	list := f.lists[0]
	return list
}

func (f *formatter) PopList() *ast.List {
	if f == nil || len(f.lists) == 0 {
		return nil
	}

	list := f.lists[0]
	f.lists = f.lists[1:]
	return list
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
		lists:    make([]*ast.List, len(file.Expressions)),
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
	copy(f.comments, file.Comments)
	copy(f.lists, file.Expressions)

	// First, we check for any comments before
	// the package statement and do those,
	// then the package statement.
	for len(f.comments) > 0 && f.comments[0].Pos() < file.Package.ParenOpen {
		f.FprintCommentGroup("", f.PopComment())
		f.Newline()
		f.Newline() // Add a line break.
	}

	// Print the package statement and any
	// line comment after it.
	f.FprintExpr(0, file.Package)
	if len(f.comments) == 0 || fset.Position(file.Name.NamePos).Line != fset.Position(f.comments[0].Pos()).Line {
		f.buf.WriteByte('\n')
		f.buf.WriteByte('\n')
	} else {
		f.buf.WriteByte(' ')
		f.buf.WriteByte(' ')
		f.FprintCommentGroup("", f.PopComment())
		f.Newline() // Add a line break.
	}

	first := true
	prevEnd := file.Name.NamePos
	for len(f.comments) != 0 || len(f.lists) != 0 {
		comment, list, node := f.Next()
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
			f.Newline()
		}

		if comment != nil {
			f.FprintCommentGroup("", comment)
		} else {
			f.FprintExpr(0, list)

			// Check whether we have any trailing line
			// comments to print before the line break.
			if comment := f.PeekComment(); comment != nil && fset.Position(comment.Pos()).Line == fset.Position(list.ParenClose).Line {
				f.SaveLineComment(f.PopComment())
			}
		}

		f.Newline()
		prevEnd = node.End()
	}

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
	// Helper to determine the line number.
	line := func(p token.Pos) int {
		pos := f.fset.Position(p)
		if !pos.IsValid() {
			panic(fmt.Sprintf("got invalid position at position %d", p))
		}

		return pos.Line
	}

	switch x := expr.(type) {
	case *ast.QuotedIdentifier:
		f.buf.WriteByte('\'')
		f.buf.WriteString(x.X.Name)
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
			if len(x.Annotations) != 1 || line(anno.Quote) != line(x.ParenOpen) {
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

		isFunc := false
		isAssembly := false
		if indentation == 0 {
			if ident, ok := x.Elements[0].(*ast.Identifier); ok {
				isFunc = ident.Name == "func" || ident.Name == "asm-func"
				isAssembly = ident.Name == "asm-func"
			}
		}

		f.buf.WriteByte('(')

		// There is always a first element, which is never indented further.
		f.FprintExpr(indentation, x.Elements[0])

		for i, elt := range x.Elements[1:] {
			var prev ast.Node = x.Elements[i] // As we index from 1 above, i is the index of the previous element.

			// Handle any block comments before
			// the element.
			for {
				comment := f.PeekComment()
				if comment == nil {
					break
				}

				commentLine := line(comment.Pos())
				thisLine := line(elt.Pos())
				prevLine := line(prev.End())

				// Handle comments on the same line as
				// the previous element specially, but
				// only if it's not the same line as us,
				// or we'll interrupt this line.
				if commentLine == prevLine && commentLine != thisLine {
					f.SaveLineComment(f.PopComment())
					continue
				}

				// Stop if we reach a comment on the
				// same line as us or later.
				if commentLine == prevLine || commentLine >= thisLine {
					break
				}

				// Add a line break after the previous
				// element if there's a gap between it
				// and the comment.
				switch commentLine - prevLine {
				case 1:
					f.Newline()
				default:
					f.Newline()
					f.Newline()
				}

				f.buf.WriteString(tabs[:indentation+1])
				f.FprintCommentGroup(tabs[:indentation+1], f.PopComment())

				prev = comment
			}

			// If the element is on a different
			// line to its predecessor, we insert
			// a single line break. Otherwise, we
			// add a space.
			lineBreak := false   // Whether to add a line break.
			doubleBreak := false // Whether to add a blank line.
			switch line(elt.Pos()) - line(prev.End()) {
			case 0:
			case 1:
				lineBreak = true
			default:
				lineBreak = true
				doubleBreak = true
			}

			if isFunc {
				switch i {
				case 0:
					// No line break before the signature.
					lineBreak = false
				case 1:
					// Always line break after the signature.
					lineBreak = true
				}
			}

			if isAssembly && i != 0 {
				lineBreak = true
			}

			if !lineBreak {
				f.buf.WriteByte(' ')

				f.FprintExpr(indentation, elt)
			} else {
				// Check whether we have any trailing line
				// comments to print before the line break.
				if comment := f.PeekComment(); comment != nil && line(comment.Pos()) == line(prev.Pos()) {
					f.SaveLineComment(f.PopComment())
				}

				if doubleBreak {
					f.Newline()
				}

				f.Newline()
				f.buf.WriteString(tabs[:indentation+1])

				f.FprintExpr(indentation+1, elt)
			}
		}

		f.buf.WriteByte(')') // Close the list.
	default:
		panic(fmt.Sprintf("%s: unexpected expression %#v", f.fset.Position(expr.Pos()), expr))
	}
}
