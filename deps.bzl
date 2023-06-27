# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

go = [
    module(
        name = "github.com/bazelbuild/buildtools",
        version = "v0.0.0-20230510134650-37bd1811516d",
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
        name = "github.com/google/go-cmp",
        version = "v0.5.9",
        packages = [
            package(
                name = "github.com/google/go-cmp/cmp",
                deps = [
                    "github.com/google/go-cmp/cmp/internal/diff",
                    "github.com/google/go-cmp/cmp/internal/flags",
                    "github.com/google/go-cmp/cmp/internal/function",
                    "github.com/google/go-cmp/cmp/internal/testprotos",
                    "github.com/google/go-cmp/cmp/internal/teststructs",
                    "github.com/google/go-cmp/cmp/internal/value",
                ],
                test_deps = [
                    "github.com/google/go-cmp/cmp/cmpopts",
                    "github.com/google/go-cmp/cmp/internal/teststructs/foo1",
                    "github.com/google/go-cmp/cmp/internal/teststructs/foo2",
                ],
            ),
            package(
                name = "github.com/google/go-cmp/cmp/cmpopts",
                deps = [
                    "github.com/google/go-cmp/cmp",
                    "github.com/google/go-cmp/cmp/internal/function",
                ],
                test_deps = [
                    "github.com/google/go-cmp/cmp/internal/flags",
                ],
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/diff",
                deps = [
                    "github.com/google/go-cmp/cmp/internal/flags",
                ],
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/flags",
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/function",
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/testprotos",
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/teststructs",
                deps = [
                    "github.com/google/go-cmp/cmp/internal/testprotos",
                ],
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/teststructs/foo1",
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/teststructs/foo2",
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/value",
                test_deps = [
                    "github.com/google/go-cmp/cmp",
                ],
            ),
        ],
    ),
    module(
        name = "github.com/google/osv-scanner",
        version = "v1.3.4",
        packages = [
            package(
                name = "github.com/google/osv-scanner/pkg/models",
                deps = [
                    "github.com/google/go-cmp/cmp",
                    "golang.org/x/exp/slices",
                ],
            ),
            package(
                name = "github.com/google/osv-scanner/pkg/osv",
                deps = [
                    "github.com/google/osv-scanner/pkg/models",
                    "golang.org/x/sync/semaphore",
                ],
            ),
        ],
        patch_args = ["-p1"],
        patches = [
            "bazel/patches/github.com_google_osv-scanner_pkg_osv_osv.go",
        ],
    ),
    module(
        name = "golang.org/x/crypto",
        version = "v0.10.0",
        packages = [
            package(
                name = "golang.org/x/crypto/ed25519",
            ),
        ],
    ),
    module(
        name = "golang.org/x/exp",
        version = "v0.0.0-20230626212559-97b1e661b5df",
        packages = [
            package(
                name = "golang.org/x/exp/constraints",
            ),
            package(
                name = "golang.org/x/exp/slices",
                deps = [
                    "golang.org/x/exp/constraints",
                ],
            ),
        ],
    ),
    module(
        name = "golang.org/x/mod",
        version = "v0.11.0",
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
        name = "golang.org/x/sync",
        version = "v0.3.0",
        packages = [
            package(
                name = "golang.org/x/sync/errgroup",
            ),
            package(
                name = "golang.org/x/sync/semaphore",
                test_deps = [
                    "golang.org/x/sync/errgroup",
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
        version = "v0.10.0",
        packages = [
            package(
                name = "golang.org/x/tools/txtar",
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
    module(
        name = "rsc.io/pdf",
        version = "v0.1.1",
        packages = [
            package(
                name = "rsc.io/pdf",
            ),
        ],
    ),
]
