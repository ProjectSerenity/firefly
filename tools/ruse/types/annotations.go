// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
)

// iterAnnotations is a simple helper function for looping over
// the annotations on a list.
//
// iterAnnotations will return an error if the annotation does
// not start with a keyword identifier.
func (c *checker) iterAnnotations(list *ast.List, f func(list, anno *ast.List, keyword *ast.Identifier) error) error {
	for _, quoted := range list.Annotations {
		anno := quoted.X
		if len(anno.Elements) == 0 {
			return c.errorf(quoted.Quote, "invalid annotation: no keyword")
		}

		keyword, ok := anno.Elements[0].(*ast.Identifier)
		if !ok {
			return c.errorf(anno.Elements[0].Pos(), "invalid annotation: invalid keyword: %v", anno.Elements[0])
		}

		err := f(list, anno, keyword)
		if err != nil {
			return err
		}
	}

	return nil
}

// Allow no annotations.
func (c *checker) checkAnnotationNone(name string, list *ast.List) error {
	if list == nil {
		return nil
	}

	return c.iterAnnotations(list, func(list, anno *ast.List, keyword *ast.Identifier) error {
		// No annotations are allowed.
		return c.errorf(anno.ParenOpen, "invalid %s annotation: unrecognised annotation type: %s", name, keyword.Name)
	})
}

// Allow no annotations in the list or its elements.
func (c *checker) checkAnnotationNoneRecurse(name string, list *ast.List) error {
	if list == nil {
		return nil
	}

	if err := c.checkAnnotationNone(name, list); err != nil {
		return err
	}

	for _, x := range list.Elements {
		next, ok := x.(*ast.List)
		if !ok {
			continue
		}

		if err := c.checkAnnotationNoneRecurse(name, next); err != nil {
			return err
		}
	}

	return nil
}

// Allow an ABI reference/declaration.
func (c *checker) checkAnnotationABI(list, anno *ast.List) error {
	switch len(anno.Elements) {
	case 1:
		return c.errorf(anno.ParenClose, "invalid ABI annotation: ABI missing")
	case 2:
		// Ok.
	default:
		return c.errorf(anno.Elements[2].Pos(), "invalid ABI annotation: %s after ABI", anno.Elements[2])
	}

	abi := anno.Elements[1]
	switch x := abi.(type) {
	case *ast.Identifier: // A named ABI, which we check more later.
	case *ast.Qualified: // An imported named ABI, which we check more later.
	case *ast.List:
		kind, _, err := c.interpretDefinition(x, "abi spec")
		if err != nil {
			return c.errorf(x.ParenOpen, "invalid ABI annotation: invalid ABI declaration: %v", err)
		}

		if kind.Name != "abi" {
			return c.errorf(x.ParenOpen, "invalid ABI annotation: invalid ABI declaration: got identifier %s, want %s", kind.Name, "abi")
		}
	default:
		return c.errorf(abi.Pos(), "invalid ABI annotation: got ABI declaration %s, want spec or value", x.Print())
	}

	return nil
}

// Allow one or more architectures.
func (c *checker) checkAnnotationArchitecture(list, anno *ast.List) error {
	for _, x := range anno.Elements[1:] {
		name, ok := x.(*ast.Identifier)
		if !ok {
			return c.errorf(x.Pos(), "invalid architecture annotation: got %s, want identifier", x)
		}

		for _, arch := range sys.All {
			if arch.Name == name.Name {
				ok = true
				break
			}
		}
		if !ok {
			return c.errorf(name.NamePos, "invalid architecture annotation: unrecognised architecture %q", name.Name)
		}
	}

	return nil
}

// Allow a CPU mode.
func (c *checker) checkAnnotationMode(list, anno *ast.List) error {
	switch len(anno.Elements) {
	case 1:
		return c.errorf(anno.ParenClose, "invalid mode annotation: mode missing")
	case 2:
		// Ok.
	default:
		return c.errorf(anno.Elements[2].Pos(), "invalid mode annotation: %s after mode", anno.Elements[2])
	}

	// We accept an integer or identifier, depending
	// on architecture.
	mode := anno.Elements[1]
	switch x := mode.(type) {
	case *ast.Identifier: // A named mode, which we check more later.
	case *ast.Literal:
		if x.Kind != token.Integer {
			return c.errorf(x.ValuePos, "invalid mode annotation: got %s, want identifier or integer", x.Kind)
		}
	default:
		return c.errorf(mode.Pos(), "invalid mode annotation: got %s, want identifier or integer", mode)
	}

	return nil
}

// Check the annotations on an assembly function declaration.
func (c *checker) checkAnnotationAsmFunc(list *ast.List) error {
	// We do have some annotations on the declaration.
	err := c.iterAnnotations(list, func(list, anno *ast.List, keyword *ast.Identifier) error {
		switch keyword.Name {
		case "abi":
			return c.checkAnnotationABI(list, anno)
		case "arch":
			return c.checkAnnotationArchitecture(list, anno)
		case "mode":
			return c.checkAnnotationMode(list, anno)
		default:
			return c.errorf(anno.ParenOpen, "invalid function annotation: unrecognised annotation type: %s", keyword.Name)
		}
	})
	if err != nil {
		return err
	}

	// There shouldn't be any on the function signature
	// expression.
	signature, ok := list.Elements[1].(*ast.List)
	if !ok {
		return c.errorf(list.Elements[1].Pos(), "invalid function signature: got %s, want list of the form (name (arg1 arg1Type) (arg2 arg2Type))", list.Elements[1])
	}
	if err := c.checkAnnotationNone("function", signature); err != nil {
		return err
	}

	// We don't currently allow annotations on parameter
	// declarations.
	for _, param := range signature.Elements[1:] {
		list, ok := param.(*ast.List)
		if !ok {
			continue
		}

		if err := c.checkAnnotationNone("function", list); err != nil {
			return err
		}
	}

	// We have a few annotations on the instructions
	// themselves.
	for _, x := range list.Elements[2:] {
		inst, ok := x.(*ast.List)
		if !ok {
			continue
		}

		err := c.iterAnnotations(inst, func(list, anno *ast.List, keyword *ast.Identifier) error {
			switch keyword.Name {
			case "mask":
				// The meaning is defined per-architecture.
			case "match":
				switch len(anno.Elements) {
				case 1:
					return c.errorf(anno.ParenClose, "invalid match annotation: instruction missing")
				case 2:
					// Ok.
				default:
					return c.errorf(anno.Elements[2].Pos(), "invalid match annotation: %s after instruction", anno.Elements[2])
				}

				instruction := anno.Elements[1]
				if _, ok := instruction.(*ast.Identifier); !ok {
					return c.errorf(instruction.Pos(), "invalid match annotation: got %s, want instruction identifier", instruction)
				}
			case "zero":
				// The meaning is defined per-architecture.
			default:
				return c.errorf(anno.ParenOpen, "invalid instruction annotation: unrecognised annotation type: %s", keyword.Name)
			}

			return nil
		})
		if err != nil {
			return err
		}

		// We also have annotations on memory addresses.
		for _, x := range inst.Elements {
			mem, ok := x.(*ast.List)
			if !ok {
				continue
			}

			err := c.iterAnnotations(mem, func(list, anno *ast.List, keyword *ast.Identifier) error {
				switch keyword.Name {
				case "bits", "bytes":
					switch len(anno.Elements) {
					case 1:
						return c.errorf(anno.ParenClose, "invalid size annotation: size missing")
					case 2:
						// Ok.
					default:
						return c.errorf(anno.Elements[2].Pos(), "invalid size annotation: %s after size", anno.Elements[2])
					}

					size := anno.Elements[1]
					if lit, ok := size.(*ast.Literal); !ok || lit.Kind != token.Integer {
						return c.errorf(size.Pos(), "invalid size annotation: got %s, want size integer", size)
					}
				default:
					return c.errorf(anno.ParenOpen, "invalid instruction annotation: unrecognised annotation type: %s", keyword.Name)
				}

				return nil
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Check the annotations on a Ruse function declaration.
func (c *checker) checkAnnotationFunc(list *ast.List) error {
	// We do have some annotations on the declaration.
	err := c.iterAnnotations(list, func(list, anno *ast.List, keyword *ast.Identifier) error {
		switch keyword.Name {
		case "abi":
			return c.checkAnnotationABI(list, anno)
		case "arch":
			return c.checkAnnotationArchitecture(list, anno)
		default:
			return c.errorf(anno.ParenOpen, "invalid function annotation: unrecognised annotation type: %s", keyword.Name)
		}
	})
	if err != nil {
		return err
	}

	// There shouldn't be any on the function signature
	// expression.
	signature, ok := list.Elements[1].(*ast.List)
	if !ok {
		return c.errorf(list.Elements[1].Pos(), "invalid function signature: got %s, want list of the form (name (arg1 arg1Type) (arg2 arg2Type))", list.Elements[1])
	}
	if err := c.checkAnnotationNone("function", signature); err != nil {
		return err
	}

	// We don't currently allow annotations on parameter
	// declarations.
	for _, param := range signature.Elements[1:] {
		list, ok := param.(*ast.List)
		if !ok {
			continue
		}

		if err := c.checkAnnotationNone("function", list); err != nil {
			return err
		}
	}

	// We have a few annotations on the function body
	// itself.
	for _, expr := range list.Elements[2:] {
		if err := c.checkAnnotationExpression(expr); err != nil {
			return err
		}
	}

	return nil
}

// Check the annotations on a Ruse expression.
func (c *checker) checkAnnotationExpression(expr ast.Expression) error {
	// For now, we don't allow any annotations on arbitrary
	// expressions.
	list, ok := expr.(*ast.List)
	if !ok {
		return nil
	}

	if err := c.checkAnnotationNone("expression", list); err != nil {
		return err
	}

	// Recurse.
	for _, expr := range list.Elements {
		if err := c.checkAnnotationExpression(expr); err != nil {
			return err
		}
	}

	return nil
}

// CheckAnnotations raises an error if any unexpected annotations
// are found in the files. This ensures that we can add new
// annotations over time with forwards compatibility.
//
// We don't always fully check the syntax of every annotation, as
// that can happen later in the compiler when we have more context
// information. Our primary goal is to detect and alert on any
// annotations in the wrong place or with an unexpected keyword.
//
// Although annotations aren't strictly part of the type system,
// this still feels like a good place to check them all in one
// go.
func (c *checker) CheckAnnotations(files []*ast.File) error {
	for _, file := range files {
		// Package statement.
		if err := c.checkAnnotationNone("package", file.Package); err != nil {
			return err
		}

		// Imports.
		for _, imp := range file.Imports {
			if err := c.checkAnnotationNone("import", imp.Group); err != nil {
				return err
			}
			if err := c.checkAnnotationNone("import", imp.List); err != nil {
				return err
			}
		}

		// Other expressions.
		for _, list := range file.Expressions {
			keyword, ok := list.Elements[0].(*ast.Identifier)
			if !ok {
				return c.errorf(list.Elements[0].Pos(), "invalid expression: got %s, want keyword", list.Elements[0])
			}

			switch keyword.Name {
			// Constants and named ABIs.
			case "let":
				// We do have some annotations on the let expression.
				err := c.iterAnnotations(list, func(list, anno *ast.List, keyword *ast.Identifier) error {
					switch keyword.Name {
					case "arch":
						return c.checkAnnotationArchitecture(list, anno)
					default:
						return c.errorf(anno.ParenOpen, "invalid annotation: unrecognised annotation type: %s", keyword.Name)
					}
				})
				if err != nil {
					return err
				}

				// If we specify a type, there can't be
				// any annotations on the name, type pair.
				if pair, ok := list.Elements[1].(*ast.List); ok {
					if err := c.checkAnnotationNone("let", pair); err != nil {
						return err
					}
				}

				// We have a few annotations on the values
				// themselves.
				if err := c.checkAnnotationExpression(list.Elements[2]); err != nil {
					return err
				}
			// Assembly functions.
			case "asm-func":
				if err := c.checkAnnotationAsmFunc(list); err != nil {
					return err
				}
			// Ruse functions.
			case "func":
				if err := c.checkAnnotationFunc(list); err != nil {
					return err
				}
			default:
				return c.errorf(keyword.NamePos, "invalid expression: got unexpected keyword %q", keyword.Name)
			}
		}
	}

	return nil
}
