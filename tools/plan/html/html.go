// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package html

import (
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"firefly-os.dev/tools/plan/types"
)

const dirMode = 0777

func mkdir(dir string) error {
	err := os.MkdirAll(dir, dirMode)
	if err != nil {
		return fmt.Errorf("failed to create directory %q: %v", dir, err)
	}

	return nil
}

// GenerateDocs produces HTML documentation for
// the given Plan document, writing HTML files
// to the given directory.
//
func GenerateDocs(dir string, file *types.File) error {
	// Start with the index page.
	err := generateItemHTML(filepath.Join(dir, "index.html"), indexTemplate, file)
	if err != nil {
		return err
	}

	// Then the sub-folders.
	enumDir := filepath.Join(dir, "enumerations")
	err = mkdir(enumDir)
	if err != nil {
		return err
	}

	for _, enumeration := range file.Enumerations {
		err = generateItemHTML(filepath.Join(enumDir, enumeration.Name.SnakeCase()+".html"), enumerationTemplate, enumeration)
		if err != nil {
			return err
		}
	}

	bitsDir := filepath.Join(dir, "bitfields")
	err = mkdir(bitsDir)
	if err != nil {
		return err
	}

	for _, bitfield := range file.Bitfields {
		err = generateItemHTML(filepath.Join(bitsDir, bitfield.Name.SnakeCase()+".html"), bitfieldTemplate, bitfield)
		if err != nil {
			return err
		}
	}

	structDir := filepath.Join(dir, "structures")
	err = mkdir(structDir)
	if err != nil {
		return err
	}

	for _, structure := range file.Structures {
		err = generateItemHTML(filepath.Join(structDir, structure.Name.SnakeCase()+".html"), structureTemplate, structure)
		if err != nil {
			return err
		}
	}

	syscallDir := filepath.Join(dir, "syscalls")
	err = mkdir(syscallDir)
	if err != nil {
		return err
	}

	for _, syscall := range file.Syscalls {
		err = generateItemHTML(filepath.Join(syscallDir, syscall.Name.SnakeCase()+".html"), syscallTemplate, syscall)
		if err != nil {
			return err
		}
	}

	return nil
}

// generateItemHTML produces HTML content for
// the item and writes it to a new file with
// the given name.
//
func generateItemHTML(name, template string, item any) error {
	f, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create %q: %v", name, err)
	}

	err = templates.ExecuteTemplate(f, template, item)
	if err != nil {
		f.Close()
		return fmt.Errorf("failed to execute template %q for %s: %v", template, name, err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("failed to close %s: %v", name, err)
	}

	return nil
}

// Use templates to define custom types and functions.

// The templates used to render type definitions
// as Rust code.
//
//go:embed templates/*_html.txt templates/css/*_css.txt
var templatesFS embed.FS

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"addOne":   addOne,
	"toString": toString,
	"toDocs":   toDocs,
}).ParseFS(templatesFS, "templates/*_html.txt", "templates/css/*_css.txt"))

const (
	enumerationTemplate = "enumeration_html.txt"
	bitfieldTemplate    = "bitfield_html.txt"
	structureTemplate   = "structure_html.txt"
	syscallTemplate     = "syscall_html.txt"
	indexTemplate       = "index_html.txt"
)

func addOne(i int) int {
	return i + 1
}

func plainDocs(docs types.Docs) string {
	var out string
	for _, part := range docs {
		switch text := part.(type) {
		case types.Text:
			out += string(text)
		case types.CodeText:
			out += string(text)
		case types.Newline:
			return out
		default:
			panic(fmt.Sprintf("unsupported documentation item %T in docs", part))
		}
	}

	return out
}

func toString(t types.Type) template.HTML {
	switch t := types.Underlying(t).(type) {
	case types.Integer:
		return template.HTML(fmt.Sprintf(`<span title="%s">%s</span>`, plainDocs(t.Docs()), t))
	case *types.Pointer:
		if t.Mutable {
			return `<span title="A pointer to writable memory.">*mutable</span> ` + toString(t.Underlying)
		} else {
			return `<span title="A pointer to readable memory.">*constant</span> ` + toString(t.Underlying)
		}
	case types.Padding:
		return template.HTML(fmt.Sprintf("%d-byte padding", t))
	case *types.Enumeration:
		return template.HTML(fmt.Sprintf(`<a href="../enumerations/%s.html" class="enumeration">%s</a>`, t.Name.SnakeCase(), t.Name.Spaced()))
	case *types.Bitfield:
		return template.HTML(fmt.Sprintf(`<a href="../bitfields/%s.html" class="bitfield">%s</a>`, t.Name.SnakeCase(), t.Name.Spaced()))
	case *types.Structure:
		return template.HTML(fmt.Sprintf(`<a href="../structures/%s.html" class="structure">%s</a>`, t.Name.SnakeCase(), t.Name.Spaced()))
	case *types.SyscallReference:
		return template.HTML(fmt.Sprintf(`<a href="../syscalls/%s.html" class="syscall">%s</a>`, t.Name.SnakeCase(), t.Name.Spaced()))
	default:
		panic(fmt.Sprintf("toString(%T): unexpected type", t))
	}
}

func toDocs(indent int, d types.Docs) template.HTML {
	var buf strings.Builder
	buf.WriteString("<p>")
	for _, item := range d {
		switch item := item.(type) {
		case types.Text:
			buf.WriteString(template.HTMLEscapeString(string(item)))
		case types.CodeText:
			buf.WriteString(`<code class="inline-code">`)
			buf.WriteString(template.HTMLEscapeString(string(item)))
			buf.WriteString(`</code>`)
		case types.ReferenceText:
			buf.WriteString(`<code class="inline-code">`)
			buf.WriteString(string(toString(item.Type)))
			buf.WriteString(`</code>`)
		case types.Newline:
			buf.WriteString("</p>\n")
			for j := 0; j < indent; j++ {
				buf.WriteByte('\t')
			}

			buf.WriteString("<p>")
		default:
			panic(fmt.Sprintf("toDocs(%T): unexpected type", item))
		}
	}

	buf.WriteString("</p>")

	return template.HTML(buf.String())
}
