// Compile the kernel.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ProjectSerenity/firefly/kbuild/internal/build"
)

// CompileLinkerScript prepares the linker script,
// inlining any constants.
//
func (ctx *Context) CompileLinkerScript() {
	// Read and store the constants.
	name := filepath.Join("arch", ctx.Architectures[0], "rt0", "constants.inc")
	f, err := os.Open(name)
	if err != nil {
		ctx.Fatalf("failed to read %s: %v", name, err)
	}

	defer f.Close()

	s := bufio.NewScanner(f)
	constants := make(map[string]string)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		parts := strings.Split(line, " equ ")
		if len(parts) != 2 {
			ctx.Fatalf("bad line in %s: %q", name, line)
		}

		constants[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}

	if err := s.Err(); err != nil {
		ctx.Fatalf("failed to parse %s: %v", name, err)
	}

	// Read the linker script, then perform the
	// replacements.

	name = filepath.Join("arch", ctx.Architectures[0], "script", "linker.ld.in")
	script, err := ioutil.ReadFile(name)
	if err != nil {
		ctx.Fatalf("failed to read %s: %v", name, err)
	}

	for from, to := range constants {
		script = bytes.ReplaceAll(script, []byte(from), []byte(to))
	}

	name = filepath.Join(ctx.WorkDir, "linker.ld")
	err = ioutil.WriteFile(name, script, 0644)
	if err != nil {
		ctx.Fatalf("failed to write linker script: %v", err)
	}
}

// CompileRT0 compiles the bootloader assembly.
//
func (ctx *Context) CompileRT0() {
	for _, goarch := range ctx.Architectures {
		err := ctx.compileRT0(goarch)
		if err != nil {
			ctx.Fatalf("failed to compile rt0 on %s: %v", goarch, err)
		}
	}
}

var nasmFormatForGoarch = map[string]string{
	"amd64": "elf64",
}

func (ctx *Context) compileRT0(goarch string) error {
	dir := filepath.Join("arch", goarch, "rt0")
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to identify rt0 assembly files: %v", err)
	}

	format, ok := nasmFormatForGoarch[goarch]
	if !ok {
		return fmt.Errorf("no nasm format for GOARCH %s", goarch)
	}

	for _, file := range files {
		name := file.Name()
		if file.IsDir() || !strings.HasSuffix(name, ".s") {
			continue
		}

		src := filepath.Join(dir, name)
		dst := filepath.Join(ctx.WorkDir, strings.TrimSuffix(name, ".s")+".o")
		redirects := fmt.Sprintf("-dNUM_REDIRECTS=%d", len(ctx.Redirects))

		ctx.assemblyObjects = append(ctx.assemblyObjects, dst)

		cmd := exec.Command(ctx.nasm, "-g", "-f", format, "-F", "dwarf", "-I", dir, "-I", ctx.WorkDir, redirects, "-o", dst, src)
		_, err = cmd.Output()
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok && len(e.Stderr) > 0 {
				ctx.Errorf("%s", e.Stderr)
			}

			return fmt.Errorf("failed to assemble %s: %v", name, err)
		}
	}

	return nil
}

// CompileKernel compiles the kernel in the current directory.
//
func (ctx *Context) CompileKernel() {
	pkgMain := "github.com/ProjectSerenity/firefly/kernel/main"
	ldflags := "-tmpdir=" + ctx.WorkDir + " -linkmode=external '-extldflags=-nostartfiles -nodefaultlibs -nostdlib -r'"
	env := build.OverrideEnv(os.Environ(),
		"GOARCH="+ctx.Architectures[0],
		"CGO_ENABLED=0",
		"GOPATH=/kernel")

	cmd := exec.Command("go", "build", "-ldflags="+ldflags, "-n", pkgMain)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			ctx.Errorf("%s", out)
		}

		ctx.Fatalf("failed to generate build script %v", err)
	}

	// This is a big replacement so we do
	// it here before the more subtle changes.
	out = bytes.ReplaceAll(out, []byte("$WORK"), []byte(ctx.WorkDir))

	// Adjust the script to remove the
	// buildid commands and the final
	// mv.

	var script bytes.Buffer
	script.Grow(len(out))
	script.WriteString("set -e\n")
	script.WriteString("export GOOS=linux\n")
	script.WriteString("export GOARCH=" + ctx.Architectures[0] + "\n")
	script.WriteString("export CGO_ENABLED=0\n")
	script.WriteString("alias pack='go tool pack'\n")
	script.WriteByte('\n')

	s := bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		line := s.Bytes()
		if bytes.HasPrefix(line, []byte("mv ")) {
			continue
		}

		i := bytes.IndexByte(line, ' ')
		if i < 0 {
			script.Write(line)
			script.WriteByte('\n')
			continue
		}

		if filepath.Base(string(line[:i])) == "buildid" && bytes.HasPrefix(line[i:], []byte(" -w ")) {
			continue
		}

		script.Write(line)
		script.WriteByte('\n')
	}

	out = script.Bytes()
	name := filepath.Join(ctx.WorkDir, "build.sh")
	err = ioutil.WriteFile(name, out, 0644)
	if err != nil {
		ctx.Fatalf("failed to write build script: %v", err)
	}

	cmd = exec.Command("sh", name)
	cmd.Env = env
	out, err = cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			ctx.Errorf("%s", out)
		}

		ctx.Fatalf("failed to run build script %v", err)
	}

	// ctx.WorkDir/go.o is an ELF object file, but the Go symbols
	// are all unexported. Our assembly entry point needs to know
	// the address of `main.main`, so we use objcopy to make that
	// symbol exported.
	//
	// Since nasm does not support externs with slashes, we also
	// create a global symbol alias for `kernel.Kmain`.

	goO := filepath.Join(ctx.WorkDir, "go.o")
	out, err = exec.Command("go", "tool", "nm", goO).Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok && len(e.Stderr) > 0 {
			ctx.Errorf("%s", e.Stderr)
		}

		ctx.Fatalf("failed to determine kmain.Kmain's location in go.o: %v", err)
	}

	var kmainAddr string
	s = bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if !strings.HasSuffix(line, "kmain.Kmain") {
			continue
		}

		i := strings.IndexByte(line, ' ')
		if i < 0 {
			ctx.Fatalf("bad nm line: %s", line)
		}

		kmainAddr = line[:i]
		break
	}

	if err := s.Err(); err != nil {
		ctx.Fatalf("failed to find kmain.Kmain's location in go.o: %v", err)
	}

	if kmainAddr == "" {
		ctx.Fatalf("failed to find kmain.Kmain's location in go.o")
	}

	out, err = exec.Command(ctx.objcopy,
		"--add-symbol", "kernel.Kmain=.text:0x"+kmainAddr,
		"--globalize-symbol", "runtime.g0",
		"--globalize-symbol", "runtime.m0",
		"--globalize-symbol", "runtime.physPageSize",
		goO, goO).CombinedOutput()

	if err != nil {
		if len(out) > 0 {
			ctx.Errorf("%s", out)
		}

		ctx.Fatalf("failed to use objcopy to export kernel.Kmain: %v", err)
	}
}

// LinkKernel links everything together.
//
func (ctx *Context) LinkKernel() {
	// We have quite a few args,
	// so we have to build them
	// up dynamically.

	ctx.kernel = filepath.Join(ctx.cwd, "bin", "kernel-"+ctx.Architectures[0]+".bin")
	args := make([]string, 0,
		5+ // linker args
			2+ // output
			len(ctx.assemblyObjects)+
			1) // go.o

	args = append(args,
		"-n",
		"-T", filepath.Join(ctx.WorkDir, "linker.ld"),
		"-static",
		"--no-ld-generated-unwind-info",
		"-o", ctx.kernel)

	args = append(args, ctx.assemblyObjects...)
	args = append(args, filepath.Join(ctx.WorkDir, "go.o"))

	out, err := exec.Command("ld", args...).CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			ctx.Errorf("%s", out)
		}

		ctx.Fatalf("failed to link kernel: %v", err)
	}
}

// BuildISO builds the kernel image into a bootable ISO
// disk image in the current directory.
//
func (ctx *Context) BuildISO() {
	// Copy the kernel image and the GRUB config
	// into a build directory, then shell out to
	// grub-mkrescue to do the rest.

	isos := filepath.Join(ctx.WorkDir, "isofiles")
	boot := filepath.Join(isos, "boot")
	grub := filepath.Join(boot, "grub")

	err := os.MkdirAll(grub, 0755)
	if err != nil {
		ctx.Fatalf("failed to prepare disk image directory: %v", err)
	}

	err = copyFile(filepath.Join(boot, "kernel.bin"), ctx.kernel)
	if err != nil {
		ctx.Fatalf("failed to write kernel image to disk image directory: %v", err)
	}

	err = copyFile(filepath.Join(grub, "grub.cfg"), filepath.Join("arch", ctx.Architectures[0], "script", "grub.cfg"))
	if err != nil {
		ctx.Fatalf("failed to write GRUB config to disk image directory: %v", err)
	}

	ctx.iso = filepath.Join(ctx.cwd, "bin", "firefly-"+ctx.Architectures[0]+".iso")
	out, err := exec.Command(ctx.grubMkrescue, "-o", ctx.iso, isos).CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			ctx.Errorf("%s", out)
		}

		ctx.Fatalf("failed to build ISO disk image: %v", err)
	}
}

func copyFile(dst, src string) error {
	d, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create %s: %v", dst, err)
	}

	defer d.Close() // We close this properly later.

	s, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open %s: %v", src, err)
	}

	defer s.Close()

	_, err = io.Copy(d, s)
	if err != nil {
		return fmt.Errorf("failed to copy %s to %s: %v", src, dst, err)
	}

	err = d.Close()
	if err != nil {
		return fmt.Errorf("failed to write to %s: %v", dst, err)
	}

	return nil
}
