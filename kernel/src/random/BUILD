# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test")

rust_library(
    name = "random",
    srcs = [
        "csprng.rs",
        "lib.rs",
        "rdrand.rs",
    ],
    crate_name = "random",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        crate("chacha20"),
        crate("sha2"),
        crate("spin"),
        crate("x86_64"),
        "//kernel/src/time",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":random"],
)

rust_doc(
    name = "docs",
    crate = ":random",
)

rust_test(
    name = "random_test",
    size = "small",
    crate = ":random",
    proc_macro_deps = [
        crate("hex-literal"),
    ],
)
