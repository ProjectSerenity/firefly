// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package parser contains a parser that takes a sequence of Plan tokens and
// produces an abstract syntax tree.
//
package parser

import (
	"bytes"
	"errors"
	"io"
	"os"

	"github.com/ProjectSerenity/firefly/tools/plan/ast"
	"github.com/ProjectSerenity/firefly/tools/plan/lexer"
	"github.com/ProjectSerenity/firefly/tools/plan/token"
)

// If src != nil, readSource converts src to a []byte if possible;
// otherwise it returns an error. If src == nil, readSource returns
// the result of reading the file specified by filename.
//
func readSource(filename string, src any) ([]byte, error) {
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

// ParseFile parses the source code of a single Plan source file and returns
// the corresponding ast.File node. The source code may be provided via
// the filename of the source file, or via the src parameter.
//
// If src != nil, ParseFile parses the source from src and the filename is
// only used when recording position information. The type of the argument
// for the src parameter must be string, []byte, or io.Reader.
// If src == nil, ParseFile parses the file specified by filename.
//
func ParseFile(filename string, src any) (f *ast.File, err error) {
	text, err := readSource(filename, src)
	if err != nil {
		return nil, err
	}

	p := newParser(filename, text)
	defer func() {
		if e := recover(); e != nil {
			// resume same panic if it's not a bailout
			if _, ok := e.(bailout); !ok {
				panic(e)
			}
		}

		err = p.err
	}()

	// parse source
	f = p.parseFile()

	return
}

// parser holds the internal state of the parser.
//
type parser struct {
	filename string

	lexemes <-chan lexer.Lexeme // Stream of lexemes.
	lexeme  lexer.Lexeme        // Current lexeme.

	// Error being reported.
	err error

	// Debugging.
	indent int // Indentation used for tracing output.

	// Comments
	comments []*ast.Comment
}

func newParser(filename string, src []byte) *parser {
	p := &parser{
		filename: filename,
		lexemes:  lexer.Scan(src),
	}

	p.next()

	return p
}

// Advance to the next token.
//
func (p *parser) next() {
	p.lexeme = <-p.lexemes
}

// A bailout panic is raised to indicate early termination.
//
type bailout struct{}

type errorPos token.Position

func (p errorPos) String() string      { return "error" }
func (p errorPos) Pos() token.Position { return token.Position(p) }
func (p errorPos) End() token.Position { return token.Position(p) }

func (p *parser) error(msg string) {
	if p.filename != "" && p.lexeme.Position.IsValid() {
		msg = p.lexeme.Position.File(p.filename) + ": " + msg
	}

	p.err = errors.New(msg)
	panic(bailout{})
}

func (p *parser) errorExpected(node ast.Node, msg string) {
	msg = "expected " + msg
	if node != nil && node.End() == p.lexeme.Position {
		// The error happened at the current position,
		// so make the error message more specific.
		lit := p.lexeme.Value
		if len(lit) > 64 {
			lit = lit[:64] + "..."
		}

		if lit == "" {
			msg += ", found " + p.lexeme.Token.String()
		} else {
			msg += ", found " + p.lexeme.Token.String() + " \"" + lit + `"`
		}
	}

	p.error(msg)
}

func (p *parser) parsePtr() *ast.Pointer {
	asteriskPos := p.lexeme.Position
	p.next()
	if p.lexeme.Token != token.Identifier {
		p.errorExpected(errorPos(p.lexeme.Position), token.Identifier.String())
	}

	return &ast.Pointer{AsteriskPos: asteriskPos, NotePos: p.lexeme.Position, Note: p.lexeme.Value}
}

func (p *parser) parseList() *ast.List {
	pos := p.lexeme.Position
	exprs := make([]ast.Expr, 0, 10) // Most lists will have fewer than 10 elements.
	p.next()

	for {
		switch p.lexeme.Token {
		case token.Identifier:
			x := &ast.Identifier{NamePos: p.lexeme.Position, Name: p.lexeme.Value}
			p.next()
			exprs = append(exprs, x)
		case token.String:
			x := &ast.String{QuotePos: p.lexeme.Position, Text: p.lexeme.Value}
			p.next()
			exprs = append(exprs, x)
		case token.Number:
			x := &ast.Number{ValuePos: p.lexeme.Position, Value: p.lexeme.Value}
			p.next()
			exprs = append(exprs, x)
		case token.Asterisk:
			x := p.parsePtr()
			p.next()
			exprs = append(exprs, x)
		case token.ParenOpen:
			x := p.parseList()
			p.next()
			exprs = append(exprs, x)
		case token.ParenClose:
			x := &ast.List{ParenOpen: pos, Elements: exprs, ParenClose: p.lexeme.Position}
			return x
		case token.Comment:
			p.addComment()
			p.next()
		case token.Error:
			p.error(p.lexeme.Value)
		default:
			p.errorExpected(errorPos(p.lexeme.Position), "closing parenthesis")
		}
	}
}

func (p *parser) expectList() *ast.List {
	for {
		switch p.lexeme.Token {
		case token.ParenOpen:
			x := p.parseList()
			p.next()
			return x
		case token.Comment:
			p.addComment()
			p.next()
		case token.EndOfFile:
			return nil
		default:
			p.errorExpected(errorPos(p.lexeme.Position), "list")
			return nil
		}
	}
}

func (p *parser) addComment() {
	c := &ast.Comment{Semicolon: p.lexeme.Position, Text: p.lexeme.Value}
	p.comments = append(p.comments, c)
}

func (p *parser) parseFile() *ast.File {
	// Expressions.
	var lists []*ast.List
	for p.lexeme.Token != token.EndOfFile {
		list := p.expectList()
		if list != nil {
			lists = append(lists, list)
		}
	}

	f := &ast.File{
		Lists: lists,
	}

	// Collect any comments into groups.
	var group []*ast.Comment
	for _, comment := range p.comments {
		if len(group) == 0 || group[len(group)-1].End().Offset()+1 == comment.Pos().Offset() {
			group = append(group, comment)
		} else {
			f.Comments = append(f.Comments, &ast.CommentGroup{List: group})
			group = append([]*ast.Comment(nil), comment)
		}
	}

	if len(group) != 0 {
		f.Comments = append(f.Comments, &ast.CommentGroup{List: group})
	}

	return f
}
