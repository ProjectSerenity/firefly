# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test", "rustfmt_test")

rust_library(
    name = "loader",
    srcs = [
        "elf.rs",
        "lib.rs",
    ],
    crate_name = "loader",
    edition = "2021",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "//kernel/memory/memlayout",
        "@crates//:x86_64",
        "@crates//:xmas-elf",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":loader"],
)

rust_doc(
    name = "docs",
    crate = ":loader",
)

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":loader"],
)

rust_test(
    name = "loader_test",
    size = "small",
    compile_data = glob(["testdata/*"]),
    crate = ":loader",
    proc_macro_deps = [
        "@crates//:hex-literal",
    ],
)
