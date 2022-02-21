# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library")

rust_library(
    name = "network",
    srcs = [
        "lib.rs",
        "tcp.rs",
        "udp.rs",
    ],
    crate_name = "network",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        crate("managed"),
        crate("spin"),
        crate("x86_64"),
        "//kernel/drivers/serial",
        "//kernel/memory/memlayout",
        "//kernel/multitasking",
        "//kernel/random",
        "//kernel/time",
        "@smoltcp",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":network"],
)

rust_doc(
    name = "docs",
    crate = ":network",
)
