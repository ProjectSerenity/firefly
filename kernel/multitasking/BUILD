# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library", "rustfmt_test")

rust_library(
    name = "multitasking",
    srcs = [
        "lib.rs",
        "process/mod.rs",
        "scheduler/mod.rs",
        "scheduler/timers.rs",
        "switch.rs",
        "thread/mod.rs",
        "thread/stacks.rs",
    ],
    compile_data = [
        "switch.s",
    ],
    crate_name = "multitasking",
    edition = "2021",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        "//kernel/cpu",
        "//kernel/drivers/serial",
        "//kernel/memory/memlayout",
        "//kernel/memory/physmem",
        "//kernel/memory/virtmem",
        "//kernel/multitasking/process/loader",
        "//kernel/segmentation",
        "//kernel/time",
        "//kernel/utils/pretty",
        "//kernel/utils/spin",
        "@crates//:lazy_static",
        "@crates//:x86_64",
        "@crates//:xmas-elf",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":multitasking"],
)

rust_doc(
    name = "docs",
    crate = ":multitasking",
)

rustfmt_test(
    name = "fmt_test",
    size = "small",
    targets = [":multitasking"],
)
