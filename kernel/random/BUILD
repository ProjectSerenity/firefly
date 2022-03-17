# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test", "rustfmt_test")

rust_library(
    name = "random",
    srcs = [
        "csprng.rs",
        "lib.rs",
        "rdrand.rs",
    ],
    crate_name = "random",
    edition = "2021",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "//kernel/time",
        "//kernel/utils/spin",
        "@crates//:chacha20",
        "@crates//:sha2",
        "@crates//:x86_64",
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

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":random"],
)

rust_test(
    name = "random_test",
    size = "small",
    crate = ":random",
    proc_macro_deps = [
        "@crates//:hex-literal",
    ],
)
