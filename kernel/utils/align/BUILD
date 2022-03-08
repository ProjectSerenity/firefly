# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test")

rust_library(
    name = "align",
    srcs = ["lib.rs"],
    crate_name = "align",
    edition = "2021",
    visibility = ["//kernel:__subpackages__"],
)

rust_clippy(
    name = "clippy",
    deps = [":align"],
)

rust_doc(
    name = "docs",
    crate = ":align",
)

rust_test(
    name = "align_test",
    size = "small",
    crate = ":align",
)
