# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "gofmt_lib",
    srcs = ["main.go"],
    importpath = "github.com/ProjectSerenity/firefly/bazel/gofmt",
    visibility = ["//visibility:private"],
)

go_binary(
    name = "gofmt",
    embed = [":gofmt_lib"],
    visibility = ["//visibility:public"],
)
