# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@io_bazel_rules_go//go:def.bzl", "go_library")
load(":build_defs.bzl", "go_yacc")

go_yacc(
    src = "parse.y",
    out = "parse.y.baz.go",
)

go_library(
    name = "build",
    srcs = glob(
        include = ["*.go"],
        exclude = ["*_test.go"],
    ),
    importpath = "github.com/bazelbuild/buildtools/build",
    tags = [
        "manual",  # Only build when needed as a dependency.
    ],
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go/github.com/bazelbuild/buildtools/tables",
        "//vendor/go/github.com/bazelbuild/buildtools/testutils",
    ],
)

# Remove the tests, as they don't play nicely when vendored into another Bazel workspace.
