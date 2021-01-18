# kbuild

kbuild is a Go program which compiles the Firefly kernel, links it, produces a bootable ISO disk image. This can then be booted using tools like QEMU, VMWare, and VirtualBox. While the plan is for kbuild to use the Go linker (go tool link) and a Go library for building ISO images, it currently uses the GNU linker (ld) and image tool (xorriso). As a result, kbuild is typically built into a Docker container to simplify development.

## Building the kbuild container image

To build the container image, ensure Docker is installed and running, then run:

```
$ docker build -t firefly/kbuild .
```

Alternatively, run the `build` script.

## Building Firefly using the kbuild container image

To build Firefly, run the following command in [`kernel`](/kernel):

```
$ docker run --rm -it -v $PWD:/kernel/src/github.com/ProjectSerenity/firefly/kernel -v /tmp:/tmp firefly/kbuild
```

Alternatively, run the `build` script.

The ISO build image can then be run using QEMU:

```
$ qemu-system-x86_64 -cdrom bin/firefly-amd64.iso -vga std -no-reboot
```

Alternatively, run the `run` script.

## Introduction to kbuild

kbuild is a Go program, which is used to build the Firefly kernel into a bootable ISO disk image. While this is much more verbose than the original Makefile that inspired it, it's much easier to debug and iterate and produces more helpful error messages. For the moment, kbuild shells out to various GNU/Linux tools:

- objcopy (to adjust the visibility of certain symbols)
- nasm (to assemble the rt0 bootloader)
- gcc/ld (to perform the final link)
- grub-mkrescue/xorriso (to construct the disk image)

The long-term plan is to replace these with code/libraries written in Go so that kbuild can be used directly to cross-compile the kernel without needing a container.

### kbuild process

kbuild takes the following steps to build the kernel:

- Before building kbuild, use [`gen-version-data.go`](gen-version-data.go) to generate offset data for the desired version of Go (`offsets-goX.Y_GOARCH.go`).
- `ctx.GetOffsets` in [`offsets.go`](offsets.go) determines the offset data for the Go version used by the Go tool. Note that this may be different from the version of Go used to build kbuild.
- `ctx.CheckDeps` in [`deps.go`](deps.go) checks that all required dependencies are available, with suitable versions. This should never fail in the kbuild container, but could be useful if using kbuild directly.
- `ctx.MakeWorkDir` in [`main.go`](main.go) creates a temporary working directory to help keep the current directory clean. If the build fails, the working directory will be printed and left in place to aid debugging. If the build succeeds, the working directory will be deleted.
- `ctx.WriteOffsets` in [`offsets.go`](offsets.go) writes the offset data to an assembly include file in the working directory (`go_asm_offsets.inc`). This tells the bootloader assembly where core parts of the Go runtime exist in memory for initialisation.
- `ctx.FindRedirects` in [`redirects.go`](redirects.go) parses the kernel to identify `//go:redirect-from` comments, which are used to allow the kernel to override parts of the Go runtime. This allows the kernel to adapt the Go runtime to running in ring 0, such as by changing how memory allocation works. This phase just collects the redirects by name. A later phase identifies the memory addresses for each symbol.
- `ctx.CompileLinkerScript` in [`compile.go`](compile.go) inlines assembly constants into the linker script (`linker.ld`).
- `ctx.CompileRT0` in [`compile.go`](compile.go) assembles the bootloader.
- `ctx.CompileKernel` in [`compile.go`](compile.go) builds the main kernel and performs an initial link, producing `go.o`.
- `ctx.LinkKernel` in [`compile.go`](compile.go) links the kernel binary (`go.o`) and the bootloader objects (`cgo_stubs.o`, `multiboot_header.o`, `rt0_32.o`, and `rt0_64.o`) using the linker script (`linker.ld`) into the final kernel image (`kernel-GOARCH.bin`), which is written to [`kernel/bin`](/kernel/bin).
- `ctx.CompleteRedirects` in [`redirects.go`](redirects.go) parses the kernel image to find the virtual memory address of each symbol identified in `ctx.FindRedirects`. The image is then overwritten to apply the redirects in the `.goredirectstbl` section.
- `ctx.BuildISO` in [`compile.go`](compile.go) uses grub-mkrescue and xorriso to build a bootable ISO disk image from the kernel image and the boot configuration (`grub.cfg`). The resulting image (`firefly-GOARCH.iso`) is written to [`kernel/bin`](/kernel/bin).

### TODO

Short-term plans:

- Improve the handling of different architectures so that additions to `ctx.Architectures` would just work.
- Switch to using the same version of Go for building kbuild and for building the kernel, once the kernel has been updated to support newer versions of Go.

Long-term plans:

- Replace the external tools with functionality written in Go so that the only external dependency is the Go tool (which we need anyway).
