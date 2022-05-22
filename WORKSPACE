# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

workspace(name = "firefly")

load("//bazel/deps:rules.bzl", "rules_deps")

# Fetch external dependencies.

rules_deps()

# Initialise external dependencies.

load("@bazel_skylib//:workspace.bzl", "bazel_skylib_workspace")

bazel_skylib_workspace()

load("@rules_cc//cc:repositories.bzl", "rules_cc_dependencies", "rules_cc_toolchains")

rules_cc_dependencies()

rules_cc_toolchains()

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

load("//bazel/deps:defs.bzl", "go_deps", "rust_deps")

rust_deps()

go_deps()

# Register our cross-compiling toolchains.
register_toolchains(
    "//bazel/cross-compiling:x86_64_cc_toolchain",
    "//bazel/cross-compiling:x86_64_rust_toolchain",
)
