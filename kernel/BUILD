# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_binary", "rust_clippy", "rust_doc", "rust_library", "rustfmt_test")
load("//bazel/cross-compiling:config-transition.bzl", "x86_64_bare_metal_rust_binary")

# Expose the manifest to the bootloader and bootimage.
exports_files(
    ["ograC.toml"],
    visibility = ["//visibility:public"],
)

rust_library(
    name = "libkernel",
    srcs = [
        "lib.rs",
        "syscalls/mod.rs",
    ],
    compile_data = [
        "syscalls/entry.s",
        "//:LICENSE",
    ],
    crate_name = "kernel",
    edition = "2021",
    visibility = ["//visibility:private"],
    deps = [
        "//kernel/cpu",
        "//kernel/drivers/pci",
        "//kernel/drivers/serial",
        "//kernel/drivers/virtio",
        "//kernel/filesystem",
        "//kernel/interrupts",
        "//kernel/memory",
        "//kernel/memory/mmio",
        "//kernel/memory/physmem",
        "//kernel/memory/segmentation",
        "//kernel/memory/virtmem",
        "//kernel/multitasking",
        "//kernel/network",
        "//kernel/random",
        "//kernel/storage",
        "//kernel/time",
        "//kernel/utils/align",
        "//kernel/utils/bitmap_index",
        "//kernel/utils/pretty",
        "//kernel/utils/spin",
        "//kernel/utils/tar",
        "//shared/syscalls",
        "@bootloader",
        "@crates//:x86_64",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":libkernel"],
)

rust_doc(
    name = "docs",
    crate = ":libkernel",
)

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":libkernel"],
)

rust_binary(
    name = "binary",
    srcs = ["main.rs"],
    rustc_flags = [
        "-Cpanic=abort",
        "-Clink-args=-nostartfiles -static -Wl,--image-base=0xffff800000000000",
        "-Ctarget-feature=+crt-static",
    ],
    tags = ["manual"],
    deps = [
        ":libkernel",
        "//kernel/cpu",
        "//kernel/drivers/serial",
        "//kernel/filesystem",
        "//kernel/memory/physmem",
        "//kernel/memory/virtmem",
        "//kernel/multitasking",
        "//kernel/network",
        "//kernel/storage",
        "//kernel/time",
        "//kernel/utils/tar",
        "@bootloader",
    ],
)

# This is the real target, as it uses
# a transition to enforce that the kernel
# binary is compiled and linked using our
# custom C/C++ toolchain and platform.

x86_64_bare_metal_rust_binary(
    name = "kernel",
    rust_binary = ":binary",
    visibility = ["//visibility:public"],
)
