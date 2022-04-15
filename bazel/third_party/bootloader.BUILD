# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@//bazel/cross-compiling:config-transition.bzl", "x86_64_bare_metal_rust_binary")
load("@rules_rust//cargo:defs.bzl", "cargo_build_script")
load("@rules_rust//rust:defs.bzl", "rust_binary", "rust_library")

licenses([
    "notice",  # MIT from expression "MIT OR Apache-2.0"
])

BOOTLOADER_VERSION = "0.9.22"

# The bootloader package is a little unusual, as we use
# both its library crate and main binary crate, plus we
# depend on its build.rs script.
#
# The library crate is a standard rust_library.
#
# The build.rs script  produces a couple of output files
# we need later on (a generated rust file containing the
# kernel's configuration, plus a modified version of the
# kernel binary). It's not currently possible to declare
# or use output files with a cargo_build_script. Instead,
# we build it as a rust_binary and then use a genrule to
# invoke it, collecting its outputs in the process.
#
# Sadly, the build script produces not only an output
# file (bootloader_config.rs) but also prints important
# information to stdout (an extra search path for the
# linker). Running the build script as a rust_binary + genrule
# is necessary so we can declare the output file, but a
# cargo_build_script is necessary to capture, parse, and
# pass on the search path to the linker. That means we
# have to do both. It would be great to be able to use
# just one. We can capture the path to the extra link
# entry, but can't extract its parent directory to pass
# to the linker.
#
# The binary crate is built as a rust_binary, passing in
# the outputs from the build script. This produces the
# bootable image in the form of an ELF executable.
#
# Once we have the bootable image, we use the bootimage
# crate to turn the ELF executable into a raw executable,
# padded to the next smallest block size (512 bytes).

# First, the bootloader library, which is a normal
# build.

rust_library(
    name = "bootloader",
    srcs = glob(["**/*.rs"]),
    aliases = select({
        # Default
        "//conditions:default": {
        },
    }),
    crate_features = [
        "default",
        "map_physical_memory",
    ],
    crate_root = "src/lib.rs",
    data = glob(
        ["**"],
        exclude = [
            # These can be manually added with overrides if needed.

            # If you run `cargo build` in this dir, the target dir can get very big very quick.
            "target/**",

            # These are not vendored from the crate - we exclude them to avoid busting caches
            # when we change how we generate BUILD files and such.
            "BUILD.bazel",
            "WORKSPACE.bazel",
            "WORKSPACE",
        ],
    ),
    edition = "2021",
    rustc_flags = [
        "--cap-lints=allow",
    ],
    tags = [
        "cargo-raze",
        "manual",
    ],
    version = BOOTLOADER_VERSION,
    visibility = ["//visibility:public"],
)

# Now, the multiple steps necessary to build
# the bootloader binary.
#
# First, we compile the build script.

rust_binary(
    name = "build_script",
    srcs = ["build.rs"],
    crate_features = [
        "binary",
        "fixedvec",
        "llvm-tools",
        "map_physical_memory",
        "toml",
        "usize_conversions",
        "x86_64",
        "xmas-elf",
    ],
    crate_root = "build.rs",
    edition = "2021",
    visibility = ["//visibility:private"],
    deps = [
        "@crates//:llvm-tools",
        "@crates//:toml",
    ],
)

# Invoke the build script, passing
# in the kernel binary and its
# Cargo.toml.

genrule(
    name = "package_kernel",
    srcs = [
        "@//kernel:kernel",
        "@//kernel:ograC.toml",
        "@rust_linux_x86_64//:rustc",
        "x86_64-bootloader.json",
    ],
    outs = [
        "bootloader_config.rs",
        "libkernel_bin-kernel.a",
    ],
    cmd = """
        export TARGET="$(location x86_64-bootloader.json)"
        export KERNEL="$(location @//kernel:kernel)"
        export KERNEL_MANIFEST="$(location @//kernel:ograC.toml)"
        export OUT_DIR="$$(realpath $(@D))"
        export PATH="$$(dirname $(location @rust_linux_x86_64//:rustc)):$$PATH"
        $(location :build_script) > /dev/null""",
    message = "Packaging the kernel binary",
    tools = [
        ":build_script",
    ],
    visibility = ["//visibility:private"],
)

# Now that we've got the file outputs, get the
# command line outputs too so we can update the
# search path for the linker.

cargo_build_script(
    name = "bootloader_build_script",
    srcs = ["build.rs"],
    build_script_env = {
        "PATH": "$$(dirname $(location @rust_linux_x86_64//:rustc)):$$PATH",
    },
    crate_features = [
        "default",
        "map_physical_memory",
    ],
    crate_root = "build.rs",
    data = glob(
        ["**"],
        exclude = [
            # These can be manually added with overrides if needed.

            # If you run `cargo build` in this dir, the target dir can get very big very quick.
            "target/**",

            # These are not vendored from the crate - we exclude them to avoid busting caches
            # when we change how we generate BUILD files and such.
            "BUILD.bazel",
            "WORKSPACE.bazel",
            "WORKSPACE",
        ],
    ) + [
        "@rust_linux_x86_64//:rustc",
    ],
    edition = "2021",
    rustc_flags = [
        "--cap-lints=allow",
    ],
    tags = [
        "cargo-raze",
        "manual",
    ],
    version = BOOTLOADER_VERSION,
    visibility = ["//visibility:private"],
    deps = [
        "@crates//:llvm-tools",
        "@crates//:toml",
    ],
)

# Build the bootloader binary using the
# information and files generated by the
# build script above.
#
# Note, we don't invoke this target directly,
# as it needs to be built for a custom
# target configuration. Instead, we invoke
# bootloader_bin_transition below.

rust_binary(
    name = "bootloader_bin",
    srcs = glob(["**/*.rs"]),
    compile_data = [
        ":bootloader_config.rs",
        ":libkernel_bin-kernel.a",
    ],
    crate_features = [
        "binary",
        "default",
        "map_physical_memory",
    ],
    crate_root = "src/main.rs",
    data = glob(
        ["**"],
        exclude = [
            # These can be manually added with overrides if needed.

            # If you run `cargo build` in this dir, the target dir can get very big very quick.
            "target/**",

            # These are not vendored from the crate - we exclude them to avoid busting caches
            # when we change how we generate BUILD files and such.
            "BUILD.bazel",
            "WORKSPACE.bazel",
            "WORKSPACE",
        ],
    ),
    edition = "2021",
    linker_script = "linker.ld",
    rustc_env = {
        # This should be OUT_DIR, but while we can get the
        # path to bootloader_config.rs, we can't get the
        # path to its parent directory. As a result, we use
        # bootloader.patch to change how the file is included.
        "BOOTLOADER_CONFIG_RS": "$(location :bootloader_config.rs)",
    },
    rustc_flags = [
        "--cap-lints=allow",
        "-Cpanic=abort",
        "-Clink-args=-nostartfiles -static -Wl,--gc-sections -Wl,--build-id=none",
        "-Ctarget-feature=+crt-static",
        # :bootloader_build_script adds to the linker's search path
        # but for some reason it doesn't add to the link list, so
        # we do that manually here.
        "-lstatic=kernel_bin-kernel",
    ],
    tags = ["manual"],
    version = BOOTLOADER_VERSION,
    visibility = ["//visibility:public"],
    deps = [
        ":bootloader",
        ":bootloader_build_script",
        "@crates//:bit_field",
        "@crates//:fixedvec",
        "@crates//:usize_conversions",
        "@crates//:x86_64",
        "@crates//:xmas-elf",
    ],
)

# This is the real target, as it uses
# a transition to enforce that the kernel
# binary is compiled and linked using our
# custom C/C++ toolchain and platform.

x86_64_bare_metal_rust_binary(
    name = "binary",
    rust_binary = ":bootloader_bin",
    visibility = ["//visibility:public"],
)
