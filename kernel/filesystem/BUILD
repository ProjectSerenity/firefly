# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library")

rust_library(
    name = "filesystem",
    srcs = ["lib.rs"],
    crate_name = "filesystem",
    edition = "2021",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "@crates//:bitflags",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":filesystem"],
)

rust_doc(
    name = "docs",
    crate = ":filesystem",
)
