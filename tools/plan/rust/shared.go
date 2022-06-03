// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package rust uses templates to render a Plan document as Rust code.
//
package rust

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/template"

	"firefly-os.dev/tools/plan/ast"
	"firefly-os.dev/tools/plan/types"
)

// GenerateSharedCode generates the shared data structures
// of the Plan document to w, using the rustfmt tool at the
// given path.
//
func GenerateSharedCode(w io.Writer, file *types.File, rustfmt string) error {
	// Start with the prelude.
	var buf bytes.Buffer
	err := sharedTemplates.ExecuteTemplate(&buf, sharedFileTemplate, file)
	if err != nil {
		return fmt.Errorf("failed to execute %s: %v", sharedFileTemplate, err)
	}

	buf.WriteString("\n\n")

	// Then add the enumeration of the syscalls.
	err = sharedTemplates.ExecuteTemplate(&buf, enumerationTemplate, file.SyscallsEnumeration())
	if err != nil {
		return fmt.Errorf("failed to append syscalls enumeration: %v", err)
	}

	buf.WriteString("\n\n")

	// Make a list of the items in the file, then sort
	// them into the order in which they appeared in
	// the order in which they were defined in the
	// original text. We then print them one by one
	// using the corresponding template for each item
	// type.

	numItems := len(file.NewIntegers) + len(file.Enumerations) + len(file.Bitfields) + len(file.Structures) + len(file.Syscalls)
	items := make([]ast.Node, 0, numItems)
	for _, integer := range file.NewIntegers {
		items = append(items, integer)
	}
	for _, enumeration := range file.Enumerations {
		items = append(items, enumeration)
	}
	for _, bitfield := range file.Bitfields {
		items = append(items, bitfield)
	}
	for _, structure := range file.Structures {
		items = append(items, structure)
	}
	for _, syscall := range file.Syscalls {
		items = append(items, syscall)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Pos().Offset() < items[j].Pos().Offset()
	})

	// Finally, write the file's items.
	for i, item := range items {
		if i > 0 {
			buf.WriteString("\n\n")
		}

		var template string
		switch item := item.(type) {
		case *types.NewInteger:
			template = integerTemplate
		case *types.Enumeration:
			template = enumerationTemplate
		case *types.Bitfield:
			template = bitfieldTemplate
		case *types.Structure:
			template = structureTemplate
		case *types.Syscall:
			// We skip syscall definitions.
			continue
		default:
			panic(fmt.Sprintf("unreachable file item type %T", item))
		}

		err := sharedTemplates.ExecuteTemplate(&buf, template, item)
		if err != nil {
			return fmt.Errorf("failed to execute template %q with %T: %v", template, item, err)
		}
	}

	// Make sure rustfmt is happy.
	var out bytes.Buffer
	cmd := exec.Command(rustfmt)
	cmd.Stdin = bytes.NewReader(buf.Bytes())
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	if err != nil {
		os.Stderr.Write(out.Bytes())
		return fmt.Errorf("failed to format Rust code: %v", err)
	}

	_, err = w.Write(out.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write Rust code: %v", err)
	}

	return nil
}

// The templates used to render type definitions
// as Rust code.
//
//go:embed templates/shared_*_rs.txt
var sharedTemplatesFS embed.FS

var sharedTemplates = template.Must(template.New("").Funcs(template.FuncMap{
	"addOne":             sharedAddOne,
	"errorEnumeration":   sharedErrorEnumeration,
	"fromU64":            sharedFromU64,
	"isEnumeration":      sharedIsEnumeration,
	"isInteger":          sharedIsInteger,
	"isPadding":          sharedIsPadding,
	"isPointer":          sharedIsPointer,
	"isSigned":           sharedIsSigned,
	"isSint64":           sharedIsSint64,
	"isUint64":           sharedIsUint64,
	"nonErrorResult":     kernelNonErrorResult,
	"oneResult":          sharedOneResult,
	"paramNamesAndTypes": sharedParamNamesAndTypes,
	"paramTypes":         sharedParamTypes,
	"toDocs":             sharedToDocs,
	"toString":           sharedToString,
	"toU64":              sharedToU64,
}).ParseFS(sharedTemplatesFS, "templates/shared_*_rs.txt"))

const (
	sharedFileTemplate  = "shared_file_rs.txt"
	integerTemplate     = "shared_integer_rs.txt"
	enumerationTemplate = "shared_enumeration_rs.txt"
	bitfieldTemplate    = "shared_bitfield_rs.txt"
	structureTemplate   = "shared_structure_rs.txt"
)

func sharedAddOne(i int) int {
	return i + 1
}

func sharedErrorEnumeration(s *types.Syscall) *types.Enumeration {
	return s.Results[len(s.Results)-1].Type.(*types.Reference).Underlying.(*types.Enumeration)
}

func sharedFromU64(t types.Type) string {
	t = types.Underlying(t)
	if integer, ok := t.(types.Integer); ok && integer == types.Uint64 {
		return ""
	}

	return fmt.Sprintf(" as %s", sharedToString(t))
}

func sharedIsEnumeration(typ types.Type) bool {
	_, ok := types.Underlying(typ).(*types.Enumeration)
	return ok
}

func sharedIsInteger(typ types.Type) bool {
	_, ok := types.Underlying(typ).(types.Integer)
	return ok
}

func sharedIsPadding(typ types.Type) bool {
	_, ok := typ.(types.Padding)
	return ok
}

func sharedIsPointer(typ types.Type) bool {
	_, ok := typ.(*types.Pointer)
	return ok
}

func sharedIsSigned(typ types.Type) bool {
	if integer, ok := types.Underlying(typ).(types.Integer); ok {
		switch integer {
		case types.Sint8, types.Sint16, types.Sint32, types.Sint64:
			return true
		}
	}

	return false
}

func sharedIsSint64(typ types.Type) bool {
	if integer, ok := types.Underlying(typ).(types.Integer); ok && integer == types.Sint64 {
		return true
	}

	return false
}

func sharedIsUint64(typ types.Type) bool {
	if integer, ok := types.Underlying(typ).(types.Integer); ok && integer == types.Uint64 {
		return true
	}

	return false
}

func sharedOneResult(s *types.Syscall) bool {
	return len(s.Results) == 1
}

func sharedParamNamesAndTypes(p types.Parameters) string {
	if len(p) == 0 {
		return ""
	}

	names := make([]string, len(p))
	for i, param := range p {
		names[i] = param.Name.SnakeCase() + ": " + sharedToString(p[i].Type)
	}

	return strings.Join(names, ", ")
}

func sharedParamTypes(p types.Parameters) string {
	if len(p) == 0 {
		return ""
	}

	names := make([]string, len(p))
	for i, param := range p {
		names[i] = sharedToString(param.Type)
	}

	return strings.Join(names, ", ")
}

func sharedToDocs(indent int, d types.Docs) string {
	if len(d) == 0 {
		return ""
	}

	var buf strings.Builder
	buf.WriteString("/// ")
	for _, item := range d {
		switch item := item.(type) {
		case types.Text:
			buf.WriteString(string(item))
		case types.CodeText:
			buf.WriteByte('`')
			buf.WriteString(string(item))
			buf.WriteByte('`')
		case types.ReferenceText:
			buf.WriteString("[`")
			buf.WriteString(sharedToString(item.Type))
			buf.WriteString("`]")
			if ref, ok := types.Underlying(item.Type).(*types.SyscallReference); ok {
				buf.WriteString("(Syscalls::")
				buf.WriteString(ref.Name.PascalCase())
				buf.WriteString(")")
			}
		case types.Newline:
			// Add a blank comment.
			buf.WriteByte('\n')
			for j := 0; j < indent; j++ {
				buf.WriteString("    ")
			}

			buf.WriteString("///\n")
			// Add the next comment.
			for j := 0; j < indent; j++ {
				buf.WriteString("    ")
			}

			buf.WriteString("/// ")
		default:
			panic(fmt.Sprintf("sharedToDocs(%T): unexpected type", item))
		}
	}

	// Add a trailing empty comment.
	buf.WriteByte('\n')
	for j := 0; j < indent; j++ {
		buf.WriteString("    ")
	}

	buf.WriteString("///")

	return buf.String()
}

func sharedToString(t types.Type) string {
	switch t := types.Underlying(t).(type) {
	case types.Integer:
		ss := map[types.Integer]string{
			types.Byte:   "u8",
			types.Uint8:  "u8",
			types.Uint16: "u16",
			types.Uint32: "u32",
			types.Uint64: "u64",
			types.Sint8:  "i8",
			types.Sint16: "i16",
			types.Sint32: "i32",
			types.Sint64: "i64",
		}

		s, ok := ss[t]
		if !ok {
			panic(fmt.Sprintf("unrecognised integer type %d", t))
		}

		return s
	case *types.Pointer:
		if t.Mutable {
			return "*mut " + sharedToString(t.Underlying)
		} else {
			return "*const " + sharedToString(t.Underlying)
		}
	case types.Padding:
		return fmt.Sprintf("[u8; %d]", t)
	case *types.NewInteger:
		return t.Name.PascalCase()
	case *types.Enumeration:
		return t.Name.PascalCase()
	case *types.Bitfield:
		return t.Name.PascalCase()
	case *types.Structure:
		return t.Name.PascalCase()
	case *types.SyscallReference:
		return t.Name.SnakeCase()
	default:
		panic(fmt.Sprintf("sharedToString(%T): unexpected type", t))
	}
}

func sharedToU64(t types.Type) string {
	if integer, ok := types.Underlying(t).(types.Integer); ok && integer == types.Uint64 {
		return ""
	}

	return " as u64"
}
