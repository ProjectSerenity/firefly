# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test")

rust_library(
    name = "virtmem",
    srcs = [
        "bitmap.rs",
        "bump.rs",
        "fixed_size_block.rs",
        "lib.rs",
        "linked_list.rs",
        "mapping.rs",
    ],
    crate_name = "virtmem",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "//kernel/drivers/serial",
        "//kernel/memory/memlayout",
        "//kernel/memory/physmem",
        "//kernel/utils/align",
        "//kernel/utils/pretty",
        "//kernel/utils/spin",
        "@bootloader",
        "@crates//:linked_list_allocator",
        "@crates//:x86_64",
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
