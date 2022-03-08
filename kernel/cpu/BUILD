# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library")

rust_library(
    name = "cpu",
    srcs = [
        "lib.rs",
        "local.rs",
    ],
    crate_name = "cpu",
    edition = "2021",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "//kernel/drivers/serial",
        "//kernel/memory/memlayout",
        "//kernel/memory/physmem",
        "//kernel/memory/virtmem",
        "//kernel/utils/spin",
        "@crates//:lazy_static",
        "@crates//:raw-cpuid",
        "@crates//:x86_64",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":cpu"],
)

rust_doc(
    name = "docs",
    crate = ":cpu",
)
