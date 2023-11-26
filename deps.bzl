# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

go = [
    module(
        name = "github.com/google/go-cmp",
        version = "v0.6.0",
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
        name = "golang.org/x/crypto",
        version = "v0.15.0",
        packages = [
            package(
                name = "golang.org/x/crypto/cryptobyte/asn1",
            ),
            package(
                name = "golang.org/x/crypto/cryptobyte",
                deps = [
                    "golang.org/x/crypto/cryptobyte/asn1",
                ],
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
