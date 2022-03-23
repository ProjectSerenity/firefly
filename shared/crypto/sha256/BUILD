# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test", "rustfmt_test")

rust_library(
    name = "sha256",
    srcs = [
        "lib.rs",
    ],
    crate_name = "sha256",
    edition = "2021",
    visibility = ["//:__subpackages__"],
)

rust_clippy(
    name = "clippy",
    deps = [":sha256"],
)

rust_doc(
    name = "docs",
    crate = ":sha256",
)

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":sha256"],
)

rust_test(
    name = "sha256_test",
    size = "small",
    crate = ":sha256",
    proc_macro_deps = [
        "@crates//:hex-literal",
    ],
)
