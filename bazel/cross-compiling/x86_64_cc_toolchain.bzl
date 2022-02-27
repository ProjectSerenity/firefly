# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

# The x86_64_cc_toolchain macro builds a C++ toolchain
# we can use to link Rust crates for custom targets.
#
# Note that we use clang specifically for the
# frontend but /usr/bin/ld for the linker. We
# can't specify other linkers as sometimes
# clang will do its own thing and use /usr/bin/ld
# anyway. We depend on ld.lld, so make sure
# it's available in /usr/bin/ld.

load(
    "@bazel_tools//tools/cpp:unix_cc_toolchain_config.bzl",
    "cc_toolchain_config",
)
load("@rules_cc//cc:defs.bzl", "cc_toolchain")

def x86_64_cc_toolchain(name):
    cc_toolchain_config_name = "%s_cc_toolchain_cfg" % name
    cc_toolchain_config(
        name = cc_toolchain_config_name,
        cpu = "@platforms//cpu:x86_64",
        compiler = "clang",
        host_system_name = "local",
        target_system_name = "freestanding",
        target_libc = "none",
        abi_version = "unknown",
        abi_libc_version = "unknown",
        toolchain_identifier = "x86_64-bare-metal",
        tool_paths = {
            "ar": "/usr/bin/ar",
            "cpp": "/bin/false",
            "gcc": "/usr/bin/clang",
            "ld": "/usr/bin/ld",
            "llvm-cov": "/bin/false",
            "nm": "/bin/false",
            "objdump": "/bin/false",
            "strip": "/bin/false",
        },
    )

    native.filegroup(name = "empty")

    # The C++ toolchain is mostly empty,
    # as we don't actually use the C++
    # compiler.
    cc_toolchain_name = "%s_cc_toolchain" % name
    cc_toolchain(
        name = cc_toolchain_name,
        all_files = ":empty",
        compiler_files = ":empty",
        dwp_files = ":empty",
        linker_files = ":empty",
        objcopy_files = ":empty",
        strip_files = ":empty",
        supports_param_files = 0,
        toolchain_config = cc_toolchain_config_name,
        toolchain_identifier = "x86_64-bare-metal",
    )

    native.toolchain(
        name = name,
        exec_compatible_with = [
            "@platforms//cpu:x86_64",
            "@platforms//os:linux",
        ],
        target_compatible_with = [
            "@platforms//cpu:x86_64",
            "@platforms//os:none",
        ],
        toolchain = cc_toolchain_name,
        toolchain_type = "@rules_cc//cc:toolchain_type",
    )
