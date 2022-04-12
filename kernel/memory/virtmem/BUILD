# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test", "rustfmt_test")

rust_library(
    name = "virtmem",
    srcs = [
        "bitmap.rs",
        "heap/fixed_size_block.rs",
        "heap/mod.rs",
        "lib.rs",
        "mapping.rs",
        "translate.rs",
    ],
    crate_name = "virtmem",
    edition = "2021",
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

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":virtmem"],
)

rust_test(
    name = "virtmem_test",
    size = "small",
    crate = ":virtmem",
)
