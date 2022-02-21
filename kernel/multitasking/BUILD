# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_clippy", "rust_doc", "rust_library")

rust_library(
    name = "multitasking",
    srcs = [
        "lib.rs",
        "scheduler/mod.rs",
        "scheduler/timers.rs",
        "thread/mod.rs",
        "thread/stacks.rs",
        "thread/switch.rs",
    ],
    compile_data = [
        "thread/switch.s",
    ],
    crate_name = "multitasking",
    visibility = ["//kernel:__subpackages__"],
    deps = [
        crate("lazy_static"),
        crate("spin"),
        crate("x86_64"),
        "//kernel/cpu",
        "//kernel/drivers/serial",
        "//kernel/memory/memlayout",
        "//kernel/memory/physmem",
        "//kernel/memory/virtmem",
        "//kernel/segmentation",
        "//kernel/time",
        "//kernel/utils/pretty",
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
