# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

GO_VERSION = "1.18.3"

def go_deps():
    go_rules_dependencies()

    go_register_toolchains(
        nogo = "@//:nogo",
        version = GO_VERSION,
    )
