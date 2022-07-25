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
    version = "0.0.1",
    sha256 = "4dccbfd22c0def164c8f47458bd50e0c7148f3d92002cdb459c2a96a68498241",
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
    version = "21fcdc447645094aecec5f5cbe4f8cc5e4984aa0",
    sha256 = "aa142aafb1d20118a35c2f41c00918d8303e56c04b23528015379186f282fef6",
)

RULES_PKG = struct(
    name = "rules_pkg",
    repo = "bazelbuild/rules_pkg",
    branch = "main",
    archive = "https://github.com/bazelbuild/rules_pkg/archive/{v}.tar.gz",
    version = "7f7bcf9c93bed9ee693b5bfedde5d72f9a2d6ea4",
    sha256 = "5909da90955dbb0eb434724f951f1f947a1794c5f33e345175a0193972aac14d",
)

RULES_PROTOBUF = struct(
    name = "com_google_protobuf",
    repo = "protocolbuffers/protobuf",
    archive = "https://github.com/protocolbuffers/protobuf/archive/v{v}.zip",
    version = "21.3",
    sha256 = "8b7178046cddb00c200c0b9920af1ffa10e06f9fb46ef66fced4a49d831f6dd9",
)

RULES_RUST = struct(
    name = "rules_rust",
    repo = "bazelbuild/rules_rust",
    branch = "main",
    archive = "https://github.com/bazelbuild/rules_rust/archive/{v}.tar.gz",
    version = "81a77acde2bf0d9a3d248806baf7ba19641135b7",
    sha256 = "0cebf89fd0f323a379c413c4567c319aa047f10ad9a5150e401fe1608f21525d",
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
