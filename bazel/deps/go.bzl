# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

GO_VERSION = "1.18.1"

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
        sum = "h1:ksErzDEI1khOiGPgpwuI7x2ebx/uXQNw7xJpn9Eq1+I=",
        version = "v1.1.0",
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
        sum = "h1:+gHMid33q6pen7kv9xvT+JRinntgeXO2AeZVd0AWD3w=",
        version = "v0.0.0-20220411224347-583f2d630306",
    )

    go_repository(
        name = "rsc_io_diff",
        importpath = "rsc.io/diff",
        sum = "h1:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ=",
        version = "v0.0.0-20190621135850-fe3479844c3c",
    )
