# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

RULES_BUILDTOOLS = struct(
    name = "com_github_bazelbuild_buildtools",
    repo = "bazelbuild/buildtools",
    archive = "https://github.com/bazelbuild/buildtools/archive/{v}.zip",
    version = "5.1.0",
    sha256 = "cc09d23397bce782542b7b4ad8e8c3de484f544df2d6d9f9add9469754cfcd20",
)

RULES_CC = struct(
    name = "rules_cc",
    repo = "bazelbuild/rules_cc",
    archive = "https://github.com/bazelbuild/rules_cc/releases/download/{v}/rules_cc-{v}.tar.gz",
    version = "0.0.2",
    sha256 = "58bff40957ace85c2de21ebfc72e53ed3a0d33af8cc20abd0ceec55c63be7de2",
)

RULES_GO = struct(
    name = "io_bazel_rules_go",
    repo = "bazelbuild/rules_go",
    archive = "https://github.com/bazelbuild/rules_go/releases/download/v{v}/rules_go-v{v}.zip",
    version = "0.34.0",
    sha256 = "16e9fca53ed6bd4ff4ad76facc9b7b651a89db1689a2877d6fd7b82aa824e366",
)

RULES_LICENSE = struct(
    name = "rules_license",
    repo = "bazelbuild/rules_license",
    branch = "main",
    archive = "https://github.com/bazelbuild/rules_license/archive/{v}.tar.gz",
    version = "9922478bcd16cbe0cd47cc8d6cd7714beae7aa05",
    sha256 = "030d2989268d04cd1beeeb4938a110ea53985093de28f6111345d28dd40edebd",
)

RULES_PKG = struct(
    name = "rules_pkg",
    repo = "bazelbuild/rules_pkg",
    branch = "main",
    archive = "https://github.com/bazelbuild/rules_pkg/archive/{v}.tar.gz",
    version = "6fe8c2ed2550b51129e16dd7c80b0efd8a59129f",
    sha256 = "f4e6a05b4e57139edc5b25e917e47e2719c7a30ee7a89f4602f3d47b7373e474",
)

RULES_PROTOBUF = struct(
    name = "com_google_protobuf",
    repo = "protocolbuffers/protobuf",
    archive = "https://github.com/protocolbuffers/protobuf/archive/v{v}.zip",
    version = "21.5",
    sha256 = "468a16f50694822291da57e304197f5322607dbed1a9d93192ff18de642c6cac",
)

RULES_RUST = struct(
    name = "rules_rust",
    repo = "bazelbuild/rules_rust",
    branch = "main",
    archive = "https://github.com/bazelbuild/rules_rust/archive/{v}.tar.gz",
    version = "cfcaf21d57791bfc6b1819a17e1492b33ca43758",
    sha256 = "0cb03108219568e57fb5958d163893b7c8db345e54f35b39859ac7ebfbc5ce62",
)

RULES_SKYLIB = struct(
    name = "bazel_skylib",
    repo = "bazelbuild/bazel-skylib",
    archive = "https://github.com/bazelbuild/bazel-skylib/releases/download/{v}/bazel-skylib-{v}.tar.gz",
    version = "1.2.1",
    sha256 = "f7be3474d42aae265405a592bb7da8e171919d74c16f082a5457840f06054728",
)

def rules_deps():
    # Although the rules are sorted in the list above,
    # their order does matter here.

    http_archive(
        name = RULES_GO.name,
        sha256 = RULES_GO.sha256,
        urls = [RULES_GO.archive.format(v = RULES_GO.version)],
    )

    # Used by buildifier.
    http_archive(
        name = RULES_PROTOBUF.name,
        sha256 = RULES_PROTOBUF.sha256,
        strip_prefix = "protobuf-{v}".format(v = RULES_PROTOBUF.version),
        urls = [RULES_PROTOBUF.archive.format(v = RULES_PROTOBUF.version)],
    )

    # For buildifier.
    http_archive(
        name = RULES_BUILDTOOLS.name,
        sha256 = RULES_BUILDTOOLS.sha256,
        strip_prefix = "buildtools-{v}".format(v = RULES_BUILDTOOLS.version),
        urls = [RULES_BUILDTOOLS.archive.format(v = RULES_BUILDTOOLS.version)],
    )

    http_archive(
        name = RULES_SKYLIB.name,
        sha256 = RULES_SKYLIB.sha256,
        urls = [RULES_SKYLIB.archive.format(v = RULES_SKYLIB.version)],
    )

    # Used to build our cross-compiling toolchain.
    http_archive(
        name = RULES_CC.name,
        sha256 = RULES_CC.sha256,
        urls = [RULES_CC.archive.format(v = RULES_CC.version)],
    )

    http_archive(
        name = RULES_RUST.name,
        sha256 = RULES_RUST.sha256,
        strip_prefix = "rules_rust-{v}".format(v = RULES_RUST.version),
        urls = [RULES_RUST.archive.format(v = RULES_RUST.version)],
    )

    http_archive(
        name = RULES_LICENSE.name,
        sha256 = RULES_LICENSE.sha256,
        strip_prefix = "rules_license-{v}".format(v = RULES_LICENSE.version),
        urls = [RULES_LICENSE.archive.format(v = RULES_LICENSE.version)],
    )

    http_archive(
        name = RULES_PKG.name,
        sha256 = RULES_PKG.sha256,
        strip_prefix = "rules_pkg-{v}".format(v = RULES_PKG.version),
        urls = [RULES_PKG.archive.format(v = RULES_PKG.version)],
    )
