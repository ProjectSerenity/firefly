# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_binary", "rust_clippy")

rust_binary(
    name = "bootimage",
    srcs = ["main.rs"],
    crate_root = "main.rs",
    edition = "2021",
    visibility = ["//visibility:public"],
    deps = [
        "@crates//:llvm-tools",
        "@crates//:thiserror",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":bootimage"],
)
