//go:generate go run gen-version-data.go

// Command kbuild is used to build the Firefly kernel into a bootable ISO disk image.
//
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/ProjectSerenity/firefly/kbuild/internal/build"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("kbuild: ")
}

type Context struct {
	GoVersion     string
	WorkDir       string
	Architectures []string

	Redirects []*SymbolRedirect

	Offsets []SymbolOffset

	// Deps.
	objcopy      string
	xorriso      string
	grubMkrescue string
	nasm         string

	// Assemblies.
	assemblyObjects []string

	// Paths
	cwd    string
	kernel string
	iso    string

	printedWorkDir bool
}

func (ctx *Context) GetGoVersion() {
	var err error
	ctx.GoVersion, err = build.GoVersion("go")
	if err != nil {
		ctx.Fatalf("failed to determine Go version: %v", err)
	}
}

func (ctx *Context) MakeWorkDir() {
	err := os.MkdirAll("bin", 0755)
	if err != nil {
		ctx.Fatalf("failed to create output directory: %v", err)
	}

	ctx.cwd, err = os.Getwd()
	if err != nil {
		ctx.Fatalf("failed to determine current directory: %v", err)
	}

	ctx.WorkDir, err = ioutil.TempDir("", "kbuild")
	if err != nil {
		ctx.Fatalf("failed to create working directory: %v", err)
	}
}

func (ctx *Context) Cleanup() {
	err := os.RemoveAll(ctx.WorkDir)
	if err != nil {
		ctx.Errorf("failed to clean up working directory %s: %v", ctx.WorkDir, err)
	}
}

func (ctx *Context) Errorf(format string, v ...interface{}) {
	if !ctx.printedWorkDir && ctx.WorkDir != "" {
		fmt.Println(ctx.WorkDir)
		ctx.printedWorkDir = true
	}

	log.Printf(format, v...)
}

func (ctx *Context) Fatalf(format string, v ...interface{}) {
	if !ctx.printedWorkDir {
		fmt.Println(ctx.WorkDir)
		ctx.printedWorkDir = true
	}

	log.Fatalf(format, v...)
}

func main() {
	ctx := &Context{
		Architectures: []string{
			"amd64",
		},
	}

	ctx.GetGoVersion()
	ctx.GetOffsets()
	ctx.CheckDeps()
	ctx.MakeWorkDir()
	ctx.WriteOffsets()
	ctx.FindRedirects()
	ctx.CompileLinkerScript()
	ctx.CompileRT0()
	ctx.CompileKernel()
	ctx.LinkKernel()
	ctx.CompleteRedirects()
	ctx.BuildISO()
	ctx.Cleanup()

	iso, err := filepath.Rel(ctx.cwd, ctx.iso)
	if err != nil {
		iso = ctx.iso
		ctx.Errorf("failed to derive ISO path: %v", err)
	}

	fmt.Printf("Successfully built %s\n", iso)
}
