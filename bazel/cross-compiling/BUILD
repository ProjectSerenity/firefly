# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

# The x86_64_bare_metal platform is used to describe
# the target execution environment for the kernel.
#
# When we build targets for the kernel, we use the
# --platforms command line option to specify this
# platform.
#
# Where possible, we use a transition to set the
# command line option implicitly, rather than the
# user needing to specify it explicitly.

load("//bazel/cross-compiling:librust.bzl", "find_librust_entrypoint")

# Find the entrypoint (lib.rs) for
# the alloc and core crates.

find_librust_entrypoint(
    name = "liballoc_entry",
    crate = "alloc",
    rustc_srcs = "@rust_linux_x86_64//lib/rustlib/src:rustc_srcs",
)

find_librust_entrypoint(
    name = "libcore_entry",
    crate = "core",
    rustc_srcs = "@rust_linux_x86_64//lib/rustlib/src:rustc_srcs",
)
