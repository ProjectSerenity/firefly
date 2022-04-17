# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rustfmt_test")

rust_library(
    name = "syscalls",
    srcs = ["lib.rs"],
    crate_name = "syscalls",
    edition = "2021",
    visibility = ["//visibility:public"],
)

rust_clippy(
    name = "syscalls_clippy",
    deps = [":syscalls"],
)

rust_doc(
    name = "syscalls_doc",
    crate = ":syscalls",
)

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":syscalls"],
)
