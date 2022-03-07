# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library")

rust_library(
    name = "segmentation",
    srcs = ["lib.rs"],
    crate_name = "segmentation",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "//kernel/cpu",
        "//kernel/utils/spin",
        "@crates//:lazy_static",
        "@crates//:x86_64",
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
