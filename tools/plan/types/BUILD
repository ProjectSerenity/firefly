# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")
load("//bazel/gofmt:gofmt.bzl", "gofmt")

go_library(
    name = "types",
    srcs = [
        "interpret.go",
        "types.go",
    ],
    importpath = "github.com/ProjectSerenity/firefly/tools/plan/types",
    visibility = ["//visibility:public"],
    deps = [
        "//tools/plan/ast",
        "//tools/plan/token",
    ],
)

gofmt(
    name = "gofmt",
    embed = [":types"],
)

go_test(
    name = "types_test",
    size = "small",
    srcs = [
        "interpret_test.go",
        "types_test.go",
    ],
    embed = [":types"],
    deps = [
        "//tools/plan/parser",
    ],
)
