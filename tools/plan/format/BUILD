# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")
load("//bazel/gofmt:gofmt.bzl", "gofmt")

go_library(
    name = "format",
    srcs = ["format.go"],
    importpath = "github.com/ProjectSerenity/firefly/tools/plan/format",
    visibility = ["//visibility:public"],
    deps = [
        "//tools/plan/ast",
        "//tools/plan/types",
    ],
)

gofmt(
    name = "gofmt",
    embed = [":format"],
)

go_test(
    name = "format_test",
    size = "small",
    srcs = ["format_test.go"],
    embed = [":format"],
    deps = [
        "//tools/plan/parser",
        "//tools/plan/types",
        "@io_rsc_diff//:diff",
    ],
)
