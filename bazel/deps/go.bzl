# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

GO_VERSION = "1.18"

def go_deps():
    go_rules_dependencies()

    go_register_toolchains(
        nogo = "@//:nogo",
        version = GO_VERSION,
    )

    gazelle_dependencies()

    # Fetch our Go dependencies for tools.

    go_repository(
        name = "com_github_BurntSushi_toml",
        importpath = "github.com/BurntSushi/toml",
        sum = "h1:dtDWrepsVPfW9H/4y7dDgFc2MBUSeJhlaDtK13CxFlU=",
        version = "v1.0.0",
    )

    go_repository(
        name = "org_golang_x_mod",
        importpath = "golang.org/x/mod",
        build_external = "external",
        sum = "h1:OJxoQ/rynoF0dcCdI7cLPktw/hR2cueqYfjm43oqK38=",
        version = "v0.5.1",
    )

    go_repository(
        name = "org_golang_x_time",
        importpath = "golang.org/x/time",
        sum = "h1:M73Iuj3xbbb9Uk1DYhzydthsj6oOd6l9bpuFcNoUvTs=",
        version = "v0.0.0-20220224211638-0e9765cccd65",
    )
