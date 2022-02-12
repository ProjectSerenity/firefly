# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test")

rust_library(
    name = "virtmem",
    srcs = [
        "bump.rs",
        "fixed_size_block.rs",
        "lib.rs",
        "linked_list.rs",
        "mapping.rs",
    ],
    crate_name = "virtmem",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        crate("linked_list_allocator"),
        crate("spin"),
        crate("x86_64"),
        "//kernel/src/drivers/serial",
        "//kernel/src/memory/memlayout",
        "//kernel/src/memory/physmem",
        "//kernel/src/utils/align",
        "//kernel/src/utils/pretty",
        "@bootloader",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":virtmem"],
)

rust_doc(
    name = "docs",
    crate = ":virtmem",
)

rust_test(
    name = "virtmem_test",
    size = "small",
    crate = ":virtmem",
)
