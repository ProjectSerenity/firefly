# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_binary", "rust_clippy", "rust_library")
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
        "src/drivers/pci.rs",
        "src/drivers/serial.rs",
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
        "src/filesystem/mod.rs",
        "src/gdt.rs",
        "src/interrupts/irq.rs",
        "src/interrupts/mod.rs",
        "src/lib.rs",
        "src/memory/constants.rs",
        "src/memory/mmio/mod.rs",
        "src/memory/mod.rs",
        "src/memory/pmm/bitmap.rs",
        "src/memory/pmm/boot_info.rs",
        "src/memory/pmm/mod.rs",
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
        "src/storage/block/mod.rs",
        "src/storage/mod.rs",
        "src/syscalls/mod.rs",
        "src/time/mod.rs",
        "src/time/rtc.rs",
        "src/time/slice.rs",
        "src/time/ticker.rs",
        "src/time/timers.rs",
        "src/utils/bitmap.rs",
        "src/utils/lazy.rs",
        "src/utils/mod.rs",
        "src/utils/once.rs",
        "src/utils/pretty/bytes.rs",
        "src/utils/pretty/mod.rs",
        "src/utils/tar/mod.rs",
        "src/utils/tar/parse_utils.rs",
    ],
    compile_data = [
        "src/multitasking/thread/switch.s",
        "src/syscalls/entry.s",
    ],
    crate_name = "kernel",
    edition = "2018",
    proc_macro_deps = [
        crate("hex-literal"),
    ],
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
        "//shared/syscalls",
        "@bootloader",
        "@smoltcp",
        "@uart_16550",
    ],
)

rust_clippy(
    name = "kernel_clippy",
    deps = [":libkernel"],
)

rust_binary(
    name = "kernel_bin",
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
        "@bootloader",
    ],
)

# This is the real target, as it uses
# a transition to enforce that the kernel
# binary is compiled and linked using our
# custom C/C++ toolchain and platform.

x86_64_bare_metal_rust_binary(
    name = "kernel",
    rust_binary = ":kernel_bin",
    visibility = ["//visibility:public"],
)
