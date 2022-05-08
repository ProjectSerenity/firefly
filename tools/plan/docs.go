// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/ProjectSerenity/firefly/tools/plan/parser"
	"github.com/ProjectSerenity/firefly/tools/plan/types"
)

const dirMode = 0777

func mkdir(dir string) error {
	err := os.MkdirAll(dir, dirMode)
	if err != nil {
		return fmt.Errorf("failed to create directory %q: %v", dir, err)
	}

	return nil
}

func init() {
	RegisterCommand("docs", "Generate documentation for a Plan document.", cmdDocs)
}

func cmdDocs(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("docs", flag.ExitOnError)

	var help bool
	var outname string
	var arch types.Arch
	flags.BoolVar(&help, "h", false, "Show this message and exit.")
	flags.StringVar(&outname, "out", "", "The path where the documentation should be written.")
	flags.Func("arch", "Instruction set architecture to target (options: x86-64).", func(s string) error {
		switch s {
		case "x86-64":
			arch = types.X86_64
		default:
			return fmt.Errorf("unrecognised architecture %q", s)
		}

		return nil
	})

	flags.Usage = func() {
		log.Printf("Usage:\n  %s %s [OPTIONS] FILE\n\n", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(2)
	}

	err := flags.Parse(args)
	if err != nil || help {
		flags.Usage()
	}

	if arch == types.InvalidArch {
		log.Printf("%s %s: -arch not specified.", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(1)
	}

	if outname == "" {
		log.Printf("%s %s: -out not specified.", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(1)
	}

	args = flags.Args()
	if len(args) != 1 {
		log.Printf("%s %s can only build one file at a time.", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(1)
	}

	filename := args[0]
	src, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open %s: %v", filename, err)
	}

	defer src.Close()

	syntax, err := parser.ParseFile(filename, src)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", filename, err)
	}

	file, err := types.Interpret(filename, syntax, arch)
	if err != nil {
		return fmt.Errorf("failed to interpret %s: %v", filename, err)
	}

	// Generate the documentation.
	err = mkdir(outname)
	if err != nil {
		return err
	}

	err = GenerateHTML(outname, file)
	if err != nil {
		return fmt.Errorf("failed to generate HTML: %v", err)
	}

	return nil
}

// GenerateHTML produces HTML documentation for
// the given Plan document, writing HTML files
// to the given directory.
//
func GenerateHTML(dir string, file *types.File) error {
	// Start with the index page.
	err := generateItemHTML(filepath.Join(dir, "index.html"), htmlIndexTemplate, file)
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
		err = generateItemHTML(filepath.Join(enumDir, enumeration.Name.SnakeCase()+".html"), htmlEnumerationTemplate, enumeration)
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
		err = generateItemHTML(filepath.Join(structDir, structure.Name.SnakeCase()+".html"), htmlStructureTemplate, structure)
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
		err = generateItemHTML(filepath.Join(syscallDir, syscall.Name.SnakeCase()+".html"), htmlSyscallTemplate, syscall)
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

	err = htmlTemplates.ExecuteTemplate(f, template, item)
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
var htmlTemplatesFS embed.FS

var htmlTemplates = template.Must(template.New("").Funcs(template.FuncMap{
	"addOne":   htmlAddOne,
	"toString": htmlString,
}).ParseFS(htmlTemplatesFS, "templates/*_html.txt", "templates/css/*_css.txt"))

const (
	htmlEnumerationTemplate = "enumeration_html.txt"
	htmlStructureTemplate   = "structure_html.txt"
	htmlSyscallTemplate     = "syscall_html.txt"
	htmlIndexTemplate       = "index_html.txt"
)

func htmlAddOne(i int) int {
	return i + 1
}

func htmlString(t types.Type) template.HTML {
	switch t := t.(type) {
	case types.Integer:
		return template.HTML(t.String())
	case *types.Pointer:
		if t.Mutable {
			return "*mutable " + htmlString(t.Underlying)
		} else {
			return "*constant " + htmlString(t.Underlying)
		}
	case *types.Reference:
		return htmlString(t.Underlying)
	case types.Padding:
		return template.HTML(fmt.Sprintf("%d-byte padding", t))
	case *types.Enumeration:
		return template.HTML(fmt.Sprintf(`<a href="../enumerations/%s.html" class="enumeration">%s</a>`, t.Name.SnakeCase(), t.Name.Spaced()))
	case *types.Structure:
		return template.HTML(fmt.Sprintf(`<a href="../structures/%s.html" class="structure">%s</a>`, t.Name.SnakeCase(), t.Name.Spaced()))
	default:
		panic(fmt.Sprintf("htmlString(%T): unexpected type", t))
	}
}
