# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

# This provides a helper function for identifying
# the source files for a Rust core crate. Currently,
# This supports the `core` and `alloc` crates.

load("@rules_rust//rust:defs.bzl", "rust_stdlib_filegroup")

def _find_librust_entrypoint(ctx):
    # Build the path to the crate's entry point.
    want = "/src/library/" + ctx.attr.crate + "/src/lib.rs"

    # Search for the path within the files identified.
    for f in ctx.attr.rustc_srcs.files.to_list():
        if f.short_path.endswith(want):
            return [
                DefaultInfo(files = depset([f])),
            ]

    fail("Unable to find liballoc enctrypoint")

find_librust_entrypoint = rule(
    implementation = _find_librust_entrypoint,
    attrs = {
        "crate": attr.string(
            doc = "The name of the crate to find.",
            mandatory = True,
            values = [
                "alloc",
                "core",
            ],
        ),
        "rustc_srcs": attr.label(allow_files = True),
    },
)

# The rust_no_std macro builds a copy of the standard
# library (crates core, alloc, and compiler-builtins)
# for a particular target configuration.
#
# TODO: Build core, compiler-builtins, and alloc using rust_library.
# This is currently hard as we'd have to use a config
# transition to ensure that we use the host Rust toolchain,
# rather than the target toolchain (as that will depend
# on this).

def rust_no_std(name, target_json, rustc, rustc_srcs, libcore_entry, liballoc_entry):
    libcore_name = "%s_libcore" % name
    libcore_rlib = "libcore_%s.rlib" % name
    native.genrule(
        name = libcore_name,
        srcs = [
            libcore_entry,
            target_json,
            rustc_srcs,
        ],
        outs = [libcore_rlib],
        cmd = """$(location {rustc}) \
                    --edition=2018 \
                    --crate-type=lib \
                    --crate-name=core \
                    --target $(location {target_json}) \
                    --remap-path-prefix=$${PWD}=. \
                    -C opt-level=2 \
                    -o $@ \
                    $(locations {libcore_entry})""".format(
            rustc = rustc,
            libcore_entry = libcore_entry,
            target_json = target_json,
            PWD = "PWD",
        ),
        message = "Building x86_64 libcore",
        tags = ["manual"],
        target_compatible_with = [
            "@platforms//cpu:x86_64",
            "@platforms//os:none",
        ],
        tools = [rustc],
    )

    libcompiler_builtins_name = "%s_compiler_builtins" % name
    libcompiler_builtins_rlib = "libcompiler_builtins_%s.rlib" % name
    native.genrule(
        name = libcompiler_builtins_name,
        srcs = [
            libcore_rlib,
            target_json,
            "@compiler_builtins//:srcs",
            "@compiler_builtins//:lib",
        ],
        outs = [libcompiler_builtins_rlib],
        cmd = """$(location {rustc}) \
                    --cfg 'feature="compiler-builtins"' \
                    --cfg 'feature="core"' \
                    --cfg 'feature="default"' \
                    --cfg 'feature="mem"' \
                    --cfg 'feature="rustc-dep-of-std"' \
                    --cfg 'feature="unstable"' \
                    -Z force-unstable-if-unmarked \
                    --crate-type=lib \
                    --crate-name=compiler_builtins \
                    --target $(location {target_json}) \
                    --extern core=$(location {libcore_rlib}) \
                    --allow unstable_name_collisions \
                    --remap-path-prefix=$${PWD}=. \
                    -C opt-level=2 \
                    -o $@ \
                    $(locations @compiler_builtins//:lib)""".format(
            rustc = rustc,
            libcore_rlib = libcore_rlib,
            target_json = target_json,
            PWD = "PWD",
        ),
        message = "Building x86_64 libcompiler_builtins",
        tags = ["manual"],
        target_compatible_with = [
            "@platforms//cpu:x86_64",
            "@platforms//os:none",
        ],
        tools = [rustc],
    )

    liballoc_name = "%s_liballoc" % name
    liballoc_rlib = "liballoc_%s.rlib" % name
    native.genrule(
        name = liballoc_name,
        srcs = [
            liballoc_entry,
            libcore_rlib,
            libcompiler_builtins_rlib,
            target_json,
            rustc_srcs,
        ],
        outs = [liballoc_rlib],
        cmd = """$(location {rustc}) \
                    --cfg 'feature="compiler-builtins-mem"' \
                    -Z force-unstable-if-unmarked \
                    --edition=2018 \
                    --crate-type=lib \
                    --crate-name=alloc \
                    --target $(location {target_json}) \
                    --extern core=$(location {libcore_rlib}) \
                    --extern compiler_builtins=$(location {libcompiler_builtins_rlib}) \
                    --remap-path-prefix=$${PWD}=. \
                    -C opt-level=2 \
                    -o $@ \
                    $(locations {liballoc_entry})""".format(
            rustc = rustc,
            liballoc_entry = liballoc_entry,
            libcore_rlib = libcore_rlib,
            libcompiler_builtins_rlib = libcompiler_builtins_rlib,
            target_json = target_json,
            PWD = "PWD",
        ),
        message = "Building x86_64 liballoc",
        tags = ["manual"],
        target_compatible_with = [
            "@platforms//cpu:x86_64",
            "@platforms//os:none",
        ],
        tools = [rustc],
    )

    # Combine the three libraries into a
    # no_std crate set.

    rust_stdlib_filegroup(
        name = name,
        srcs = [
            libcore_rlib,
            libcompiler_builtins_rlib,
            liballoc_rlib,
        ],
        visibility = ["//visibility:public"],
    )
