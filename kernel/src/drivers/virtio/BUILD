# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
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
    visibility = ["//kernel:__subpackages__"],
    deps = [
        crate("bitflags"),
        crate("spin"),
        crate("x86_64"),
        "//kernel/src/drivers/pci",
        "//kernel/src/drivers/serial",
        "//kernel/src/interrupts",
        "//kernel/src/memory/memlayout",
        "//kernel/src/memory/mmio",
        "//kernel/src/memory/physmem",
        "//kernel/src/memory/virtmem",
        "//kernel/src/multitasking/thread",
        "//kernel/src/network",
        "//kernel/src/random",
        "//kernel/src/storage",
        "//kernel/src/utils/align",
        "//kernel/src/utils/bitmap_index",
        "@smoltcp",
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
