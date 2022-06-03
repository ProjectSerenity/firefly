# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@rules_rust//rust:repositories.bzl", "rules_rust_dependencies", "rust_register_toolchains")

RUST_VERSION = "nightly"

RUST_ISO_DATE = "2022-06-01"

LLVM_TOOLS = struct(
    name = "llvm-tools-nightly-x86_64-unknown-linux-gnu",
    sum = "3d6fd09a80fc91b7f5fbe38c0dc46e33b43759d2c0e2596191cb9b9810f5c08a",
)

RUST = struct(
    name = "rust-nightly-x86_64-unknown-linux-gnu",
    sum = "56a40def04ad486b296a8e127b14da08005e175c9306b2556f5ae97ef2b1cda2",
)

RUST_SRC = struct(
    name = "rust-src-nightly",
    sum = "428296a1aca2bf4515c74df5d0493fa6ab25dcc5399a29fa6533da94365bcad8",
)

RUST_STD = struct(
    name = "rust-std-nightly-x86_64-unknown-linux-gnu",
    sum = "36b87e79cf917c7e779cce18cf7de6a7a9e1f34d0c69ec27d9cc942ee185864b",
)

RUST_RUSTFMT = struct(
    name = "rustfmt-nightly-x86_64-unknown-linux-gnu",
    sum = "95ffd06e467ff5c67680a6a0a9b8371935059c40c32ff83a736d73a3b79eba47",
)

RUST_NO_STD = struct(
    name = "rust-std-nightly-x86_64-unknown-none",
    sum = "75aab9b1dd91c32f73da82bbf8ce67c566b32a0acc4265ea993bc59bdc2a0bb4",
)

def rust_deps():
    # Fetch libcore, liballoc, and libcompiler-builtins
    # for the x86_64-unknown-none build target.

    http_archive(
        name = "rust_none_x86_64",
        build_file = "//third_party:no_std.BUILD",
        sha256 = RUST_NO_STD.sum,
        strip_prefix = "rust-std-nightly-x86_64-unknown-none",
        type = "tgz",
        urls = ["https://static.rust-lang.org/dist/" + RUST_ISO_DATE + "/" + RUST_NO_STD.name + ".tar.gz"],
    )

    # Set up the Rust crates we depend on. Most of these are fetched
    # using the experimental crates_repository functionality in rules_rust.

    rules_rust_dependencies()

    rust_register_toolchains(
        edition = "2021",
        iso_date = RUST_ISO_DATE,
        rustfmt_version = RUST_VERSION,
        sha256s = {
            RUST_ISO_DATE + "/" + LLVM_TOOLS.name: LLVM_TOOLS.sum,
            RUST_ISO_DATE + "/" + RUST.name: RUST.sum,
            RUST_ISO_DATE + "/" + RUST_SRC.name: RUST_SRC.sum,
            RUST_ISO_DATE + "/" + RUST_STD.name: RUST_STD.sum,
            RUST_ISO_DATE + "/" + RUST_RUSTFMT.name: RUST_RUSTFMT.sum,
        },
        version = RUST_VERSION,
    )
