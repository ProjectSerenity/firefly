# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@com_github_bazelbuild_buildtools//buildifier:def.bzl", "buildifier")
load("//bazel:qemu.bzl", "qemu")

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
