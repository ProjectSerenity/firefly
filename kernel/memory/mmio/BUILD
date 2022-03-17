# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rustfmt_test")

rust_library(
    name = "mmio",
    srcs = ["lib.rs"],
    crate_name = "mmio",
    edition = "2021",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "//kernel/memory/memlayout",
        "//kernel/memory/physmem",
        "//kernel/memory/virtmem",
        "//kernel/utils/spin",
        "@crates//:x86_64",
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

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":mmio"],
)
