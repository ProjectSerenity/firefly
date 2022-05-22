# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")
load("//bazel/gofmt:gofmt.bzl", "gofmt")

go_library(
    name = "update-deps_lib",
    srcs = [
        "go.go",
        "main.go",
        "rules.go",
        "rust.go",
    ],
    importpath = "github.com/ProjectSerenity/firefly/tools/update-deps",
    visibility = ["//visibility:private"],
    deps = [
        "@com_github_bazelbuild_buildtools//build:go_default_library",
        "@com_github_burntsushi_toml//:toml",
        "@org_golang_x_mod//module",
        "@org_golang_x_mod//semver",
        "@org_golang_x_mod//sumdb",
        "@org_golang_x_time//rate",
    ],
)

gofmt(
    name = "gofmt",
    embed = [":update-deps_lib"],
)

go_binary(
    name = "update-deps",
    embed = [":update-deps_lib"],
    visibility = ["//visibility:public"],
)

go_test(
    name = "update-deps_test",
    size = "small",
    srcs = [
        "fields_test.go",
        "go_test.go",
        "rules_test.go",
        "rust_test.go",
    ],
    data = glob(["testdata/*"]),
    embed = [":update-deps_lib"],
    deps = [
        "@com_github_bazelbuild_buildtools//build:go_default_library",
        "@org_golang_x_time//rate",
    ],
)
