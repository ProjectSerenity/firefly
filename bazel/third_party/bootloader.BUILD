# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_library")

licenses([
    "notice",  # MIT from expression "MIT OR Apache-2.0"
])

# Make the target configuration available to //bazel/cross-compiling.
exports_files(
    ["x86_64-bootloader.json"],
    visibility = ["@//:__subpackages__"],
)

rust_library(
    name = "bootloader",
    srcs = glob(["**/*.rs"]),
    aliases = select({
        # Default
        "//conditions:default": {
        },
    }),
    crate_features = [
        "default",
        "map_physical_memory",
    ],
    crate_root = "src/lib.rs",
    data = glob(
        ["**"],
        exclude = [
            # These can be manually added with overrides if needed.

            # If you run `cargo build` in this dir, the target dir can get very big very quick.
            "target/**",

            # These are not vendored from the crate - we exclude them to avoid busting caches
            # when we change how we generate BUILD files and such.
            "BUILD.bazel",
            "WORKSPACE.bazel",
            "WORKSPACE",
        ],
    ),
    edition = "2018",
    rustc_flags = [
        "--cap-lints=allow",
    ],
    tags = [
        "cargo-raze",
        "manual",
    ],
    version = "0.9.21",
    visibility = ["//visibility:public"],
)
