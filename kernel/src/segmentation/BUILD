# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library")

rust_library(
    name = "segmentation",
    srcs = ["lib.rs"],
    crate_name = "segmentation",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        crate("lazy_static"),
        crate("spin"),
        crate("x86_64"),
        "//kernel/src/cpu",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":segmentation"],
)

rust_doc(
    name = "docs",
    crate = ":segmentation",
)
