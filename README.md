# Firefly

Firefly is a research OS inspired by [Plan 9 from Bell Labs](https://9p.io/plan9/). Firefly's planned name was Serenity, but [SerenityOS](https://github.com/SerenityOS/serenity) got there first. This project is in no way associated with SerenityOS.

This repository consists of:

- the Firefly kernel in [`kernel`](/kernel)
- the Firefly kernel static analyser in ['kcheck'](/kcheck)
- the Firefly image builder in ['imgbuild'](/imgbuild)
- the Pure64 bootloader (modified for Firefly) in ['Pure64'](/Pure64)

Firefly is intended for executing cloud-native containerised server software. As a result, there are no plans to add a graphical user interface, device drivers, or a concept of users. Instead, the priority is to support userland applications on a virtual machine, with strong separation between processes. Firefly will provide a highly stable ABI, with syscalls providing the sole interface between userland processes and the kernel.

Drawing inspiration from Plan 9, _everything is a filesystem_. Overlay filesystems are a fundamental component of Firefly, with network resources, system information, and disk filesystems mounted onto a virtual filesystem. Process namespaces are used to produce independent resource trees for processes, filesystems, and network resources.

# Building Firefly

While Firefly can be built locally, builds using a Docker container provide consistency.
The full set of dependencies for building Firefly are defined in the ['Dockerfile'](/Dockerfile).
Helper scripts are provided for common tasks:

- building the Docker container used to build Firefly with ['./build-docker-builder'](/build-docker-builder)
- building Firefly using the Docker container with ['./docker-build'](/docker-build)
- running Firefly using QEMU with ['./run-qemu'](/run-qemu)
- cleaning the build environment with ['./clean'](/clean)

# Running Firefly

As stated above, Firefly can be run with QEMU using the ['./run-qemu'](/run-qemu) helper script.
Firefly has the following dependencies:

- An Intel x86_64 processor, of Ivy Bridge generation or later
- A VESA-compatible display, supporting 1024x768 resolution with 24-bit colour
- At least 128 MiB of RAM

## FAQ

### Why make a new OS?

Firefly is primarily an experiment in producing equivalent capabilities for executing cloud-native applications as modern Linux, with a dramatically smaller attack surface and clearer security outcomes. I reckon creating a new OS from scratch will require less work than stripping the irrelevant functionality from Linux.

### Why write the kernel in C?

C was designed for writing kernels (amongst other things) and has a lot of existing code and examples. While other languages will provide benefits over C (such as memory safety or better tooling), they will be lacking in existing code and samples. By defining the interface between userland and the kernel in the syscall ABI, it will be possible to rewrite the kernel in another language at a later point. I reckon building new tooling around C will require less work than writing the kernel in another language.
