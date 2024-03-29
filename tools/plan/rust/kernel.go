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
	"strings"
	"text/template"

	"firefly-os.dev/tools/plan/types"
)

// GenerateKernelCode generates the kernel's implementation,
// of the Plan document to w, using the rustfmt tool at the
// given path.
func GenerateKernelCode(w io.Writer, file *types.File, arch types.Arch, rustfmt string) error {
	// Start with the prelude.
	var buf bytes.Buffer
	err := kernelTemplates.ExecuteTemplate(&buf, kernelFileTemplate, file)
	if err != nil {
		return fmt.Errorf("failed to execute %s: %v", kernelFileTemplate, err)
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
//go:embed templates/kernel_*_rs.txt
var kernelTemplatesFS embed.FS

var kernelTemplates = template.Must(template.New("").Funcs(template.FuncMap{
	"addOne":             sharedAddOne,
	"errorEnumeration":   sharedErrorEnumeration,
	"fromU64":            sharedFromU64,
	"isEnumeration":      sharedIsEnumeration,
	"isInteger":          sharedIsInteger,
	"isNewInteger":       sharedIsNewInteger,
	"isPadding":          sharedIsPadding,
	"isPointer":          sharedIsPointer,
	"isSigned":           sharedIsSigned,
	"isSint64":           sharedIsSint64,
	"isUint64":           sharedIsUint64,
	"nonErrorResult":     kernelNonErrorResult,
	"oneResult":          sharedOneResult,
	"paramNamesAndTypes": sharedParamNamesAndTypes,
	"paramTypes":         sharedParamTypes,
	"toDocs":             kernelToDocs,
	"toString":           sharedToString,
	"toU64":              sharedToU64,
	"unusedArgs":         kernelUnusedArgs,
}).ParseFS(kernelTemplatesFS, "templates/kernel_*_rs.txt"))

const (
	kernelFileTemplate = "kernel_file_rs.txt"
)

func kernelNonErrorResult(s *types.Syscall, variable string) string {
	resultType := types.Underlying(s.Results[0].Type)
	switch t := resultType.(type) {
	case *types.Enumeration:
		return fmt.Sprintf("%s.as_%s()%s", variable, sharedToString(t.Type), sharedToU64(t.Type))
	case *types.NewInteger:
		return fmt.Sprintf("%s.0%s", variable, sharedToU64(t.Type))
	default:
		return variable + sharedToU64(resultType)
	}
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

// kernelUnusedArgs returns a (possibly empty)
// slice of argument numbers that are not used
// by a syscall.
//
// For example, if a syscall takes two arguments,
// then kernelUnusedArgs would return a slice
// containing the integers 3, 4, 5, 6 to represent
// the four arguments it does not use.
func kernelUnusedArgs(params types.Parameters) []int {
	if len(params) >= 6 {
		return nil
	}

	out := make([]int, 6-len(params))
	for i := range out {
		out[i] = len(params) + i + 1
	}

	return out
}
