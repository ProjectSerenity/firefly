// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

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

	"github.com/ProjectSerenity/firefly/tools/plan/ast"
	"github.com/ProjectSerenity/firefly/tools/plan/types"
)

// RustUserspace translates the file to Rust for userspace,
// writing the result to w.
//
func RustUserspace(w io.Writer, file *types.File, rustfmt string) error {
	// Start with the prelude.
	var buf bytes.Buffer
	err := rustTemplates.ExecuteTemplate(&buf, rustFileUserTemplate, file)
	if err != nil {
		return fmt.Errorf("failed to execute %s: %v", rustFileUserTemplate, err)
	}

	buf.WriteString("\n\n")

	// Then add the enumeration of the syscalls.
	err = rustTemplates.ExecuteTemplate(&buf, rustEnumerationTemplate, file.SyscallsEnumeration())
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

	numItems := len(file.Enumerations) + len(file.Structures) + len(file.Syscalls)
	items := make([]ast.Node, 0, numItems)
	for _, enumeration := range file.Enumerations {
		items = append(items, enumeration)
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
		case *types.Enumeration:
			template = rustEnumerationTemplate
		case *types.Structure:
			template = rustStructureTemplate
		case *types.Syscall:
			template = rustSyscallTemplate
		default:
			panic(fmt.Sprintf("unreachable file item type %T", item))
		}

		err := rustTemplates.ExecuteTemplate(&buf, template, item)
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

// RustKernelspace translates the file to Rust for kernelspace,
// writing the result to w.
//
func RustKernelspace(w io.Writer, file *types.File, rustfmt string) error {
	// Start with the prelude.
	var buf bytes.Buffer
	err := rustTemplates.ExecuteTemplate(&buf, rustFileKernelTemplate, file)
	if err != nil {
		return fmt.Errorf("failed to execute %s: %v", rustFileKernelTemplate, err)
	}

	buf.WriteString("\n\n")

	// Then add the enumeration of the syscalls.
	err = rustTemplates.ExecuteTemplate(&buf, rustEnumerationTemplate, file.SyscallsEnumeration())
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

	numItems := len(file.Enumerations) + len(file.Structures) + len(file.Syscalls)
	items := make([]ast.Node, 0, numItems)
	for _, enumeration := range file.Enumerations {
		items = append(items, enumeration)
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
		case *types.Enumeration:
			template = rustEnumerationTemplate
		case *types.Structure:
			template = rustStructureTemplate
		case *types.Syscall:
			// We skip syscall definitions.
			continue
		default:
			panic(fmt.Sprintf("unreachable file item type %T", item))
		}

		err := rustTemplates.ExecuteTemplate(&buf, template, item)
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

// Use templates to define custom types and functions.

// The templates used to render type definitions
// as Rust code.
//
//go:embed templates/*_rs.txt
var rustTemplatesFS embed.FS

var rustTemplates = template.Must(template.New("").Funcs(template.FuncMap{
	"fieldDefinition":           rustFieldDefinition,
	"funcSignature":             rustSyscallSignature,
	"traitSignature":            rustSyscallTraitSignature,
	"callSyscallImplementation": rustCallSyscallImplementation,
	"recvResults":               rustSyscallRecvResults,
	"toString":                  rustString,
	"toDocs":                    rustDocs,
	"constructor":               rustConstructor,
}).ParseFS(rustTemplatesFS, "templates/*"))

const (
	rustEnumerationTemplate = "enumeration_rs.txt"
	rustStructureTemplate   = "structure_rs.txt"
	rustSyscallTemplate     = "syscall_rs.txt"
	rustFileUserTemplate    = "file_user_rs.txt"
	rustFileKernelTemplate  = "file_kernel_rs.txt"
)

func rustString(t types.Type) string {
	switch t := t.(type) {
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
			panic(fmt.Sprintf("unrecognised basic type %d", t))
		}

		return s
	case *types.Pointer:
		if t.Mutable {
			return "*mut " + rustString(t.Underlying)
		} else {
			return "*const " + rustString(t.Underlying)
		}
	case *types.Reference:
		return t.Name.PascalCase()
	case types.Padding:
		return fmt.Sprintf("[u8; %d]", t)
	case *types.Enumeration:
		return t.Name.PascalCase()
	case *types.Structure:
		return t.Name.PascalCase()
	default:
		panic(fmt.Sprintf("rustString(%T): unexpected type", t))
	}
}

func rustDocs(indent int, d types.Docs) string {
	var buf strings.Builder
	for _, item := range d {
		buf.WriteString("/// ")
		buf.WriteString(item)
		buf.WriteByte('\n')
		for j := 0; j < indent; j++ {
			buf.WriteString("    ")
		}
	}

	buf.WriteString("///")

	return buf.String()
}

func rustConstructor(variable string, varType types.Type) string {
	if ref, ok := varType.(*types.Reference); ok {
		varType = ref.Underlying
	}

	if enum, ok := varType.(*types.Enumeration); ok {
		enumType := enum.Name.PascalCase()
		intType := rustString(enum.Type)
		return fmt.Sprintf("%s::from_%s(%s as %s).expect(\"invalid %s\")", enumType, intType, variable, intType, enumType)
	} else {
		return fmt.Sprintf("%s as %s", variable, rustString(varType))
	}
}

func rustParamNames(p types.Parameters) string {
	if len(p) == 0 {
		return ""
	}

	names := make([]string, len(p))
	for i, param := range p {
		names[i] = param.Name.SnakeCase()
	}

	return strings.Join(names, ", ")
}

func rustParamNamesAndTypes(p types.Parameters) string {
	if len(p) == 0 {
		return ""
	}

	names := make([]string, len(p))
	for i, param := range p {
		names[i] = param.Name.SnakeCase() + ": " + rustString(p[i].Type)
	}

	return strings.Join(names, ", ")
}

func rustParamTypes(p types.Parameters) string {
	if len(p) == 0 {
		return ""
	}

	names := make([]string, len(p))
	for i, param := range p {
		names[i] = rustString(param.Type)
	}

	return strings.Join(names, ", ")
}

func rustFieldDefinition(f *types.Field) string {
	// We make padding fields private and
	// all other fields public.
	if _, ok := f.Type.(types.Padding); ok {
		return fmt.Sprintf("#[allow(dead_code)]\n    _%s: %s,", f.Name.SnakeCase(), rustString(f.Type))
	} else {
		return fmt.Sprintf("pub %s: %s,", f.Name.SnakeCase(), rustString(f.Type))
	}
}

func rustSyscallSignature(s *types.Syscall) string {
	switch len(s.Results) {
	case 0:
		return fmt.Sprintf("%s(%s)", s.Name.SnakeCase(), rustParamNamesAndTypes(s.Args))
	case 1:
		return fmt.Sprintf("%s(%s) -> %s", s.Name.SnakeCase(), rustParamNamesAndTypes(s.Args), rustString(s.Results[0].Type))
	case 2:
		return fmt.Sprintf("%s(%s) -> Result<%s>", s.Name.SnakeCase(), rustParamNamesAndTypes(s.Args), rustParamTypes(s.Results))
	}

	panic(fmt.Sprintf("syscall has %d results", len(s.Results)))
}

func rustSyscallTraitSignature(s *types.Syscall) string {
	reg := "_registers: *mut SavedRegisters"
	switch len(s.Results) {
	case 0:
		if len(s.Args) == 0 {
			return fmt.Sprintf("%s(%s)", s.Name.SnakeCase(), reg)
		} else {
			return fmt.Sprintf("%s(%s, %s)", s.Name.SnakeCase(), reg, rustParamNamesAndTypes(s.Args))
		}
	case 1:
		if len(s.Args) == 0 {
			return fmt.Sprintf("%s(%s) -> %s", s.Name.SnakeCase(), reg, rustString(s.Results[0].Type))
		} else {
			return fmt.Sprintf("%s(%s, %s) -> %s", s.Name.SnakeCase(), reg, rustParamNamesAndTypes(s.Args), rustString(s.Results[0].Type))
		}
	case 2:
		if len(s.Args) == 0 {
			return fmt.Sprintf("%s(%s) -> Result<%s>", s.Name.SnakeCase(), reg, rustParamTypes(s.Results))
		} else {
			return fmt.Sprintf("%s(%s, %s) -> Result<%s>", s.Name.SnakeCase(), reg, rustParamNamesAndTypes(s.Args), rustParamTypes(s.Results))
		}
	}

	panic(fmt.Sprintf("syscall has %d results", len(s.Results)))
}

func rustCallSyscallImplementation(s *types.Syscall) string {
	// Most values are u64, so we can ignore
	// conversions to and from it.
	fromU64 := func(t types.Type) string {
		if ref, ok := t.(*types.Reference); ok {
			t = ref.Underlying
		}

		if integer, ok := t.(types.Integer); ok && integer == types.Uint64 {
			return ""
		}

		return fmt.Sprintf(" as %s", rustString(t))
	}

	toU64 := func(t types.Type) string {
		if ref, ok := t.(*types.Reference); ok {
			t = ref.Underlying
		}

		if integer, ok := t.(types.Integer); ok && integer == types.Uint64 {
			return ""
		}

		return fmt.Sprintf(" as u64")
	}

	const indent = "        "
	var buf strings.Builder
	for i, arg := range s.Args {
		fmt.Fprintf(&buf, "let param%d: %s = ", i+1, rustString(arg.Type))
		argType := arg.Type
		if ref, ok := argType.(*types.Reference); ok {
			argType = ref.Underlying
		}

		if enum, ok := argType.(*types.Enumeration); ok {
			fmt.Fprintf(&buf, "match %s::from_%s(arg%d%s) {", enum.Name.PascalCase(), rustString(enum.Type), i+1, fromU64(enum.Type))
			buf.WriteByte('\n')
			buf.WriteString(indent)
			buf.WriteString("    Some(value) => value,")
			buf.WriteByte('\n')
			buf.WriteString(indent)
			buf.WriteString("    None => return SyscallResults{ value: 0, error: Error::IllegalParameter.as_u64() },")
			buf.WriteByte('\n')
			buf.WriteString(indent)
			buf.WriteString("};")
		} else {
			fmt.Fprintf(&buf, "arg%d%s;", i+1, fromU64(argType))
		}

		buf.WriteByte('\n')
		buf.WriteString(indent)
	}

	switch len(s.Results) {
	case 0:
	case 1, 2:
		buf.WriteString("let result = ")
	}

	buf.WriteString("<FireflyABI as SyscallABI>::")
	buf.WriteString(s.Name.SnakeCase())
	buf.WriteString("(registers")
	for i := range s.Args {
		buf.WriteString(", param")
		buf.WriteByte('1' + byte(i))
	}
	buf.WriteString(");\n")
	buf.WriteString(indent)

	switch len(s.Results) {
	case 0:
		buf.WriteString("SyscallResults { value: 0, error: Error::NoError.as_u64() }")
	case 1:
		resultType := s.Results[0].Type
		if ref, ok := resultType.(*types.Reference); ok {
			resultType = ref.Underlying
		}

		noError := "error: Error::NoError.as_u64()"
		if enum, ok := resultType.(*types.Enumeration); ok {
			fmt.Fprintf(&buf, "SyscallResults { value: result.as_%s()%s, %s }", rustString(enum.Type), toU64(enum.Type), noError)
		} else {
			fmt.Fprintf(&buf, "SyscallResults { value: result%s, %s }", toU64(resultType), noError)
		}
	case 2:
		buf.WriteString("match result {\n")
		buf.WriteString(indent)
		buf.WriteString("    Ok(value) => ")

		resultType := s.Results[0].Type
		if ref, ok := resultType.(*types.Reference); ok {
			resultType = ref.Underlying
		}

		noError := "error: Error::NoError.as_u64()"
		if enum, ok := resultType.(*types.Enumeration); ok {
			fmt.Fprintf(&buf, "SyscallResults { value: value.as_%s()%s, %s },\n", rustString(enum.Type), toU64(enum.Type), noError)
		} else if integer, ok := resultType.(types.Integer); ok && integer == types.Uint64 {
			fmt.Fprintf(&buf, "SyscallResults { value, %s },\n", noError)
		} else {
			fmt.Fprintf(&buf, "SyscallResults { value: value%s, %s },\n", toU64(resultType), noError)
		}

		resultType = s.Results[1].Type
		if ref, ok := resultType.(*types.Reference); ok {
			resultType = ref.Underlying
		}

		buf.WriteString(indent)
		buf.WriteString("    Err(error) => ")
		noValue := "value: 0"
		if enum, ok := resultType.(*types.Enumeration); ok {
			fmt.Fprintf(&buf, "SyscallResults { %s, error: error.as_%s()%s },\n", noValue, rustString(enum.Type), toU64(enum.Type))
		} else if integer, ok := resultType.(types.Integer); ok && integer == types.Uint64 {
			fmt.Fprintf(&buf, "SyscallResults { %s, error },\n", noValue)
		} else {
			fmt.Fprintf(&buf, "SyscallResults { %s, error: error%s },\n", noValue, toU64(resultType))
		}

		buf.WriteString(indent)
		buf.WriteByte('}')
	}

	return buf.String()
}

func rustSyscallRecvResults(s *types.Syscall) string {
	switch len(s.Results) {
	case 0:
		return "_"
	case 1:
		return "(result1, _)"
	case 2:
		return "(result1, result2)"
	}

	panic(fmt.Sprintf("syscall has %d results", len(s.Results)))
}
