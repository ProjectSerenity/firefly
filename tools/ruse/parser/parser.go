// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package parser implements a parser for Ruse source files.
//
// Input may be provided in a variety of forms (see the various Parse* functions);
// the output is an abstract syntax tree (AST) representing the Ruse source.
package parser

import (
	"bytes"
	"errors"
	"fmt"
	"go/scanner"
	"io"
	"os"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/lexer"
	"firefly-os.dev/tools/ruse/token"
)

// If src != nil, readSource converts src to a []byte if possible;
// otherwise it returns an error. If src == nil, readSource returns
// the result of reading the file specified by filename.
func readSource(filename string, src interface{}) ([]byte, error) {
	if src != nil {
		switch s := src.(type) {
		case string:
			return []byte(s), nil
		case []byte:
			return s, nil
		case *bytes.Buffer:
			// is io.Reader, but src is already available in []byte form
			if s != nil {
				return s.Bytes(), nil
			}
		case io.Reader:
			return io.ReadAll(s)
		}
		return nil, errors.New("invalid source")
	}
	return os.ReadFile(filename)
}

// ParseFile parses the source code of a single Ruse source file and returns
// the corresponding ast.File node. The source code may be provided via
// the filename of the source file, or via the src parameter.
//
// If src != nil, ParseFile parses the source from src and the filename is
// only used when recording position information. The type of the argument
// for the src parameter must be string, []byte, or io.Reader.
// If src == nil, ParseFile parses the file specified by filename.
//
// The mode parameter controls the amount of source text parsed and other
// optional parser functionality. Position information is recorded in the
// file set fset, which must not be nil.
func ParseFile(fset *token.FileSet, filename string, src interface{}, mode Mode) (f *ast.File, err error) {
	if fset == nil {
		panic("parser.ParseFile: no FileSet provided (fset == nil)")
	}

	// get source
	text, err := readSource(filename, src)
	if err != nil {
		return nil, err
	}

	var p parser
	defer func() {
		if e := recover(); e != nil {
			// resume same panic if it's not a bailout
			if e == io.ErrUnexpectedEOF {
				err = io.ErrUnexpectedEOF
				return
			}

			if _, ok := e.(bailout); !ok {
				panic(e)
			}
		}
		p.errors.Sort()
		err = p.errors.Err()
	}()

	// parse source
	p.init(fset, filename, text, mode)
	f = p.parseFile()

	return
}

// ParseExpressionFrom is a convenience function for parsing an expression.
// The arguments have the same meaning as for ParseFile, but the source must
// be a valid Ruse expression. Specifically, fset must not be nil.
func ParseExpressionFrom(fset *token.FileSet, filename string, src interface{}, mode Mode) (expr ast.Expression, err error) {
	if fset == nil {
		panic("parser.ParseExprFrom: no FileSet provided (fset == nil)")
	}

	// get source
	text, err := readSource(filename, src)
	if err != nil {
		return nil, err
	}

	var p parser
	defer func() {
		if e := recover(); e != nil {
			// resume same panic if it's not a bailout
			if e == io.ErrUnexpectedEOF {
				err = io.ErrUnexpectedEOF
				return
			}

			if _, ok := e.(bailout); !ok {
				panic(e)
			}
		}
		p.errors.Sort()
		err = p.errors.Err()
	}()

	// parse expr
	p.init(fset, filename, text, mode)

	expr = p.parseExpression()

	// Report an error if there's more tokens.
	p.expect(token.EndOfFile)

	if p.errors.Len() > 0 {
		p.errors.Sort()
		return nil, p.errors.Err()
	}

	return expr, nil
}

// ParseExpression is a convenience function for obtaining the AST of an expression x.
// The position information recorded in the AST is undefined. The filename used
// in error messages is the empty string.
func ParseExpression(x string) (ast.Expression, error) {
	return ParseExpressionFrom(token.NewFileSet(), "", []byte(x), 0)
}

// A Mode value is a set of flags (or 0).
// They control the amount of source code parsed and other optional
// parser functionality.
type Mode uint

const (
	ParseComments Mode = 1 << iota // parse comments and add them to AST
	Trace                          // print a trace of parsed productions
	AllErrors                      // report all errors (not just the first 10 on different lines)
)

var traceOutput io.Writer = os.Stdout // Changable for tests.

// The parser structure holds the parser's internal state.
type parser struct {
	file    *token.File
	errors  scanner.ErrorList
	lexemes <-chan lexer.Lexeme

	// Tracing/debugging
	mode   Mode // parsing mode
	trace  bool // == (mode & Trace != 0)
	indent int  // indentation used for tracing output

	// Comments
	comments    []*ast.CommentGroup
	leadComment *ast.CommentGroup // last lead comment
	lineComment *ast.CommentGroup // last line comment

	// Next token
	lex lexer.Lexeme

	// Error recovery
	// (used to limit the number of calls to parser.advance
	// w/o making scanning progress - avoids potential endless
	// loops across multiple parser functions during error recovery)
	syncPos token.Pos // last synchronization position
	syncCnt int       // number of parser.advance calls without progress

	// Non-syntactic parser control
	exprLev int  // < 0: in control clause, >= 0: in expression
	inRhs   bool // if set, the parser is parsing a rhs expression
}

func (p *parser) init(fset *token.FileSet, filename string, src []byte, mode Mode) {
	p.file = fset.AddFile(filename, -1, len(src))
	p.lexemes = lexer.Scan(p.file, src)

	p.mode = mode
	p.trace = mode&Trace != 0 // for convenience (p.trace is used frequently)

	p.next()
}

// ----------------------------------------------------------------------------
// Parsing support

func (p *parser) printTrace(a ...any) {
	const dots = ". . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . "
	const n = len(dots)
	pos := p.file.Position(p.lex.Position)
	fmt.Fprintf(traceOutput, "%5d:%3d: ", pos.Line, pos.Column)
	i := 2 * p.indent
	for i > n {
		fmt.Fprint(traceOutput, dots)
		i -= n
	}
	// i <= n
	fmt.Fprint(traceOutput, dots[0:i])
	fmt.Fprintln(traceOutput, a...)
}

func trace(p *parser, msg string) *parser {
	p.printTrace(msg, "(")
	p.indent++
	return p
}

// Usage pattern: defer un(trace(p, "..."))
func un(p *parser) {
	p.indent--
	p.printTrace(")")
}

// Advance to the next token.
func (p *parser) next0() {
	// Because of one-token look-ahead, print the previous token
	// when tracing as it provides a more readable output. The
	// very first token (!p.pos.IsValid()) is not initialized
	// (it is ILLEGAL), so don't print it .
	if p.trace && p.lex.Position.IsValid() {
		s := p.lex.Token.String()
		switch {
		case p.lex.Token == token.Identifier:
			p.printTrace(s, "\""+p.lex.Value+"\"")
		case p.lex.Token.IsLiteral():
			p.printTrace(s, p.lex.Value)
		default:
			p.printTrace(s)
		}
	}

	p.lex = <-p.lexemes
}

// Consume a comment and return it and the line on which it ends.
func (p *parser) consumeComment() (comment *ast.Comment, endline int) {
	// Scan the comment for '\n' chars and adjust endline accordingly.
	endline = p.file.Line(p.lex.Position)
	comment = &ast.Comment{Semicolon: p.lex.Position, Text: p.lex.Value}
	p.next0()

	return
}

// Consume a group of adjacent comments, add it to the parser's
// comments list, and return it together with the line at which
// the last comment in the group ends. A non-comment token or n
// empty lines terminate a comment group.
func (p *parser) consumeCommentGroup(n int) (comments *ast.CommentGroup, endline int) {
	var list []*ast.Comment
	endline = p.file.Line(p.lex.Position)
	for p.lex.Token == token.Comment && p.file.Line(p.lex.Position) <= endline+n {
		var comment *ast.Comment
		comment, endline = p.consumeComment()
		list = append(list, comment)
	}

	// add comment group to the comments list
	comments = &ast.CommentGroup{List: list}
	p.comments = append(p.comments, comments)

	return
}

// Advance to the next non-comment token. In the process, collect
// any comment groups encountered, and remember the last lead and
// line comments.
//
// A lead comment is a comment group that starts and ends in a
// line without any other tokens and that is followed by a non-comment
// token on the line immediately after the comment group.
//
// A line comment is a comment group that follows a non-comment
// token on the same line, and that has no tokens after it on the line
// where it ends.
//
// Lead and line comments may be considered documentation that is
// stored in the AST.
func (p *parser) next() {
	p.leadComment = nil
	p.lineComment = nil
	prev := p.lex.Position
	p.next0()

	if p.lex.Token == token.Comment {
		var comment *ast.CommentGroup
		var endline int

		if p.file.Line(p.lex.Position) == p.file.Line(prev) {
			// The comment is on same line as the previous token; it
			// cannot be a lead comment but may be a line comment.
			comment, endline = p.consumeCommentGroup(0)
			if p.file.Line(p.lex.Position) != endline || p.lex.Token == token.EndOfFile {
				// The next token is on a different line, thus
				// the last comment group is a line comment.
				p.lineComment = comment
			}
		}

		// consume successor comments, if any
		endline = -1
		for p.lex.Token == token.Comment {
			comment, endline = p.consumeCommentGroup(1)
		}

		if endline+1 == p.file.Line(p.lex.Position) {
			// The next token is following on the line immediately after the
			// comment group, thus the last comment group is a lead comment.
			p.leadComment = comment
		}
	}
}

// A bailout panic is raised to indicate early termination.
type bailout struct{}

type errorPos token.Pos

func (p errorPos) Pos() token.Pos { return token.Pos(p) }
func (p errorPos) End() token.Pos { return token.Pos(p) }

func (p *parser) error(node ast.Node, msg string) {
	epos := p.file.Position(node.Pos())

	// If AllErrors is not set, discard errors reported on the same line
	// as the last recorded error and stop parsing if there are more than
	// 10 errors.
	if p.mode&AllErrors == 0 {
		n := len(p.errors)
		if n > 0 && p.errors[n-1].Pos.Line == epos.Line {
			return // discard - likely a spurious error
		}
		if n > 10 {
			panic(bailout{})
		}
	}

	p.errors.Add(epos, msg)
}

func (p *parser) errorExpected(node ast.Node, msg string) {
	msg = "expected " + msg
	if node != nil && node.End() == p.lex.Position {
		// the error happened at the current position;
		// make the error message more specific
		switch {
		case p.lex.Token.IsLiteral():
			// print 123 rather than 'INT', etc.
			msg += ", found " + p.lex.Value
		case p.lex.Token == token.Error:
			msg += ", found " + p.lex.Value
		default:
			msg += ", found '" + p.lex.Token.String() + "'"
		}
	}

	p.error(node, msg)
}

func (p *parser) expect(tok token.Token) token.Pos {
	pos := p.lex.Position
	if p.lex.Token != tok {
		p.errorExpected(errorPos(pos), "'"+tok.String()+"'")
	}

	p.next() // make progress
	return pos
}

// ----------------------------------------------------------------------------
// parsers

func (p *parser) parseList() *ast.List {
	if p.trace {
		defer un(trace(p, "parseList"))
	}

	pos := p.lex.Position
	exprs := make([]ast.Expression, 0, 10) // Most expr lists will have fewer than 10 entries.
	p.next()

	for {
		if p.lex.Token == token.ParenClose {
			x := &ast.List{ParenOpen: pos, Elements: exprs, ParenClose: p.lex.Position}
			return x
		}

		exprs = append(exprs, p.parseExpr())
	}
}

func (p *parser) parseExpression() ast.Expression {
	if p.trace {
		defer un(trace(p, "parseExpression"))
	}

	return p.parseExpr()
}

// parseExpr must onlt be called by p.parseExpression
// or p.parseList.
func (p *parser) parseExpr() ast.Expression {
	switch p.lex.Token {
	case token.Identifier:
		x := &ast.Identifier{NamePos: p.lex.Position, Name: p.lex.Value}
		p.next()
		if p.lex.Token == token.Period {
			q := &ast.Qualified{X: x, Period: p.lex.Position}
			p.next()
			q.Y = p.expectIdentifier()
			return q
		}

		return x
	case token.Integer, token.String:
		x := &ast.Literal{ValuePos: p.lex.Position, Kind: p.lex.Token, Value: p.lex.Value}
		p.next()
		return x
	case token.Quote:
		pos := p.lex.Position
		p.next()
		x := p.parseExpression()
		if ident, ok := x.(*ast.Identifier); ok {
			return &ast.QuotedIdentifier{Quote: pos, X: ident}
		}

		list, ok := x.(*ast.List)
		if !ok {
			p.errorExpected(errorPos(pos), "identifier or list, found "+x.Print())
			return nil
		}

		if len(list.Elements) == 0 {
			p.error(errorPos(list.ParenClose), "annotations must contain at least one expression")
			return nil
		}

		if _, ok := list.Elements[0].(*ast.Identifier); !ok {
			p.errorExpected(list.Elements[0], "identifier")
			return nil
		}

		// An arbitrary number of quoted lists may
		// preceed a list, in which case those quoted
		// lists become the annotations on the list.
		// A quoted list cannot be unattached.
		annotations := []*ast.QuotedList{{Quote: pos, X: list}}
		for p.lex.Token == token.Quote {
			// Must be another annotation.
			p.next()
			list := p.expectList()
			if list == nil {
				return nil
			}

			if len(list.Elements) == 0 {
				p.error(errorPos(list.ParenClose), "annotations must contain at least one expression")
				return nil
			}

			if _, ok := list.Elements[0].(*ast.Identifier); !ok {
				p.errorExpected(list.Elements[0], "identifier")
				return nil
			}

			// There must not be a blank line between
			// annotations.
			if p.file.Line(pos)+1 < p.file.Line(list.ParenOpen) {
				p.errorExpected(annotations[len(annotations)-1], "attached list or annotation")
				return nil
			}

			annotations = append(annotations, &ast.QuotedList{Quote: pos, X: list})
			pos = p.lex.Position
		}

		list = p.expectList()
		if list != nil {
			// There must not be a blank line between
			// the last annotation and the list it is
			// attached to.
			list.Annotations = annotations
			if p.file.Line(pos)+1 < p.file.Line(list.ParenOpen) {
				p.errorExpected(annotations[len(annotations)-1], "attached list or annotation")
				return nil
			}
		}

		return list
	case token.ParenOpen:
		x := p.parseList()
		p.next()
		return x
	case token.EndOfFile:
		panic(io.ErrUnexpectedEOF)
	case token.Error:
		p.error(errorPos(p.lex.Position), p.lex.Value)
		p.next()
		return nil
	default:
		p.errorExpected(errorPos(p.lex.Position), "expression")
		return nil
	}
}

func (p *parser) expectList() *ast.List {
	if p.trace {
		defer un(trace(p, "expectList"))
	}

	if p.lex.Token != token.ParenOpen {
		p.errorExpected(errorPos(p.lex.Position), "list")
		return nil
	}

	x := p.parseList()
	p.next()
	return x
}

func (p *parser) expectIdentifier() *ast.Identifier {
	if p.trace {
		defer un(trace(p, "expectIdentifier"))
	}

	if p.lex.Token != token.Identifier {
		p.errorExpected(errorPos(p.lex.Position), "identifier")
		return nil
	}

	x := &ast.Identifier{NamePos: p.lex.Position, Name: p.lex.Value}
	p.next()
	return x
}

func (p *parser) parseFile() *ast.File {
	if p.trace {
		defer un(trace(p, "parseFile"))
	}

	// Package clause.
	doc := p.leadComment
	if p.lex.Token != token.ParenOpen {
		p.errorExpected(errorPos(p.lex.Position), "package name")
		return nil
	}

	list := p.parseList()
	p.next()
	if len(list.Elements) != 2 {
		p.errorExpected(list, "package name")
		return nil
	}

	if ident, ok := list.Elements[0].(*ast.Identifier); !ok || ident.Name != "package" {
		p.errorExpected(list, "package name")
		return nil
	}

	name, ok := list.Elements[1].(*ast.Identifier)
	if !ok {
		p.errorExpected(list.Elements[1], "package name")
		return nil
	}

	if name.Name == "_" {
		p.error(name, "invalid package name _")
		return nil
	}

	// Imports.
	var imports []*ast.Import
	var exprs []*ast.List
	for p.lex.Token != token.EndOfFile {
		expr := p.parseExpression()
		if expr == nil {
			break
		}

		e, ok := expr.(*ast.List)
		if !ok {
			p.errorExpected(expr, "list")
			break
		}

		if len(e.Elements) == 0 {
			p.error(e, "invalid top-level list")
			return nil
		}

		if id, ok := e.Elements[0].(*ast.Identifier); !ok || id.Name != "import" {
			exprs = append(exprs, e)
			break
		}

		if len(e.Elements) < 2 {
			p.error(e, "invalid import: no import path")
			return nil
		}

		// Either a single import, consisting of an optional
		// name and an import path, or a sequence of lists,
		// each containing an optional name and an import
		// path.
		if _, ok := e.Elements[1].(*ast.List); !ok {
			// Single import.
			var name *ast.Identifier
			var path *ast.Literal
			switch len(e.Elements) {
			case 2:
				path, ok = e.Elements[1].(*ast.Literal)
				if !ok || path.Kind != token.String {
					p.error(e.Elements[1], "invalid import: expected import path string")
					return nil
				}
			case 3:
				name, ok = e.Elements[1].(*ast.Identifier)
				if !ok || name.Name == "_" {
					p.error(e.Elements[1], "invalid import: expected import name symbol")
					return nil
				}

				path, ok = e.Elements[2].(*ast.Literal)
				if !ok || path.Kind != token.String {
					p.error(e.Elements[2], "invalid import: expected import path string")
					return nil
				}
			default:
				p.error(e.Elements[3], "unexpected expression after import")
				return nil
			}

			imports = append(imports, &ast.Import{
				ParenOpen:  e.ParenOpen,
				Name:       name,
				Path:       path,
				ParenClose: e.ParenClose,
			})

			continue
		}

		// Sequence of imports.
		for _, elt := range e.Elements[1:] {
			list, ok := elt.(*ast.List)
			if !ok {
				p.error(elt, "invalid import: expected import list expression")
				return nil
			}

			var name *ast.Identifier
			var path *ast.Literal
			switch len(list.Elements) {
			case 0:
				p.error(list, "invalid import list expression: no import path")
				return nil
			case 1:
				path, ok = list.Elements[0].(*ast.Literal)
				if !ok || path.Kind != token.String {
					p.error(list.Elements[0], "invalid import: expected import path string")
					return nil
				}
			case 2:
				name, ok = list.Elements[0].(*ast.Identifier)
				if !ok || name.Name == "_" {
					p.error(list.Elements[0], "invalid import: expected import name symbol")
					return nil
				}

				path, ok = list.Elements[1].(*ast.Literal)
				if !ok || path.Kind != token.String {
					p.error(list.Elements[1], "invalid import: expected import path string")
					return nil
				}
			default:
				p.error(list.Elements[2], "unexpected expression after import list expression")
				return nil
			}

			imports = append(imports, &ast.Import{
				ParenOpen:  list.ParenOpen,
				Name:       name,
				Path:       path,
				ParenClose: list.ParenClose,
			})

			continue
		}
	}

	// Expressions.
	for p.lex.Token != token.EndOfFile {
		expr := p.parseExpression()
		if expr == nil {
			break
		}

		e, ok := expr.(*ast.List)
		if !ok {
			p.errorExpected(expr, "list")
			break
		}

		if len(e.Elements) == 0 {
			p.error(e, "invalid top-level list")
			return nil
		}

		exprs = append(exprs, e)
	}

	f := &ast.File{
		Doc:         doc,
		Package:     list.ParenOpen,
		Name:        name,
		Imports:     imports,
		Expressions: exprs,
		Comments:    p.comments,
	}

	return f
}
