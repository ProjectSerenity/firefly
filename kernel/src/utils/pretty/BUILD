# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library")

rust_library(
    name = "pretty",
    srcs = [
        "bytes.rs",
        "lib.rs",
    ],
    crate_name = "pretty",
    visibility = ["//kernel:__subpackages__"],
)

rust_clippy(
    name = "clippy",
    deps = [":pretty"],
)

rust_doc(
    name = "docs",
    crate = ":pretty",
)
