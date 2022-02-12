# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test")

rust_library(
    name = "bitmap_index",
    srcs = ["lib.rs"],
    crate_name = "bitmap_index",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        crate("x86_64"),
        "//kernel/utils/align",
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

rust_test(
    name = "bitmap_index_test",
    size = "small",
    crate = ":bitmap_index",
    deps = [
        crate("x86_64"),
        "//kernel/utils/align",
    ],
)
