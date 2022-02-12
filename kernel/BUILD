# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_binary", "rust_clippy", "rust_doc", "rust_library")
load("//bazel/cross-compiling:config-transition.bzl", "x86_64_bare_metal_rust_binary")

# Make the target configuration available to //bazel/cross-compiling.
exports_files(
    ["x86_64-firefly.json"],
    visibility = ["//:__subpackages__"],
)

# Expose the manifest to the bootloader and bootimage.
exports_files(
    ["Cargo.toml"],
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
    ],
    crate_name = "kernel",
    edition = "2018",
    visibility = ["//visibility:private"],
    deps = [
        crate("lazy_static"),
        crate("managed"),
        crate("spin"),
        crate("volatile"),
        crate("x86_64"),
        "//kernel/cpu",
        "//kernel/drivers/pci",
        "//kernel/drivers/serial",
        "//kernel/drivers/virtio",
        "//kernel/filesystem",
        "//kernel/interrupts",
        "//kernel/memory/memlayout",
        "//kernel/memory/mmio",
        "//kernel/memory/physmem",
        "//kernel/memory/virtmem",
        "//kernel/multitasking/thread",
        "//kernel/network",
        "//kernel/random",
        "//kernel/segmentation",
        "//kernel/storage",
        "//kernel/time",
        "//kernel/utils/align",
        "//kernel/utils/bitmap_index",
        "//kernel/utils/pretty",
        "//kernel/utils/tar",
        "//shared/syscalls",
        "@bootloader",
        "@smoltcp",
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
        "//kernel/drivers/pci",
        "//kernel/drivers/serial",
        "//kernel/memory/physmem",
        "//kernel/memory/virtmem",
        "//kernel/multitasking/thread",
        "//kernel/network",
        "//kernel/random",
        "//kernel/time",
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
    rust_toolchain = "//bazel/cross-compiling:x86_64_rust_toolchain",
    visibility = ["//visibility:public"],
)
