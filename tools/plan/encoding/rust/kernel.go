// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

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

	"github.com/ProjectSerenity/firefly/tools/plan/ast"
	"github.com/ProjectSerenity/firefly/tools/plan/types"
)

// GenerateKernelCode generates the kernel's implementation,
// of the Plan document to w, using the rustfmt tool at the
// given path.
//
func GenerateKernelCode(w io.Writer, file *types.File, rustfmt string) error {
	// Start with the prelude.
	var buf bytes.Buffer
	err := kernelTemplates.ExecuteTemplate(&buf, kernelFileTemplate, file)
	if err != nil {
		return fmt.Errorf("failed to execute %s: %v", kernelFileTemplate, err)
	}

	buf.WriteString("\n\n")

	// Then add the enumeration of the syscalls.
	err = kernelTemplates.ExecuteTemplate(&buf, enumerationTemplate, file.SyscallsEnumeration())
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
			template = enumerationTemplate
		case *types.Structure:
			template = structureTemplate
		case *types.Syscall:
			// We skip syscall definitions.
			continue
		default:
			panic(fmt.Sprintf("unreachable file item type %T", item))
		}

		err := kernelTemplates.ExecuteTemplate(&buf, template, item)
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
//go:embed templates/*_rs.txt templates/kernel/*_rs.txt
var kernelTemplatesFS embed.FS

var kernelTemplates = template.Must(template.New("").Funcs(template.FuncMap{
	"fromU64":        sharedFromU64,
	"handleSyscall":  kernelHandleSyscall,
	"isEnumeration":  sharedIsEnumeration,
	"isPadding":      sharedIsPadding,
	"toDocs":         kernelToDocs,
	"toString":       sharedToString,
	"toU64":          sharedToU64,
	"traitSignature": kernelTraitSignature,
}).ParseFS(kernelTemplatesFS, "templates/*_rs.txt", "templates/kernel/*_rs.txt"))

const (
	kernelFileTemplate = "file_rs.txt"
)

func kernelHandleSyscall(s *types.Syscall) string {
	const indent = "        "
	var buf strings.Builder
	for i, arg := range s.Args {
		fmt.Fprintf(&buf, "let param%d: %s = ", i+1, sharedToString(arg.Type))
		argType := types.Underlying(arg.Type)
		if enum, ok := argType.(*types.Enumeration); ok {
			fmt.Fprintf(&buf, "match %s::from_%s(arg%d%s) {", enum.Name.PascalCase(), sharedToString(enum.Type), i+1, sharedFromU64(enum.Type))
			buf.WriteByte('\n')
			buf.WriteString(indent)
			buf.WriteString("    Some(value) => value,")
			buf.WriteByte('\n')
			buf.WriteString(indent)
			if len(s.Results) == 1 {
				enum := s.Results[0].Type.(*types.Reference).Underlying.(*types.Enumeration)
				fmt.Fprintf(&buf, "    None => return SyscallResults{ value: %s::IllegalParameter.as_%s()%s, error: 0 },",
					enum.Name.PascalCase(), sharedToString(enum.Type), sharedToU64(enum.Type))
			} else {
				enum := s.Results[1].Type.(*types.Reference).Underlying.(*types.Enumeration)
				fmt.Fprintf(&buf, "    None => return SyscallResults{ value: 0, error: %s::IllegalParameter.as_%s()%s },",
					enum.Name.PascalCase(), sharedToString(enum.Type), sharedToU64(enum.Type))
			}
			buf.WriteByte('\n')
			buf.WriteString(indent)
			buf.WriteString("};")
		} else {
			fmt.Fprintf(&buf, "arg%d%s;", i+1, sharedFromU64(argType))
		}

		buf.WriteByte('\n')
		buf.WriteString(indent)
	}

	buf.WriteString("let result = <FireflyABI as SyscallABI>::")
	buf.WriteString(s.Name.SnakeCase())
	buf.WriteString("(registers")
	for i := range s.Args {
		buf.WriteString(", param")
		buf.WriteByte('1' + byte(i))
	}
	buf.WriteString(");\n")
	buf.WriteString(indent)

	switch len(s.Results) {
	case 1:
		resultType := types.Underlying(s.Results[0].Type)
		enum := resultType.(*types.Enumeration)
		noError := "error: Error::NoError.as_u64()"
		fmt.Fprintf(&buf, "SyscallResults { value: result.as_%s()%s, %s }", sharedToString(enum.Type), sharedToU64(enum.Type), noError)
	case 2:
		buf.WriteString("match result {\n")
		buf.WriteString(indent)
		buf.WriteString("    Ok(value) => ")

		resultType := types.Underlying(s.Results[0].Type)
		enum := s.Results[1].Type.(*types.Reference).Underlying.(*types.Enumeration)
		noError := fmt.Sprintf("error: %s::NoError.as_%s()%s", enum.Name.PascalCase(), sharedToString(enum.Type), sharedToU64(enum.Type))
		if enum, ok := resultType.(*types.Enumeration); ok {
			fmt.Fprintf(&buf, "SyscallResults { value: value.as_%s()%s, %s },\n", sharedToString(enum.Type), sharedToU64(enum.Type), noError)
		} else if integer, ok := resultType.(types.Integer); ok && integer == types.Uint64 {
			fmt.Fprintf(&buf, "SyscallResults { value, %s },\n", noError)
		} else {
			fmt.Fprintf(&buf, "SyscallResults { value: value%s, %s },\n", sharedToU64(resultType), noError)
		}

		resultType = types.Underlying(s.Results[1].Type)
		buf.WriteString(indent)
		buf.WriteString("    Err(error) => ")
		noValue := "value: 0"
		enum = resultType.(*types.Enumeration)
		fmt.Fprintf(&buf, "SyscallResults { %s, error: error.as_%s()%s },\n", noValue, sharedToString(enum.Type), sharedToU64(enum.Type))
		buf.WriteString(indent)
		buf.WriteByte('}')
	}

	return buf.String()
}

func kernelToDocs(indent int, d types.Docs) string {
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
			if _, ok := types.Underlying(item.Type).(*types.SyscallReference); ok {
				buf.WriteString("(SyscallABI::")
				buf.WriteString(sharedToString(item.Type))
				buf.WriteString(")")
			}
		case types.Newline:
			buf.WriteByte('\n')
			for j := 0; j < indent; j++ {
				buf.WriteString("    ")
			}

			buf.WriteString("/// ")
		default:
			panic(fmt.Sprintf("kernelToDocs(%T): unexpected type", item))
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

func kernelTraitSignature(s *types.Syscall) string {
	reg := "_registers: *mut SavedRegisters"
	switch len(s.Results) {
	case 1:
		if len(s.Args) == 0 {
			return fmt.Sprintf("%s(%s) -> %s", s.Name.SnakeCase(), reg, sharedToString(s.Results[0].Type))
		} else {
			return fmt.Sprintf("%s(%s, %s) -> %s", s.Name.SnakeCase(), reg, sharedParamNamesAndTypes(s.Args), sharedToString(s.Results[0].Type))
		}
	case 2:
		if len(s.Args) == 0 {
			return fmt.Sprintf("%s(%s) -> Result<%s>", s.Name.SnakeCase(), reg, sharedParamTypes(s.Results))
		} else {
			return fmt.Sprintf("%s(%s, %s) -> Result<%s>", s.Name.SnakeCase(), reg, sharedParamNamesAndTypes(s.Args), sharedParamTypes(s.Results))
		}
	}

	panic(fmt.Sprintf("syscall has %d results", len(s.Results)))
}
