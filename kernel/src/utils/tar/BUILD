# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rust_test")

rust_library(
    name = "tar",
    srcs = [
        "lib.rs",
        "parse_utils.rs",
    ],
    crate_name = "tar",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "//kernel/src/drivers/serial",
        "//kernel/src/filesystem",
        "//kernel/src/storage",
        "//kernel/src/utils/align",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":tar"],
)

rust_doc(
    name = "docs",
    crate = ":tar",
)

rust_test(
    name = "tar_test",
    size = "small",
    compile_data = glob(["testdata/*.tar"]),
    crate = ":tar",
    proc_macro_deps = [
        crate("hex-literal"),
    ],
    deps = [
        crate("sha2"),
    ],
)
