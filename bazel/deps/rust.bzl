# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@rules_rust//rust:repositories.bzl", "rules_rust_dependencies", "rust_register_toolchains")

RUST_VERSION = "nightly"

RUST_ISO_DATE = "2022-05-01"

LLVM_TOOLS = struct(
    name = "llvm-tools-nightly-x86_64-unknown-linux-gnu",
    sum = "ea73b3a7de098affc64b12fcd604f414c0d0aca849a6d226f0c3c1a3c26a0480",
)

RUST = struct(
    name = "rust-nightly-x86_64-unknown-linux-gnu",
    sum = "107420deb243bd2346af0484e6480c3a49862db878b3acb05e28dbdb26f7d9f9",
)

RUST_SRC = struct(
    name = "rust-src-nightly",
    sum = "84d0badd8e3e05282bd1c03e28517cbe50dba540cf2cfe96ee723ae83c0175dc",
)

RUST_STD = struct(
    name = "rust-std-nightly-x86_64-unknown-linux-gnu",
    sum = "ac05606dccd8e7da4862b6b2cb2fc4224638a9a1313fdd6ee915eefa4cad54b5",
)

RUST_RUSTFMT = struct(
    name = "rustfmt-nightly-x86_64-unknown-linux-gnu",
    sum = "2d1b35c4d12310fe25c4ad36b35f90a222cf0235d249520630078eb099e7bdd7",
)

RUST_NO_STD = struct(
    name = "rust-std-nightly-x86_64-unknown-none",
    sum = "eee67cfab3e4f8f2f88975d3e2262b1b7eadc6ca32af354b4b3c7f532cc297b4",
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
