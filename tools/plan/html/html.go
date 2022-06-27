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
	type Item struct {
		Name string
		Item any
	}

	type Type struct {
		Name     string
		Template string
		Items    []Item
	}

	var types []Type

	arrayItems := make([]Item, len(file.Arrays))
	for i, array := range file.Arrays {
		arrayItems[i] = Item{
			Name: array.Name.SnakeCase(),
			Item: array,
		}
	}

	types = append(types, Type{
		Name:     "arrays",
		Template: arrayTemplate,
		Items:    arrayItems,
	})

	bitfieldItems := make([]Item, len(file.Bitfields))
	for i, bitfield := range file.Bitfields {
		bitfieldItems[i] = Item{
			Name: bitfield.Name.SnakeCase(),
			Item: bitfield,
		}
	}

	types = append(types, Type{
		Name:     "bitfields",
		Template: bitfieldTemplate,
		Items:    bitfieldItems,
	})

	enumerationItems := make([]Item, len(file.Enumerations))
	for i, enumeration := range file.Enumerations {
		enumerationItems[i] = Item{
			Name: enumeration.Name.SnakeCase(),
			Item: enumeration,
		}
	}

	types = append(types, Type{
		Name:     "enumerations",
		Template: enumerationTemplate,
		Items:    enumerationItems,
	})

	integerItems := make([]Item, len(file.NewIntegers))
	for i, integer := range file.NewIntegers {
		integerItems[i] = Item{
			Name: integer.Name.SnakeCase(),
			Item: integer,
		}
	}

	types = append(types, Type{
		Name:     "integers",
		Template: integerTemplate,
		Items:    integerItems,
	})

	structureItems := make([]Item, len(file.Structures))
	for i, structure := range file.Structures {
		structureItems[i] = Item{
			Name: structure.Name.SnakeCase(),
			Item: structure,
		}
	}

	types = append(types, Type{
		Name:     "structures",
		Template: structureTemplate,
		Items:    structureItems,
	})

	syscallItems := make([]Item, len(file.Syscalls))
	for i, syscall := range file.Syscalls {
		syscallItems[i] = Item{
			Name: syscall.Name.SnakeCase(),
			Item: syscall,
		}
	}

	types = append(types, Type{
		Name:     "syscalls",
		Template: syscallTemplate,
		Items:    syscallItems,
	})

	groupItems := make([]Item, len(file.Groups))
	for i, group := range file.Groups {
		groupItems[i] = Item{
			Name: group.Name.SnakeCase(),
			Item: group,
		}
	}

	types = append(types, Type{
		Name:     "groups",
		Template: groupTemplate,
		Items:    groupItems,
	})

	for _, typ := range types {
		const dirMode = 0777
		typeDir := filepath.Join(dir, typ.Name)
		err := os.MkdirAll(typeDir, dirMode)
		if err != nil {
			return fmt.Errorf("failed to create directory %q: %v", typeDir, err)
		}

		for _, item := range typ.Items {
			err = generateItemHTML(filepath.Join(typeDir, item.Name+".html"), typ.Template, item.Item, true)
			if err != nil {
				return err
			}
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
	indexTemplate      = "index_html.txt"
	itemPrefixTemplate = "item-prefix_html.txt"
	itemSuffixTemplate = "item-suffix_html.txt"

	arrayTemplate       = "array_html.txt"
	bitfieldTemplate    = "bitfield_html.txt"
	enumerationTemplate = "enumeration_html.txt"
	integerTemplate     = "integer_html.txt"
	structureTemplate   = "structure_html.txt"
	syscallTemplate     = "syscall_html.txt"
	groupTemplate       = "group_html.txt"
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
	case *types.Array:
		return "array"
	case *types.Bitfield:
		return "bitfield"
	case *types.Enumeration:
		return "enumeration"
	case *types.NewInteger:
		return "integer"
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
	case *types.Array:
		return item.Groups
	case *types.Bitfield:
		return item.Groups
	case *types.Enumeration:
		return item.Groups
	case *types.NewInteger:
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
	case *types.Array:
		return item.Name.Spaced()
	case *types.Bitfield:
		return item.Name.Spaced()
	case *types.Enumeration:
		return item.Name.Spaced()
	case *types.NewInteger:
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
	case *types.Array:
		return "Array"
	case *types.Bitfield:
		return "Bitfield"
	case *types.Enumeration:
		return "Enumeration"
	case *types.NewInteger:
		return "Integer"
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
	case *types.Array:
		return template.HTML(fmt.Sprintf("%s%dx %s%s", prefix, item.Count, toString(item.Type), suffix))
	case *types.Bitfield:
		return prefix + toString(item.Type) + suffix
	case *types.Enumeration:
		return prefix + toString(item.Type) + suffix
	case *types.NewInteger:
		return prefix + toString(item.Type) + suffix
	default:
		return ""
	}
}

func toString(t types.Type) template.HTML {
	switch t := types.Underlying(t).(type) {
	case *types.Array:
		return template.HTML(fmt.Sprintf(`<a href="../arrays/%s.html" class="array">%s</a>`, t.Name.SnakeCase(), t.Name.Spaced()))
	case *types.Bitfield:
		return template.HTML(fmt.Sprintf(`<a href="../bitfields/%s.html" class="bitfield">%s</a>`, t.Name.SnakeCase(), t.Name.Spaced()))
	case *types.Enumeration:
		return template.HTML(fmt.Sprintf(`<a href="../enumerations/%s.html" class="enumeration">%s</a>`, t.Name.SnakeCase(), t.Name.Spaced()))
	case types.Integer:
		return template.HTML(fmt.Sprintf(`<span title="%s">%s</span>`, plainDocs(t.Docs()), t))
	case *types.Structure:
		return template.HTML(fmt.Sprintf(`<a href="../structures/%s.html" class="structure">%s</a>`, t.Name.SnakeCase(), t.Name.Spaced()))
	case *types.SyscallReference:
		return template.HTML(fmt.Sprintf(`<a href="../syscalls/%s.html" class="syscall">%s</a>`, t.Name.SnakeCase(), t.Name.Spaced()))

	case types.Padding:
		return template.HTML(fmt.Sprintf("%d-byte padding", t))
	case *types.Pointer:
		if t.Mutable {
			return `<span title="A pointer to writable memory.">*mutable</span> ` + toString(t.Underlying)
		} else {
			return `<span title="A pointer to readable memory.">*constant</span> ` + toString(t.Underlying)
		}
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
