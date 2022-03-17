# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test", "rustfmt_test")

rust_library(
    name = "bitmap_index",
    srcs = ["lib.rs"],
    crate_name = "bitmap_index",
    edition = "2021",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "//kernel/utils/align",
        "@crates//:x86_64",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":bitmap_index"],
)

rust_doc(
    name = "docs",
    crate = ":bitmap_index",
)

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":bitmap_index"],
)

rust_test(
    name = "bitmap_index_test",
    size = "small",
    crate = ":bitmap_index",
    deps = [
        "//kernel/utils/align",
        "@crates//:x86_64",
    ],
)
