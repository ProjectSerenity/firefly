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
	BaseAddr  *ast.Literal
	Types     *types.Package
	Imports   []string
	Constants []*types.Constant // Named constants.
	Literals  []*types.Constant // Unnamed constant literals.
	Functions []*ssafir.Function
}

// MachineCode can be used as a placeholder for a
// compiled function.
type MachineCode []byte

// EncodeTo writes the machine code implementation
// of fun to w. If the function is not fully
// compiled, EncodeTo will return an error.
func EncodeTo(w io.Writer, fset *token.FileSet, arch *sys.Arch, fun *ssafir.Function) error {
	if code, ok := fun.Extra.(MachineCode); ok {
		_, err := w.Write(code)
		return err
	}

	switch arch {
	case sys.X86_64:
		return encodeX86(w, fset, fun)
	default:
		return fmt.Errorf("unsupported architecture: %s", arch.Name)
	}
}

// assemble compiles an assembly function.
func assemble(fset *token.FileSet, arch *sys.Arch, pkg *types.Package, assembly *ast.List, info *types.Info, sizes types.Sizes) (*ssafir.Function, error) {
	switch arch {
	case sys.X86_64:
		return assembleX86(fset, arch, pkg, assembly, info, sizes)
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", arch.Name)
	}
}

// compile compiles a Ruse function.
func compile(fset *token.FileSet, arch *sys.Arch, pkg *types.Package, expr *ast.List, info *types.Info, sizes types.Sizes) (*ssafir.Function, error) {
	// Find the function details.
	name := expr.Elements[1].(*ast.List).Elements[0].(*ast.Identifier)
	function := info.Definitions[name].(*types.Function)
	signature := function.Type().(*types.Signature)
	fun := &ssafir.Function{
		Name: name.Name,
		Code: expr,
		Func: function,
		Type: signature,

		NamedValues: make(map[*types.Variable][]*ssafir.Value),
	}

	// Compile the body.
	c := &compiler{
		fset:  fset,
		arch:  arch,
		pkg:   pkg,
		info:  info,
		fun:   fun,
		list:  expr,
		sizes: sizes,

		vars: make(map[*types.Variable]*ssafir.Value),
	}

	c.AddCallingConvention()
	c.AddFunctionPrelude()
	c.AddFunctionInitialValues()
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
		Name:    pkg.Name,
		Path:    pkg.Path,
		Types:   pkg,
		Imports: pkg.Imports,
	}

	// Process any package-level annotations.
	for _, file := range files {
		for _, x := range file.Package.Annotations {
			anno := x.X
			keyword := anno.Elements[0].(*ast.Identifier)
			switch keyword.Name {
			case "base-address":
				addr := anno.Elements[1].(*ast.Literal)
				if p.Name != "main" {
					return nil, fmt.Errorf("%s: invalid package annotation: base address can only be specified by package main", fset.Position(x.Quote))
				}
				if p.BaseAddr != nil {
					return nil, fmt.Errorf("%s: invalid package annotation: base address already specified at %s", fset.Position(x.Quote), fset.Position(p.BaseAddr.ValuePos))
				}

				p.BaseAddr = addr
			default:
				panic("unexpected keyword " + keyword.Name)
			}
		}
	}

	// Identify all package-level constants.
	for _, file := range files {
	consts:
		for _, expr := range file.Expressions {
			// Skip other definitions.
			if expr.Elements[0].(*ast.Identifier).Name != "let" {
				continue
			}

			// Process any annotations.
			var sectionSymbol string
			for _, anno := range expr.Annotations {
				keyword := anno.X.Elements[0].(*ast.Identifier)
				switch keyword.Name {
				case "arch":
					// Ignore declarations for other architectures.
					nameElt := anno.X.Elements[1]
					ident, ok := nameElt.(*ast.Identifier)
					if !ok {
						return nil, fmt.Errorf("%s: invalid architecture declaration: got %s, want identifier", fset.Position(nameElt.Pos()), nameElt)
					}

					got, ok := sys.ArchByName[ident.Name]
					if !ok {
						return nil, fmt.Errorf("%s: invalid architecture declaration: architecture %s undefined", fset.Position(ident.NamePos), ident.Name)
					}

					if got != arch {
						continue consts
					}
				case "section":
					switch ref := anno.X.Elements[1].(type) {
					case *ast.Identifier:
						obj := info.Uses[ref]
						if obj == nil {
							return nil, fmt.Errorf("%s: invalid section reference %q", fset.Position(ref.Pos()), ref.Print())
						}

						con, ok := obj.(*types.Constant)
						if !ok {
							return nil, fmt.Errorf("%s: invalid section reference %q: got %s, want constant", fset.Position(ref.Pos()), ref.Print(), obj)
						}

						if _, ok := con.Type().(types.Section); !ok {
							return nil, fmt.Errorf("%s: invalid section reference %q: got %s, want section", fset.Position(ref.Pos()), ref.Print(), con.Type())
						}

						sectionSymbol = pkg.Path + "." + ref.Name
					case *ast.Qualified:
						obj := info.Uses[ref.X]
						if obj == nil {
							return nil, fmt.Errorf("%s: invalid section reference %q", fset.Position(ref.Pos()), ref.Print())
						}

						imp, ok := obj.(*types.Import)
						if !ok {
							return nil, fmt.Errorf("%s: invalid section reference %q: got %s, want import", fset.Position(ref.Pos()), ref.Print(), obj)
						}

						imported := imp.Imported()
						obj = info.Uses[ref.Y]
						if obj == nil {
							return nil, fmt.Errorf("%s: invalid section reference %q: not found in package %s", fset.Position(ref.Pos()), ref.Print(), imported.Path)
						}

						con, ok := obj.(*types.Constant)
						if !ok {
							return nil, fmt.Errorf("%s: invalid section reference %q: got %s, want constant", fset.Position(ref.Pos()), ref.Print(), obj)
						}

						if _, ok := con.Type().(types.Section); !ok {
							return nil, fmt.Errorf("%s: invalid section reference %q: got %s, want section", fset.Position(ref.Pos()), ref.Print(), con.Type())
						}

						sectionSymbol = imported.Path + "." + ref.Y.Name
					}
				}
			}

			// Find the identifier.
			var name *ast.Identifier
			switch x := expr.Elements[1].(type) {
			case *ast.Identifier:
				name = x
			case *ast.List:
				name = x.Elements[0].(*ast.Identifier)
			}

			con := info.Definitions[name].(*types.Constant)
			con.SetSection(sectionSymbol)
			p.Constants = append(p.Constants, con)
		}
	}

	// Compile the functions.
	for _, file := range files {
	funcs:
		for _, expr := range file.Expressions {
			// Skip constant definitions.
			keyword := expr.Elements[0].(*ast.Identifier)
			if keyword.Name != "func" && keyword.Name != "asm-func" {
				continue
			}

			// Process any annotations.
			var sectionSymbol string
			for _, anno := range expr.Annotations {
				keyword := anno.X.Elements[0].(*ast.Identifier)
				switch keyword.Name {
				case "arch":
					// Ignore declarations for other architectures.
					nameElt := anno.X.Elements[1]
					ident, ok := nameElt.(*ast.Identifier)
					if !ok {
						return nil, fmt.Errorf("%s: invalid architecture declaration: got %s, want identifier", fset.Position(nameElt.Pos()), nameElt)
					}

					got, ok := sys.ArchByName[ident.Name]
					if !ok {
						return nil, fmt.Errorf("%s: invalid architecture declaration: architecture %s undefined", fset.Position(ident.NamePos), ident.Name)
					}

					if got != arch {
						continue funcs
					}
				case "section":
					switch ref := anno.X.Elements[1].(type) {
					case *ast.Identifier:
						obj := info.Uses[ref]
						if obj == nil {
							return nil, fmt.Errorf("%s: invalid section reference %q", fset.Position(ref.Pos()), ref.Print())
						}

						con, ok := obj.(*types.Constant)
						if !ok {
							return nil, fmt.Errorf("%s: invalid section reference %q: got %s, want constant", fset.Position(ref.Pos()), ref.Print(), obj)
						}

						if _, ok := con.Type().(types.Section); !ok {
							return nil, fmt.Errorf("%s: invalid section reference %q: got %s, want section", fset.Position(ref.Pos()), ref.Print(), con.Type())
						}

						sectionSymbol = pkg.Path + "." + ref.Name
					case *ast.Qualified:
						obj := info.Uses[ref.X]
						if obj == nil {
							return nil, fmt.Errorf("%s: invalid section reference %q", fset.Position(ref.Pos()), ref.Print())
						}

						imp, ok := obj.(*types.Import)
						if !ok {
							return nil, fmt.Errorf("%s: invalid section reference %q: got %s, want import", fset.Position(ref.Pos()), ref.Print(), obj)
						}

						imported := imp.Imported()
						obj = info.Uses[ref.Y]
						if obj == nil {
							return nil, fmt.Errorf("%s: invalid section reference %q: not found in package %s", fset.Position(ref.Pos()), ref.Print(), imported.Path)
						}

						con, ok := obj.(*types.Constant)
						if !ok {
							return nil, fmt.Errorf("%s: invalid section reference %q: got %s, want constant", fset.Position(ref.Pos()), ref.Print(), obj)
						}

						if _, ok := con.Type().(types.Section); !ok {
							return nil, fmt.Errorf("%s: invalid section reference %q: got %s, want section", fset.Position(ref.Pos()), ref.Print(), con.Type())
						}

						sectionSymbol = imported.Path + "." + ref.Y.Name
					}
				}
			}

			var err error
			var fun *ssafir.Function
			switch keyword.Name {
			case "func":
				fun, err = compile(fset, arch, pkg, expr, info, sizes)
			case "asm-func":
				fun, err = assemble(fset, arch, pkg, expr, info, sizes)
			default:
				continue
			}

			if err != nil {
				return nil, err
			}

			fun.Section = sectionSymbol
			p.Functions = append(p.Functions, fun)
		}
	}

	return p, nil
}

type compiler struct {
	fset  *token.FileSet
	arch  *sys.Arch
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
	if c.fun.Type.Result() != nil {
		result.Uses++
	}

	c.currentBlock.Kind = ssafir.BlockReturn
	c.currentBlock.End = end
	c.currentBlock.Control = result
}

func (c *compiler) AddCallingConvention() {
	params := make([]int, len(c.fun.Type.Params()))
	for i, param := range c.fun.Type.Params() {
		params[i] = c.sizes.SizeOf(param.Type())
	}

	var result int
	if res := c.fun.Type.Result(); res != nil {
		result = c.sizes.SizeOf(res)
	}

	abi := c.fun.Func.ABI()
	c.fun.Params = c.arch.Parameters(abi, params)
	c.fun.Result = c.arch.Result(abi, result)
}

func (c *compiler) AddFunctionPrelude() {
	b := c.Block(c.list.Elements[2].Pos(), ssafir.BlockNormal)
	c.fun.Entry = b
}

func (c *compiler) AddFunctionInitialValues() {
	c.lastMemoryState = c.fun.Entry.NewValue(c.list.Elements[0].Pos(), c.list.Elements[1].End(), ssafir.OpMakeMemoryState, ssafir.MemoryState{})
	params := c.fun.Type.Params()
	if len(params) == 0 {
		return
	}

	c.args = make([]*ssafir.Value, len(params))
	for i, param := range params {
		v := c.fun.Entry.NewValueInt(param.Pos(), param.End(), ssafir.OpParameter, param.Type(), int64(i))
		c.args[i] = v
		c.vars[param] = v
		c.fun.NamedValues[params[i]] = []*ssafir.Value{v}
	}
}

// tempLink stores a link-level action that
// needs to take place, but with some extra
// context needed during the assembly phase.
type tempLink struct {
	Link         *ssafir.Link
	InnerOffset  int     // Offset within an instruction.
	InnerAddress uintptr // Address within an instruction.
}
