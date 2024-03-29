# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

# The x86_64_rust_toolchain macro builds a Rust toolchain
# we can use to cross-compile to a bare metal target.

load("@rules_rust//rust:toolchain.bzl", "rust_toolchain")

# x86_64_rust_toolchain takes a label to
# a LLVM target description. This is used
# to customise the toolchain.

def x86_64_rust_toolchain(name, target_triple, stdlib_filegroup):
    rust_toolchain_name = "%s_rust_toolchain" % name
    rust_toolchain(
        name = rust_toolchain_name,
        binary_ext = "",
        cargo = "@rust_linux_x86_64__x86_64-unknown-linux-gnu__nightly_tools//:cargo",
        clippy_driver = "@rust_linux_x86_64__x86_64-unknown-linux-gnu__nightly_tools//:clippy_driver_bin",
        default_edition = "2021",
        dylib_ext = ".so",
        exec_triple = "x86_64-unknown-linux-gnu",
        opt_level = {
            "dbg": "0",
            "fastbuild": "2",
            "opt": "3",
        },
        os = "none",
        rust_doc = "@rust_linux_x86_64__x86_64-unknown-linux-gnu__nightly_tools//:rustdoc",
        rust_std = stdlib_filegroup,
        rustc = "@rust_linux_x86_64__x86_64-unknown-linux-gnu__nightly_tools//:rustc",
        rustc_lib = "@rust_linux_x86_64__x86_64-unknown-linux-gnu__nightly_tools//:rustc_lib",
        rustfmt = "@rust_linux_x86_64__x86_64-unknown-linux-gnu__nightly_tools//:rustfmt_bin",
        staticlib_ext = ".a",
        stdlib_linkflags = [],
        target_triple = target_triple,
        visibility = ["//visibility:public"],
    )

    native.toolchain(
        name = name,
        exec_compatible_with = [
            "@platforms//cpu:x86_64",
            "@platforms//os:linux",
        ],
        target_compatible_with = [
            "@platforms//cpu:x86_64",
            "@platforms//os:none",
        ],
        toolchain = rust_toolchain_name,
        toolchain_type = "@rules_rust//rust:toolchain",
        visibility = ["//visibility:public"],
    )
