# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test", "rustfmt_test")

rust_library(
    name = "heap",
    srcs = [
        "fixed_size_block.rs",
        "lib.rs",
    ],
    crate_name = "heap",
    edition = "2021",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "//kernel/memory",
        "//kernel/memory/virtmem",
        "//kernel/utils/spin",
        "@crates//:linked_list_allocator",
        "@crates//:x86_64",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":heap"],
)

rust_doc(
    name = "docs",
    crate = ":heap",
)

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":heap"],
)

rust_test(
    name = "heap_test",
    size = "small",
    crate = ":heap",
)
