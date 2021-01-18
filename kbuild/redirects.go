// Analyse the kernel to identify symbol redirections.

package main

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// SymbolRedirect maps one symbol to another.
//
type SymbolRedirect struct {
	Comment string

	SrcSymbol string
	DstSymbol string

	SrcVirtAddr uint64
	DstVirtAddr uint64
}

// FindRedirects parses the kernel source code to identify
// every instance of a //go:redirect-from comment, used to
// redirect one symbol to another.
//
func (ctx *Context) FindRedirects() {
	var sourceFiles []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) == ".go" && !strings.HasSuffix(path, "_test.go") {
			sourceFiles = append(sourceFiles, path)
		}

		return nil
	})

	if err != nil {
		ctx.Fatalf("failed to find kernel source files: %v", err)
	}

	const (
		redirectComment = "//go:redirect-from"
		pkgPrefix       = "github.com/ProjectSerenity/firefly/kernel"
	)

	fset := token.NewFileSet()
	for _, file := range sourceFiles {
		f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			ctx.Fatalf("failed to parse %s: %v", file, err)
		}

		cmap := ast.NewCommentMap(fset, f, f.Comments)
		cmap.Filter(f)
		for node := range cmap {
			decl, ok := node.(*ast.FuncDecl)
			if !ok || decl.Doc == nil {
				continue
			}

			for _, comment := range decl.Doc.List {
				if !strings.HasPrefix(comment.Text, redirectComment) {
					continue
				}

				slashed := filepath.ToSlash(filepath.Dir(file))
				pkgPath := path.Join(pkgPrefix, slashed)

				// Build a fully qualified name to the function.
				name := fmt.Sprintf("%s.%s", pkgPath, decl.Name)
				from := strings.TrimSpace(strings.TrimPrefix(comment.Text, redirectComment))

				ctx.Redirects = append(ctx.Redirects, &SymbolRedirect{
					Comment:   fset.Position(comment.Pos()).String(),
					SrcSymbol: from,
					DstSymbol: name,
				})
			}
		}
	}
}

// CompleteRedirects finds the redirected symbols in the
// kernel image, populating the redirect addresses. Having
// done so, CompleteRedirects writes the redirects into
// the image.
//
func (ctx *Context) CompleteRedirects() {
	ef, err := elf.Open(ctx.kernel)
	if err != nil {
		ctx.Fatalf("failed to read %s for redirected symbols: %v", ctx.kernel, err)
	}

	defer ef.Close()

	redirectsSection := ef.Section(".goredirectstbl")
	if redirectsSection == nil {
		ctx.Fatalf("failed to find .goredirectstbl section in %s", ctx.kernel)
	}

	redirectsOffset := redirectsSection.Offset

	symbols, err := ef.Symbols()
	if err != nil {
		ctx.Fatalf("failed to identify symbols in %s: %v", ctx.kernel, err)
	}

	badSymbols := false
	for _, redirect := range ctx.Redirects {
		for _, symbol := range symbols {
			if symbol.Name == redirect.SrcSymbol {
				redirect.SrcVirtAddr = symbol.Value
			}

			if symbol.Name == redirect.DstSymbol {
				redirect.DstVirtAddr = symbol.Value
			}
		}

		switch {
		case redirect.SrcVirtAddr == 0:
			badSymbols = true
			ctx.Errorf("%s: could not find src address for %q", redirect.Comment, redirect.SrcSymbol)
		case redirect.DstVirtAddr == 0:
			badSymbols = true
			ctx.Errorf("%s: could not find dst address for %q", redirect.Comment, redirect.DstSymbol)
		}
	}

	ef.Close()

	if badSymbols {
		os.Exit(1)
	}

	f, err := os.OpenFile(ctx.kernel, os.O_WRONLY, 0755)
	if err != nil {
		ctx.Fatalf("failed to open %s to overwrite redirects: %v", ctx.kernel, err)
	}

	defer f.Close()

	_, err = f.Seek(int64(redirectsOffset), io.SeekStart)
	if err != nil {
		ctx.Fatalf("failed to seek %s to redirects table: %v", ctx.kernel, err)
	}

	for _, redirect := range ctx.Redirects {
		err = binary.Write(f, binary.LittleEndian, redirect.SrcVirtAddr)
		if err != nil {
			ctx.Fatalf("failed to write src address for %s to %s: %v", redirect.SrcSymbol, ctx.kernel, err)
		}

		err = binary.Write(f, binary.LittleEndian, redirect.DstVirtAddr)
		if err != nil {
			ctx.Fatalf("failed to write dst address for %s to %s: %v", redirect.DstSymbol, ctx.kernel, err)
		}
	}

	err = f.Close()
	if err != nil {
		ctx.Fatalf("failed to close %s after overwriting redirects: %v", ctx.kernel, err)
	}
}
