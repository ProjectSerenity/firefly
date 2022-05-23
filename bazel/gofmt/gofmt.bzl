# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@io_bazel_rules_go//go:def.bzl", "GoSource")

def _gofmt_impl(ctx):
    # Declare the output file.
    out = ctx.actions.declare_file(ctx.label.name + "/diff")

    # Build the set of arguments.
    args = ctx.actions.args()
    args.add("-gofmt", ctx.file._gofmt)
    args.add("-out", out)
    srcs = []
    for pkg in ctx.attr.embed:
        args.add_all(pkg[GoSource].srcs)
        srcs += pkg[GoSource].srcs

    ctx.actions.run(
        outputs = [out],
        inputs = srcs,
        executable = ctx.file._driver,
        tools = [ctx.file._gofmt],
        arguments = [args],
        mnemonic = "GoFmt",
    )

    return [DefaultInfo(
        files = depset([out]),
    )]

gofmt = rule(
    _gofmt_impl,
    attrs = {
        "embed": attr.label_list(
            providers = [GoSource],
            doc = "The Go packages or source files to format.",
        ),
        "_driver": attr.label(
            default = "@//bazel/gofmt",
            allow_single_file = True,
            executable = True,
            cfg = "exec",
        ),
        "_gofmt": attr.label(
            default = "@go_sdk//:bin/gofmt",
            allow_single_file = True,
            executable = True,
            cfg = "exec",
        ),
    },
    doc = "Checks that Go code is formatted according to `gofmt -s`, returning an error and printing a diff if not.",
)
