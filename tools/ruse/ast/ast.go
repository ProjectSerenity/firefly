// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package ast declares the types used to represent syntax trees for Ruse packages.
package ast

import (
	"strings"

	"firefly-os.dev/tools/ruse/token"
)

// IsExported returns whether the given string starts with
// an upper-case letter.
func IsExported(s string) bool {
	return token.IsExported(s)
}

// ----------------------------------------------------------------------------
// Interfaces
//
// All nodes contain position information marking the beginning of
// the corresponding source text segment; it is accessible via the
// Pos accessor method. Nodes may contain additional position info
// for language constructs where comments may be found between parts
// of the construct (typically any larger, parenthesized subpart).
// That position information is needed to properly position comments
// when printing the construct.

// All node types implement the Node interface.
type Node interface {
	Pos() token.Pos // position of first character belonging to the node
	End() token.Pos // position of first character immediately after the node
}

// All expression nodes implement the Expr interface.
type Expression interface {
	Node
	Print() string  // Print a simple representation of the expr.
	String() string // Return a simple description of the element type.
	exprNode()
}

// ----------------------------------------------------------------------------
// Comments

// A Comment node represents a single #-style comment.
type Comment struct {
	Semicolon token.Pos // position of "#" starting the comment
	Text      string    // comment text (excluding '\n')
}

func (c *Comment) Pos() token.Pos { return c.Semicolon }
func (c *Comment) End() token.Pos { return c.Semicolon + token.Pos(len(c.Text)) }

// A CommentGroup represents a sequence of comments
// with no other tokens and no empty lines between.
type CommentGroup struct {
	List []*Comment // len(List) > 0
}

func (g *CommentGroup) Pos() token.Pos { return g.List[0].Pos() }
func (g *CommentGroup) End() token.Pos { return g.List[len(g.List)-1].End() }

func isWhitespace(ch byte) bool { return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' }

func stripTrailingWhitespace(s string) string {
	i := len(s)
	for i > 0 && isWhitespace(s[i-1]) {
		i--
	}
	return s[0:i]
}

// Text returns the text of the comment group.
//
// Comment markers (';'), the first space of a line comment, and
// leading and trailing empty lines are removed. Multiple empty
// lines are reduced to one, and trailing space on lines is trimmed.
// Unless the result is empty, it is newline-terminated.
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

// ----------------------------------------------------------------------------
// Expressions and types

// An expression is represented by a tree consisting of one
// or more of the following concrete expression nodes.
type (
	// A BadExpr node is a placeholder for expressions containing
	// syntax errors for which no correct expression nodes can be
	// created.
	//
	BadExpression struct {
		From, To token.Pos // position range of bad expression
	}

	// An Identifier node represents an identifier.
	Identifier struct {
		NamePos token.Pos // identifier position
		Name    string    // identifier name
	}

	// A Literal node represents a literal of basic type.
	Literal struct {
		ValuePos token.Pos   // literal position
		Kind     token.Token // Integer, String, or Period
		Value    string      // literal string; e.g. 42, 0x7f, "foo", or .
	}

	// A ListExpr node represents a parenthesized list.
	List struct {
		ParenOpen   token.Pos     // position of "("
		Annotations []*QuotedList // optional preceeding annotations (which don't affect position)
		Elements    []Expression  // list elements
		ParenClose  token.Pos     // position of ")"
	}

	// A QuotedIdentifier node represents an identifier,
	// preceded with a quote.
	QuotedIdentifier struct {
		Quote token.Pos   // position of "'"
		X     *Identifier // quoted identifier
	}

	// A QuotedList node represents an annotation,
	// in the form of a list, preceded with a quote.
	QuotedList struct {
		Quote token.Pos // position of "'"
		X     *List     // quoted list
	}

	// A Qualified node represents two identifiers,
	// joined with a period.
	Qualified struct {
		X      *Identifier // prefix identifier
		Period token.Pos   // position of "."
		Y      *Identifier // suffix identifier
	}
)

// Pos and End implementations for expression/type nodes.

func (x *BadExpression) Pos() token.Pos    { return x.From }
func (x *Identifier) Pos() token.Pos       { return x.NamePos }
func (x *Literal) Pos() token.Pos          { return x.ValuePos }
func (x *List) Pos() token.Pos             { return x.ParenOpen }
func (x *QuotedIdentifier) Pos() token.Pos { return x.Quote }
func (x *QuotedList) Pos() token.Pos       { return x.Quote }
func (x *Qualified) Pos() token.Pos        { return x.X.Pos() }

func (x *BadExpression) End() token.Pos    { return x.To }
func (x *Identifier) End() token.Pos       { return token.Pos(int(x.NamePos) + len(x.Name)) }
func (x *Literal) End() token.Pos          { return token.Pos(int(x.ValuePos) + len(x.Value)) }
func (x *List) End() token.Pos             { return x.ParenClose + 1 }
func (x *QuotedIdentifier) End() token.Pos { return x.X.End() }
func (x *QuotedList) End() token.Pos       { return x.X.End() }
func (x *Qualified) End() token.Pos        { return x.Y.End() }

func (x *BadExpression) Print() string { return "<bad expr>" }
func (x *Identifier) Print() string    { return x.Name }
func (x *Literal) Print() string       { return x.Value }
func (x *List) Print() string {
	var buf strings.Builder
	buf.WriteByte('(')
	for i, e := range x.Elements {
		if i > 0 {
			buf.WriteByte(' ')
		}

		buf.WriteString(e.Print())
	}

	buf.WriteByte(')')

	return buf.String()
}
func (x *QuotedIdentifier) Print() string { return "quoted " + x.X.Print() }
func (x *QuotedList) Print() string       { return "quoted " + x.X.Print() }
func (x *Qualified) Print() string        { return x.X.Print() + "." + x.Y.Print() }

func (x *BadExpression) String() string    { return "bad expr" }
func (x *Identifier) String() string       { return "identifier" }
func (x *Literal) String() string          { return "literal" }
func (x *List) String() string             { return "list" }
func (x *QuotedIdentifier) String() string { return "quoted " + x.X.String() }
func (x *QuotedList) String() string       { return "quoted " + x.X.String() }
func (x *Qualified) String() string        { return "qualified identifier" }

// exprNode() ensures that only expression/type nodes can be
// assigned to an Expr.
func (*BadExpression) exprNode()    {}
func (*Identifier) exprNode()       {}
func (*Literal) exprNode()          {}
func (*List) exprNode()             {}
func (*QuotedIdentifier) exprNode() {}
func (*QuotedList) exprNode()       {}
func (*Qualified) exprNode()        {}

// ----------------------------------------------------------------------------
// Convenience functions for Idents

// NewIdentifier creates a new Identifier without position.
// Useful for ASTs generated by code other than the Go parser.
func NewIdentifier(name string) *Identifier { return &Identifier{token.NoPos, name} }

// ----------------------------------------------------------------------------
// Files and packages

// A File node represents a Ruse source file.
//
// The Comments list contains all comments in the source file in order of
// appearance, including the comments that are pointed to from other nodes
// via Doc and Comment fields.
//
// For correct printing of source code containing comments (using packages
// ruse/format and ruse/printer), special care must be taken to update comments
// when a File's syntax tree is modified: For printing, comments are interspersed
// between tokens based on their position. If syntax tree nodes are
// removed or moved, relevant comments in their vicinity must also be removed
// (from the File.Comments list) or moved accordingly (by updating their
// positions). A CommentMap may be used to facilitate some of these operations.
//
// Whether and how a comment is associated with a node depends on the
// interpretation of the syntax tree by the manipulating program: Except for Doc
// and Comment comments directly associated with nodes, the remaining comments
// are "free-floating" (see also issues #18593, #20744).
type File struct {
	Doc         *CommentGroup   // associated documentation; or nil
	Package     *List           // the "package" expression
	Name        *Identifier     // package name
	Imports     []*Import       // imports in this file
	Expressions []*List         // top-level expressions; or nil
	Comments    []*CommentGroup // list of all comments in the source file
}

func (f *File) Pos() token.Pos { return f.Package.ParenOpen }
func (f *File) End() token.Pos {
	if n := len(f.Expressions); n > 0 {
		return f.Expressions[n-1].End()
	}
	return f.Name.End()
}

// An ImportSpec node represents a single package import.
type Import struct {
	Doc     *CommentGroup // associated documentation; or nil.
	Group   *List         // parent list node, if any.
	List    *List         // underlying list node.
	Name    *Identifier   // local package name; or nil.
	Path    *Literal      // import path.
	Comment *CommentGroup // line comments; or nil.
}

func (s *Import) Pos() token.Pos { return s.List.ParenOpen }
func (s *Import) End() token.Pos { return s.List.ParenClose + 1 }

// A Package node represents a set of source files
// collectively building a Go package.
type Package struct {
	Name  string           // package name
	Files map[string]*File // Ruse source files by filename
}

func (p *Package) Pos() token.Pos { return token.NoPos }
func (p *Package) End() token.Pos { return token.NoPos }
