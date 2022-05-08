# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")
load("//bazel/gofmt:gofmt.bzl", "gofmt")

go_library(
    name = "plan_lib",
    srcs = [
        "build.go",
        "docs.go",
        "format.go",
        "main.go",
        "rust.go",
    ],
    embedsrcs = glob(["templates/**"]),
    importpath = "github.com/ProjectSerenity/firefly/tools/plan",
    visibility = ["//visibility:private"],
    deps = [
        "//tools/plan/ast",
        "//tools/plan/format",
        "//tools/plan/lexer",
        "//tools/plan/parser",
        "//tools/plan/token",
        "//tools/plan/types",
    ],
)

gofmt(
    name = "gofmt",
    embed = [":plan_lib"],
)

go_binary(
    name = "plan",
    embed = [":plan_lib"],
    visibility = ["//visibility:public"],
)

go_test(
    name = "plan_test",
    size = "small",
    srcs = [
        "rust_test.go",
    ],
    data = glob(["testdata/**"]) + ["@rust_linux_x86_64//:rustfmt"],
    embed = [":plan_lib"],
    env = {
        "rustfmt": "$(locations @rust_linux_x86_64//:rustfmt)",
    },
    deps = [
        "//tools/plan/parser",
    ],
)
