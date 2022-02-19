# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

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
        "@com_github_BurntSushi_toml//:toml",
        "@com_github_bazelbuild_buildtools//build:go_default_library",
        "@org_golang_x_mod//module:go_default_library",
        "@org_golang_x_mod//semver:go_default_library",
        "@org_golang_x_mod//sumdb:go_default_library",
    ],
)

go_binary(
    name = "update-deps",
    embed = [":update-deps_lib"],
    visibility = ["//visibility:public"],
)
