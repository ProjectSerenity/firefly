# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("//tools/plan:defs.bzl", "plan_docs", "plan_format", "plan_library")

plan_library(
    name = "syscalls",
    src = "x86_64.plan",
    arch = "x86-64",
    visibility = ["//visibility:public"],
)

plan_docs(
    name = "docs",
    embed = ":syscalls",
)

plan_format(
    name = "syscalls_format",
    embed = ":syscalls",
)
