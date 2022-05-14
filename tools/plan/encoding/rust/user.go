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

	"github.com/ProjectSerenity/firefly/tools/plan/ast"
	"github.com/ProjectSerenity/firefly/tools/plan/types"
)

// GenerateUserCode generates the user implementation, of
// the Plan document to w, using the rustfmt tool at the
// given path.
//
func GenerateUserCode(w io.Writer, file *types.File, rustfmt string) error {
	// Start with the prelude.
	var buf bytes.Buffer
	err := templates.ExecuteTemplate(&buf, fileUserTemplate, file)
	if err != nil {
		return fmt.Errorf("failed to execute %s: %v", fileUserTemplate, err)
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
			template = syscallTemplate
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

func constructor(variable string, varType types.Type) string {
	if ref, ok := varType.(*types.Reference); ok {
		varType = ref.Underlying
	}

	if enum, ok := varType.(*types.Enumeration); ok {
		enumType := enum.Name.PascalCase()
		intType := toString(enum.Type)
		return fmt.Sprintf("%s::from_%s(%s%s).expect(\"invalid %s\")", enumType, intType, variable, fromU64(enum.Type), enumType)
	} else {
		return fmt.Sprintf("%s as %s", variable, toString(varType))
	}
}

func fieldDefinition(f *types.Field) string {
	// We make padding fields private and
	// all other fields public.
	if _, ok := f.Type.(types.Padding); ok {
		return fmt.Sprintf("#[allow(dead_code)]\n    _%s: %s,", f.Name.SnakeCase(), toString(f.Type))
	} else {
		return fmt.Sprintf("pub %s: %s,", f.Name.SnakeCase(), toString(f.Type))
	}
}

func syscallSignature(s *types.Syscall) string {
	switch len(s.Results) {
	case 1:
		return fmt.Sprintf("%s(%s) -> %s", s.Name.SnakeCase(), paramNamesAndTypes(s.Args), toString(s.Results[0].Type))
	case 2:
		return fmt.Sprintf("%s(%s) -> Result<%s>", s.Name.SnakeCase(), paramNamesAndTypes(s.Args), paramTypes(s.Results))
	}

	panic(fmt.Sprintf("syscall has %d results", len(s.Results)))
}

func syscallRecvResults(s *types.Syscall) string {
	switch len(s.Results) {
	case 1:
		return "(result1, _)"
	case 2:
		return "(result1, result2)"
	}

	panic(fmt.Sprintf("syscall has %d results", len(s.Results)))
}
