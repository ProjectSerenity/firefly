# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rustfmt_test")

rust_library(
    name = "network",
    srcs = [
        "device.rs",
        "lib.rs",
        "tcp.rs",
        "udp.rs",
    ],
    crate_name = "network",
    edition = "2021",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "//kernel/drivers/serial",
        "//kernel/memory/memlayout",
        "//kernel/multitasking",
        "//kernel/random",
        "//kernel/time",
        "//kernel/utils/spin",
        "@crates//:managed",
        "@crates//:smoltcp",
        "@crates//:x86_64",
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

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":network"],
)
