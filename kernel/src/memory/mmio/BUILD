# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library")

rust_library(
    name = "mmio",
    srcs = ["lib.rs"],
    crate_name = "mmio",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        crate("spin"),
        crate("x86_64"),
        "//kernel/src/memory/memlayout",
        "//kernel/src/memory/physmem",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":mmio"],
)

rust_doc(
    name = "docs",
    crate = ":mmio",
)
