# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

workspace(name = "firefly")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Fetch external dependencies.

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "d6b2513456fe2229811da7eb67a444be7785f5323c6708b38d851d2b51e54d83",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.30.0/rules_go-v0.30.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.30.0/rules_go-v0.30.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "de69a09dc70417580aabf20a28619bb3ef60d038470c7cf8442fafcf627c21cb",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.24.0/bazel-gazelle-v0.24.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.24.0/bazel-gazelle-v0.24.0.tar.gz",
    ],
)

# Used by buildifier.
http_archive(
    name = "com_google_protobuf",
    sha256 = "f94faa42d49c0450226d1e9700ab5f5c3d8e5b757df41bc741bd304fd353eb63",
    strip_prefix = "protobuf-3.15.5",
    urls = ["https://github.com/protocolbuffers/protobuf/archive/v3.15.5.zip"],
)

# For buildifier.
http_archive(
    name = "com_github_bazelbuild_buildtools",
    sha256 = "932160d5694e688cb7a05ac38efba4b9a90470c75f39716d85fb1d2f95eec96d",
    strip_prefix = "buildtools-4.0.1",
    urls = ["https://github.com/bazelbuild/buildtools/archive/4.0.1.zip"],
)

http_archive(
    name = "bazel_skylib",
    sha256 = "c6966ec828da198c5d9adbaa94c05e3a1c7f21bd012a0b29ba8ddbccb2c93b0d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.1.1/bazel-skylib-1.1.1.tar.gz",
        "https://github.com/bazelbuild/bazel-skylib/releases/download/1.1.1/bazel-skylib-1.1.1.tar.gz",
    ],
)

# Used to build our cross-compiling toolchain.
http_archive(
    name = "rules_cc",
    sha256 = "4dccbfd22c0def164c8f47458bd50e0c7148f3d92002cdb459c2a96a68498241",
    urls = ["https://github.com/bazelbuild/rules_cc/releases/download/0.0.1/rules_cc-0.0.1.tar.gz"],
)

http_archive(
    name = "rules_rust",
    sha256 = "391d6a7f34c89d475e03e02f71957663c9aff6dbd8b8c974945e604828eebe11",
    strip_prefix = "rules_rust-f569827113d0f1058f33da4a449ddd34be35a011",
    urls = [
        # `main` branch as of 2022-02-08
        "https://github.com/bazelbuild/rules_rust/archive/f569827113d0f1058f33da4a449ddd34be35a011.tar.gz",
    ],
)

# Initialise external dependencies.

load("@bazel_skylib//:workspace.bzl", "bazel_skylib_workspace")

bazel_skylib_workspace()

load("@rules_cc//cc:repositories.bzl", "rules_cc_dependencies", "rules_cc_toolchains")

rules_cc_dependencies()

rules_cc_toolchains()

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(
    nogo = "@//:nogo",
    version = "1.17.7",
)

gazelle_dependencies()

load("//bazel/deps:defs.bzl", "rust_deps")

rust_deps()

load("@crates//:defs.bzl", "pinned_rust_install")

pinned_rust_install()

# Register our cross-compiling toolchains.
register_toolchains(
    "//bazel/cross-compiling:x86_64_cc_toolchain",
    "//bazel/cross-compiling:x86_64_rust_toolchain",
)

# Fetch our Go dependencies for tools.

go_repository(
    name = "com_github_BurntSushi_toml",
    importpath = "github.com/BurntSushi/toml",
    sum = "h1:dtDWrepsVPfW9H/4y7dDgFc2MBUSeJhlaDtK13CxFlU=",
    version = "v1.0.0",
)
