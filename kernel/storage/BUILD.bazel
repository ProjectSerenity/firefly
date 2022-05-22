# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rustfmt_test")

rust_library(
    name = "storage",
    srcs = [
        "block/mod.rs",
        "lib.rs",
    ],
    crate_name = "storage",
    edition = "2021",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "//kernel/utils/spin",
        "@crates//:bitflags",
        "@crates//:x86_64",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":storage"],
)

rust_doc(
    name = "docs",
    crate = ":storage",
)

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":storage"],
)
