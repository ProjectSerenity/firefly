# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

go = [
    module(
        name = "github.com/bazelbuild/buildtools",
        version = "v0.0.0-20230113180850-180a94ab3a3a",
        packages = [
            package(
                name = "github.com/bazelbuild/buildtools/build",
                build_file = "bazel/patches/github.com_bazelbuild_buildtools_build.BUILD",
            ),
            package(
                name = "github.com/bazelbuild/buildtools/tables",
                no_tests = True,  # The tests don't play nicely when vendored into another Bazel workspace.
            ),
            package(
                name = "github.com/bazelbuild/buildtools/testutils",
            ),
        ],
    ),
    module(
        name = "golang.org/x/crypto",
        version = "v0.5.0",
        packages = [
            package(
                name = "golang.org/x/crypto/ed25519",
            ),
        ],
    ),
    module(
        name = "golang.org/x/mod",
        version = "v0.7.0",
        packages = [
            package(
                name = "golang.org/x/mod/internal/lazyregexp",
            ),
            package(
                name = "golang.org/x/mod/module",
                deps = [
                    "golang.org/x/mod/internal/lazyregexp",
                    "golang.org/x/mod/semver",
                    "golang.org/x/xerrors",
                ],
            ),
            package(
                name = "golang.org/x/mod/semver",
            ),
            package(
                name = "golang.org/x/mod/sumdb",
                deps = [
                    "golang.org/x/mod/internal/lazyregexp",
                    "golang.org/x/mod/module",
                    "golang.org/x/mod/sumdb/note",
                    "golang.org/x/mod/sumdb/tlog",
                ],
                test_deps = [
                    "golang.org/x/mod/sumdb/note",
                    "golang.org/x/mod/sumdb/tlog",
                ],
            ),
            package(
                name = "golang.org/x/mod/sumdb/dirhash",
            ),
            package(
                name = "golang.org/x/mod/sumdb/note",
                deps = [
                    "golang.org/x/crypto/ed25519",
                ],
                test_deps = [
                    "golang.org/x/crypto/ed25519",
                ],
            ),
            package(
                name = "golang.org/x/mod/sumdb/tlog",
            ),
            package(
                name = "golang.org/x/mod/zip",
                deps = [
                    "golang.org/x/mod/module",
                ],
                test_size = "medium",
                test_deps = [
                    "golang.org/x/mod/module",
                    "golang.org/x/mod/sumdb/dirhash",
                    "golang.org/x/tools/txtar",
                ],
            ),
        ],
    ),
    module(
        name = "golang.org/x/time",
        version = "v0.3.0",
        packages = [
            package(
                name = "golang.org/x/time/rate",
            ),
        ],
    ),
    module(
        name = "golang.org/x/tools",
        version = "v0.5.0",
        packages = [
            package(
                name = "golang.org/x/tools/txtar",
            ),
        ],
    ),
    module(
        name = "golang.org/x/vuln",
        version = "v0.0.0-20230118164824-4ec8867cc0e6",
        packages = [
            package(
                name = "golang.org/x/vuln/internal/semver",
            ),
            package(
                name = "golang.org/x/vuln/osv",
                deps = [
                    "golang.org/x/mod/semver",
                    "golang.org/x/vuln/internal/semver",
                ],
            ),
        ],
    ),
    module(
        name = "golang.org/x/xerrors",
        version = "v0.0.0-20220907171357-04be3eba64a2",
        packages = [
            package(
                name = "golang.org/x/xerrors",
                deps = [
                    "golang.org/x/xerrors/internal",
                ],
            ),
            package(
                name = "golang.org/x/xerrors/internal",
            ),
        ],
    ),
    module(
        name = "rsc.io/diff",
        version = "v0.0.0-20190621135850-fe3479844c3c",
        packages = [
            package(
                name = "rsc.io/diff",
            ),
        ],
    ),
]
