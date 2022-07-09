# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@rules_rust//rust:repositories.bzl", "rules_rust_dependencies", "rust_register_toolchains")

RUST_VERSION = "nightly"

RUST_ISO_DATE = "2022-07-01"

LLVM_TOOLS = struct(
    name = "llvm-tools-nightly-x86_64-unknown-linux-gnu",
    sum = "d4ac6f2a3f63ac3efd96305c7a96938195aa6af3fb455f9ecffaf1b8652792ed",
)

RUST = struct(
    name = "rust-nightly-x86_64-unknown-linux-gnu",
    sum = "153135ef5bc6f276d84d368e83417f32efdeb684e9ea18280e2be1570ca2fe46",
)

RUST_SRC = struct(
    name = "rust-src-nightly",
    sum = "3c9d57b8c8591a92f9e17fd2f1cbea511760a897908f25998234e7e12aa6a310",
)

RUST_STD = struct(
    name = "rust-std-nightly-x86_64-unknown-linux-gnu",
    sum = "424f0055b8f838dbe5c97228cea9c0b17faaecd1b76e92eb5d3cd469f0fe7886",
)

RUST_RUSTFMT = struct(
    name = "rustfmt-nightly-x86_64-unknown-linux-gnu",
    sum = "d70de1e5ab40f129d839194caccb351bcc056ef91d4abb9d98a4862aef591540",
)

RUST_NO_STD = struct(
    name = "rust-std-nightly-x86_64-unknown-none",
    sum = "d786fe0d2d79061e77213e6d535bd7ba7682e164c92df85607eee24346a9a1ed",
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
