# Firefly

Firefly is a research OS inspired by [Plan 9 from Bell Labs](https://9p.io/plan9/). Firefly's planned name was Serenity, but [SerenityOS](https://github.com/SerenityOS/serenity) got there first. This project is in no way associated with SerenityOS.

Note that Firefly is an early prototype and is not yet ready for use.

This repository consists of:

- the Firefly kernel in [`kernel`](/kernel)
- code shared between the kernel and userspace in [`shared`](/shared)
- code to manage the Bazel build system in [`bazel`](/bazel)

Firefly is intended for executing cloud-native containerised server software. As a result, there are no plans to add a graphical user interface, device drivers, or a concept of users. Instead, the priority is to support userland applications on a virtual machine, with strong separation between processes. Firefly will provide a highly stable ABI, with syscalls providing the sole interface between userland processes and the kernel.

# Building Firefly

Firefly is built using the [Bazel](https://bazel.build/) build system. You will need to install Bazel to build Firefly. It is recommended that you use [Bazelisk](https://github.com/bazelbuild/bazelisk), rather than using Bazel directly, to ensure the right version of Bazel is used.

While Bazel manages most of the build, some tools are currently used from the host for now. This currently consists of:

- Clang (expected in `/usr/bin/clang`)
- LLD (expected in `/usr/bin/ld`)
- LLVM-tools (as installed with `rustup component add llvm-tools-preview`)

Once Bazel and the above host tools are prepared, the following commands are common:

- `bazel build //kernel`:     Build the kernel binary.
- `bazel build //:image`:     Build a bootable Firefly disk image.
- `bazel test //...`:         Run all tests.
- `bazel run //:update-deps`: Update managed dependencies.
- `bazel run //:qemu`:        Build a bootable disk image and run it in Qemu.

When the set of external Rust crates changes, such as by updating crates, or updating rules_rust, the lockfile for external crates needs to be updated using: `CARGO_BAZEL_REPIN=true bazel build @crates//...`.

## FAQ

### Why make a new OS?

Firefly is primarily an experiment in producing equivalent capabilities for executing cloud-native applications as modern Linux, with a dramatically smaller attack surface and clearer security outcomes. I reckon creating a new OS from scratch will require less work than stripping the irrelevant functionality from Linux.

### Why write the kernel in Rust?

While Firefly was originally written in C, due to the author's lack of Rust experience, a modern OS deserves a modern programming language. The kernel has now been rewritten in Rust, and the plan is to use Rust for the integrated userland components as well. Rust provides more modern functionality like package management, code modules, integrated unit tests, putting it far ahead of C in usability. Furthermore, Rust has strong safety features without compromising runtime performance. This is perfectly suited to an OS kernel.
