// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package html uses templates to render a Plan document as HTML documentation.
//
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
	err := generateItemHTML(filepath.Join(dir, "index.html"), indexTemplate, file, false)
	if err != nil {
		return err
	}

	// Then the sub-folders.
	intDir := filepath.Join(dir, "integers")
	err = mkdir(intDir)
	if err != nil {
		return err
	}

	for _, integer := range file.NewIntegers {
		err = generateItemHTML(filepath.Join(intDir, integer.Name.SnakeCase()+".html"), integerTemplate, integer, true)
		if err != nil {
			return err
		}
	}

	enumDir := filepath.Join(dir, "enumerations")
	err = mkdir(enumDir)
	if err != nil {
		return err
	}

	for _, enumeration := range file.Enumerations {
		err = generateItemHTML(filepath.Join(enumDir, enumeration.Name.SnakeCase()+".html"), enumerationTemplate, enumeration, true)
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
		err = generateItemHTML(filepath.Join(bitsDir, bitfield.Name.SnakeCase()+".html"), bitfieldTemplate, bitfield, true)
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
		err = generateItemHTML(filepath.Join(structDir, structure.Name.SnakeCase()+".html"), structureTemplate, structure, true)
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
		err = generateItemHTML(filepath.Join(syscallDir, syscall.Name.SnakeCase()+".html"), syscallTemplate, syscall, true)
		if err != nil {
			return err
		}
	}

	groupDir := filepath.Join(dir, "groups")
	err = mkdir(groupDir)
	if err != nil {
		return err
	}

	for _, group := range file.Groups {
		err = generateItemHTML(filepath.Join(groupDir, group.Name.SnakeCase()+".html"), groupTemplate, group, true)
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
func generateItemHTML(name, template string, item any, isItem bool) error {
	f, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create %q: %v", name, err)
	}

	if isItem {
		err = templates.ExecuteTemplate(f, itemPrefixTemplate, item)
		if err != nil {
			f.Close()
			return fmt.Errorf("failed to execute template %q for %s: %v", itemPrefixTemplate, name, err)
		}
	}

	err = templates.ExecuteTemplate(f, template, item)
	if err != nil {
		f.Close()
		return fmt.Errorf("failed to execute template %q for %s: %v", template, name, err)
	}

	if isItem {
		err = templates.ExecuteTemplate(f, itemSuffixTemplate, item)
		if err != nil {
			f.Close()
			return fmt.Errorf("failed to execute template %q for %s: %v", itemSuffixTemplate, name, err)
		}
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
	"addOne":               addOne,
	"join":                 strings.Join,
	"toItemClass":          toItemClass,
	"toItemGroups":         toItemGroups,
	"toItemName":           toItemName,
	"toItemTitle":          toItemTitle,
	"toItemUnderlyingType": toItemUnderlyingType,
	"toString":             toString,
	"toDocs":               toDocs,
}).ParseFS(templatesFS, "templates/*_html.txt", "templates/css/*_css.txt"))

const (
	itemPrefixTemplate  = "item-prefix_html.txt"
	itemSuffixTemplate  = "item-suffix_html.txt"
	integerTemplate     = "integer_html.txt"
	enumerationTemplate = "enumeration_html.txt"
	bitfieldTemplate    = "bitfield_html.txt"
	structureTemplate   = "structure_html.txt"
	syscallTemplate     = "syscall_html.txt"
	groupTemplate       = "group_html.txt"
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

func toItemClass(item any) string {
	switch item.(type) {
	case *types.NewInteger:
		return "integer"
	case *types.Enumeration:
		return "enumeration"
	case *types.Bitfield:
		return "bitfield"
	case *types.Structure:
		return "structure"
	case *types.Syscall:
		return "syscall"
	case *types.Group:
		return "group"
	default:
		panic(fmt.Sprintf("toItemClass(%T): unexpected type", item))
	}
}

func toItemGroups(item any) []types.Name {
	switch item := item.(type) {
	case *types.NewInteger:
		return item.Groups
	case *types.Enumeration:
		return item.Groups
	case *types.Bitfield:
		return item.Groups
	case *types.Structure:
		return item.Groups
	case *types.Syscall:
		return item.Groups
	default:
		return nil
	}
}

func toItemName(item any) string {
	switch item := item.(type) {
	case *types.NewInteger:
		return item.Name.Spaced()
	case *types.Enumeration:
		return item.Name.Spaced()
	case *types.Bitfield:
		return item.Name.Spaced()
	case *types.Structure:
		return item.Name.Spaced()
	case *types.Syscall:
		return item.Name.Spaced()
	case *types.Group:
		return item.Name.Spaced()
	default:
		panic(fmt.Sprintf("toItemName(%T): unexpected type", item))
	}
}

func toItemTitle(item any) string {
	switch item.(type) {
	case *types.NewInteger:
		return "Integer"
	case *types.Enumeration:
		return "Enumeration"
	case *types.Bitfield:
		return "Bitfield"
	case *types.Structure:
		return "Structure"
	case *types.Syscall:
		return "Syscall"
	case *types.Group:
		return "Group"
	default:
		panic(fmt.Sprintf("toItemTitle(%T): unexpected type", item))
	}
}

func toItemUnderlyingType(item any) template.HTML {
	const (
		prefix = ` (<code class="inline-code">`
		suffix = `</code>)`
	)

	switch item := item.(type) {
	case *types.NewInteger:
		return prefix + toString(item.Type) + suffix
	case *types.Enumeration:
		return prefix + toString(item.Type) + suffix
	case *types.Bitfield:
		return prefix + toString(item.Type) + suffix
	default:
		return ""
	}
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
