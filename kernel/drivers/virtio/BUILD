# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library")

rust_library(
    name = "virtio",
    srcs = [
        "block/cache.rs",
        "block/mod.rs",
        "entropy/mod.rs",
        "features.rs",
        "lib.rs",
        "network/mod.rs",
        "transports/mod.rs",
        "transports/pci.rs",
        "virtqueues/mod.rs",
        "virtqueues/split.rs",
    ],
    crate_name = "virtio",
    edition = "2021",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "//kernel/drivers/pci",
        "//kernel/drivers/serial",
        "//kernel/interrupts",
        "//kernel/memory/memlayout",
        "//kernel/memory/mmio",
        "//kernel/memory/physmem",
        "//kernel/memory/virtmem",
        "//kernel/multitasking",
        "//kernel/network",
        "//kernel/random",
        "//kernel/storage",
        "//kernel/utils/align",
        "//kernel/utils/bitmap_index",
        "//kernel/utils/spin",
        "@crates//:bitflags",
        "@crates//:smoltcp",
        "@crates//:x86_64",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":virtio"],
)

rust_doc(
    name = "docs",
    crate = ":virtio",
)
