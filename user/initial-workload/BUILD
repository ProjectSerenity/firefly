# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_pkg//:pkg.bzl", "pkg_tar")
load("@rules_rust//rust:defs.bzl", "rust_binary", "rust_clippy", "rust_doc", "rust_library", "rustfmt_test")
load("//bazel/cross-compiling:config-transition.bzl", "x86_64_bare_metal_rust_binary")

rust_library(
    name = "initial-workload_lib",
    srcs = [
        "lib.rs",
    ],
    crate_name = "initial_workload",
    edition = "2021",
    visibility = ["//visibility:private"],
)

rust_clippy(
    name = "clippy",
    deps = [":initial-workload_lib"],
)

rust_doc(
    name = "docs",
    crate = ":initial-workload_lib",
)

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":initial-workload_lib"],
)

rust_binary(
    name = "initial-workload_bin",
    srcs = ["main.rs"],
    rustc_flags = [
        "-Cpanic=abort",
        "-Clink-args=-nostartfiles -s -static",
        "-Ctarget-feature=+crt-static",
    ],
    tags = ["manual"],
    deps = [
        ":initial-workload_lib",
        "//user/firefly",
    ],
)

x86_64_bare_metal_rust_binary(
    name = "initial-workload",
    rust_binary = ":initial-workload_bin",
    visibility = ["//visibility:public"],
)

pkg_tar(
    name = "tar",
    files = {
        ":initial-workload": "initial-workload",
    },
    visibility = ["//visibility:public"],
)
