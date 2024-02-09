# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

workspace(name = "firefly")

GO_VERSION = "1.21.5"

# Initialise external dependencies.

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "278b7ff5a826f3dc10f04feaf0b70d48b68748ccd512d7f98bf442077f043fe3",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.41.0/rules_go-v0.41.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.41.0/rules_go-v0.41.0.zip",
    ],
)

# Arm A64 instruction set architecture, version ID030323, March 2023.
http_archive(
    name = "a64manual",
    build_file = "@//bazel:a64xml.BUILD",
    sha256 = "4be65585693d1bf1f1765bcc1a2493ce5df99718cc33d6e98fc3e83f76019bfd",
    url = "https://developer.arm.com/-/media/developer/products/architecture/armv9-a-architecture/2023-03/ISA_A64_xml_A_profile-2023-03.tar.gz?rev=3ddc4fac5a824f1fa5a615e2ec21d3aa&hash=3998E2AB39837E332B6AE2533EE26EC6",
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(
    nogo = "@//:nogo",
    version = GO_VERSION,
)
