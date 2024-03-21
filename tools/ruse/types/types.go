// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package types implements a type checker for Ruse source files.
package types

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/constant"
	"firefly-os.dev/tools/ruse/parser"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
)

var (
	ErrNoFiles = errors.New("types: package has no files")
)

// Type represents a type in the Ruse type system.
type Type interface {
	Underlying() Type // Primitive types return themselves.
	String() string   // Returns a the type's name.
}

// TypeAndValue stores the type of an expression and
// its value if the expression is constant.
type TypeAndValue struct {
	Type  Type
	Value constant.Value
}

// AssignableTo returns whether a value of type value
// can be assigned to a value of type base.
func AssignableTo(base, value Type) bool {
	// TODO: make AssignableTo more sophisticated.
	uBase := Underlying(base)
	uValue := Underlying(value)
	if uBase == uValue {
		return true
	}

	switch uValue {
	case UntypedBool:
		return uBase == Bool
	case UntypedInt:
		switch uBase {
		case Int,
			Int8,
			Int16,
			Int32,
			Int64,
			Byte,
			Uint,
			Uint8,
			Uint16,
			Uint32,
			Uint64,
			Uintptr:
			return true
		}
	case UntypedString:
		if uBase == String {
			return true
		}
	}

	if aBase, ok := uBase.(*Array); ok {
		aValue, ok := uValue.(*Array)
		if ok &&
			aBase.Length() == aValue.Length() &&
			aBase.Element() == aValue.Element() {
			return true
		}

		return false
	}

	return false
}

// typesList returns the string representation for
// a list of types in Ruse form.
func typesList(types []Type) string {
	var b strings.Builder
	b.WriteByte('(')
	for i, typ := range types {
		if i > 0 {
			b.WriteByte(' ')
		}

		b.WriteString(typ.String())
	}

	b.WriteByte(')')

	return b.String()
}

// paramsList returns the string representation for
// a list of types in Ruse form.
func paramsList(types []*Variable) string {
	var b strings.Builder
	b.WriteByte('(')
	for i, typ := range types {
		if i > 0 {
			b.WriteByte(' ')
		}

		b.WriteString(typ.Type().String())
	}

	b.WriteByte(')')

	return b.String()
}

// Underlying returns the base type.
func Underlying(t Type) Type {
	for t != nil {
		next := t.Underlying()
		if next == t {
			return t
		}

		t = next
	}

	panic("unreachable")
}

// Sizes describes the memory sizes of an architecture.
type Sizes interface {
	SizeOf(typ Type) int      // Size in bytes.
	AlignmentOf(typ Type) int // Alignment in bytes.
}

// SizesFor returns an implementation of the Sizes type
// for the given architecture.
func SizesFor(a *sys.Arch) Sizes {
	return standardSizes{
		WordSize:     a.PointerSize,
		MaxAlignment: a.MaxAlignment,
	}
}

// standardSizes is a helper type that simplifies the
// implementation of the Sizes interface.
type standardSizes struct {
	WordSize     int // Architecture word size in bytes.
	MaxAlignment int // Largest alignment in bytes.
}

var _ Sizes = standardSizes{}

var basicSizes = [...]int{
	KindBool:   1,
	KindInt8:   1,
	KindInt16:  2,
	KindInt32:  4,
	KindInt64:  8,
	KindUint8:  1,
	KindUint16: 2,
	KindUint32: 4,
	KindUint64: 8,
}

func (s standardSizes) SizeOf(typ Type) int {
	switch t := Underlying(typ).(type) {
	case *Basic:
		if int(t.kind) < len(basicSizes) && basicSizes[t.kind] != 0 {
			return basicSizes[t.kind]
		}

		if t.kind == KindString || t.kind == KindUntypedString {
			return 2 * s.WordSize
		}
	case *Array:
		return int(t.Length()) * s.SizeOf(t.Element())
	default:
		panic(fmt.Sprintf("unexpected underlying type: %T", t))
	}

	return s.WordSize
}

func (s standardSizes) AlignmentOf(typ Type) int {
	if array, ok := typ.(*Array); ok {
		return s.AlignmentOf(array.Element())
	}

	size := s.SizeOf(typ)
	if size <= s.MaxAlignment {
		return size
	}

	return s.MaxAlignment
}

// Info holds type information about a set of Ruse
// code.
//
// Only the fields that are initialised before calling
// Check are populated.
type Info struct {
	List        []Type                          // The list of types in an implementation-defined order.
	Indices     map[Type]int                    // Mapping each type to its position in List.
	Types       map[ast.Expression]TypeAndValue // The type (and value for constants) of each expression.
	Definitions map[*ast.Identifier]Object      // Identifiers that define a new object.
	Uses        map[*ast.Identifier]Object      // Identifiers that refer to an object.
	Packages    map[string]*Package             // Packages available to import.
}

// Package represents a type-checked Ruse package.
type Package struct {
	Path string // The full package path.
	Name string // The local package name.

	// The packages imported by this one.
	Imports []string

	scope *Scope
}

func (p *Package) Scope() *Scope {
	if p == nil {
		return Universe
	}

	if p.scope == nil {
		p.scope = new(Scope)
	}

	return p.scope
}

// Check performs type checking on the given files.
//
// Path is the complete package path, such as
// "firefly-os.dev/kernel/multitasking".
//
// Any optional info is populated, as described
// in the Info type.
func Check(packagePath string, fset *token.FileSet, files []*ast.File, arch *sys.Arch, info *Info) (*Package, error) {
	if len(files) == 0 {
		return nil, ErrNoFiles
	}

	if info == nil {
		info = new(Info)
	}

	pkg := &Package{
		Path:  packagePath,
		scope: NewScope(Universe, token.NoPos, token.NoPos, fmt.Sprintf("package %s", packagePath)),
	}

	checker := &checker{
		fset:   fset,
		info:   info,
		pkg:    pkg,
		arch:   arch,
		sigs:   make(map[token.Pos]*Signature),
		funcs:  make(map[token.Pos]*Function),
		names:  make(map[token.Pos]string),
		consts: make(map[ast.Expression]constant.Value),
	}

	return pkg, checker.Check(files)
}

// A checker is used to store state while type
// checking the files of a single Ruse package.
type checker struct {
	Error  func(err error)
	fset   *token.FileSet
	info   *Info
	pkg    *Package
	arch   *sys.Arch
	sigs   map[token.Pos]*Signature
	funcs  map[token.Pos]*Function
	names  map[token.Pos]string
	consts map[ast.Expression]constant.Value
}

func (c *checker) newType(typ Type) {
	if c.info.List != nil {
		if c.info.Indices != nil {
			c.info.Indices[typ] = len(c.info.List)
		}

		c.info.List = append(c.info.List, typ)
	}
}

func (c *checker) define(ident *ast.Identifier, obj Object) {
	if c.info.Definitions != nil {
		c.info.Definitions[ident] = obj
	}
}

func (c *checker) use(ident *ast.Identifier, obj Object) {
	if c.info.Uses != nil {
		c.info.Uses[ident] = obj
	}
}

func (c *checker) record(expr ast.Expression, typ Type, value constant.Value) {
	if value != nil {
		c.consts[expr] = value
	}

	if c.info.Types != nil {
		c.info.Types[expr] = TypeAndValue{Type: typ, Value: value}
	}
}

func (c *checker) Check(files []*ast.File) error {
	// Start by checking for any unexpected
	// annotations.
	if err := c.CheckAnnotations(files); err != nil {
		return err
	}

	seenImport := make(map[string]bool)
	fileScopes := make([]*Scope, len(files))
	for i, file := range files {
		if c.pkg.Name == "" {
			c.pkg.Name = file.Name.Name
		} else if file.Name.Name != c.pkg.Name {
			return c.errorf(file.Package.ParenOpen, "found package name %q, expected %q", file.Name.Name, c.pkg.Name)
		}

		// Check that the path is valid, given
		// the package name.
		if c.pkg.Name != "main" && strings.TrimSuffix(c.pkg.Name, "_test") != path.Base(c.pkg.Path) {
			return c.errorf(file.Package.ParenOpen, "found package name %q, expected %q or %q", file.Name.Name, "main", path.Base(c.pkg.Path))
		}

		// Create the file scope.
		scope := NewScope(c.pkg.scope, file.Pos(), file.End(), fmt.Sprintf("file %d", i))
		fileScopes[i] = scope

		// Resolve any imports.
		for _, imp := range file.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return c.errorf(imp.Path.ValuePos, "found malformed import path: %v", err)
			}

			dep := c.info.Packages[importPath]
			if dep == nil {
				return c.errorf(imp.Path.ValuePos, "no package found for import %q", importPath)
			}

			var name string
			if imp.Name != nil {
				name = imp.Name.Name
			} else {
				// Check that the import
				// path is a valid identifier.
				name = path.Base(importPath)
				x, err := parser.ParseExpression(name)
				if err != nil {
					return c.errorf(imp.Path.ValuePos, "invalid import path: base %q is not a valid identifier: %v", importPath, err)
				}

				if _, ok := x.(*ast.Identifier); !ok {
					return c.errorf(imp.Path.ValuePos, "invalid import path: base %q is not a valid identifier: got %v", importPath, x)
				}
			}

			if !seenImport[importPath] {
				seenImport[importPath] = true
				c.pkg.Imports = append(c.pkg.Imports, importPath)
			}

			ref := NewImport(scope, imp.List.ParenOpen, imp.List.ParenClose, c.pkg, name, dep)

			// Imports only affect the file scope,
			// not the entire package.
			scope.Insert(ref)
		}

		// Having added any imports, we now
		// mark the file scope as read-only,
		// so that insertions are applied to
		// the package scope.
		scope.readonly = true

		// Check the top-level expressions.
		//
		// For now, we skip function bodies.
		// We do that once we have fully checked
		// all top-level expressions, so we should
		// not need any new types.
		for _, expr := range file.Expressions {
			kind, _, err := c.interpretDefinition(expr, "top-level list")
			if err != nil {
				return c.error(err)
			}

			switch kind.Name {
			case "asm-func":
				c.use(kind, specialForms[SpecialFormAsmFunc])
				err := c.CheckTopLevelAsmFuncDecl(scope, expr)
				if err != nil {
					return err
				}
			case "func":
				c.use(kind, specialForms[SpecialFormFunc])
				err := c.CheckTopLevelFuncDecl(scope, expr)
				if err != nil {
					return err
				}
			case "let":
				c.use(kind, specialForms[SpecialFormLet])
				err := c.CheckTopLevelLet(scope, expr)
				if err != nil {
					return err
				}
			default:
				return c.errorf(kind.NamePos, "invalid top-level function: %s is not a builtin function", kind.Name)
			}
		}
	}

	sort.Strings(c.pkg.Imports)

	// We do a second pass, where we
	// type-check function bodies, now
	// that we have all package-level
	// declarations.
	for i, file := range files {
		scope := fileScopes[i]
		for _, expr := range file.Expressions {
			kind, _, err := c.interpretDefinition(expr, "top-level list")
			if err != nil {
				return c.error(err)
			}

			switch kind.Name {
			case "asm-func":
				err := c.ResolveAsmFuncBody(scope, expr)
				if err != nil {
					return err
				}
			case "func":
				_, err := c.ResolveFuncBody(scope, expr)
				if err != nil {
					return err
				}
			case "let":
				err := c.ResolveLetBody(scope, expr)
				if err != nil {
					return err
				}
			default:
				return c.errorf(kind.NamePos, "invalid top-level function: %s is not a builtin function", kind.Name)
			}
		}
	}

	// Finally, we process any
	// annotations on the package
	// statement.
	for i, file := range files {
		err := c.iterAnnotations(file.Package, func(list, anno *ast.List, keyword *ast.Identifier) error {
			switch keyword.Name {
			case "base-address":
				// Nothing to do here.
			case "sections":
				for _, elt := range anno.Elements[1:] {
					_, typ, err := c.ResolveExpression(fileScopes[i], elt)
					if err != nil {
						return err
					}

					_, ok := typ.(Section)
					if !ok {
						return c.errorf(elt.Pos(), "cannot use %s (%s) as section", elt.Print(), elt)
					}
				}
			default:
				return c.errorf(anno.ParenOpen, "invalid package annotation: unrecognised annotation type: %s", keyword.Name)
			}

			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *checker) GetNameTypePair(scope *Scope, x ast.Expression) (name *ast.Identifier, typ Type, err error) {
	pair, ok := x.(*ast.List)
	if !ok {
		return nil, nil, fmt.Errorf("want a (name type) list, found %s", x)
	}

	if len(pair.Elements) != 2 {
		return nil, nil, fmt.Errorf("want a (name type) list, found %d elements", len(pair.Elements))
	}

	name, ok = pair.Elements[0].(*ast.Identifier)
	if !ok {
		return nil, nil, fmt.Errorf("invalid name: want an identifier, found %s", pair.Elements[0])
	}

	typeName, ok := pair.Elements[1].(*ast.Identifier)
	if !ok {
		return nil, nil, fmt.Errorf("invalid type: want an identifier, found %s", pair.Elements[1])
	}

	_, obj := scope.LookupParent(typeName.Name, token.NoPos)
	if obj == nil {
		return nil, nil, fmt.Errorf("undefined type: %s", typeName.Name)
	}

	typ = obj.Type()
	c.use(typeName, obj)
	c.record(typeName, typ, nil)

	return name, typ, nil
}

func (c *checker) checkArchitectures(anno *ast.QuotedList) (ok bool, err error) {
	var archOk bool
	for _, x := range anno.X.Elements[1:] {
		arch, ok := x.(*ast.Identifier)
		if !ok {
			return false, c.errorf(x.Pos(), "invalid architecture: expected identifier, found %s", x)
		}

		if arch.Name == c.arch.Name {
			// Match!
			archOk = true
			break
		}

		// Check the arch is valid.
		ok = false
		for _, a := range sys.All {
			if a.Name == arch.Name {
				ok = true
				break
			}
		}

		if !ok {
			return false, c.errorf(x.Pos(), "unrecognised architecture: %s", arch.Name)
		}
	}

	return archOk, nil
}

func (c *checker) checkFuncSignature(parent *Scope, fun *ast.List) error {
	// Handle any common annotations.
	alignment := 1
	for _, anno := range fun.Annotations {
		kind := anno.X.Elements[0].(*ast.Identifier) // Enforced by the parser.
		switch kind.Name {
		case "align":
			// We expect a single uint32.
			if len(anno.X.Elements) != 2 {
				return c.errorf(anno.X.ParenOpen, "invalid alignment annotation: must specify one alignment value, got %d arguments", len(anno.X.Elements)-1)
			}

			alignElt := anno.X.Elements[1]
			lit, ok := alignElt.(*ast.Literal)
			if !ok || lit.Kind != token.Integer {
				return c.errorf(alignElt.Pos(), "invalid alignment annotation: must specify one alignment value, got %s", alignElt)
			}

			val, err := strconv.ParseUint(lit.Value, 0, 32)
			if err != nil {
				return c.errorf(lit.ValuePos, "invalid alignment annotation: %v", err)
			}

			if val == 0 {
				return c.errorf(alignElt.Pos(), "invalid alignment annotation: invalid alignment %s", lit.Value)
			}

			if alignment != 1 {
				return c.errorf(lit.ValuePos, "cannot specify alignment more than once")
			}

			alignment = int(val)
		}
	}

	// Unpack the declaration.
	decl, ok := fun.Elements[1].(*ast.List)
	if !ok {
		return c.errorf(fun.Elements[1].Pos(), "invalid function declaration: declaration must be a list of the function name, parameters, and result type, found %s", fun.Elements[1])
	}

	if len(decl.Elements) == 0 {
		return c.errorf(fun.ParenClose, "invalid function declaration: declaration missing function name, parameters, and result type")
	}

	name, ok := decl.Elements[0].(*ast.Identifier)
	if !ok {
		return c.errorf(decl.Elements[0].Pos(), "invalid function declaration: expected function name, found %s", decl.Elements[0])
	}

	params := decl.Elements[1:]
	var resultType Type
	var resultTypeName string
	if len(params) != 0 {
		// Check whether we have a result type,
		// which will be a final element that is
		// an identifier. If not, we only have
		// parameters.
		result, ok := params[len(params)-1].(*ast.Identifier)
		if ok {
			_, obj := parent.LookupParent(result.Name, token.NoPos)
			if obj == nil {
				return c.errorf(result.NamePos, "undefined type: %s", result.Name)
			}

			resultType = obj.Type()
			resultTypeName = result.Name
			c.use(result, obj)
			c.record(result, resultType, nil)
			params = params[:len(params)-1] // Trim the result type from the parameter list.
		}
	}

	scope := NewScope(parent, fun.Elements[2].Pos(), fun.ParenClose, "function "+name.Name)

	var buf strings.Builder
	buf.WriteString("(func")
	paramTypes := make([]*Variable, len(params))
	for i, param := range params {
		name, typ, err := c.GetNameTypePair(parent, param)
		if err != nil {
			return c.errorf(param.Pos(), "bad parameter %d: %v", i+1, err)
		}

		obj := NewParameter(scope, param.Pos(), param.End(), c.pkg, name.Name, typ)
		if other := scope.Insert(obj); other != nil {
			return c.errorf(param.Pos(), "%s redeclared: previous declaration at %s", name.Name, c.fset.Position(other.Pos()))
		}

		paramTypes[i] = obj
		c.define(name, obj)
		c.record(param, typ, nil)
		c.record(name, typ, nil)
		c.names[params[i].Pos()] = name.Name
		fmt.Fprintf(&buf, " (%s)", typ)
	}

	if resultTypeName != "" {
		buf.WriteByte(' ')
		buf.WriteString(resultTypeName)
	}

	buf.WriteByte(')')

	signature := NewSignature(buf.String(), paramTypes, resultType)
	c.newType(signature)
	function := NewFunction(parent, fun.ParenOpen, fun.ParenClose, c.pkg, name.Name, signature, alignment)
	c.sigs[fun.ParenOpen] = signature
	c.funcs[fun.ParenOpen] = function
	c.names[fun.ParenOpen] = "function " + name.Name
	c.define(name, function)
	c.record(fun, signature, nil)
	c.record(fun.Elements[0], signature, nil)
	c.record(decl, signature, nil)
	c.record(name, signature, nil)
	if other := parent.Insert(function); other != nil {
		return c.errorf(fun.ParenOpen, "%s redeclared: previous declaration at %s", name.Name, c.fset.Position(other.Pos()))
	}

	return nil
}

func (c *checker) CheckTopLevelAsmFuncDecl(parent *Scope, fun *ast.List) error {
	// Named assembly function declaration.
	//
	// Takes one of the following forms:
	//
	// - (asm-func (name) ...)                                   ; Assembly function declaration, declaring function 'name' with no parameters or result.
	// - (asm-func (name result) ...)                            ; Assembly function declaration, declaring function 'name' with result type 'result' and no parameters.
	// - (asm-func (name (arg1 typ1) (arg2 type2)) ...)          ; Assembly function declaration, declaring function 'name' with parameters 'arg1' and 'arg2' and types 'type1' and 'type2' and no result.
	// - (asm-func (name (arg1 type1) (arg2 type2) result) ...)  ; Assembly function declaration, declaring function 'name' with parameters and a result type.
	//
	// Function definitions take the following annotations:
	//
	// - '(abi abi)                                              ; Optional ABI declaration, specifying the calling convention for this function.
	// - '(arch architecture...)                                 ; Opttonal architecture declaration, specifying the architectures for which this declaration is valid.
	// - '(mode mode)                                            ; Optional CPU mode indicating how instructions should be encoded.
	switch len(fun.Elements) {
	case 1:
		return c.errorf(fun.ParenClose, "invalid assembly function declaration: no function name or body")
	case 2:
		return c.errorf(fun.ParenClose, "invalid assembly function declaration: empty function body")
	}

	var seenABI bool
	for _, anno := range fun.Annotations {
		kind := anno.X.Elements[0].(*ast.Identifier) // Enforced by the parser.
		switch kind.Name {
		case "abi":
			if seenABI {
				return c.errorf(anno.X.ParenOpen, "duplicate ABI spec")
			}

			seenABI = true
		case "align":
			// We handle this in checkFuncSignature.
		case "arch":
			// Check that this matches the target architecture.
			archOk, err := c.checkArchitectures(anno)
			if err != nil {
				return err
			}

			// Skip this function so its type signature
			// cannot clash with another of the same
			// name and another architecture.
			if !archOk {
				return nil
			}
		case "mode":
			if len(anno.X.Elements[1:]) != 1 {
				return c.errorf(anno.X.ParenOpen, "invalid mode: expected one mode value")
			}

			// Otherwise ignored by the type checker.
			continue
		case "section":
			// Nothing to do yet.
		default:
			return c.errorf(kind.NamePos, "unrecognised annotation: %s", kind.Name)
		}
	}

	return c.checkFuncSignature(parent, fun)
}

func (c *checker) CheckTopLevelFuncDecl(parent *Scope, fun *ast.List) error {
	// Named function declaration.
	//
	// Takes one of the following forms:
	//
	// - (func (name) ...)                                   ; Function declaration, declaring function 'name' with no parameters or result.
	// - (func (name result) ...)                            ; Function declaration, declaring function 'name' with result type 'result' and no parameters.
	// - (func (name (arg1 typ1) (arg2 type2)) ...)          ; Function declaration, declaring function 'name' with parameters 'arg1' and 'arg2' and types 'type1' and 'type2' and no result.
	// - (func (name (arg1 type1) (arg2 type2) result) ...)  ; Function declaration, declaring function 'name' with parameters and a result type.
	//
	// Function definitions take the following annotations:
	//
	// - '(abi abi)                                          ; Optional ABI declaration, specifying the calling convention for this function.
	// - '(arch architecture...)                             ; Opttonal architecture declaration, specifying the architectures for which this declaration is valid.
	switch len(fun.Elements) {
	case 1:
		return c.errorf(fun.ParenClose, "invalid function declaration: no function name or body")
	case 2:
		return c.errorf(fun.ParenClose, "invalid function declaration: empty function body")
	}

	var seenABI bool
	for _, anno := range fun.Annotations {
		kind := anno.X.Elements[0].(*ast.Identifier) // Enforced by the parser.
		switch kind.Name {
		case "abi":
			if seenABI {
				return c.errorf(anno.X.ParenOpen, "duplicate ABI spec")
			}

			seenABI = true
		case "align":
			// We handle this in checkFuncSignature.
		case "arch":
			// Check that this matches the target architecture.
			archOk, err := c.checkArchitectures(anno)
			if err != nil {
				return err
			}

			// Skip this function so its type signature
			// cannot clash with another of the same
			// name and another architecture.
			if !archOk {
				return nil
			}
		case "mode":
			if len(anno.X.Elements[1:]) != 1 {
				return c.errorf(anno.X.ParenOpen, "invalid mode: expected one mode value")
			}

			// Otherwise ignored by the type checker.
			continue
		case "section":
			// Nothing to do yet.
		default:
			return c.errorf(kind.NamePos, "unrecognised annotation: %s", kind.Name)
		}
	}

	return c.checkFuncSignature(parent, fun)
}

func (c *checker) ResolveAsmFuncBody(scope *Scope, fun *ast.List) error {
	function := c.funcs[fun.ParenOpen]
	if function == nil {
		return c.errorf(fun.ParenOpen, "internal error: no function object found")
	}

	sig := c.sigs[fun.ParenOpen]
	if sig == nil {
		return c.errorf(fun.ParenOpen, "internal error: no function signature found")
	}

	name, ok := c.names[fun.ParenOpen]
	if !ok {
		return c.errorf(fun.ParenOpen, "internal error: no function name found")
	}

	// Resolve the ABI if one is specified.
	for _, anno := range fun.Annotations {
		kind := anno.X.Elements[0].(*ast.Identifier) // Enforced by the parser.
		switch kind.Name {
		case "abi":
			elt := anno.X.Elements[1]
			_, typ, err := c.ResolveExpression(scope, elt)
			if err != nil {
				return err
			}

			abi, ok := typ.(ABI)
			if !ok {
				return c.errorf(elt.Pos(), "cannot use %s (%s) as ABI", elt.Print(), elt)
			}

			function.SetABI(abi.abi)
		case "section":
			elt := anno.X.Elements[1]
			_, typ, err := c.ResolveExpression(scope, elt)
			if err != nil {
				return err
			}

			_, ok := typ.(Section)
			if !ok {
				return c.errorf(elt.Pos(), "cannot use %s (%s) as section", elt.Print(), elt)
			}
		}
	}

	// Fetch the function body's scope.
	scope = scope.Innermost(fun.Elements[2].Pos())
	if scope == nil {
		return c.errorf(fun.Elements[2].Pos(), "internal error: failed to find scope")
	}

	for _, expr := range fun.Elements[2:] {
		// Assembly functions are a little odd, in that
		// most of the syntax isn't checked. This is
		// because the syntax varies between architectures.
		//
		// Instead, we only check a limited set of Ruse
		// expression forms, which can be used inline in
		// assembly. We leave it to the assembler to identify
		// other syntax errors.
		//
		// As a result, we generally ignore errors here
		// and just skip unfamiliar syntax, unlike in the
		// rest of the type checker.
		//
		// Possible inline syntax is one or more lists in
		// a list. For example, in x86:
		//
		// ```
		// 	(asm-func foo
		// 		(mov rax (string-pointer bar)))
		// ```
		list, ok := expr.(*ast.List)
		if !ok {
			// Probably a position label or similar.
			continue
		}

		for _, elt := range list.Elements {
			fun, ok := elt.(*ast.List)
			if !ok || len(fun.Elements) < 2 {
				continue
			}

			ident, ok := fun.Elements[0].(*ast.Identifier)
			if !ok {
				continue
			}

			switch ident.Name {
			case "len":
				// String reference: must consist of an
				// identifier that is bound to a string
				// constant.
				if len(fun.Elements) != 2 {
					return c.errorf(fun.Elements[2].Pos(), "%s has too many arguments in call to len: expected %d, found %d", name, 1, len(fun.Elements)-1)
				}

				arg := fun.Elements[1]
				obj, typ, err := c.ResolveExpression(scope, arg)
				if err != nil {
					return err
				}

				// TODO: Add support for more types to special form len.
				if !AssignableTo(String, typ) {
					return c.errorf(arg.Pos(), "%s has invalid argument: %s (%s) for len", name, arg.Print(), typ)
				}

				// Make the length of a constant string also
				// a constant.
				con, ok := obj.(*Constant)
				if !ok {
					return c.errorf(arg.Pos(), "%s has invalid argument: %s (non-constant string)", name, arg.Print())
				}

				val := constant.StringVal(con.value)
				value := constant.MakeInt64(int64(len(val)))

				c.record(fun, Int, value)
			case "string-pointer":
				// String reference; must consist of an
				// identifier that is bound to a string
				// constant.
				if len(fun.Elements) != 2 {
					return c.errorf(fun.Elements[2].Pos(), "%s has too many arguments in call to string-pointer: expected %d, found %d", name, 1, len(fun.Elements)-1)
				}

				arg := fun.Elements[1]
				obj, typ, err := c.ResolveExpression(scope, arg)
				if err != nil {
					return err
				}

				if !AssignableTo(String, typ) {
					return c.errorf(arg.Pos(), "%s has invalid argument: %s (%s) for string-pointer", name, arg.Print(), typ)
				}

				con, ok := obj.(*Constant)
				if !ok {
					return c.errorf(arg.Pos(), "%s has invalid argument: %s (non-constant string)", name, arg.Print())
				}

				c.record(fun, String, con.value)
			case "@":
				// Memory reference; must consist of an
				// identifier that is bound to a string
				// or array constant.
				if len(fun.Elements) != 2 {
					return c.errorf(fun.Elements[2].Pos(), "%s has too many arguments in reference: expected %d, found %d", name, 1, len(fun.Elements)-1)
				}

				arg := fun.Elements[1]
				obj, typ, err := c.ResolveExpression(scope, arg)
				if err != nil {
					return err
				}

				// Handle function references first.
				if _, ok := obj.(*Function); ok {
					c.record(fun, typ, nil)
					break
				}

				underlying := Underlying(typ)
				if _, isArray := underlying.(*Array); !isArray && underlying != String && underlying != UntypedString {
					return c.errorf(arg.Pos(), "%s has invalid argument: %s (%s) for reference", name, arg.Print(), typ)
				}

				con, ok := obj.(*Constant)
				if !ok {
					return c.errorf(arg.Pos(), "%s has invalid argument: %s (non-constant reference)", name, arg.Print())
				}

				c.record(fun, typ, con.value)
			case "int8", "int16", "int32", "int64",
				"uint8", "uint16", "uint32", "uint64":
				// This is where we reference a Ruse constant
				// and insert it into the assembly.
				if len(fun.Elements) != 2 {
					return c.errorf(fun.Elements[2].Pos(), "%s has too many arguments in constant: expected %d, found %d", name, 1, len(fun.Elements)-1)
				}

				// First, get the result type.
				_, obj := scope.LookupParent(ident.Name, token.NoPos)

				result := obj.Type()
				c.use(ident, obj)
				c.record(ident, result, nil)

				// Next, resolve the value and check
				// it's assignable to the type.
				arg := fun.Elements[1]
				obj, typ, err := c.ResolveExpression(scope, arg)
				if err != nil {
					return c.errorf(arg.Pos(), "%s has invalid argument: %v", name, err)
				}

				if !AssignableTo(result, typ) {
					return c.errorf(arg.Pos(), "%s has invalid argument: cannot assign %s (%s) to %s", name, arg, typ, result)
				}

				value, ok := c.consts[arg]
				if !ok {
					return c.errorf(arg.Pos(), "%s has invalid argument: %s (non-constant value %s)", name, arg.Print(), obj)
				}

				c.record(fun, result, value)
			default:
				// Ignore unrecognised syntax.
				continue
			}
		}
	}

	return nil
}

func (c *checker) ResolveFuncBody(scope *Scope, fun *ast.List) (result Type, err error) {
	function := c.funcs[fun.ParenOpen]
	if function == nil {
		return nil, c.errorf(fun.ParenOpen, "internal error: no function object found")
	}

	sig := c.sigs[fun.ParenOpen]
	if sig == nil {
		return nil, c.errorf(fun.ParenOpen, "internal error: no function signature found")
	}

	name, ok := c.names[fun.ParenOpen]
	if !ok {
		return nil, c.errorf(fun.ParenOpen, "internal error: no function name found")
	}

	// Resolve the ABI if one is specified.
	for _, anno := range fun.Annotations {
		kind := anno.X.Elements[0].(*ast.Identifier) // Enforced by the parser.
		switch kind.Name {
		case "abi":
			elt := anno.X.Elements[1]
			_, typ, err := c.ResolveExpression(scope, elt)
			if err != nil {
				return nil, err
			}

			abi, ok := typ.(ABI)
			if !ok {
				return nil, c.errorf(elt.Pos(), "cannot use %s (%s) as ABI", elt.Print(), elt)
			}

			function.SetABI(abi.abi)
		case "section":
			elt := anno.X.Elements[1]
			_, typ, err := c.ResolveExpression(scope, elt)
			if err != nil {
				return nil, err
			}

			_, ok := typ.(Section)
			if !ok {
				return nil, c.errorf(elt.Pos(), "cannot use %s (%s) as section", elt.Print(), elt)
			}
		}
	}

	// Handle functions with no body.
	if len(fun.Elements) < 3 {
		if sig.result == nil {
			// No need to return anything, as there's no
			// return type.
			return nil, nil
		}

		return nil, c.errorf(fun.ParenClose, "%s has return type %s but no function body", name, sig.result)
	}

	// Fetch the function body's scope.
	scope = scope.Innermost(fun.Elements[2].Pos())
	if scope == nil {
		return nil, c.errorf(fun.Elements[2].Pos(), "internal error: failed to find scope")
	}

	for i, expr := range fun.Elements[2:] {
		isLast := i+3 == len(fun.Elements)
		_, result, err := c.ResolveExpression(scope, expr)
		if err != nil {
			return nil, err
		}

		if isLast && sig.result != nil && !AssignableTo(sig.result, result) {
			return nil, c.errorf(expr.Pos(), "%s has return type %s but returns value of incompatible type %s", name, sig.result, result)
		}
	}

	return sig.result, nil
}

func (c *checker) ResolveLetBody(scope *Scope, let *ast.List) (err error) {
	// Resolve the section if one is specified.
	for _, anno := range let.Annotations {
		kind := anno.X.Elements[0].(*ast.Identifier) // Enforced by the parser.
		switch kind.Name {
		case "section":
			elt := anno.X.Elements[1]
			_, typ, err := c.ResolveExpression(scope, elt)
			if err != nil {
				return err
			}

			_, ok := typ.(Section)
			if !ok {
				return c.errorf(elt.Pos(), "cannot use %s (%s) as section", elt.Print(), elt)
			}
		}
	}

	return nil
}

func (c *checker) checkArrayType(scope *Scope, typeName *ast.Identifier, elements int) (element, array Type, err error) {
	parts := strings.Split(typeName.Name, "/")

	// Handle multi-dimensional arrays by starting
	// with the deepest (right-most) element type
	// and working backwards to the ancestor type.
	for len(parts) > 3 {
		// Take the last three elements, which
		// must be array/<length>/<element>.
		if parts[len(parts)-3] != "array" {
			return nil, nil, c.errorf(typeName.NamePos, "invalid array type %s: subtype %s/%s/%s is not a valid array type", typeName.Name, parts[len(parts)-3], parts[len(parts)-2], parts[len(parts)-1])
		}

		length, err := strconv.ParseUint(parts[len(parts)-2], 0, 64)
		if err != nil {
			return nil, nil, c.errorf(typeName.NamePos, "invalid array type %q: bad array length %s: %v", typeName.Name, parts[len(parts)-2], err)
		}

		if element == nil {
			_, elt := scope.LookupParent(parts[len(parts)-1], token.NoPos)
			if elt == nil {
				return nil, nil, c.errorf(typeName.NamePos, "undefined array element type: %s", parts[len(parts)-1])
			}

			element = elt.Type()
		}

		// Build this type.
		array = NewArray(uint(length), element)

		// Save this type as the element type
		// and move to the parent type.
		element = array
		parts = parts[:len(parts)-2]
	}

	switch len(parts) {
	case 3:
		length, err := strconv.ParseUint(parts[1], 0, 64)
		if err != nil {
			return nil, nil, c.errorf(typeName.NamePos, "invalid array type %q: bad array length %s: %v", typeName.Name, parts[1], err)
		}

		if elements >= 0 && length != uint64(elements) {
			return nil, nil, c.errorf(typeName.NamePos, "invalid array type %q: got array length %d with %d elements", typeName.Name, length, elements)
		}

		if element == nil {
			_, elt := scope.LookupParent(parts[2], token.NoPos)
			if elt == nil {
				return nil, nil, c.errorf(typeName.NamePos, "undefined array element type: %s", parts[2])
			}

			element = elt.Type()
		}

		array = NewArray(uint(length), element)
	case 2:
		if element == nil {
			_, elt := scope.LookupParent(parts[1], token.NoPos)
			if elt == nil {
				return nil, nil, c.errorf(typeName.NamePos, "undefined array element type: %s", parts[1])
			}

			element = elt.Type()
		}

		array = NewArray(uint(elements), element)
	default:
		return nil, nil, c.errorf(typeName.NamePos, "invalid array type: %s", typeName.Name)
	}

	return element, array, nil
}

func (c *checker) ResolveExpression(scope *Scope, expr ast.Expression) (Object, Type, error) {
	switch x := expr.(type) {
	case *ast.List:
		// Function call.
		//
		// Start by resolving the function, which
		// may be a special form.
		if len(x.Elements) == 0 {
			return nil, nil, c.errorf(x.ParenClose, "cannot call nil function")
		}

		if name, ok := x.Elements[0].(*ast.Identifier); ok {
			_, obj := scope.LookupParent(name.Name, token.NoPos)
			if form, ok := obj.(*SpecialForm); ok {
				// Special form.
				signature, typ, err := specialFormTypes[form.id](c, scope, x)
				if err != nil {
					return nil, nil, err
				}

				if typ != nil && signature == nil {
					c.use(name, form)
					c.record(name, typ, nil)
					return nil, typ, nil
				}

				fun := NewFunction(nil, x.ParenOpen, x.ParenClose, nil, form.Name(), signature, 1)

				c.use(name, form)
				c.record(name, signature, nil)
				return fun, signature.result, nil
			}
		}

		// Normal function call.
		obj, typ, err := c.ResolveExpression(scope, x.Elements[0])
		if err != nil {
			return nil, nil, err
		}

		sig, ok := typ.(*Signature)
		if !ok {
			// Check for a type cast.
			typeName, ok := x.Elements[0].(*ast.Identifier)
			if !ok {
				return nil, nil, c.errorf(x.Elements[0].Pos(), "cannot call non-function type %s", typ)
			}

			_, obj := scope.LookupParent(typeName.Name, token.NoPos)
			if obj == nil && strings.HasPrefix(typeName.Name, "array/") {
				// Array types are of the form array/length/element
				// (or array/element for literals).
				eltType, typ, err := c.checkArrayType(scope, typeName, len(x.Elements[1:]))
				if err != nil {
					return nil, nil, err
				}

				obj = NewTypeName(nil, token.NoPos, token.NoPos, nil, typ.String(), typ)

				// Build the array.
				values := make([]constant.Value, len(x.Elements[1:]))
				for i, v := range x.Elements[1:] {
					_, argType, err := c.ResolveExpression(scope, v)
					if err != nil {
						return nil, nil, err
					}

					if !AssignableTo(eltType, argType) {
						return nil, nil, c.errorf(v.Pos(), "cannot cast %s (%s) to %s in %s", v.Print(), argType, eltType, typeName.Name)
					}

					values[i] = c.consts[v]
					if values[i] == nil {
						return nil, nil, c.errorf(v.Pos(), "cannot use non-constant value %s in array constant", v.Print())
					}
				}

				value := constant.MakeArray(typ.String(), values)
				c.record(x, typ, value)

				obj = NewConstant(scope, x.ParenOpen, x.ParenClose+1, c.pkg, typeName.Name, typ, value, 1)

				return obj, typ, nil
			}

			if obj == nil {
				return nil, nil, c.errorf(typeName.NamePos, "undefined type: %s", typeName.Name)
			}

			typ = obj.Type()
			if len(x.Elements[1:]) != 1 {
				return nil, nil, c.errorf(x.Elements[1].Pos(), "cannot cast %d values to %s", len(x.Elements[1:]), typ)
			}

			obj, argType, err := c.ResolveExpression(scope, x.Elements[1])
			if err != nil {
				return nil, nil, err
			}

			if !AssignableTo(typ, argType) {
				return nil, nil, c.errorf(x.Elements[1].Pos(), "cannot cast %s (%s) to %s", x.Elements[1].Print(), argType, typ)
			}

			value := c.consts[x.Elements[1]]
			c.record(x, typ, value)

			return obj, typ, nil
		}

		// Check the parameters, then if they match,
		// return the result type.

		// Start by getting the argument types.
		argTypes := make([]Type, len(x.Elements[1:]))
		for i, expr := range x.Elements[1:] {
			_, argTypes[i], err = c.ResolveExpression(scope, expr)
			if err != nil {
				return nil, nil, err
			}
		}

		// TODO: handle variadic functions.

		if len(x.Elements[1:]) > len(sig.params) {
			return nil, nil, c.errorf(x.ParenOpen, "too many arguments in call to %s:\n\thave %s\n\twant %s", sig, typesList(argTypes), paramsList(sig.params))
		} else if len(x.Elements[1:]) < len(sig.params) {
			return nil, nil, c.errorf(x.ParenOpen, "not enough arguments in call to %s:\n\thave %s\n\twant %s", sig, typesList(argTypes), paramsList(sig.params))
		}

		for i, arg := range argTypes {
			param := sig.params[i].Type()
			if !AssignableTo(param, arg) {
				return nil, nil, c.errorf(x.Elements[i+1].Pos(), "cannot use %s (%s) as %s value in argument to %s", x.Elements[i+1].Print(), arg, param, sig)
			}
		}

		c.record(x, sig.result, nil)

		return obj, sig.result, nil
	case *ast.Identifier:
		_, obj := scope.LookupParent(x.Name, token.NoPos)
		if obj == nil && strings.HasPrefix(x.Name, "array/") {
			_, typ, err := c.checkArrayType(scope, x, -1)
			if err != nil {
				return nil, nil, err
			}

			obj = NewTypeName(nil, token.NoPos, token.NoPos, nil, typ.String(), typ)

			c.use(x, obj)
			c.record(x, typ, nil)

			return obj, typ, nil
		}

		if obj == nil {
			return nil, nil, c.errorf(x.NamePos, "undefined: %s", x.Name)
		}

		var value constant.Value
		if con, ok := obj.(*Constant); ok {
			value = con.Value()
		}

		typ := obj.Type()
		c.use(x, obj)
		c.record(x, typ, value)
		return obj, typ, nil
	case *ast.Literal:
		var typ Type
		var value constant.Value
		switch x.Kind {
		case token.Integer:
			typ = UntypedInt
			value = constant.MakeFromLiteral(x.Value, token.Integer)
		case token.String:
			typ = UntypedString
			value = constant.MakeFromLiteral(x.Value, token.String)
		case token.Period:
			return nil, nil, c.errorf(x.ValuePos, "unexpected %s", x.Kind)
		default:
			return nil, nil, c.errorf(x.ValuePos, "unexpected literal kind %s", x.Kind)
		}

		obj := NewConstant(nil, token.NoPos, token.NoPos, nil, x.Value, typ, value, 1)

		c.record(x, typ, value)
		return obj, typ, nil
	case *ast.Qualified:
		// The left hand side should be an
		// import reference, the right hand
		// is resolved in the imported
		// scope.
		_, lhs := scope.LookupParent(x.X.Name, token.NoPos)
		if lhs == nil {
			return nil, nil, c.errorf(x.X.NamePos, "invalid qualified expression value %s: package %q is not defined", x.Print(), x.X.Name)
		}

		pkg, ok := lhs.(*Import)
		if !ok {
			return nil, nil, c.errorf(x.X.NamePos, "invalid qualified expression value %s: expected imported package reference, got %#v", x.Print(), lhs)
		}

		rhs := pkg.imported.scope.Lookup(x.Y.Name)
		if rhs == nil {
			return nil, nil, c.errorf(x.Y.NamePos, "invalid qualified expression value %s: expression %q is not defined", x.Print(), x.Y.Name)
		}

		typ := rhs.Type()
		c.use(x.X, pkg)
		c.use(x.Y, rhs)
		c.record(x, typ, nil)
		return rhs, typ, nil
	case *ast.QuotedIdentifier:
		// Quoted identifiers have no type.
		return nil, nil, nil
	default:
		return nil, nil, c.errorf(expr.Pos(), "unexpected expression type %s", expr)
	}
}

func (c *checker) ResolveLet(scope *Scope, let *ast.List) (Type, error) {
	// Value declaration.
	//
	// Takes one of the following forms:
	//
	// - (let name value)         ; Immutable data declaration, declaring 'name', with value 'value'.
	// - (let (name type) value)  ; Immutable data declaration, declaring 'name' with type 'type', with value 'value'.

	var name, typeName *ast.Identifier
	if err := c.checkFixedArgsList(let, "value declaration", "name", "value"); err != nil {
		return nil, c.error(err)
	}

	// Check the annotations.
	alignment := 1
	for _, anno := range let.Annotations {
		keyword := anno.X.Elements[0].(*ast.Identifier)
		switch keyword.Name {
		case "align":
			// We expect a single uint32.
			if len(anno.X.Elements) != 2 {
				return nil, c.errorf(anno.X.ParenOpen, "invalid alignment annotation: must specify one alignment value, got %d arguments", len(anno.X.Elements)-1)
			}

			alignElt := anno.X.Elements[1]
			lit, ok := alignElt.(*ast.Literal)
			if !ok || lit.Kind != token.Integer {
				return nil, c.errorf(alignElt.Pos(), "invalid alignment annotation: must specify one alignment value, got %s", alignElt)
			}

			val, err := strconv.ParseUint(lit.Value, 0, 32)
			if err != nil {
				return nil, c.errorf(lit.ValuePos, "invalid alignment annotation: %v", err)
			}

			if val == 0 {
				return nil, c.errorf(alignElt.Pos(), "invalid alignment annotation: invalid alignment %s", lit.Value)
			}

			if alignment != 1 {
				return nil, c.errorf(lit.ValuePos, "cannot specify alignment more than once")
			}

			alignment = int(val)
		}
	}

	// Determine whether we're binding to a name (with an
	// inferred type) or a name with an explicit type.
	switch n := let.Elements[1].(type) {
	case *ast.Identifier:
		name = n
		// We will have to infer the type from the value.
	case *ast.List:
		if err := c.checkFixedArgsList(n, "value declaration", "type"); err != nil {
			return nil, c.error(err)
		}

		// Check we have two identifiers.
		var ok bool
		name = n.Elements[0].(*ast.Identifier) // This is checked in checkFixedArgsList.
		typeName, ok = n.Elements[1].(*ast.Identifier)
		if !ok {
			return nil, c.errorf(n.Elements[1].Pos(), "invalid value declaration: type must be an identifier, found %s", n.Elements[1])
		}
	default:
		return nil, c.errorf(let.Elements[1].Pos(), "invalid value declaration: name must be an identifier or (identifier type) list, found %s", let.Elements[1])
	}

	// We now have the name and type. Time to
	// handle the value.
	// Constant.
	var typ Type
	var obj Object
	var value constant.Value // Only for constant values.
	switch v := let.Elements[2].(type) {
	case *ast.Identifier, *ast.List:
		_, value, err := c.ResolveExpression(scope, v)
		if err != nil {
			return nil, err
		}

		// Check any declared type matches.
		if typeName == nil {
			typ = value
		} else {
			_, obj := scope.LookupParent(typeName.Name, token.NoPos)
			if obj == nil {
				return nil, c.errorf(typeName.NamePos, "undefined type: %s", typeName.Name)
			}

			typ = obj.Type()
			c.use(typeName, obj)
			c.record(typeName, typ, nil)
			if !AssignableTo(typ, value) {
				return nil, c.errorf(let.ParenOpen, "cannot assign %s (%s) to value of type %s", value, value, typ)
			}
		}

		obj = NewVariable(scope, let.ParenClose, scope.End(), c.pkg, name.Name, typ)
		c.names[let.ParenOpen] = "value " + name.Name
	case *ast.Literal:
		switch v.Kind {
		case token.Integer:
			// Check any declared type matches.
			if typeName == nil {
				typ = UntypedInt
			} else {
				_, obj := scope.LookupParent(typeName.Name, token.NoPos)
				if obj == nil {
					return nil, c.errorf(typeName.NamePos, "undefined type: %s", typeName.Name)
				}

				typ = obj.Type()
				c.use(typeName, obj)
				c.record(typeName, typ, nil)
				if !AssignableTo(typ, UntypedInt) {
					return nil, c.errorf(let.ParenOpen, "cannot assign integer literal to value of type %s", typ)
				}
			}

			value = constant.MakeFromLiteral(v.Value, token.Integer)
		case token.String:
			// Check any declared type matches.
			if typeName == nil {
				typ = UntypedString
			} else {
				_, obj := scope.LookupParent(typeName.Name, token.NoPos)
				if obj == nil {
					return nil, c.errorf(typeName.NamePos, "undefined type: %s", typeName.Name)
				}

				typ = obj.Type()
				c.use(typeName, obj)
				c.record(typeName, typ, nil)
				if !AssignableTo(typ, UntypedString) {
					return nil, c.errorf(let.ParenOpen, "cannot assign string literal to value of type %s", typ)
				}
			}

			value = constant.MakeFromLiteral(v.Value, token.String)
		default:
			return nil, c.errorf(v.ValuePos, "invalid value declaration: unexpected value type for value %s: %s", name.Name, v)
		}

		obj = NewConstant(scope, let.ParenClose, scope.End(), c.pkg, name.Name, typ, value, alignment)
		c.names[let.ParenOpen] = "constant " + name.Name
	default:
		// TODO: handle top-level lets that assign another constant to a new name.
		return nil, c.errorf(let.Elements[2].Pos(), "invalid value declaration: unexpected value type for value %s: %s", name.Name, let.Elements[2])
	}

	sig := &Signature{
		name: "let",
		params: []*Variable{
			NewParameter(nil, let.Elements[1].Pos(), let.Elements[1].End(), nil, "name", typ),
			NewParameter(nil, let.Elements[2].Pos(), let.Elements[2].End(), nil, "value", typ),
		},
		result: typ,
	}

	c.define(name, obj)
	c.record(let, typ, value)
	c.record(name, typ, value)
	c.record(let.Elements[0], sig, value)
	c.record(let.Elements[1], typ, value)
	c.record(let.Elements[2], typ, value)
	if other := scope.Insert(obj); other != nil {
		return nil, c.errorf(let.ParenOpen, "%s redeclared: previous declaration at %s", name.Name, c.fset.Position(other.Pos()))
	}

	return typ, nil
}

func (c *checker) CheckTopLevelLet(parent *Scope, let *ast.List) error {
	// Constant declaration.
	//
	// Takes one of the following forms:
	//
	// - (let name value)         ; Immutable data declaration, declaring 'name', with value 'value'.
	// - (let (name type) value)  ; Immutable data declaration, declaring 'name' with type 'type', with value 'value'.

	var name, typeName *ast.Identifier
	if err := c.checkFixedArgsList(let, "constant declaration", "name", "value"); err != nil {
		return c.error(err)
	}

	// Check the annotations.
	alignment := 1
	for _, anno := range let.Annotations {
		keyword := anno.X.Elements[0].(*ast.Identifier)
		switch keyword.Name {
		case "align":
			// We expect a single uint32.
			if len(anno.X.Elements) != 2 {
				return c.errorf(anno.X.ParenOpen, "invalid alignment annotation: must specify one alignment value, got %d arguments", len(anno.X.Elements)-1)
			}

			alignElt := anno.X.Elements[1]
			lit, ok := alignElt.(*ast.Literal)
			if !ok || lit.Kind != token.Integer {
				return c.errorf(alignElt.Pos(), "invalid alignment annotation: must specify one alignment value, got %s", alignElt)
			}

			val, err := strconv.ParseUint(lit.Value, 0, 32)
			if err != nil {
				return c.errorf(lit.ValuePos, "invalid alignment annotation: %v", err)
			}

			if val == 0 {
				return c.errorf(alignElt.Pos(), "invalid alignment annotation: invalid alignment %s", lit.Value)
			}

			if alignment != 1 {
				return c.errorf(lit.ValuePos, "cannot specify alignment more than once")
			}

			alignment = int(val)
		}
	}

	// Determine whether we're binding to a name (with an
	// inferred type) or a name with an explicit type.
	switch n := let.Elements[1].(type) {
	case *ast.Identifier:
		name = n
		// We will have to infer the type from the value.
	case *ast.List:
		if err := c.checkFixedArgsList(n, "constant declaration", "type"); err != nil {
			return c.error(err)
		}

		// Check we have two identifiers.
		var ok bool
		name = n.Elements[0].(*ast.Identifier) // This is checked in checkFixedArgsList.
		typeName, ok = n.Elements[1].(*ast.Identifier)
		if !ok {
			return c.errorf(n.Elements[1].Pos(), "invalid constant declaration: type must be an identifier, found %s", n.Elements[1])
		}
	default:
		return c.errorf(let.Elements[1].Pos(), "invalid constant declaration: name must be an identifier or (identifier type) list, found %s", let.Elements[1])
	}

	// We now have the name and type. Time to
	// handle the value.
	// Constant.
	var typ Type
	var value constant.Value
	switch v := let.Elements[2].(type) {
	case *ast.List:
		// We can assign constants using a function
		// call, but only if it resolves to a constant
		// expression.
		_, constantType, err := c.ResolveExpression(parent, v)
		if err != nil {
			return err
		}

		value = c.consts[v]
		_, isABI := constantType.(ABI)
		_, isSection := constantType.(Section)
		if value == nil && !isABI && !isSection {
			return c.errorf(v.ParenOpen, "cannot use non-constant value %s in constant declaration", v.Print())
		}

		// Check the declared type matches.
		if typeName == nil {
			typ = constantType
		} else {
			_, obj := parent.LookupParent(typeName.Name, token.NoPos)
			if obj == nil {
				return c.errorf(typeName.NamePos, "undefined type: %s", typeName.Name)
			}

			typ = obj.Type()
			c.use(typeName, obj)
			c.record(typeName, typ, nil)
			if !AssignableTo(typ, constantType) {
				return c.errorf(let.ParenOpen, "cannot assign %s to constant of type %s", constantType, typ)
			}
		}
	case *ast.Literal:
		switch v.Kind {
		case token.Integer:
			// Check any declared type matches.
			if typeName == nil {
				typ = UntypedInt
			} else {
				_, obj := parent.LookupParent(typeName.Name, token.NoPos)
				if obj == nil {
					return c.errorf(typeName.NamePos, "undefined type: %s", typeName.Name)
				}

				typ = obj.Type()
				c.use(typeName, obj)
				c.record(typeName, typ, nil)
				if !AssignableTo(typ, UntypedInt) {
					return c.errorf(let.ParenOpen, "cannot assign integer literal to constant of type %s", typ)
				}
			}

			value = constant.MakeFromLiteral(v.Value, token.Integer)
		case token.String:
			// Check any declared type matches.
			if typeName == nil {
				typ = UntypedString
			} else {
				_, obj := parent.LookupParent(typeName.Name, token.NoPos)
				if obj == nil {
					return c.errorf(typeName.NamePos, "undefined type: %s", typeName.Name)
				}

				typ = obj.Type()
				c.use(typeName, obj)
				c.record(typeName, typ, nil)
				if !AssignableTo(typ, UntypedString) {
					return c.errorf(let.ParenOpen, "cannot assign string literal to constant of type %s", typ)
				}
			}

			value = constant.MakeFromLiteral(v.Value, token.String)
		default:
			return c.errorf(v.ValuePos, "invalid constant declaration: unexpected value type for constant %s: %s", name.Name, v)
		}
	case *ast.Qualified:
		// The left hand side should be an
		// import reference, the right hand
		// is resolved in the imported
		// scope.
		_, lhs := parent.LookupParent(v.X.Name, token.NoPos)
		if lhs == nil {
			return c.errorf(v.X.NamePos, "invalid constant declaration: invalid qualified expression value %s: package %q is not defined", v.Print(), v.X.Name)
		}

		pkg, ok := lhs.(*Import)
		if !ok {
			return c.errorf(v.X.NamePos, "invalid constant declaration: invalid qualified expression value %s: expected imported package reference, got %#v", v.Print(), lhs)
		}

		rhs := pkg.imported.scope.Lookup(v.Y.Name)
		if rhs == nil {
			return c.errorf(v.Y.NamePos, "invalid constant declaration: invalid qualified expression value %s: expression %q is not defined", v.Print(), v.Y.Name)
		}

		con, ok := rhs.(*Constant)
		if !ok {
			return c.errorf(v.X.NamePos, "invalid constant declaration: cannot assign %s to constant", rhs)
		}

		// Check any declared type matches.
		if typeName == nil {
			typ = rhs.Type()
		} else {
			_, obj := parent.LookupParent(typeName.Name, token.NoPos)
			if obj == nil {
				return c.errorf(typeName.NamePos, "undefined type: %s", typeName.Name)
			}

			typ = obj.Type()
			c.use(typeName, obj)
			c.record(typeName, typ, nil)
			if !AssignableTo(typ, rhs.Type()) {
				return c.errorf(let.ParenOpen, "cannot assign %s to constant of type %s", rhs.Type(), typ)
			}
		}

		value = con.value
	default:
		// TODO: handle top-level lets that assign another constant to a new name.
		return c.errorf(let.Elements[2].Pos(), "invalid constant declaration: unexpected value type for constant %s: %s", name.Name, let.Elements[2])
	}

	sig := &Signature{
		name: "let",
		params: []*Variable{
			NewParameter(nil, let.Elements[1].Pos(), let.Elements[1].End(), nil, "name", typ),
			NewParameter(nil, let.Elements[2].Pos(), let.Elements[2].End(), nil, "value", typ),
		},
		result: typ,
	}

	obj := NewConstant(parent, let.ParenOpen, let.ParenClose, c.pkg, name.Name, typ, value, alignment)
	c.names[let.ParenOpen] = "constant " + name.Name
	c.define(name, obj)
	c.record(let, typ, value)
	c.record(name, typ, value)
	c.record(let.Elements[0], sig, value)
	c.record(let.Elements[1], typ, value)
	c.record(let.Elements[2], typ, value)
	if other := parent.Insert(obj); other != nil {
		return c.errorf(let.ParenOpen, "%s redeclared: previous declaration at %s", name.Name, c.fset.Position(other.Pos()))
	}

	return nil
}
