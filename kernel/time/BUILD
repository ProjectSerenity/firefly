# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test")

rust_library(
    name = "time",
    srcs = [
        "lib.rs",
        "rtc.rs",
        "slice.rs",
        "ticker.rs",
    ],
    crate_name = "time",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        crate("spin"),
        crate("x86_64"),
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":time"],
)

rust_doc(
    name = "docs",
    crate = ":time",
)

rust_test(
    name = "time_test",
    size = "small",
    crate = ":time",
)
