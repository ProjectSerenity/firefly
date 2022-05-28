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

	"firefly-os.dev/tools/plan/ast"
	"firefly-os.dev/tools/plan/types"
)

// GenerateUserCode generates the user implementation, of
// the Plan document to w, using the rustfmt tool at the
// given path.
//
func GenerateUserCode(w io.Writer, file *types.File, rustfmt string) error {
	// Start with the prelude.
	var buf bytes.Buffer
	err := userTemplates.ExecuteTemplate(&buf, userFileTemplate, file)
	if err != nil {
		return fmt.Errorf("failed to execute %s: %v", userFileTemplate, err)
	}

	buf.WriteString("\n\n")

	// Then add the enumeration of the syscalls.
	err = userTemplates.ExecuteTemplate(&buf, enumerationTemplate, file.SyscallsEnumeration())
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
		case *types.Enumeration:
			template = enumerationTemplate
		case *types.Bitfield:
			template = bitfieldTemplate
		case *types.Structure:
			template = structureTemplate
		case *types.Syscall:
			template = userSyscallTemplate
		default:
			panic(fmt.Sprintf("unreachable file item type %T", item))
		}

		err := userTemplates.ExecuteTemplate(&buf, template, item)
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
//go:embed templates/*_rs.txt templates/user/*_rs.txt
var userTemplatesFS embed.FS

var userTemplates = template.Must(template.New("").Funcs(template.FuncMap{
	"errorEnumeration":   sharedErrorEnumeration,
	"fromU64":            sharedFromU64,
	"isEnumeration":      sharedIsEnumeration,
	"isPadding":          sharedIsPadding,
	"oneResult":          sharedOneResult,
	"paramNamesAndTypes": sharedParamNamesAndTypes,
	"paramTypes":         sharedParamTypes,
	"toDocs":             userToDocs,
	"toString":           sharedToString,
	"toU64":              sharedToU64,
}).ParseFS(userTemplatesFS, "templates/*_rs.txt", "templates/user/*_rs.txt"))

const (
	userSyscallTemplate = "syscall_rs.txt"
	userFileTemplate    = "file_rs.txt"
)

func userToDocs(indent int, d types.Docs) string {
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
			panic(fmt.Sprintf("userToDocs(%T): unexpected type", item))
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
