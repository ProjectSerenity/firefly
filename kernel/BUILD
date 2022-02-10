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
        "src/cpu/mod.rs",
        "src/drivers/mod.rs",
        "src/drivers/virtio/block/cache.rs",
        "src/drivers/virtio/block/mod.rs",
        "src/drivers/virtio/entropy/mod.rs",
        "src/drivers/virtio/features.rs",
        "src/drivers/virtio/mod.rs",
        "src/drivers/virtio/network/mod.rs",
        "src/drivers/virtio/transports/mod.rs",
        "src/drivers/virtio/transports/pci.rs",
        "src/drivers/virtio/virtqueues/mod.rs",
        "src/drivers/virtio/virtqueues/split.rs",
        "src/interrupts/irq.rs",
        "src/interrupts/mod.rs",
        "src/lib.rs",
        "src/memory/constants.rs",
        "src/memory/mmio/mod.rs",
        "src/memory/mod.rs",
        "src/memory/vmm/bump.rs",
        "src/memory/vmm/fixed_size_block.rs",
        "src/memory/vmm/linked_list.rs",
        "src/memory/vmm/mapping.rs",
        "src/memory/vmm/mod.rs",
        "src/multitasking/cpu_local.rs",
        "src/multitasking/mod.rs",
        "src/multitasking/thread/mod.rs",
        "src/multitasking/thread/scheduler.rs",
        "src/multitasking/thread/switch.rs",
        "src/network/mod.rs",
        "src/network/tcp.rs",
        "src/network/udp.rs",
        "src/random/csprng.rs",
        "src/random/mod.rs",
        "src/random/rdrand.rs",
        "src/syscalls/mod.rs",
        "src/time/mod.rs",
        "src/time/rtc.rs",
        "src/time/slice.rs",
        "src/time/ticker.rs",
        "src/time/timers.rs",
        "src/utils/lazy.rs",
        "src/utils/mod.rs",
        "src/utils/once.rs",
    ],
    compile_data = [
        "src/multitasking/thread/switch.s",
        "src/syscalls/entry.s",
    ],
    crate_name = "kernel",
    edition = "2018",
    rustc_flags = [
        "-Cpanic=abort",
    ],
    visibility = ["//visibility:private"],
    deps = [
        crate("bitflags"),
        crate("chacha20"),
        crate("lazy_static"),
        crate("linked_list_allocator"),
        crate("managed"),
        crate("pic8259"),
        crate("raw-cpuid"),
        crate("sha2"),
        crate("spin"),
        crate("volatile"),
        crate("x86_64"),
        "//kernel/src/drivers/pci",
        "//kernel/src/drivers/serial",
        "//kernel/src/filesystem",
        "//kernel/src/memory/physmem",
        "//kernel/src/segmentation",
        "//kernel/src/storage",
        "//kernel/src/utils/align",
        "//kernel/src/utils/bitmap_index",
        "//kernel/src/utils/pretty",
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
    srcs = ["src/main.rs"],
    rustc_flags = [
        "-Cpanic=abort",
        "-Clink-args=-nostartfiles -static -Wl,--image-base=0xffff800000000000",
        "-Ctarget-feature=+crt-static",
    ],
    tags = ["manual"],
    deps = [
        crate("x86_64"),
        ":libkernel",
        "//kernel/src/drivers/pci",
        "//kernel/src/drivers/serial",
        "//kernel/src/memory/physmem",
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
