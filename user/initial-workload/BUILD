# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_cc//cc:defs.bzl", "cc_binary")
load("//bazel/cross-compiling:config-transition.bzl", "x86_64_bare_metal_rust_binary")

cc_binary(
    name = "binary",
    srcs = ["main.S"],
    features = ["fully_static_link"],
    linkopts = ["-nostdlib"],
    linkstatic = True,
    visibility = ["//visibility:private"],
)

x86_64_bare_metal_rust_binary(
    name = "initial-workload",
    rust_binary = ":binary",
    visibility = ["//visibility:public"],
)
