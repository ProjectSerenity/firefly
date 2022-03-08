# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library")

rust_library(
    name = "spin",
    srcs = [
        "lib.rs",
    ],
    crate_name = "spin",
    edition = "2018",
    visibility = ["//kernel:__subpackages__"],
)

rust_clippy(
    name = "clippy",
    deps = [":spin"],
)

rust_doc(
    name = "docs",
    crate = ":spin",
)
