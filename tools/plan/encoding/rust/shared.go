// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package rust

import (
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/ProjectSerenity/firefly/tools/plan/types"
)

// Use templates to define custom types and functions.

// The templates used to render type definitions
// as Rust code.
//
//go:embed templates/*_rs.txt
var templatesFS embed.FS

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"fieldDefinition":           fieldDefinition,
	"funcSignature":             syscallSignature,
	"traitSignature":            syscallTraitSignature,
	"callSyscallImplementation": callSyscallImplementation,
	"recvResults":               syscallRecvResults,
	"toString":                  toString,
	"toDocs":                    toDocs,
	"fromU64":                   fromU64,
	"toU64":                     toU64,
	"constructor":               constructor,
}).ParseFS(templatesFS, "templates/*"))

const (
	enumerationTemplate = "enumeration_rs.txt"
	structureTemplate   = "structure_rs.txt"
	syscallTemplate     = "syscall_rs.txt"
	fileUserTemplate    = "file_user_rs.txt"
	fileKernelTemplate  = "file_kernel_rs.txt"
)

// Most values are u64, so we can ignore
// conversions to and from it.
func fromU64(t types.Type) string {
	if ref, ok := t.(*types.Reference); ok {
		t = ref.Underlying
	}

	if integer, ok := t.(types.Integer); ok && integer == types.Uint64 {
		return ""
	}

	return fmt.Sprintf(" as %s", toString(t))
}

func toU64(t types.Type) string {
	if ref, ok := t.(*types.Reference); ok {
		t = ref.Underlying
	}

	if integer, ok := t.(types.Integer); ok && integer == types.Uint64 {
		return ""
	}

	return fmt.Sprintf(" as u64")
}

func toString(t types.Type) string {
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
			panic(fmt.Sprintf("unrecognised integer type %d", t))
		}

		return s
	case *types.Pointer:
		if t.Mutable {
			return "*mut " + toString(t.Underlying)
		} else {
			return "*const " + toString(t.Underlying)
		}
	case *types.Reference:
		if _, ok := t.Underlying.(*types.SyscallReference); ok {
			return t.Name.SnakeCase()
		} else {
			return t.Name.PascalCase()
		}
	case types.Padding:
		return fmt.Sprintf("[u8; %d]", t)
	case *types.Enumeration:
		return t.Name.PascalCase()
	case *types.Structure:
		return t.Name.PascalCase()
	default:
		panic(fmt.Sprintf("toString(%T): unexpected type", t))
	}
}

func toDocs(indent int, d types.Docs) string {
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
			buf.WriteString(toString(item.Type))
			buf.WriteString("`]")
		case types.Newline:
			buf.WriteByte('\n')
			for j := 0; j < indent; j++ {
				buf.WriteString("    ")
			}

			buf.WriteString("/// ")
		default:
			panic(fmt.Sprintf("toDocs(%T): unexpected type", item))
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

func paramNamesAndTypes(p types.Parameters) string {
	if len(p) == 0 {
		return ""
	}

	names := make([]string, len(p))
	for i, param := range p {
		names[i] = param.Name.SnakeCase() + ": " + toString(p[i].Type)
	}

	return strings.Join(names, ", ")
}

func paramTypes(p types.Parameters) string {
	if len(p) == 0 {
		return ""
	}

	names := make([]string, len(p))
	for i, param := range p {
		names[i] = toString(param.Type)
	}

	return strings.Join(names, ", ")
}
