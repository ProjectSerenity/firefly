// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"strconv"
	"strings"

	"firefly-os.dev/tools/plan/ast"
)

// interpretDefinition ensures that the given list
// consists of an identifier followed by at least
// one further element.
func (i *interpreter) interpretDefinition(list *ast.List) (kind *ast.Identifier, rest []ast.Expr, err *positionalError) {
	if len(list.Elements) == 0 {
		return nil, nil, i.errorf(list, "empty definition")
	}

	var ok bool
	kind, ok = list.Elements[0].(*ast.Identifier)
	if !ok {
		return nil, nil, i.errorf(list.Elements[0], "definition kind must be an identifier, found %s", list.Elements[0])
	}

	rest = list.Elements[1:]
	if len(rest) == 0 {
		return nil, nil, i.errorf(kind, "definition must have at least one field, found none")
	}

	return kind, rest, nil
}

// interpretLists ensures that the given elements
// are all lists, returning them.
func (i *interpreter) interpretLists(elts []ast.Expr) ([]*ast.List, *positionalError) {
	out := make([]*ast.List, len(elts))
	for j, elt := range elts {
		list, ok := elt.(*ast.List)
		if !ok {
			return nil, i.errorf(elt, "expected a list, found %s", elt)
		}

		out[j] = list
	}

	return out, nil
}

// interpretName ensures that the given elements
// are all identifiers, returning the name they
// describe.
func (i *interpreter) interpretName(elts []ast.Expr) (Name, *positionalError) {
	out := make(Name, len(elts))
	for j, elt := range elts {
		ident, ok := elt.(*ast.Identifier)
		if !ok {
			return nil, i.errorf(elt, "expected an identifier, found %s", elt)
		}

		out[j] = ident.Name
	}

	return out, nil
}

// interpretDocs ensures that the given elements are all
// strings, returning them. The returned strings are the
// result of splitting the docs by newlines.
func (i *interpreter) interpretDocs(elts []ast.Expr) (Docs, *positionalError) {
	docs := make(Docs, 0, len(elts))
	spaceNeeded := false
	addPlainText := func(elt *ast.String, cast func(s string) DocsItem) *positionalError {
		raw, err := strconv.Unquote(elt.Text)
		if err != nil {
			return i.errorf(elt, "invalid string: %v", err)
		}

		// We use an empty string to indicate
		// a newline.
		if raw == "" {
			docs = append(docs, Newline{})
			spaceNeeded = false
			return nil
		}

		// Split into lines if necessary.
		lines := strings.Split(raw, "\n")
		for i, line := range lines {
			if i > 0 {
				docs = append(docs, Newline{})
				spaceNeeded = false
			}

			if line == "" {
				continue
			}

			// Auto-insert a separating space unless
			// the line begines with a full stop, which
			// is likely after a formatting expression.
			if spaceNeeded && !strings.HasPrefix(line, ".") {
				docs = append(docs, Text(" "))
			}

			docs = append(docs, cast(line))
			spaceNeeded = true
		}

		return nil
	}

	for _, elt := range elts {
		switch elt := elt.(type) {
		case *ast.String:
			err := addPlainText(elt, func(s string) DocsItem { return Text(s) })
			if err != nil {
				return nil, err
			}
		case *ast.List:
			def, elts, err := i.interpretDefinition(elt)
			if err != nil {
				return nil, err.Context("invalid formatting expression")
			}

			switch def.Name {
			case "code":
				for _, elt := range elts {
					str, ok := elt.(*ast.String)
					if !ok {
						return nil, i.errorf(elt, "invalid formatting expression: expected a string, found %s", elt)
					}

					err = addPlainText(str, func(s string) DocsItem { return CodeText(s) })
					if err != nil {
						return nil, err
					}
				}
			case "reference":
				typ, err := i.interpretTypeReference(elts)
				if err != nil {
					return nil, err.Context("invalid reference formatting expression")
				}

				if spaceNeeded {
					docs = append(docs, Text(" "))
				}

				docs = append(docs, ReferenceText{typ})
				spaceNeeded = true
			default:
				return nil, i.errorf(def, "unrecognised formatting expression kind %q", def.Name)
			}
		default:
			return nil, i.errorf(elt, "expected a string or formatting expression, found %s", elt)
		}
	}

	// We don't want a trailing newline or auto-space.
	for len(docs) > 0 {
		switch item := docs[len(docs)-1].(type) {
		case Newline:
			docs = docs[:len(docs)-1]
			continue
		case Text:
			if item == " " {
				docs = docs[:len(docs)-1]
				continue
			}
		}

		break
	}

	return docs, nil
}

// interpretField parses the list elements as a
// field definition.
func (i *interpreter) interpretField(list *ast.List) (field *Field, err *positionalError) {
	field = &Field{
		Node: list,
	}

	// Skip the first element, which is the 'field'
	// identifier.
	parts, err := i.interpretLists(list.Elements[1:])
	if err != nil {
		return nil, err.Context("invalid field definition")
	}

	for _, part := range parts {
		def, elts, err := i.interpretDefinition(part)
		if err != nil {
			return nil, err.Context("invalid field definition")
		}

		switch def.Name {
		case "name":
			name, err := i.interpretName(elts)
			if err != nil {
				return nil, err.Context("invalid field name")
			}

			if field.Name != nil {
				return nil, i.errorf(def, "invalid field definition: name already defined")
			}

			field.Name = name
		case "docs":
			docs, err := i.interpretDocs(elts)
			if err != nil {
				return nil, err.Context("invalid field docs")
			}

			if field.Docs != nil {
				return nil, i.errorf(def, "invalid field definition: docs already defined")
			}

			field.Docs = docs
		case "type":
			typ, err := i.interpretType(elts)
			if err != nil {
				return nil, err.Context("invalid field type")
			}

			if field.Type != nil {
				return nil, i.errorf(def, "invalid field definition: type already defined")
			}

			field.Type = typ
		case "padding":
			num, ok := elts[0].(*ast.Number)
			if !ok {
				return nil, i.errorf(elts[0], "invalid padding definition: expected a number, found %s", elts[0])
			}

			if len(elts) > 1 {
				return nil, i.errorf(elts[1], "invalid padding definition: unexpected %s after size", elts[1])
			}

			size, err := strconv.ParseUint(num.Value, 10, 16)
			if err != nil {
				// Unwrap the error if possible.
				if e, ok := err.(interface{ Unwrap() error }); ok {
					err = e.Unwrap()
				}

				return nil, i.errorf(num, "invalid padding definition: invalid padding size: %v", err)
			}

			if field.Type != nil {
				return nil, i.errorf(def, "invalid field definition: type already defined")
			}

			field.Type = Padding(size)
		default:
			return nil, i.errorf(def, "unrecognised field definition kind %q", def.Name)
		}
	}

	// Make sure the field is complete.
	if field.Name == nil {
		return nil, i.errorf(list, "field has no name definition")
	} else if field.Docs == nil {
		return nil, i.errorf(list, "field has no docs definition")
	} else if field.Type == nil {
		return nil, i.errorf(list, "field has no type definition")
	}

	return field, nil
}

// interpretParameter parses the list elements as a
// parameter definition.
func (i *interpreter) interpretParameter(list *ast.List) (param *Parameter, err *positionalError) {
	param = &Parameter{
		Node: list,
	}

	// Skip the first element, which is the 'param'
	// identifier.
	parts, err := i.interpretLists(list.Elements[1:])
	if err != nil {
		return nil, err.Context("invalid parameter definition")
	}

	for _, part := range parts {
		def, elts, err := i.interpretDefinition(part)
		if err != nil {
			return nil, err.Context("invalid parameter definition")
		}

		switch def.Name {
		case "name":
			name, err := i.interpretName(elts)
			if err != nil {
				return nil, err.Context("invalid parameter name")
			}

			if param.Name != nil {
				return nil, i.errorf(def, "invalid parameter definition: name already defined")
			}

			param.Name = name
		case "docs":
			docs, err := i.interpretDocs(elts)
			if err != nil {
				return nil, err.Context("invalid parameter docs")
			}

			if param.Docs != nil {
				return nil, i.errorf(def, "invalid parameter definition: docs already defined")
			}

			param.Docs = docs
		case "type":
			typ, err := i.interpretType(elts)
			if err != nil {
				return nil, err.Context("invalid parameter type")
			}

			if param.Type != nil {
				return nil, i.errorf(def, "invalid parameter definition: type already defined")
			}

			param.Type = typ
		default:
			return nil, i.errorf(def, "unrecognised parameter definition kind %q", def.Name)
		}
	}

	// Make sure the parameter is complete.
	if param.Name == nil {
		return nil, i.errorf(list, "parameter has no name definition")
	} else if param.Docs == nil {
		return nil, i.errorf(list, "parameter has no docs definition")
	} else if param.Type == nil {
		return nil, i.errorf(list, "parameter has no type definition")
	}

	return param, nil
}

// interpretValue parses the list elements as an
// enumeration's value definition.
func (i *interpreter) interpretValue(list *ast.List) (value *Value, err *positionalError) {
	value = &Value{
		Node: list,
	}

	// Skip the first element, which is the 'value'
	// identifier.
	parts, err := i.interpretLists(list.Elements[1:])
	if err != nil {
		return nil, err.Context("invalid value definition")
	}

	for _, part := range parts {
		def, elts, err := i.interpretDefinition(part)
		if err != nil {
			return nil, err.Context("invalid value definition")
		}

		switch def.Name {
		case "name":
			name, err := i.interpretName(elts)
			if err != nil {
				return nil, err.Context("invalid value name")
			}

			if value.Name != nil {
				return nil, i.errorf(def, "invalid value definition: name already defined")
			}

			value.Name = name
		case "docs":
			docs, err := i.interpretDocs(elts)
			if err != nil {
				return nil, err.Context("invalid value docs")
			}

			if value.Docs != nil {
				return nil, i.errorf(def, "invalid value definition: docs already defined")
			}

			value.Docs = docs
		default:
			return nil, i.errorf(def, "unrecognised value definition kind %q", def.Name)
		}
	}

	// Make sure the value is complete.
	if value.Name == nil {
		return nil, i.errorf(list, "value has no name definition")
	} else if value.Docs == nil {
		return nil, i.errorf(list, "value has no docs definition")
	}

	return value, nil
}
