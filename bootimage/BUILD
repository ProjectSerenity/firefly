# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@rules_rust//rust:defs.bzl", "rust_binary", "rust_clippy")

rust_binary(
    name = "binary",
    srcs = ["main.rs"],
    crate_root = "main.rs",
    edition = "2021",
    visibility = ["//visibility:private"],
    deps = [
        "@crates//:llvm-tools",
        "@crates//:thiserror",
    ],
)

rust_clippy(
    name = "clippy",
    deps = [":binary"],
)

genrule(
    name = "bootimage",
    srcs = ["@bootloader//:binary"],
    outs = ["image.bin"],
    cmd = """
        bootimage="$(location :binary)"
        input="$(location @bootloader//:binary)"
        output="$$(realpath $(location image.bin))"

        # Check whether the stage two bootloader is
        # small enough to be loaded.
        #
        # This is just to help, so if GDB is not
        # installed, we skip the check, rather
        # than failing the build.
        if command -v gdb &> /dev/null ; then
            sectors=`gdb -batch -ex "file $$input" -ex "p (&_rest_of_bootloader_end_addr - &_rest_of_bootloader_start_addr) / 512"`
            sectors=`echo $$sectors | awk '{print $$3}'`
            if [ "$$sectors" -gt "127" ]; then
                echo "\033[1;31mERROR:\033[0m Bootloader stage 2 takes up $$sectors disk sectors, which is too large."
                exit 1
            fi
        fi

        # Repackage the bootable image to a raw binary.
        rm -f $$output
        $$bootimage $$input $$output""",
    message = "Making bootable image",
    tools = [":binary"],
    visibility = ["//visibility:public"],
)
