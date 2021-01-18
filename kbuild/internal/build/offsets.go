// Package build is used to help build data for kbuild.
//
package build

import (
	"bufio"
	"bytes"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/mod/semver"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("")
}

// GoVersion returns gotool's version in the form goX.Y,
// provided gotool is the path to the Go tool.
//
func GoVersion(gotool string) (version string, err error) {
	out, err := exec.Command(gotool, "version").Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok && len(e.Stderr) > 0 {
			os.Stderr.Write(e.Stderr)
		}

		return "", fmt.Errorf("failed to determine %s's version: %v", gotool, err)
	}

	version = strings.TrimPrefix(string(out), "go version ")
	i := strings.IndexByte(version, ' ')
	if i > 0 {
		version = version[:i]
	}

	majmin, ok := goMajorMinorVersion(version)
	if !ok {
		return "", fmt.Errorf("failed to determine %s's version: got invalid version %s", gotool, version)
	}

	return majmin, nil
}

// goMajorMinorVersion returns v in the form goX.Y,
// provided v is a valid Go version string.
//
func goMajorMinorVersion(v string) (majmin string, ok bool) {
	if !strings.HasPrefix(v, "go") {
		return "", false
	}

	s := "v" + v[2:]
	if !semver.IsValid(s) {
		return "", false
	}

	majmin = strings.Replace(semver.MajorMinor(s), "v", "go", 1)

	return majmin, true
}

// OverrideEnv takes the current environ and a set of
// overrides. OverrideEnv ensures that the variables
// in overrides are set, with their corresponding
// values, whether they appear in the original environ
// or not. OverrideEnv does not modify environ.
//
// Typical use:
//
// 	env := OverrideEnv(os.Environ(), "GOOS=linux", "GOARCH=amd64")
//
func OverrideEnv(environ []string, overrides ...string) []string {
	out := make([]string, len(environ), len(environ)+len(overrides))
	copy(out, environ)

	variable := func(s string) string {
		i := strings.IndexByte(s, '=')
		if i < 0 {
			panic("invalid environment variable: " + s)
		}

		return s[:i]
	}

	for _, override := range overrides {
		v := variable(override)
		done := false
		for i, got := range out {
			if variable(got) == v {
				out[i] = override
				done = true
				break
			}
		}

		if done {
			continue
		}

		out = append(out, override)
	}

	return out
}

// Entry maps a symbol name to a memory offset.
//
type Entry struct {
	Symbol string
	Offset uintptr
}

// DeriveOffsets returns the offsets in memory for
// the important parts of a Go binary for a given
// version.
//
// The gotool should be a path to the Go tool. The
// goversion should be the Go version supported by
// gotool, in the form "goX.Y". The goarch should
// be the chosen machine architecture. The goarch
// must be a valid value for GOARCH, supported by
// Linux, according to `gotool tool dist list`.
//
func DeriveOffsets(gotool, goversion, goarch string) ([]*Entry, error) {
	// Confirm that gotool works, and that goarch is
	// supported by Linux, according to gotool.

	var buf bytes.Buffer
	cmd := exec.Command(gotool, "tool", "dist", "list")
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to check %s: failed to run `%s tool dist list`: %v", goarch, gotool, err)
	}

	ok := false
	want := "linux/" + goarch
	s := bufio.NewScanner(&buf)
	for s.Scan() {
		entry := strings.TrimSpace(s.Text())
		if entry == want {
			ok = true
			break
		}
	}

	if err = s.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse output from `%s tool dist list`: %v", gotool, err)
	}

	if !ok {
		return nil, fmt.Errorf("%s is not supported by Linux according to `%s tool dist list`", goarch, gotool)
	}

	// Create a temporary directory to use to build
	// a minimal binary, which can then be analised
	// to identify the offsets.

	dir, err := ioutil.TempDir("", "assembly-offsets")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary working directory: %v", err)
	}

	defer os.RemoveAll(dir)

	// Write a dummy Go program into the temp directory,
	// then pretend to build it, capturing the commands
	// that would have been run.

	const (
		dummyGoProgram = "package main\n\nfunc main(){}\n"
		dummyGoMod     = "module dummy\n"
	)

	err = ioutil.WriteFile(filepath.Join(dir, "main.go"), []byte(dummyGoProgram), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write dummy Go program to %s: %v", dir, err)
	}

	err = ioutil.WriteFile(filepath.Join(dir, "go.mod"), []byte(dummyGoMod), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write dummy Go module to %s: %v", dir, err)
	}

	// Use -a to ensure we do everything and -work to print
	// the working directory and not delete it.
	cmd = exec.Command(gotool, "build", "-a", "-work")
	cmd.Dir = dir
	cmd.Env = OverrideEnv(os.Environ(), "GOCACHE="+dir, "GOOS=linux", "GOARCH="+goarch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			os.Stderr.Write(out)
		}

		return nil, fmt.Errorf("failed to build a dummy Go program: %v", err)
	}

	var workDir string
	s = bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if strings.HasPrefix(line, "WORK=") {
			workDir = strings.TrimPrefix(line, "WORK=")
			break
		}
	}

	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("failed to determine dummy build's working directory: %v", err)
	}

	if workDir == "" {
		return nil, fmt.Errorf("failed to determine dummy build's working directory")
	}

	defer os.RemoveAll(workDir)

	// Find all generated go_asm.h files, identifying
	// the offsets for the g, m and stack structures,
	// which we then collect.

	const (
		goasm       = "go_asm.h"
		define      = "#define "
		gPrefix     = "g_"
		mPrefix     = "m_"
		stackPrefix = "stack_"
	)

	entries := make([]*Entry, 0, 64)
	err = filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Base(path) != goasm {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open %s: %v", path, err)
		}

		defer f.Close()

		s := bufio.NewScanner(f)
		for s.Scan() {
			// Note that s.Bytes() points to reused memory,
			// so don't store its contents without copying
			// them.
			line := bytes.TrimPrefix(s.Bytes(), []byte("#define "))

			// We're only interested in the offsets for the
			// g, m and stack structures.
			if !bytes.HasPrefix(line, []byte(gPrefix)) &&
				!bytes.HasPrefix(line, []byte(mPrefix)) &&
				!bytes.HasPrefix(line, []byte(stackPrefix)) {
				continue
			}

			tokens := bytes.Fields(line)
			if len(tokens) != 2 {
				continue
			}

			// Parse the offset as an unsigned integer,
			// which might be in base 10 or base 16.
			offset, err := strconv.ParseUint(string(tokens[1]), 0, 64)
			if err != nil {
				return fmt.Errorf("failed to parse definition %s: invalid offset: %v", line, tokens[1])
			}

			entries = append(entries, &Entry{
				Symbol: "GO_" + strings.ToUpper(string(tokens[0])),
				Offset: uintptr(offset),
			})
		}

		if err := s.Err(); err != nil {
			return fmt.Errorf("failed to parse %s: %v", path, err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to read %s files: %v", goasm, err)
	}

	return entries, nil
}

// WriteOffsets writes entries to name, in the form
// of a Go file containing the entry data.
//
func WriteOffsets(name, pkg, goversion, goarch string, entries []*Entry) error {
	tmpl, err := template.New("").Parse(entriesTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse entries template: %v", err)
	}

	var data struct {
		Goarch        string
		Version       string
		VersionSuffix string
		Package       string
		Symbols       []*Entry
	}

	data.Goarch = goarch
	data.Version = goversion
	data.VersionSuffix = strings.Replace(strings.TrimPrefix(goversion, "go"), ".", "", -1)
	data.Package = pkg
	data.Symbols = entries

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return fmt.Errorf("failed to execute template: %v", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Println(err)
		formatted = buf.Bytes()
	}

	err = ioutil.WriteFile(name, formatted, 0644)
	if err != nil {
		return fmt.Errorf("failed to write %s: %v", name, err)
	}

	return nil
}

const entriesTemplate = `// Code generated by gen-version-data.go for {{.Version}} -- DO NOT EDIT.

package {{.Package}}

func init() {
	offsetsByVersion["{{.Version}}"] = SymbolOffsetsForGo{{.VersionSuffix}}
}

// SymbolOffsetsForGo{{.VersionSuffix}} includes the data on important symbols
// in the Go runtime for {{.Version}}.
//
var SymbolOffsetsForGo{{.VersionSuffix}} = []SymbolOffset{
	{{range .Symbols -}}{
		Symbol: "{{.Symbol}}",
		Offset: {{.Offset}},
	},
	{{end}}
}

`
