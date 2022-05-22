# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")
load("//bazel/gofmt:gofmt.bzl", "gofmt")

go_library(
    name = "bootimage_lib",
    srcs = ["main.go"],
    importpath = "github.com/ProjectSerenity/firefly/tools/bootimage",
    visibility = ["//visibility:private"],
)

gofmt(
    name = "gofmt",
    embed = [":bootimage_lib"],
)

go_binary(
    name = "bootimage",
    embed = [":bootimage_lib"],
    visibility = ["//visibility:public"],
)
