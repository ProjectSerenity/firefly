# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")
load("//bazel/gofmt:gofmt.bzl", "gofmt")

go_library(
    name = "crate-deps_lib",
    srcs = ["main.go"],
    importpath = "github.com/ProjectSerenity/firefly/tools/crate-deps",
    visibility = ["//visibility:private"],
    deps = [
        "@com_github_bazelbuild_buildtools//build:go_default_library",
        "@com_github_bazelbuild_buildtools//labels:go_default_library",
    ],
)

gofmt(
    name = "gofmt",
    embed = [":crate-deps_lib"],
)

go_binary(
    name = "crate-deps",
    embed = [":crate-deps_lib"],
    visibility = ["//visibility:public"],
)

go_test(
    name = "crate-deps_test",
    size = "small",
    srcs = [
        "crate_build_test.go",
        "manifest_test.go",
    ],
    data = glob(["testdata/*"]),
    embed = [":crate-deps_lib"],
    deps = [
        "@com_github_bazelbuild_buildtools//build:go_default_library",
        "@com_github_bazelbuild_buildtools//labels:go_default_library",
    ],
)
