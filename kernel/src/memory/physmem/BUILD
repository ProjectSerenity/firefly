# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test")

rust_library(
    name = "physmem",
    srcs = [
        "bitmap.rs",
        "boot_info.rs",
        "lib.rs",
    ],
    crate_name = "physmem",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        crate("lazy_static"),
        crate("spin"),
        crate("x86_64"),
        "//kernel/src/drivers/serial",
        "//kernel/src/utils/bitmap_index",
        "//kernel/src/utils/pretty",
        "@bootloader",
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

rust_test(
    name = "physmem_test",
    size = "small",
    crate = ":physmem",
)
