// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package types contains the Plan interpreter, which is used to
// validate a Plan source file's structure and contents, then store
// the result in a more constrained representation.
package types

import (
	"fmt"
	"sort"
	"strconv"

	"firefly-os.dev/tools/plan/ast"
)

// requiredErrorValues is the set of values that
// any error enumeration must start with. The easiest,
// but not only, way to do this is to define them
// for a base error enumeration, then embed that in
// more complex error enumerations.
var requiredErrorValues = []Name{
	{"no", "error"},
	{"bad", "syscall"},
	{"illegal", "arg1"},
	{"illegal", "arg2"},
	{"illegal", "arg3"},
	{"illegal", "arg4"},
	{"illegal", "arg5"},
	{"illegal", "arg6"},
}

// positionalError represents an error that has
// occurred at a specific location within a Plan
// file.
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
func (i *interpreter) pos(node ast.Node) string {
	return node.Pos().File(i.filename)
}

// errorf produces a positional error, referring
// to node.
func (i *interpreter) errorf(node ast.Node, format string, v ...any) *positionalError {
	return &positionalError{
		Pos: i.pos(node),
		Msg: fmt.Sprintf(format, v...),
	}
}

// interpretFile is the entry point for the
// interpreter.
func (i *interpreter) interpretFile(file *ast.File) *positionalError {
	for _, list := range file.Lists {
		def, _, err := i.interpretDefinition(list)
		if err != nil {
			return err.Context("invalid top-level definition")
		}

		switch def.Name {
		case "array":
			array, err := i.interpretArray(list)
			if err != nil {
				return err
			}

			i.out.Arrays = append(i.out.Arrays, array)
		case "bitfield":
			bitfield, err := i.interpretBitfield(list)
			if err != nil {
				return err
			}

			i.out.Bitfields = append(i.out.Bitfields, bitfield)
		case "enumeration":
			enumeration, err := i.interpretEnumeration(list)
			if err != nil {
				return err
			}

			i.out.Enumerations = append(i.out.Enumerations, enumeration)
		case "integer":
			integer, err := i.interpretNewInteger(list)
			if err != nil {
				return err
			}

			i.out.NewIntegers = append(i.out.NewIntegers, integer)
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
		case "group":
			group, err := i.interpretGroup(list)
			if err != nil {
				return err
			}

			i.out.Groups = append(i.out.Groups, group)
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

	// Make sure that each enumeration with a
	// name ending in "error" has all the values
	// we expect from an error enumeration.
	errorEnumerations := make(map[string]string)
	var isErrorEnumeration func(enum *Enumeration) (isError bool, missing string)
	isErrorEnumeration = func(enum *Enumeration) (isError bool, missing string) {
		name := enum.Name.Spaced()

		// Check if we already know the answer.
		missing, ok := errorEnumerations[name]
		if ok {
			return missing == "", missing
		}

		// Check whether we have all of the values
		// we require at the start of the enumeration.
		for i, want := range requiredErrorValues {
			wantName := want.Spaced()
			if i >= len(enum.Values) {
				errorEnumerations[name] = wantName
				return false, wantName
			}

			if enum.Values[i].Name.Spaced() != wantName {
				errorEnumerations[name] = wantName
				return false, wantName
			}
		}

		errorEnumerations[name] = ""
		return true, ""
	}

	for _, enumeration := range i.out.Enumerations {
		// Ignore non-errors.
		name := enumeration.Name
		if name[len(name)-1] != "error" {
			continue
		}

		isError, missing := isErrorEnumeration(enumeration)
		if !isError {
			return i.errorf(enumeration.Node, "enumeration %q is not an error enumeration: missing value %q", name.Spaced(), missing)
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
				align := typ.Alignment(i.arch)
				if offset%align != 0 {
					name := field.Name.Spaced()
					return i.errorf(field.Node, "field %q is not aligned: %d-aligned field found at offset %d", name, align, offset)
				}
			}

			offset += field.Type.Size(i.arch)
		}

		align := structure.Alignment(i.arch)
		if offset%align != 0 {
			name := structure.Name.Spaced()
			return i.errorf(structure.Node, "structure %q is not aligned: %d-aligned structure ends at offset %d", name, align, offset)
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
			argType := Underlying(arg.Type)
			if argType == nil {
				return i.errorf(arg.Node, "arg%d %q is an invalid type reference", j, arg.Name.Spaced())
			} else if !argType.Parameter(i.arch) {
				name := arg.Name.Spaced()
				return i.errorf(arg.Node, "arg%d %q has invalid type: %s cannot be stored in a parameter", j+1, name, argType)
			}
		}

		for j, result := range syscall.Results {
			resultType := Underlying(result.Type)
			if resultType == nil {
				return i.errorf(result.Node, "result%d %q is an invalid type reference", j, result.Name.Spaced())
			} else if !resultType.Parameter(i.arch) {
				name := result.Name.Spaced()
				return i.errorf(result.Node, "result%d %q has invalid type: %s cannot be stored in a parameter", j+1, name, resultType)
			}
		}

		// Any syscall could fail, such as if it
		// has been disabled. As a result, its
		// final result must be an error enumeration.
		if len(syscall.Results) == 0 {
			return i.errorf(syscall.Node, "cannot handle errors in %s: syscall has no results", syscall)
		} else {
			result := syscall.Results[len(syscall.Results)-1]
			resultType := Underlying(result.Type)
			enum, ok := resultType.(*Enumeration)
			if !ok {
				return i.errorf(result.Node, "cannot handle errors in %s: expected final result to be enumeration, found %s", syscall, resultType)
			}

			isError, missing := isErrorEnumeration(enum)
			if !isError {
				return i.errorf(result.Node, "cannot handle errors in %s: %s is not an error enumeration: missing value %q", syscall, enum, missing)
			}
		}
	}

	// Complete each item group's item references,
	// ensuring each refers to a valid item.
	for _, group := range i.out.Groups {
		// Check that the group name doesn't clash
		// with an item name.
		groupName := group.Name.Spaced()
		if _, ok := i.typedefs[groupName]; ok {
			return i.errorf(group.Node, "type %q is already defined", groupName)
		}

		for _, item := range group.List {
			typename := item.Name.Spaced()
			typ, ok := i.typedefs[typename]
			if !ok {
				return i.errorf(item.Node, "type %q is not defined", typename)
			}

			var value any = typ

			var want string
			switch typ := typ.(type) {
			case *Array:
				want = "array"
				typ.Groups = append(typ.Groups, group.Name)
			case *Bitfield:
				want = "bitfield"
				typ.Groups = append(typ.Groups, group.Name)
			case *Enumeration:
				want = "enumeration"
				typ.Groups = append(typ.Groups, group.Name)
			case *NewInteger:
				want = "integer"
				typ.Groups = append(typ.Groups, group.Name)
			case *Structure:
				want = "structure"
				typ.Groups = append(typ.Groups, group.Name)
			case *SyscallReference:
				want = "syscall"
				for _, syscall := range i.out.Syscalls {
					gotName := syscall.Name.Spaced()
					if gotName == typename {
						value = syscall
						syscall.Groups = append(syscall.Groups, group.Name)
						break
					}
				}
			default:
				return i.errorf(item.Node, "%s reference %q resolved to a %T", item.Type, typename, value)
			}

			if item.Type != want {
				return i.errorf(item.Node, "%s reference %q resolved to a %T", item.Type, typename, value)
			}

			item.Underlying = value
		}
	}

	// Now that we've resolved all groups, we should
	// sort the group names in each item.
	sortGroups := func(groups []Name) {
		if len(groups) == 0 {
			return
		}

		sort.Slice(groups, func(i, j int) bool {
			// First, try the easy path where the
			// first part of both names differ.
			if groups[i][0] != groups[j][0] {
				return groups[i][0] < groups[j][0]
			}

			// Otherwise, we take the more involved
			// route.
			return groups[i].Spaced() < groups[j].Spaced()
		})
	}

	for _, array := range i.out.Arrays {
		sortGroups(array.Groups)
	}

	for _, bitfield := range i.out.Bitfields {
		sortGroups(bitfield.Groups)
	}

	for _, enumeration := range i.out.Enumerations {
		sortGroups(enumeration.Groups)
	}

	for _, integer := range i.out.NewIntegers {
		sortGroups(integer.Groups)
	}

	for _, structure := range i.out.Structures {
		sortGroups(structure.Groups)
	}

	for _, syscall := range i.out.Syscalls {
		sortGroups(syscall.Groups)
	}

	return nil
}

// interpretArray parses the list elements as
// an array definition.
func (i *interpreter) interpretArray(list *ast.List) (*Array, *positionalError) {
	// Skip the first element, which is the 'array'
	// identifier.
	parts, err := i.interpretLists(list.Elements[1:])
	if err != nil {
		return nil, err.Context("invalid array")
	}

	array := &Array{
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
				return nil, err.Context("invalid array name")
			}

			if array.Name != nil {
				return nil, i.errorf(def, "invalid array definition: name already defined")
			}

			array.Name = name
		case "docs":
			docs, err := i.interpretDocs(elts)
			if err != nil {
				return nil, err.Context("invalid array docs")
			}

			if array.Docs != nil {
				return nil, i.errorf(def, "invalid array definition: docs already defined")
			}

			array.Docs = docs
		case "size":
			num, ok := elts[0].(*ast.Number)
			if !ok {
				return nil, i.errorf(elts[0], "invalid array size definition: expected a number, found %s", elts[0])
			}

			if len(elts) > 1 {
				return nil, i.errorf(elts[1], "invalid array size definition: unexpected %s after size", elts[1])
			}

			size, err := strconv.ParseUint(num.Value, 10, 64)
			if err != nil {
				// Unwrap the error if possible.
				if e, ok := err.(interface{ Unwrap() error }); ok {
					err = e.Unwrap()
				}

				return nil, i.errorf(num, "invalid array size definition: invalid size: %v", err)
			}

			if size == 0 {
				return nil, i.errorf(num, "invalid array size definition: size must be larger than zero")
			}

			if array.Count != 0 {
				return nil, i.errorf(def, "invalid array definition: size already defined")
			}

			array.Count = size
		case "type":
			typ, err := i.interpretType(elts)
			if err != nil {
				return nil, err.Context("invalid array element type")
			}

			if array.Type != nil {
				return nil, i.errorf(def, "invalid array definition: type already defined")
			}

			array.Type = typ
		default:
			return nil, i.errorf(def, "unrecognised array definition kind %q", def.Name)
		}
	}

	// Make sure the array is complete.
	if array.Name == nil {
		return nil, i.errorf(list, "array has no name definition")
	} else if array.Docs == nil {
		return nil, i.errorf(list, "array has no docs definition")
	} else if array.Count == 0 {
		return nil, i.errorf(list, "array has no size definition")
	} else if array.Type == nil {
		return nil, i.errorf(list, "array has no type definition")
	}

	// Track the type definition.
	typename := array.Name.Spaced()
	if i.typedefs[typename] != nil {
		return nil, i.errorf(array.Node, "type %q is already defined", typename)
	}

	// Complete any references to the type.
	i.typedefs[typename] = array
	for _, ref := range i.typeuses[typename] {
		ref.Underlying = array
	}
	i.typeuses[typename] = nil

	return array, nil
}

// interpretBitfield parses the list elements as a
// bitfield definition.
func (i *interpreter) interpretBitfield(list *ast.List) (*Bitfield, *positionalError) {
	// Skip the first element, which is the 'bitfield'
	// identifier.
	parts, err := i.interpretLists(list.Elements[1:])
	if err != nil {
		return nil, err.Context("invalid bitfield")
	}

	bitfield := &Bitfield{
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
				return nil, err.Context("invalid bitfield name")
			}

			if bitfield.Name != nil {
				return nil, i.errorf(def, "invalid bitfield definition: name already defined")
			}

			bitfield.Name = name
		case "docs":
			docs, err := i.interpretDocs(elts)
			if err != nil {
				return nil, err.Context("invalid bitfield docs")
			}

			if bitfield.Docs != nil {
				return nil, i.errorf(def, "invalid bitfield definition: docs already defined")
			}

			bitfield.Docs = docs
		case "type":
			typ, err := i.interpretType(elts)
			if err != nil {
				return nil, err.Context("invalid bitfield type")
			}

			integer, ok := typ.(Integer)
			if !ok {
				return nil, i.errorf(elts[0], "invalid bitfield type: must be an integer type, found %s", typ)
			}

			if bitfield.Type != InvalidInteger {
				return nil, i.errorf(def, "invalid bitfield definition: type already defined")
			}

			bitfield.Type = integer
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
			bitfield.Values = append(bitfield.Values, value)
		default:
			return nil, i.errorf(def, "unrecognised bitfield definition kind %q", def.Name)
		}
	}

	// Make sure the bitfield is complete.
	if bitfield.Name == nil {
		return nil, i.errorf(list, "bitfield has no name definition")
	} else if bitfield.Docs == nil {
		return nil, i.errorf(list, "bitfield has no docs definition")
	} else if bitfield.Type == InvalidInteger {
		return nil, i.errorf(list, "bitfield has no type definition")
	} else if bitfield.Values == nil {
		return nil, i.errorf(list, "bitfield has no value definitions")
	}

	if len(bitfield.Values) > bitfield.Type.Bits() {
		got := len(bitfield.Values)
		max := bitfield.Type.Bits()
		return nil, i.errorf(list, "bitfield has %d values, which exceeds capacity of %s (max %d)", got, bitfield.Type, max)
	}

	// Track the type definition.
	typename := bitfield.Name.Spaced()
	if i.typedefs[typename] != nil {
		return nil, i.errorf(bitfield.Node, "type %q is already defined", typename)
	}

	// Complete any references to the type.
	i.typedefs[typename] = bitfield
	for _, ref := range i.typeuses[typename] {
		ref.Underlying = bitfield
	}
	i.typeuses[typename] = nil

	return bitfield, nil
}

// interpretEnumeration parses the list elements as
// an enum definition.
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
		case "embed":
			typ, err := i.interpretTypeReference(elts)
			if err != nil {
				return nil, err.Context("invalid enumeration embedding")
			}

			ref, ok := typ.(*Reference)
			if !ok {
				return nil, i.errorf(elts[0], "invalid embedded type: expected an enumeration, found %s", typ)
			}

			if ref.Underlying == nil {
				return nil, i.errorf(elts[0], "invalid embedded type: type %q has not yet been defined", ref.Name.Spaced())
			}

			enum, ok := ref.Underlying.(*Enumeration)
			if !ok {
				return nil, i.errorf(elts[0], "invalid embedded type: expected an enumeration, found %s", ref.Underlying)
			}

			for _, value := range enum.Values {
				// Make sure the value isn't a duplicate.
				name := value.Name.Spaced()
				if other, ok := values[name]; ok {
					return nil, i.errorf(elts[0], "embedded value %q already defined at %s", name, i.pos(other.Node))
				}

				values[name] = value
				enumeration.Values = append(enumeration.Values, value)
			}

			enumeration.Embeds = append(enumeration.Embeds, enum)
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

// interpretNewInteger parses the list elements as a
// new integer definition.
func (i *interpreter) interpretNewInteger(list *ast.List) (*NewInteger, *positionalError) {
	// Skip the first element, which is the 'integer'
	// identifier.
	parts, err := i.interpretLists(list.Elements[1:])
	if err != nil {
		return nil, err.Context("invalid integer")
	}

	newInteger := &NewInteger{
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
				return nil, err.Context("invalid integer name")
			}

			if newInteger.Name != nil {
				return nil, i.errorf(def, "invalid integer definition: name already defined")
			}

			newInteger.Name = name
		case "docs":
			docs, err := i.interpretDocs(elts)
			if err != nil {
				return nil, err.Context("invalid integer docs")
			}

			if newInteger.Docs != nil {
				return nil, i.errorf(def, "invalid integer definition: docs already defined")
			}

			newInteger.Docs = docs
		case "type":
			typ, err := i.interpretType(elts)
			if err != nil {
				return nil, err.Context("invalid integer type")
			}

			integer, ok := typ.(Integer)
			if !ok {
				return nil, i.errorf(elts[0], "invalid integer type: must be an integer type, found %s", typ)
			}

			if newInteger.Type != InvalidInteger {
				return nil, i.errorf(def, "invalid integer definition: type already defined")
			}

			newInteger.Type = integer
		default:
			return nil, i.errorf(def, "unrecognised integer definition kind %q", def.Name)
		}
	}

	// Make sure the integer is complete.
	if newInteger.Name == nil {
		return nil, i.errorf(list, "integer has no name definition")
	} else if newInteger.Docs == nil {
		return nil, i.errorf(list, "integer has no docs definition")
	} else if newInteger.Type == InvalidInteger {
		return nil, i.errorf(list, "integer has no type definition")
	}

	// Track the type definition.
	typename := newInteger.Name.Spaced()
	if i.typedefs[typename] != nil {
		return nil, i.errorf(newInteger.Node, "type %q is already defined", typename)
	}

	// Complete any references to the type.
	i.typedefs[typename] = newInteger
	for _, ref := range i.typeuses[typename] {
		ref.Underlying = newInteger
	}
	i.typeuses[typename] = nil

	return newInteger, nil
}

// interpretStructure parses the list elements as
// a struct definition.
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

// interpretGroup parses the list elements as a
// group definition.
func (i *interpreter) interpretGroup(list *ast.List) (*Group, *positionalError) {
	// Skip the first element, which is the 'group'
	// identifier.
	parts, err := i.interpretLists(list.Elements[1:])
	if err != nil {
		return nil, err.Context("invalid group")
	}

	group := &Group{
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
				return nil, err.Context("invalid group name")
			}

			if group.Name != nil {
				return nil, i.errorf(def, "invalid group definition: name already defined")
			}

			group.Name = name
		case "docs":
			docs, err := i.interpretDocs(elts)
			if err != nil {
				return nil, err.Context("invalid group docs")
			}

			if group.Docs != nil {
				return nil, i.errorf(def, "invalid group definition: docs already defined")
			}

			group.Docs = docs
		case "array", "bitfield", "enumeration", "integer", "structure", "syscall":
			name, err := i.interpretName(elts)
			if err != nil {
				return nil, err
			}

			// We populate Underlying later.
			ref := &ItemReference{
				Type: def.Name,
				Name: name,
				Node: part,
			}

			group.List = append(group.List, ref)
		default:
			return nil, i.errorf(def, "unrecognised group definition kind %q", def.Name)
		}
	}

	// Make sure the group is complete.
	if group.Name == nil {
		return nil, i.errorf(list, "group has no name definition")
	} else if group.Docs == nil {
		return nil, i.errorf(list, "group has no docs definition")
	} else if len(group.List) == 0 {
		return nil, i.errorf(list, "group has no item definitions")
	}

	return group, nil
}

// interpretType ensures that the given elements form a
// valid type reference, returning the parsed type.
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
