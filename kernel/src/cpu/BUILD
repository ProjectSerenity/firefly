# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library")

rust_library(
    name = "cpu",
    srcs = [
        "lib.rs",
        "local.rs",
    ],
    crate_name = "cpu",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        crate("lazy_static"),
        crate("raw-cpuid"),
        crate("x86_64"),
        "//kernel/src/drivers/serial",
        "//kernel/src/memory/memlayout",
        "//kernel/src/memory/physmem",
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
