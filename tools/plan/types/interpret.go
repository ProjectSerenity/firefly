// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package interpreter contains the Plan interpreter, which is used
// to validate a Plan source file's structure and contents, then store
// the result in a more constrained representation.
//
package types

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ProjectSerenity/firefly/tools/plan/ast"
)

// positionalError represents an error that has
// occurred at a specific location within a Plan
// file.
//
type positionalError struct {
	Pos string
	Msg string
}

func (e *positionalError) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
}

func (e *positionalError) Context(msg string) *positionalError {
	return &positionalError{
		Pos: e.Pos,
		Msg: msg + ": " + e.Msg,
	}
}

// interpreter is used to process a Plan source
// file, producing a structured representation
// for the interface it defines.
//
type interpreter struct {
	filename string
	out      *File
	arch     Arch

	// We use this to track type definitinos
	// and ensure that every referenced type
	// is defined.
	typedefs map[string]Type         // Mapping of type name to the defined type.
	typerefs map[string]ast.Node     // Location of first reference to each type.
	typeuses map[string][]*Reference // References to the type that are not yet resolved.
}

// Interpret processes a Plan source file,
// producing a structured representation for
// the interface it defines.
//
func Interpret(filename string, file *ast.File, arch Arch) (*File, error) {
	i := &interpreter{
		filename: filename,
		out:      new(File),
		arch:     arch,
		typerefs: make(map[string]ast.Node),
		typeuses: make(map[string][]*Reference),
		typedefs: map[string]Type{
			// Add the synthetic enumeration for the set
			// of all syscalls.
			"syscalls": &Enumeration{
				Name: Name{"syscalls"},
				Type: Uint64,
			},
		},
	}

	for name, value := range integers {
		i.typedefs[name] = value
	}

	err := i.interpretFile(file)
	if err != nil {
		return nil, err
	}

	return i.out, nil
}

// pos returns the position of the given node in
// the file being interpreted.
//
func (i *interpreter) pos(node ast.Node) string {
	return node.Pos().File(i.filename)
}

// errorf produces a positional error, referring
// to node.
//
func (i *interpreter) errorf(node ast.Node, format string, v ...any) *positionalError {
	return &positionalError{
		Pos: i.pos(node),
		Msg: fmt.Sprintf(format, v...),
	}
}

// interpretFile is the entry point for the
// interpreter.
//
func (i *interpreter) interpretFile(file *ast.File) *positionalError {
	for _, list := range file.Lists {
		def, _, err := i.interpretDefinition(list)
		if err != nil {
			return err.Context("invalid top-level definition")
		}

		switch def.Name {
		case "enumeration":
			enumeration, err := i.interpretEnumeration(list)
			if err != nil {
				return err
			}

			i.out.Enumerations = append(i.out.Enumerations, enumeration)
		case "structure":
			structure, err := i.interpretStructure(list)
			if err != nil {
				return err
			}

			i.out.Structures = append(i.out.Structures, structure)
		case "syscall":
			syscall, err := i.interpretSyscall(list)
			if err != nil {
				return err
			}

			i.out.Syscalls = append(i.out.Syscalls, syscall)
		default:
			return i.errorf(def, "unrecognised definition kind %q", def.Name)
		}
	}

	// Type-check the interpreted structures.
	for typename, definition := range i.typedefs {
		if definition == nil {
			return i.errorf(i.typerefs[typename], "type %q is not defined", typename)
		}
	}

	// Check that each structure's fields are
	// complete and properly aligned.
	for _, structure := range i.out.Structures {
		offset := 0
		nonPadding := false
		for _, field := range structure.Fields {
			switch typ := field.Type.(type) {
			case Padding:
				// We don't need to check that padding fields
				// are aligned, as they aren't referenced.
			case nil:
				return i.errorf(field.Node, "field %q has an invalid type reference", field.Name.Spaced())
			default:
				nonPadding = true

				// Check alignment.
				size := typ.Size(i.arch)
				if offset%size != 0 {
					name := field.Name.Spaced()
					return i.errorf(field.Node, "field %q is not aligned: %d-byte field found at offset %d", name, size, offset)
				}
			}

			offset += field.Type.Size(i.arch)
		}

		if !nonPadding {
			return i.errorf(structure.Node, "structure has no non-padding fields")
		}
	}

	// Check that each syscall's args and results
	// are integer types, enumerations (which are
	// integer types under the hood), or pointers.
	for _, syscall := range i.out.Syscalls {
		for j, arg := range syscall.Args {
			argType := arg.Type
			if ref, ok := argType.(*Reference); ok {
				argType = ref.Underlying
			}

			switch argType.(type) {
			case Integer, *Enumeration, *Pointer:
			case nil:
				return i.errorf(arg.Node, "arg%d %q is an invalid type reference", j, arg.Name.Spaced())
			default:
				name := arg.Name.Spaced()
				return i.errorf(arg.Node, "arg%d %q has invalid type: %s cannot be stored in a register", j+1, name, argType)
			}
		}

		for j, result := range syscall.Results {
			resultType := result.Type
			if ref, ok := resultType.(*Reference); ok {
				resultType = ref.Underlying
			}

			switch resultType.(type) {
			case Integer, *Enumeration, *Pointer:
			case nil:
				return i.errorf(result.Node, "result%d %q is an invalid type reference", j, result.Name.Spaced())
			default:
				name := result.Name.Spaced()
				return i.errorf(result.Node, "result%d %q has invalid type: %s cannot be stored in a register", j+1, name, resultType)
			}
		}
	}

	return nil
}

// interpretEnumeration parses the list elements as
// an enum definition.
//
func (i *interpreter) interpretEnumeration(list *ast.List) (*Enumeration, *positionalError) {
	// Skip the first element, which is the 'enumeration'
	// identifier.
	parts, err := i.interpretLists(list.Elements[1:])
	if err != nil {
		return nil, err.Context("invalid enumeration")
	}

	enumeration := &Enumeration{
		Node: list,
	}

	values := make(map[string]*Value)
	for _, part := range parts {
		def, elts, err := i.interpretDefinition(part)
		if err != nil {
			return nil, err
		}

		switch def.Name {
		case "name":
			name, err := i.interpretName(elts)
			if err != nil {
				return nil, err.Context("invalid enumeration name")
			}

			if enumeration.Name != nil {
				return nil, i.errorf(def, "invalid enumeration definition: name already defined")
			}

			enumeration.Name = name
		case "docs":
			docs, err := i.interpretDocs(elts)
			if err != nil {
				return nil, err.Context("invalid enumeration docs")
			}

			if enumeration.Docs != nil {
				return nil, i.errorf(def, "invalid enumeration definition: docs already defined")
			}

			enumeration.Docs = docs
		case "type":
			typ, err := i.interpretType(elts)
			if err != nil {
				return nil, err.Context("invalid enumeration type")
			}

			integer, ok := typ.(Integer)
			if !ok {
				return nil, i.errorf(elts[0], "invalid enumeration type: must be an integer type, found %s", typ)
			}

			if enumeration.Type != InvalidInteger {
				return nil, i.errorf(def, "invalid enumeration definition: type already defined")
			}

			enumeration.Type = integer
		case "value":
			value, err := i.interpretValue(part)
			if err != nil {
				return nil, err
			}

			// Make sure the value isn't a duplicate.
			name := value.Name.Spaced()
			if other, ok := values[name]; ok {
				return nil, i.errorf(part, "value %q already defined at %s", name, i.pos(other.Node))
			}

			values[name] = value
			enumeration.Values = append(enumeration.Values, value)
		default:
			return nil, i.errorf(def, "unrecognised enumeration definition kind %q", def.Name)
		}
	}

	// Make sure the enumeration is complete.
	if enumeration.Name == nil {
		return nil, i.errorf(list, "enumeration has no name definition")
	} else if enumeration.Docs == nil {
		return nil, i.errorf(list, "enumeration has no docs definition")
	} else if enumeration.Type == InvalidInteger {
		return nil, i.errorf(list, "enumeration has no type definition")
	} else if enumeration.Values == nil {
		return nil, i.errorf(list, "enumeration has no value definitions")
	}

	if uint64(len(enumeration.Values)) > enumeration.Type.Max() {
		got := len(enumeration.Values)
		max := enumeration.Type.Max()
		return nil, i.errorf(list, "enumeration has %d values, which exceeds capacity of %s (max %d)", got, enumeration.Type, max)
	}

	// Track the type definition.
	typename := enumeration.Name.Spaced()
	if i.typedefs[typename] != nil {
		return nil, i.errorf(enumeration.Node, "type %q is already defined", typename)
	}

	// Complete any references to the type.
	i.typedefs[typename] = enumeration
	for _, ref := range i.typeuses[typename] {
		ref.Underlying = enumeration
	}
	i.typeuses[typename] = nil

	return enumeration, nil
}

// interpretStructure parses the list elements as
// a struct definition.
//
func (i *interpreter) interpretStructure(list *ast.List) (*Structure, *positionalError) {
	// Skip the first element, which is the 'structure'
	// identifier.
	parts, err := i.interpretLists(list.Elements[1:])
	if err != nil {
		return nil, err.Context("invalid structure")
	}

	structure := &Structure{
		Node: list,
	}

	for _, part := range parts {
		def, elts, err := i.interpretDefinition(part)
		if err != nil {
			return nil, err
		}

		switch def.Name {
		case "name":
			name, err := i.interpretName(elts)
			if err != nil {
				return nil, err.Context("invalid structure name")
			}

			if structure.Name != nil {
				return nil, i.errorf(def, "invalid structure definition: name already defined")
			}

			structure.Name = name
		case "docs":
			docs, err := i.interpretDocs(elts)
			if err != nil {
				return nil, err.Context("invalid structure docs")
			}

			if structure.Docs != nil {
				return nil, i.errorf(def, "invalid structure definition: docs already defined")
			}

			structure.Docs = docs
		case "field":
			field, err := i.interpretField(part)
			if err != nil {
				return nil, err
			}

			// Make sure the field isn't a duplicate.
			for _, other := range structure.Fields {
				if other.Name.Spaced() == field.Name.Spaced() {
					return nil, i.errorf(part, "field %q already defined at %s", field.Name.Spaced(), i.pos(other.Node))
				}
			}

			structure.Fields = append(structure.Fields, field)
		default:
			return nil, i.errorf(def, "unrecognised structure definition kind %q", def.Name)
		}
	}

	// Make sure the structure is complete.
	if structure.Name == nil {
		return nil, i.errorf(list, "structure has no name definition")
	} else if structure.Docs == nil {
		return nil, i.errorf(list, "structure has no docs definition")
	} else if structure.Fields == nil {
		return nil, i.errorf(list, "structure has no field definitions")
	}

	// Track the type definition.
	typename := structure.Name.Spaced()
	if i.typedefs[typename] != nil {
		return nil, i.errorf(structure.Node, "type %q is already defined", typename)
	}

	// Complete any references to the type.
	i.typedefs[typename] = structure
	for _, ref := range i.typeuses[typename] {
		ref.Underlying = structure
	}
	i.typeuses[typename] = nil

	return structure, nil
}

// interpretSyscall parses the list elements as
// a syscall definition.
//
func (i *interpreter) interpretSyscall(list *ast.List) (*Syscall, *positionalError) {
	// Skip the first element, which is the 'syscall'
	// identifier.
	parts, err := i.interpretLists(list.Elements[1:])
	if err != nil {
		return nil, err.Context("invalid syscall")
	}

	var args [6]*Parameter
	var results [2]*Parameter
	syscall := &Syscall{
		Node: list,
	}

	for _, part := range parts {
		def, elts, err := i.interpretDefinition(part)
		if err != nil {
			return nil, err
		}

		switch def.Name {
		case "name":
			name, err := i.interpretName(elts)
			if err != nil {
				return nil, err.Context("invalid syscall name")
			}

			if syscall.Name != nil {
				return nil, i.errorf(def, "invalid syscall definition: name already defined")
			}

			syscall.Name = name
		case "docs":
			docs, err := i.interpretDocs(elts)
			if err != nil {
				return nil, err.Context("invalid syscall docs")
			}

			if syscall.Docs != nil {
				return nil, i.errorf(def, "invalid syscall definition: docs already defined")
			}

			syscall.Docs = docs
		case "arg1", "arg2", "arg3", "arg4", "arg5", "arg6":
			index := def.Name[3] - '1'
			arg, err := i.interpretParameter(part)
			if err != nil {
				return nil, err
			}

			// Make sure the arg isn't a duplicate.
			if args[index] != nil {
				return nil, i.errorf(part, "%s %q already defined at %s", def.Name, arg.Name.Spaced(), i.pos(args[index].Node))
			}

			args[index] = arg
		case "result1", "result2":
			index := def.Name[6] - '1'
			result, err := i.interpretParameter(part)
			if err != nil {
				return nil, err
			}

			// Make sure the result isn't a duplicate.
			if results[index] != nil {
				return nil, i.errorf(part, "%s %q already defined at %s", def.Name, result.Name.Spaced(), i.pos(results[index].Node))
			}

			results[index] = result
		default:
			return nil, i.errorf(def, "unrecognised syscall definition kind %q", def.Name)
		}
	}

	// Make sure the syscall is complete.
	if syscall.Name == nil {
		return nil, i.errorf(list, "syscall has no name definition")
	} else if syscall.Docs == nil {
		return nil, i.errorf(list, "syscall has no docs definition")
	}

	// Make sure the args and results are
	// in order.
	lastArg := -1
	for j, arg := range args {
		if arg != nil {
			lastArg = j
		}
	}

	for j, arg := range args[:lastArg+1] {
		if arg == nil {
			return nil, i.errorf(args[lastArg].Node, "arg%d is defined but arg%d is missing", lastArg+1, j+1)
		}
	}

	lastResult := -1
	for j, result := range results {
		if result != nil {
			lastResult = j
		}
	}

	for j, result := range results[:lastResult+1] {
		if result == nil {
			return nil, i.errorf(results[lastResult].Node, "result%d is defined but result%d is missing", lastResult+1, j+1)
		}
	}

	syscall.Args = args[:lastArg+1]
	syscall.Results = results[:lastResult+1]

	// Track the syscall definition.
	name := syscall.Name.Spaced()
	if i.typedefs[name] != nil {
		return nil, i.errorf(syscall.Node, "cannot define syscall: type %q is already defined", name)
	}

	// Complete any references to the type.
	sysref := &SyscallReference{Name: syscall.Name}
	i.typedefs[name] = sysref
	for _, ref := range i.typeuses[name] {
		ref.Underlying = sysref
	}
	i.typeuses[name] = nil

	return syscall, nil
}

// interpretField parses the list elements as a
// field definition.
//
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
//
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
//
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

// interpretDefinition ensures that the given list
// consists of an identifier followed by at least
// one further element.
//
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
//
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
//
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
//
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

// interpretType ensures that the given elements form a
// valid type reference, returning the parsed type.
//
func (i *interpreter) interpretType(elts []ast.Expr) (Type, *positionalError) {
	// For now, we only allow one pointer, which must
	// be the first part of the type. It must always
	// be followed by an underlying type.
	switch first := elts[0].(type) {
	case *ast.Pointer:
		var mutable bool
		switch first.Note {
		case "constant":
		case "mutable":
			mutable = true
		default:
			return nil, i.errorf(first, "invalid pointer note: want \"constant\" or \"mutable\", found %q", first.Note)
		}

		// Determine the underlying type. For now, we
		// only allow quite basic underlying types.
		ref, err := i.interpretTypeReference(elts[1:])
		if err != nil {
			return nil, err
		}

		ptr := &Pointer{
			Mutable:    mutable,
			Underlying: ref,
		}

		return ptr, nil
	case *ast.Identifier:
		// Get the rest of the name, including the
		// first element.
		return i.interpretTypeReference(elts)
	default:
		return nil, i.errorf(elts[0], "expected a type definition, found %s", elts[0])
	}
}

// interpretTypeReference ensures that the given list
// elements form a valid basic type reference, returning
// the parsed reference.
//
func (i *interpreter) interpretTypeReference(elts []ast.Expr) (Type, *positionalError) {
	// We don't allow pointer types here, so the list
	// elements should all be identifiers.
	name, err := i.interpretName(elts)
	if err != nil {
		return nil, err.Context("invalid type reference")
	}

	if len(name) == 1 && integers[name[0]] != 0 {
		return integers[name[0]], nil
	}

	// See whether this type has already
	// been defined.
	typename := name.Spaced()
	if underlying := i.typedefs[typename]; underlying != nil {
		ref := &Reference{
			Name:       name,
			Underlying: underlying,
		}

		return ref, nil
	}

	// Note that we've referenced this
	// type.
	ref := &Reference{
		Name:       name,
		Underlying: nil, // This will be populated later when the type is defined.
	}

	i.typedefs[typename] = nil
	i.typerefs[typename] = elts[0]
	i.typeuses[typename] = append(i.typeuses[typename], ref)

	return ref, nil
}
