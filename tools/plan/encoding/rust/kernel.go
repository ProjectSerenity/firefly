// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package rust

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

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
	err := templates.ExecuteTemplate(&buf, fileKernelTemplate, file)
	if err != nil {
		return fmt.Errorf("failed to execute %s: %v", fileKernelTemplate, err)
	}

	buf.WriteString("\n\n")

	// Then add the enumeration of the syscalls.
	err = templates.ExecuteTemplate(&buf, enumerationTemplate, file.SyscallsEnumeration())
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

		err := templates.ExecuteTemplate(&buf, template, item)
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

func syscallTraitSignature(s *types.Syscall) string {
	reg := "_registers: *mut SavedRegisters"
	switch len(s.Results) {
	case 1:
		if len(s.Args) == 0 {
			return fmt.Sprintf("%s(%s) -> %s", s.Name.SnakeCase(), reg, toString(s.Results[0].Type))
		} else {
			return fmt.Sprintf("%s(%s, %s) -> %s", s.Name.SnakeCase(), reg, paramNamesAndTypes(s.Args), toString(s.Results[0].Type))
		}
	case 2:
		if len(s.Args) == 0 {
			return fmt.Sprintf("%s(%s) -> Result<%s>", s.Name.SnakeCase(), reg, paramTypes(s.Results))
		} else {
			return fmt.Sprintf("%s(%s, %s) -> Result<%s>", s.Name.SnakeCase(), reg, paramNamesAndTypes(s.Args), paramTypes(s.Results))
		}
	}

	panic(fmt.Sprintf("syscall has %d results", len(s.Results)))
}

func callSyscallImplementation(s *types.Syscall) string {
	const indent = "        "
	var buf strings.Builder
	for i, arg := range s.Args {
		fmt.Fprintf(&buf, "let param%d: %s = ", i+1, toString(arg.Type))
		argType := arg.Type
		if ref, ok := argType.(*types.Reference); ok {
			argType = ref.Underlying
		}

		if enum, ok := argType.(*types.Enumeration); ok {
			fmt.Fprintf(&buf, "match %s::from_%s(arg%d%s) {", enum.Name.PascalCase(), toString(enum.Type), i+1, fromU64(enum.Type))
			buf.WriteByte('\n')
			buf.WriteString(indent)
			buf.WriteString("    Some(value) => value,")
			buf.WriteByte('\n')
			buf.WriteString(indent)
			if len(s.Results) == 1 {
				enum := s.Results[0].Type.(*types.Reference).Underlying.(*types.Enumeration)
				fmt.Fprintf(&buf, "    None => return SyscallResults{ value: %s::IllegalParameter.as_%s()%s, error: 0 },",
					enum.Name.PascalCase(), toString(enum.Type), toU64(enum.Type))
			} else {
				enum := s.Results[1].Type.(*types.Reference).Underlying.(*types.Enumeration)
				fmt.Fprintf(&buf, "    None => return SyscallResults{ value: 0, error: %s::IllegalParameter.as_%s()%s },",
					enum.Name.PascalCase(), toString(enum.Type), toU64(enum.Type))
			}
			buf.WriteByte('\n')
			buf.WriteString(indent)
			buf.WriteString("};")
		} else {
			fmt.Fprintf(&buf, "arg%d%s;", i+1, fromU64(argType))
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
		resultType := s.Results[0].Type
		if ref, ok := resultType.(*types.Reference); ok {
			resultType = ref.Underlying
		}

		enum := resultType.(*types.Enumeration)
		noError := "error: Error::NoError.as_u64()"
		fmt.Fprintf(&buf, "SyscallResults { value: result.as_%s()%s, %s }", toString(enum.Type), toU64(enum.Type), noError)
	case 2:
		buf.WriteString("match result {\n")
		buf.WriteString(indent)
		buf.WriteString("    Ok(value) => ")

		resultType := s.Results[0].Type
		if ref, ok := resultType.(*types.Reference); ok {
			resultType = ref.Underlying
		}

		enum := s.Results[1].Type.(*types.Reference).Underlying.(*types.Enumeration)
		noError := fmt.Sprintf("error: %s::NoError.as_%s()%s", enum.Name.PascalCase(), toString(enum.Type), toU64(enum.Type))
		if enum, ok := resultType.(*types.Enumeration); ok {
			fmt.Fprintf(&buf, "SyscallResults { value: value.as_%s()%s, %s },\n", toString(enum.Type), toU64(enum.Type), noError)
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
		enum = resultType.(*types.Enumeration)
		fmt.Fprintf(&buf, "SyscallResults { %s, error: error.as_%s()%s },\n", noValue, toString(enum.Type), toU64(enum.Type))
		buf.WriteString(indent)
		buf.WriteByte('}')
	}

	return buf.String()
}
