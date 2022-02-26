# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library")

rust_library(
    name = "storage",
    srcs = [
        "block/mod.rs",
        "lib.rs",
    ],
    crate_name = "storage",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        crate("bitflags"),
        crate("x86_64"),
        "//kernel/utils/spin",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":storage"],
)

rust_doc(
    name = "docs",
    crate = ":storage",
)
