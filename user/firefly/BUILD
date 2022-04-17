# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rustfmt_test")

rust_library(
    name = "firefly",
    srcs = [
        "lib.rs",
        "syscalls.rs",
    ],
    crate_name = "firefly",
    edition = "2021",
    visibility = ["//visibility:public"],
    deps = [
        "//shared/syscalls",
    ],
)

rust_clippy(
    name = "firefly_clippy",
    deps = [":firefly"],
)

rust_doc(
    name = "firefly_doc",
    crate = ":firefly",
)

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":firefly"],
)
