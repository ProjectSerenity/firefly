# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

workspace(name = "firefly")

# Initialise external dependencies.

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "6b65cb7917b4d1709f9410ffe00ecf3e160edf674b78c54a894471320862184f",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.39.0/rules_go-v0.39.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.39.0/rules_go-v0.39.0.zip",
    ],
)

# Intel x86 manual, volume 2, version 325383-079US, March 2023.
http_file(
    name = "x86manual",
    sha256 = "bc4348020f5d5a27b0207c61e3c88d4e610eaf428658dc5d08a1cc84f98a719b",
    url = "https://cdrdv2.intel.com/v1/dl/getContent/671110",
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

GO_VERSION = "1.20.2"

go_rules_dependencies()

go_register_toolchains(
    nogo = "@//:nogo",
    version = GO_VERSION,
)
