# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test", "rustfmt_test")

rust_library(
    name = "memory",
    srcs = [
        "constants.rs",
        "lib.rs",
        "phys_addr.rs",
        "phys_frame.rs",
        "phys_range.rs",
        "virt_addr.rs",
        "virt_page.rs",
        "virt_range.rs",
    ],
    crate_name = "memory",
    edition = "2021",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "//kernel/utils/align",
        "@crates//:x86_64",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":memory"],
)

rust_doc(
    name = "docs",
    crate = ":memory",
)

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":memory"],
)

rust_test(
    name = "memory_test",
    size = "small",
    crate = ":memory",
)
