# Cross-compiling

To build the kernel, we need to be able to cross-compile to a
target with no OS. This requires us to set up:

1. A [Bazel Platform](https://docs.bazel.build/versions/main/platforms.html) to represent the target environment.
2. A [Rust toolchain](https://bazelbuild.github.io/rules_rust/flatten.html#rust_toolchain) for the target.
3. A [C++ toolchain](https://docs.bazel.build/versions/main/be/c-cpp.html#cc_toolchain) for the target (to statically compile libc and link the binary).
4. A [Bazel Transition](https://docs.bazel.build/versions/main/skylark/config.html#user-defined-transitions) to ensure we use the right toolchain for each build target.

Each of these tasks is covered in more detail below.

## The `x86_64_bare_metal` Platform

We start by defining a Platform describing an x86_64 platform with no OS.
This is pretty simple and is defined in [`BUILD`](./BUILD). We still need
to configure Bazel to use the Platform, but we tackle that in the Transition
below.

## The `x86_64_rust_toolchain` Rust toolchain

Next, we define a macro to instantiate a cross-compiling Rust toolchain so
we can compile the kernel for the bare metal platform. This is defined in
[`x86_64_rust_toolchain.bzl`](./x86_64_rust_toolchain.bzl) and instantiated
for both the kernel and the bootloader (separately) in [`BUILD`](./BUILD).
It's mostly a copy of the standard Rust toolchain. There are three things
of note though:

1. We specify a custom LLVM target configuration file.
2. We provide a filegroup for the standard libraries we use (alloc, compiler-builtins, core).
3. The toolchain currently only runs on Linux. In future this should be expanded.

To be able to use the toolchain, we need to have a pre-built copy of the
main libraries we use. Namely, [`core`](https://doc.rust-lang.org/core/),
[`alloc`](https://doc.rust-lang.org/alloc/), and `[compiler-builtins`](https://github.com/rust-lang/compiler-builtins).
Ideally, we would download these pre-built, but that's not an option, as
we use a custom target configuration. Instead, we download the code and
build them ourselves. Ideally this would use rust_library as normal, but
compiling the standard library is non-trivial. For now, we invoke the copy
of `rustc` included in rules_rust in a genrule. An obvious improvement
here would be to migrate to rust_library.

The toolchain is then instantiated in [`BUILD`](./BUILD).

## The `x86_64_cc_toolchain` C++ toolchain

Next, we define the cross-compiling C++ toolchain. This shouldn't really
be necessary, but rules_rust defers to the C++ toolchain for linking. See
[John Millikin's blog post](https://john-millikin.com/notes-on-cross-compiling-rust)
for more background. We don't currently plan to have any C++ code in the kernel,
so there's no need for the C++ compiler part to work, just the linker. For now,
we defer to the host, which needs to have LLD.

An obvious improvement here would be to build/download LLD automatically, so
that we wouldn't need to depend on the host's configuration.

The toolchain is defined in [`x86_64_cc_toolchain.bzl`](./x86_64_cc_toolchain.bzl).
As with the Rust toolchain above, this currently only works on Linux, which
should be fixed. The toolchain is included in a toolchain suite, which selects
on the x86_64 CPU.

The toolchain is then instantiated in [`BUILD`](./BUILD).

## The `x86_64_bare_metal_rust_binary` Transition

Having created our platform and toolchains, we could build the kernel as
a normal rust_binary by setting the following command line options:

```
$ bazel build \
     --cpu=x86_64 \
     --crosstool_top=//bazel/cross-compiling:x86_64_cc_toolchain_suite \
     --extra_toolchains=//bazel/cross-compiling:x86_64_cc_toolchain \
     --extra_toolchains=//bazel/cross-compiling:x86_64_rust_toolchain \
     --host_crosstool_top=@bazel_tools//tools/cpp:toolchain \
     --platforms=//bazel/cross-compiling:x86_64_bare_metal \
     //kernel:binary
```

This is fine, and it works, but it's tedious. To make it less so, we add a new
build target that performs an outbound config transition before invoking the
kernel build to enable the flags specified above.

We create the x86_64_bare_metal_rust_binary rule, which simply copies its
input to its output, applying a transition in the process. That transition
applies the command line options specified above. That means we can instead
build the kernel as follows:

```
$ bazel build //kernel
```

Much simpler!
