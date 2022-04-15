# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

# The x86_64_bare_metal platform is used to describe
# the target execution environment for the kernel.
#
# When we build targets for the kernel, we use the
# --platforms command line option to specify this
# platform.
#
# Where possible, we use a transition to set the
# command line option implicitly, rather than the
# user needing to specify it explicitly.

load("@rules_rust//rust:defs.bzl", "rust_stdlib_filegroup")
load("//bazel:copy_file.bzl", "copy_file")
load("//bazel/cross-compiling:x86_64_cc_toolchain.bzl", "x86_64_cc_toolchain")
load("//bazel/cross-compiling:x86_64_rust_toolchain.bzl", "x86_64_rust_toolchain")

# Package the core, compiler-builtins,
# and alloc crates.

copy_file(
    name = "liballoc.rlib",
    src = "@rust_none_x86_64//:liballoc",
)

copy_file(
    name = "libcompiler_builtins.rlib",
    src = "@rust_none_x86_64//:libcompiler_builtins",
)

copy_file(
    name = "libcore.rlib",
    src = "@rust_none_x86_64//:libcore",
)

copy_file(
    name = "librustc_std_workspace_core.rlib",
    src = "@rust_none_x86_64//:librustc_std_workspace_core",
)

rust_stdlib_filegroup(
    name = "bare_metal_base",
    srcs = [
        ":liballoc.rlib",
        ":libcompiler_builtins.rlib",
        ":libcore.rlib",
        ":librustc_std_workspace_core.rlib",
    ],
)

# Set up our toolchains now that we have
# all the underlying pieces.

platform(
    name = "x86_64_bare_metal",
    constraint_values = [
        "@platforms//cpu:x86_64",
        "@platforms//os:none",
    ],
    visibility = ["//visibility:public"],
)

x86_64_rust_toolchain(
    name = "x86_64_rust_toolchain",
    stdlib_filegroup = ":bare_metal_base",
    target_triple = "x86_64-unknown-none",
)

x86_64_cc_toolchain(
    name = "x86_64_cc_toolchain",
)
