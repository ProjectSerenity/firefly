# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library")

rust_library(
    name = "interrupts",
    srcs = [
        "irq.rs",
        "lib.rs",
    ],
    crate_name = "interrupts",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        crate("lazy_static"),
        crate("pic8259"),
        crate("x86_64"),
        "//kernel/drivers/serial",
        "//kernel/segmentation",
        "//kernel/utils/spin",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":interrupts"],
)

rust_doc(
    name = "docs",
    crate = ":interrupts",
)
