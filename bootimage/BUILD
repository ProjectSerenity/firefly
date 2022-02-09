# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@crates//:defs.bzl", "crate")
load("@rules_rust//rust:defs.bzl", "rust_binary", "rust_clippy")

rust_binary(
    name = "bootimage_bin",
    srcs = ["main.rs"],
    crate_root = "main.rs",
    edition = "2018",
    visibility = ["//visibility:private"],
    deps = [
        crate("llvm-tools"),
        crate("thiserror"),
    ],
)

rust_clippy(
    name = "bootimage_clippy",
    deps = [":bootimage_bin"],
)

genrule(
    name = "bootimage",
    srcs = ["@bootloader//:binary"],
    outs = ["image.bin"],
    cmd = """
        bootimage="$(location :bootimage_bin)"
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
    tools = [":bootimage_bin"],
    visibility = ["//visibility:public"],
)
