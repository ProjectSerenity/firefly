# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")
load("//bazel/gofmt:gofmt.bzl", "gofmt")

go_library(
    name = "parser",
    srcs = ["parser.go"],
    importpath = "github.com/ProjectSerenity/firefly/tools/plan/parser",
    visibility = ["//visibility:public"],
    deps = [
        "//tools/plan/ast",
        "//tools/plan/lexer",
        "//tools/plan/token",
    ],
)

gofmt(
    name = "gofmt",
    embed = [":parser"],
)

go_test(
    name = "parser_test",
    size = "small",
    srcs = ["parser_test.go"],
    data = glob(["testdata/**"]),
    embed = [":parser"],
    deps = [
        "//tools/plan/ast",
        "//tools/plan/token",
        "@io_rsc_diff//:diff",
    ],
)
