# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test", "rustfmt_test")

rust_library(
    name = "physmem",
    srcs = [
        "bitmap.rs",
        "boot_info.rs",
        "lib.rs",
    ],
    crate_name = "physmem",
    edition = "2021",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "//kernel/drivers/serial",
        "//kernel/utils/bitmap_index",
        "//kernel/utils/pretty",
        "//kernel/utils/spin",
        "@bootloader",
        "@crates//:lazy_static",
        "@crates//:x86_64",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":physmem"],
)

rust_doc(
    name = "docs",
    crate = ":physmem",
)

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":physmem"],
)

rust_test(
    name = "physmem_test",
    size = "small",
    crate = ":physmem",
)
