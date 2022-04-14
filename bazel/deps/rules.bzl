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

RULES_GAZELLE = struct(
    name = "bazel_gazelle",
    repo = "bazelbuild/bazel-gazelle",
    archive = "https://github.com/bazelbuild/bazel-gazelle/releases/download/v{v}/bazel-gazelle-v{v}.tar.gz",
    version = "0.24.0",
    sha256 = "de69a09dc70417580aabf20a28619bb3ef60d038470c7cf8442fafcf627c21cb",
)

RULES_GO = struct(
    name = "io_bazel_rules_go",
    repo = "bazelbuild/rules_go",
    archive = "https://github.com/bazelbuild/rules_go/releases/download/v{v}/rules_go-v{v}.zip",
    version = "0.31.0",
    sha256 = "f2dcd210c7095febe54b804bb1cd3a58fe8435a909db2ec04e31542631cf715c",
)

RULES_LICENSE = struct(
    name = "rules_license",
    repo = "bazelbuild/rules_license",
    branch = "main",
    archive = "https://github.com/bazelbuild/rules_license/archive/{v}.tar.gz",
    version = "4a20d4e66ffefd3c0db5b72f7a7a5ea69c9d6806",
    sha256 = "12c6c87972d251a19bad450ad81b3c3a4ff69280f979d24c5a84798af03282ef",
)

RULES_PKG = struct(
    name = "rules_pkg",
    repo = "bazelbuild/rules_pkg",
    archive = "https://github.com/bazelbuild/rules_pkg/releases/download/{v}/rules_pkg-{v}.tar.gz",
    version = "0.7.0",
    sha256 = "8a298e832762eda1830597d64fe7db58178aa84cd5926d76d5b744d6558941c2",
)

RULES_PROTOBUF = struct(
    name = "com_google_protobuf",
    repo = "protocolbuffers/protobuf",
    archive = "https://github.com/protocolbuffers/protobuf/archive/v{v}.zip",
    version = "3.20.0",
    sha256 = "6bd1e17170ccb166029a1a5857e864020a18ed019a63fa8f1ac3b605f7be4466",
)

RULES_RUST = struct(
    name = "rules_rust",
    repo = "bazelbuild/rules_rust",
    branch = "main",
    archive = "https://github.com/bazelbuild/rules_rust/archive/{v}.tar.gz",
    version = "c3f56c2d50c29c97c513f158bcb1dfef1bd52f1e",
    sha256 = "ab3de8c51a210bae2987bd0d3379b84d50c0e053219fad14969a93a842f1eabc",
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

    http_archive(
        name = RULES_GAZELLE.name,
        sha256 = RULES_GAZELLE.sha256,
        urls = [RULES_GAZELLE.archive.format(v = RULES_GAZELLE.version)],
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
        urls = [RULES_PKG.archive.format(v = RULES_PKG.version)],
    )
