# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

workspace(name = "firefly")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Fetch external dependencies.

# Used by buildifier.
http_archive(
    name = "io_bazel_rules_go",
    sha256 = "2b1641428dff9018f9e85c0384f03ec6c10660d935b750e3fa1492a281a53b0f",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.29.0/rules_go-v0.29.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.29.0/rules_go-v0.29.0.zip",
    ],
)

# Used by buildifier.
http_archive(
    name = "com_google_protobuf",
    sha256 = "f94faa42d49c0450226d1e9700ab5f5c3d8e5b757df41bc741bd304fd353eb63",
    strip_prefix = "protobuf-3.15.5",
    urls = ["https://github.com/protocolbuffers/protobuf/archive/v3.15.5.zip"],
)

# For buildifier.
http_archive(
    name = "com_github_bazelbuild_buildtools",
    sha256 = "932160d5694e688cb7a05ac38efba4b9a90470c75f39716d85fb1d2f95eec96d",
    strip_prefix = "buildtools-4.0.1",
    urls = ["https://github.com/bazelbuild/buildtools/archive/4.0.1.zip"],
)

http_archive(
    name = "bazel_skylib",
    sha256 = "c6966ec828da198c5d9adbaa94c05e3a1c7f21bd012a0b29ba8ddbccb2c93b0d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.1.1/bazel-skylib-1.1.1.tar.gz",
        "https://github.com/bazelbuild/bazel-skylib/releases/download/1.1.1/bazel-skylib-1.1.1.tar.gz",
    ],
)

# Initialise external dependencies.

load("@bazel_skylib//:workspace.bzl", "bazel_skylib_workspace")

bazel_skylib_workspace()

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(
    version = "1.17",
)
