# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

# This transition is used to ensure that a Rust
# binary is built with a custom toolchain for the
# target we need. The binary is expressed using
# a `rust_binary` as usual, then a second target
# is made from x86_64_bare_metal_rust_binary,
# specifying the binary to configure.

def _bare_metal_impl(settings, attr):
    rust_toolchain = str(attr.rust_toolchain)
    return {
        "//command_line_option:cpu": "x86_64",
        "//command_line_option:crosstool_top": "//bazel/cross-compiling:x86_64_cc_toolchain_suite",
        "//command_line_option:extra_toolchains": [
            "//bazel/cross-compiling:x86_64_cc_toolchain",
            rust_toolchain,
        ],
        "//command_line_option:host_crosstool_top": "@bazel_tools//tools/cpp:toolchain",
        "//command_line_option:platforms": "//bazel/cross-compiling:x86_64_bare_metal",
    }

_x86_64_bare_metal_platform_transition = transition(
    implementation = _bare_metal_impl,
    inputs = [],
    outputs = [
        "//command_line_option:cpu",
        "//command_line_option:crosstool_top",
        "//command_line_option:extra_toolchains",
        "//command_line_option:host_crosstool_top",
        "//command_line_option:platforms",
    ],
)

# Copy the rust_binary's output and propagate its runfiles.
def _transition_rule_impl(ctx):
    rust_binary = ctx.attr.rust_binary[0]
    outfile = ctx.actions.declare_file(ctx.label.name)
    rust_binary_outfile = rust_binary[DefaultInfo].files.to_list()[0]

    ctx.actions.run_shell(
        inputs = [rust_binary_outfile],
        outputs = [outfile],
        command = "cp %s %s" % (rust_binary_outfile.path, outfile.path),
    )

    return [DefaultInfo(
        files = depset([outfile]),
        data_runfiles = rust_binary[DefaultInfo].data_runfiles,
    )]

_transition_rule = rule(
    implementation = _transition_rule_impl,
    attrs = {
        # Outgoing edge transition
        "rust_binary": attr.label(cfg = _x86_64_bare_metal_platform_transition),
        "rust_toolchain": attr.label(cfg = _x86_64_bare_metal_platform_transition),
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    },
)

def x86_64_bare_metal_rust_binary(name, rust_binary, rust_toolchain, visibility):
    _transition_rule(
        name = name,
        rust_binary = rust_binary,
        rust_toolchain = rust_toolchain,
        visibility = visibility,
    )
