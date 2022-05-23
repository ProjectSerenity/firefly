// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package ast contains the types representing Plan syntax trees.
//
package ast

import (
	"strings"
	"unicode"

	"firefly-os.dev/tools/plan/token"
)

// All nodes contain position information marking the
// beginning and and of the node.

// Node represents a node in the syntax tree.
//
type Node interface {
	String() string      // A textual description of the node.
	Pos() token.Position // Location of the first character of the node.
	End() token.Position // Location of the first character after the node.
}

// Expr represents an expression in the syntax tree.
//
type Expr interface {
	Node

	// Width returns the number of bytes needed to
	// store the expression, assuming it is printed
	// on one line.
	//
	Width() int
}

// Expression nodes.

type (
	// Identifier represents a Plan identifier.
	//
	Identifier struct {
		NamePos token.Position // Identifier position.
		Name    string         // Identifier name.
	}

	// String represents a basic string literal.
	//
	String struct {
		QuotePos token.Position // Position of the opening double quote.
		Text     string         // String's content, including quotes: e.g. `"foo"`.
	}

	// Number represents a basic number literal.
	//
	Number struct {
		ValuePos token.Position // Position of the start of the value.
		Value    string         // Number's digits: e.g. `1234`.
	}

	// Pointer represents a pointer type with mutability
	// note.
	//
	Pointer struct {
		AsteriskPos token.Position
		NotePos     token.Position
		Note        string
	}

	// List represents a parenthesised list.
	//
	List struct {
		ParenOpen  token.Position // Position of "(".
		Elements   []Expr         // List elements.
		ParenClose token.Position // Position of ")".
	}
)

// Pos and End implementations for expression/type nodes.

var (
	_ Node = (*Identifier)(nil)
	_ Node = (*String)(nil)
	_ Node = (*Number)(nil)
	_ Node = (*Pointer)(nil)
	_ Node = (*List)(nil)

	_ Expr = (*Identifier)(nil)
	_ Expr = (*String)(nil)
	_ Expr = (*Number)(nil)
	_ Expr = (*Pointer)(nil)
	_ Expr = (*List)(nil)
)

func (x *Identifier) String() string { return "identifier" }
func (x *String) String() string     { return "string" }
func (x *Number) String() string     { return "number" }
func (x *Pointer) String() string    { return "pointer" }
func (x *List) String() string       { return "list" }

func (x *Identifier) Pos() token.Position { return x.NamePos }
func (x *String) Pos() token.Position     { return x.QuotePos }
func (x *Number) Pos() token.Position     { return x.ValuePos }
func (x *Pointer) Pos() token.Position    { return x.AsteriskPos }
func (x *List) Pos() token.Position       { return x.ParenOpen }

func (x *Identifier) End() token.Position { return x.NamePos.Advance(len(x.Name)) }
func (x *String) End() token.Position     { return x.QuotePos.Advance(len(x.Text)) }
func (x *Number) End() token.Position     { return x.ValuePos.Advance(len(x.Value)) }
func (x *Pointer) End() token.Position    { return x.NotePos.Advance(len(x.Note)) }
func (x *List) End() token.Position       { return x.ParenClose.Advance(1) }

func (x *Identifier) Width() int { return len(x.Name) }
func (x *String) Width() int     { return len(x.Text) }
func (x *Number) Width() int     { return len(x.Value) }
func (x *Pointer) Width() int    { return 1 + len(x.Note) }
func (x *List) Width() int {
	if len(x.Elements) == 0 {
		return 2 // Just the parentheses.
	}

	w := 2 + len(x.Elements) - 1 // The parentheses plus a space between each element.
	for _, e := range x.Elements {
		w += e.Width()
	}

	return w
}

// A Comment node represents a single ;-style comment.
//
type Comment struct {
	Semicolon token.Position // Position of ';' starting the comment.
	Text      string         // Comment text (including the ';', excluding any trailing '\n').
}

func (c *Comment) String() string      { return "comment" }
func (c *Comment) Pos() token.Position { return c.Semicolon }
func (c *Comment) End() token.Position { return c.Semicolon.Advance(len(c.Text)) }

// A CommentGroup represents a sequence of comments
// with no other tokens and no empty lines between.
//
type CommentGroup struct {
	List []*Comment // len(List) > 0
}

func (g *CommentGroup) String() string      { return "comment" }
func (g *CommentGroup) Pos() token.Position { return g.List[0].Pos() }
func (g *CommentGroup) End() token.Position { return g.List[len(g.List)-1].End() }

func isWhitespace(r rune) bool {
	return unicode.IsSpace(r)
}

func stripTrailingWhitespace(s string) string {
	i := len(s)
	for i > 0 && isWhitespace(rune(s[i-1])) {
		i--
	}
	return s[:i]
}

// Text returns the text of the comment group.
//
// Comment markers (';'), the first space of a line comment, and
// leading and trailing empty lines are removed. Multiple empty
// lines are reduced to one, and trailing space on lines is trimmed.
// Unless the result is empty, it is newline-terminated.
//
func (g *CommentGroup) Text() string {
	lines := g.Lines()
	if len(lines) == 0 {
		return ""
	}

	// Add final "" entry to get trailing newline from Join.
	if n := len(lines); n > 0 && lines[n-1] != "" {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// Lines returns the text of the comment group, as a sequence of
// lines.
//
// Comment markers (';'), the first space of a line comment, and
// leading and trailing empty lines are removed. Multiple empty
// lines are reduced to one, and trailing space on lines is trimmed.
//
func (g *CommentGroup) Lines() []string {
	if g == nil {
		return nil
	}

	// Make a copy of the group so we can remove redundant
	// comments and make changes without affecting g.
	comments := make([]string, len(g.List))
	for i, c := range g.List {
		comments[i] = c.Text
	}

	lines := make([]string, 0, 10) // Most comments are fewer than 10 lines.
	for _, c := range comments {
		// Remove comment marker.
		// ;-style comments have no newline at the end.
		c = c[1:]
		// strip first space - required for Example tests
		if len(c) > 0 && c[0] == ' ' {
			c = c[1:]
		}

		// Strip trailing white space and add to list.
		lines = append(lines, stripTrailingWhitespace(c))
	}

	// Remove leading blank lines; convert runs of
	// interior blank lines to a single blank line.
	n := 0
	for _, line := range lines {
		if line != "" || n > 0 && lines[n-1] != "" {
			lines[n] = line
			n++
		}
	}
	lines = lines[:n]

	return lines
}

// File represents a file of Plan source.
//
type File struct {
	Comments []*CommentGroup // All comments in the file.
	Lists    []*List         // Top-level expressions, or nil.
}

var _ Node = (*File)(nil)

func (f *File) String() string      { return "file" }
func (f *File) Pos() token.Position { return token.FileStart }
func (f *File) End() token.Position {
	end := token.FileStart
	if n := len(f.Comments); n > 0 {
		if e := f.Comments[n-1].End(); e > end {
			end = e
		}
	}
	if n := len(f.Lists); n > 0 {
		if e := f.Lists[n-1].End(); e > end {
			end = e
		}
	}

	return end
}
