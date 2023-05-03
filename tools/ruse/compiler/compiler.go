// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package compiler analyses a type-checked Ruse syntax tree,
// producing an abstract intermediate representation.
package compiler

import (
	"fmt"
	"io"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/ssafir"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

// Package contains the set of constants and functions
// defined in a Ruse package.
type Package struct {
	Name      string
	Path      string
	Constants []*types.Constant
	Functions []*ssafir.Function
}

// EncodeTo writes the machine code implementation
// of fun to w. If the function is not fully
// compiled, EncodeTo will return an error.
func EncodeTo(w io.Writer, fset *token.FileSet, arch *sys.Arch, fun *ssafir.Function) error {
	switch arch.Name {
	case "x86-64":
		return encodeX86(w, fset, fun)
	default:
		return fmt.Errorf("unsupported architecture: %s", arch.Name)
	}
}

// assemble compiles an assembly function.
func assemble(fset *token.FileSet, arch *sys.Arch, pkg *types.Package, assembly *ast.List, info *types.Info, sizes types.Sizes) (*ssafir.Function, error) {
	switch arch.Name {
	case "x86-64":
		return assembleX86(fset, pkg, assembly, info, sizes)
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", arch.Name)
	}
}

// compile compiles a Ruse function.
func compile(fset *token.FileSet, pkg *types.Package, expr *ast.List, info *types.Info, sizes types.Sizes) (*ssafir.Function, error) {
	// Find the function details.
	name := expr.Elements[1].(*ast.List).Elements[0].(*ast.Identifier)
	function := info.Definitions[name].(*types.Function)
	signature := function.Type().(*types.Signature)
	fun := &ssafir.Function{
		Name: name.Name,
		Type: signature,

		NamedValues: make(map[*types.Variable][]*ssafir.Value),
	}

	// Compile the body.
	c := &compiler{
		fset:  fset,
		pkg:   pkg,
		info:  info,
		fun:   fun,
		list:  expr,
		sizes: sizes,

		vars: make(map[*types.Variable]*ssafir.Value),
	}

	c.AddFunctionPrelude()
	for i, x := range expr.Elements[2:] {
		isLast := i+3 == len(expr.Elements)
		if !isLast || signature.Result() == nil {
			_, err := c.CompileExpression(x)
			if err != nil {
				return nil, err
			}

			if isLast {
				result := c.Value(x.Pos(), x.End(), ssafir.OpMakeResult, ssafir.Result{}, c.lastMemoryState)
				c.Return(c.list.ParenClose, result)
			}
		} else {
			v, err := c.CompileExpression(x)
			if err != nil {
				return nil, err
			}

			result := c.Value(x.Pos(), x.End(), ssafir.OpMakeResult, ssafir.Result{Value: v.Type}, v, c.lastMemoryState)
			c.Return(x.End(), result)
		}
	}

	return fun, nil
}

// Compile processes the syntax tree and type information,
// returning the corresponding intermediate representation.
func Compile(fset *token.FileSet, arch *sys.Arch, pkg *types.Package, files []*ast.File, info *types.Info, sizes types.Sizes) (*Package, error) {
	p := &Package{
		Name: pkg.Name,
		Path: pkg.Path,
	}

	// Identify all package-level constants.
	for _, file := range files {
		for _, expr := range file.Expressions {
			// Skip other definitions.
			if expr.Elements[0].(*ast.Identifier).Name != "let" {
				continue
			}

			// Find the identifier.
			var name *ast.Identifier
			switch x := expr.Elements[1].(type) {
			case *ast.Identifier:
				name = x
			case *ast.List:
				name = x.Elements[0].(*ast.Identifier)
			}

			p.Constants = append(p.Constants, info.Definitions[name].(*types.Constant))
		}
	}

	// Compile the functions.
	for _, file := range files {
	exprs:
		for _, expr := range file.Expressions {
			// Ignore functions for other architectures.
			for _, anno := range expr.Annotations {
				if ident, ok := anno.X.Elements[0].(*ast.Identifier); !ok || ident.Name != "arch" {
					continue
				}

				if ident, ok := anno.X.Elements[1].(*ast.Identifier); !ok || ident.Name != arch.Name {
					continue exprs
				}
			}

			// Skip constant definitions.
			var err error
			var fun *ssafir.Function
			switch expr.Elements[0].(*ast.Identifier).Name {
			case "func":
				fun, err = compile(fset, pkg, expr, info, sizes)
			case "asm-func":
				fun, err = assemble(fset, arch, pkg, expr, info, sizes)
			default:
				continue
			}

			if err != nil {
				return nil, err
			}

			p.Functions = append(p.Functions, fun)
		}
	}

	return p, nil
}

type compiler struct {
	fset  *token.FileSet
	pkg   *types.Package
	info  *types.Info
	fun   *ssafir.Function
	list  *ast.List
	sizes types.Sizes

	args            []*ssafir.Value
	vars            map[*types.Variable]*ssafir.Value
	currentBlock    *ssafir.Block
	lastMemoryState *ssafir.Value
}

func (c *compiler) Block(pos token.Pos, kind ssafir.BlockKind) *ssafir.Block {
	c.currentBlock = c.fun.NewBlock(pos, kind)
	return c.currentBlock
}

func (c *compiler) Value(pos, end token.Pos, op ssafir.Op, typ types.Type, args ...*ssafir.Value) *ssafir.Value {
	return c.currentBlock.NewValue(pos, end, op, typ, args...)
}

func (c *compiler) ValueInt(pos, end token.Pos, op ssafir.Op, typ types.Type, extra int64, args ...*ssafir.Value) *ssafir.Value {
	return c.currentBlock.NewValueInt(pos, end, op, typ, extra, args...)
}

func (c *compiler) ValueExtra(pos, end token.Pos, op ssafir.Op, typ types.Type, extra any, args ...*ssafir.Value) *ssafir.Value {
	return c.currentBlock.NewValueExtra(pos, end, op, typ, extra, args...)
}

func (c *compiler) Return(end token.Pos, result *ssafir.Value) {
	c.currentBlock.Kind = ssafir.BlockReturn
	c.currentBlock.End = end
	c.currentBlock.Control = result
}

func (c *compiler) AddFunctionPrelude() {
	b := c.Block(c.list.Elements[2].Pos(), ssafir.BlockNormal)
	c.fun.Entry = b
	c.lastMemoryState = b.NewValue(c.list.ParenOpen, c.list.ParenClose, ssafir.OpMakeMemoryState, ssafir.MemoryState{})
	params := c.fun.Type.Params()
	if len(params) == 0 {
		return
	}

	c.args = make([]*ssafir.Value, len(params))
	for i, param := range params {
		v := b.NewValueInt(param.Pos(), param.End(), ssafir.OpParameter, param.Type(), int64(i))
		c.args[i] = v
		c.vars[param] = v
		c.fun.NamedValues[params[i]] = []*ssafir.Value{v}
	}
}
