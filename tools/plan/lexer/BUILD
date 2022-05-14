# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")
load("//bazel/gofmt:gofmt.bzl", "gofmt")

go_library(
    name = "lexer",
    srcs = ["lexer.go"],
    importpath = "github.com/ProjectSerenity/firefly/tools/plan/lexer",
    visibility = ["//visibility:public"],
    deps = [
        "//tools/plan/token",
    ],
)

gofmt(
    name = "gofmt",
    embed = [":lexer"],
)

go_test(
    name = "lexer_test",
    size = "small",
    srcs = ["lexer_test.go"],
    data = glob(["testdata/**"]),
    embed = [":lexer"],
    deps = [
        "@rsc_io_diff//:diff",
    ],
)
