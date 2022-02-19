# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

RULES_BUILDTOOLS = struct(
    name = "com_github_bazelbuild_buildtools",
    repo = "bazelbuild/buildtools",
    archive = "https://github.com/bazelbuild/buildtools/archive/{v}.zip",
    version = "4.0.1",
    sha256 = "932160d5694e688cb7a05ac38efba4b9a90470c75f39716d85fb1d2f95eec96d",
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
    version = "0.30.0",
    sha256 = "d6b2513456fe2229811da7eb67a444be7785f5323c6708b38d851d2b51e54d83",
)

RULES_PROTOBUF = struct(
    name = "com_google_protobuf",
    repo = "protocolbuffers/protobuf",
    archive = "https://github.com/protocolbuffers/protobuf/archive/v{v}.zip",
    version = "3.15.5",
    sha256 = "f94faa42d49c0450226d1e9700ab5f5c3d8e5b757df41bc741bd304fd353eb63",
)

RULES_RUST = struct(
    name = "rules_rust",
    repo = "bazelbuild/rules_rust",
    archive = "https://github.com/bazelbuild/rules_rust/archive/{v}.tar.gz",
    version = "f569827113d0f1058f33da4a449ddd34be35a011",
    sha256 = "391d6a7f34c89d475e03e02f71957663c9aff6dbd8b8c974945e604828eebe11",
)

RULES_SKYLIB = struct(
    name = "bazel_skylib",
    repo = "bazelbuild/bazel-skylib",
    archive = "https://github.com/bazelbuild/bazel-skylib/releases/download/{v}/bazel-skylib-{v}.tar.gz",
    version = "1.1.1",
    sha256 = "c6966ec828da198c5d9adbaa94c05e3a1c7f21bd012a0b29ba8ddbccb2c93b0d",
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
