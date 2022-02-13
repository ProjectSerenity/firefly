# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test")

rust_library(
    name = "memlayout",
    srcs = [
        "addr_range.rs",
        "lib.rs",
    ],
    crate_name = "memlayout",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        crate("x86_64"),
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":memlayout"],
)

rust_doc(
    name = "docs",
    crate = ":memlayout",
)

rust_test(
    name = "memlayout_test",
    size = "small",
    crate = ":memlayout",
)
