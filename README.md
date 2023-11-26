# Firefly

Firefly is a research OS inspired by [Plan 9 from Bell Labs](https://9p.io/plan9/). Firefly's planned name was Serenity, but [SerenityOS](https://github.com/SerenityOS/serenity) got there first. This project is unrelated to SerenityOS.

Note that Firefly is an early prototype and is not yet ready for use.

This repository consists of:

- the Firefly kernel in [`kernel`](/kernel)
- the Firefly bootloader (forked from [`bootloader`](https://crates.io/crates/bootloader)) in [`bootloader`](/bootloader)
- user code that runs on Firefly in [`user`](/user)
- code shared between the kernel, bootloader, and userspace in [`shared`](/shared)
- code to manage the Bazel build system in [`bazel`](/bazel)
- tools to manage the repository in [`tools`](/tools)
- third-party code is vendored using `bazel run //tools/vendor-deps` into [`vendor`](/vendor)

Firefly is intended for executing cloud-native containerised server software. As a result, there are no plans to add a graphical user interface, device drivers, or a concept of users. Instead, the priority is to support userland applications on a virtual machine, with strong separation between processes. Firefly will provide a highly stable ABI, with syscalls providing the sole interface between userland processes and the kernel.

# Building Firefly

Firefly is built using the [Bazel](https://bazel.build/) build system. You will need to install Bazel to build Firefly. It is recommended that you use [Bazelisk](https://github.com/bazelbuild/bazelisk), rather than using Bazel directly, to ensure the right version of Bazel is used.

While Bazel manages most of the build, some tools are currently used from the host for now. This currently consists of:

- Clang (expected in `/usr/bin/clang`)
- LLD (expected in `/usr/bin/ld`)

External dependencies are managed by [VDM](https://github.com/ProjectSerenity/vdm).

Once Bazel and the above host tools are prepared, the following commands are common:

- `bazel build //kernel`:               Build the kernel binary.
- `bazel build //bootloader`:           Build the bootloader binary.
- `bazel build //:image`:               Build a bootable Firefly disk image.
- `bazel build //...`:                  Build all code, documentation, and lints.
- `bazel test //...`:                   Run all tests.
- `vdm update`:                         Update managed dependencies.
- `vdm vendor`:                         Vendor managed dependencies from [`deps.bzl`](/deps.bzl).
- `vdm check //shared/... //tools/...`: Check for vulnerabilities in dependencies.
- `bazel run //:qemu`:                  Build a bootable disk image and run it in Qemu.

# FAQ

## Why make a new OS?

Firefly is primarily an experiment in producing equivalent capabilities for executing cloud-native applications as modern Linux, with a dramatically smaller attack surface and clearer security outcomes. I reckon creating a new OS from scratch will require less work than stripping the irrelevant functionality from Linux.

## Why write the kernel in Rust?

A modern OS deserves a modern programming language. Rust provides more modern functionality like package management, code modules, integrated unit tests, putting it far ahead of C in usability. Furthermore, Rust has strong safety features without compromising runtime performance. This is perfectly suited to an OS kernel.
