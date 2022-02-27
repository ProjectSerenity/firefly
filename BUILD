# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@bazel_gazelle//:def.bzl", "gazelle")
load("@com_github_bazelbuild_buildtools//buildifier:def.bzl", "buildifier")
load("@io_bazel_rules_go//go:def.bzl", "nogo")
load("//bazel:qemu.bzl", "qemu")

# Configure nogo.
nogo(
    name = "nogo",
    config = "nogo-config.json",
    visibility = ["//visibility:public"],
    deps = [
        # Run by default in `go test`. See https://pkg.go.dev/cmd/go#hdr-Test_packages
        "@org_golang_x_tools//go/analysis/passes/atomic",
        "@org_golang_x_tools//go/analysis/passes/bools",
        "@org_golang_x_tools//go/analysis/passes/buildtag",
        "@org_golang_x_tools//go/analysis/passes/errorsas",
        "@org_golang_x_tools//go/analysis/passes/ifaceassert",
        "@org_golang_x_tools//go/analysis/passes/nilfunc",
        "@org_golang_x_tools//go/analysis/passes/printf",
        "@org_golang_x_tools//go/analysis/passes/stringintconv",
        # Optional analysers we've enabled.
        "@org_golang_x_tools//go/analysis/passes/assign",
        "@org_golang_x_tools//go/analysis/passes/composite",
        "@org_golang_x_tools//go/analysis/passes/copylock",
        "@org_golang_x_tools//go/analysis/passes/sortslice",
        "@org_golang_x_tools//go/analysis/passes/structtag",
        "@org_golang_x_tools//go/analysis/passes/unsafeptr",
    ],
)

# Configure Gazelle

# gazelle:prefix github.com/ProjectSerenity/firefly
# gazelle:build_file_name BUILD,BUILD.bazel

# Allow Gazelle to be run with `bazel run //:gazelle`.
gazelle(name = "gazelle")

# Allow Buildifier to be run with `bazel run //:buildifier`.
buildifier(
    name = "buildifier",
    lint_mode = "fix",
    lint_warnings = ["all"],
    mode = "fix",
)

# Allow the bootable image to be built with `bazel build //:image`.
alias(
    name = "image",
    actual = "//bootimage:bootimage",
)

# Allow the image to be run in Qemu with `bazel run //:qemu`.
qemu(
    name = "qemu",
    drive = "//user/initial-workload:tar",
    image = "//bootimage:image.bin",
    options = [
        "-drive",
        "format=raw,file=image.bin",
        "-device",
        "virtio-net,netdev=net0,disable-legacy=on,disable-modern=off",
        "-netdev",
        "user,id=net0",
        "-object",
        "filter-dump,id=filter0,netdev=net0,file=virtio-net.pcap",
        "-device",
        "virtio-rng-pci,disable-legacy=on,disable-modern=off",
        "-serial",
        "stdio",
        "-display",
        "none",
    ],
    qemu = "qemu-system-x86_64",
)

# Allow our dependencies to be updated with `bazel run //:update-deps`.
alias(
    name = "update-deps",
    actual = "//tools/update-deps",
)
