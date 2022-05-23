# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("bazel/library.bzl", "Plan")

def _plan_format_impl(ctx):
    prefix = ctx.label.name + "/"
    out = ctx.actions.declare_file(prefix + "formatted.plan")
    cmd = "{plan} format -arch {arch} -check -out {dst} {src}".format(
        plan = ctx.executable._plan.path,
        arch = ctx.attr.embed[Plan].arch,
        src = ctx.attr.embed[Plan].path.path,
        dst = out.path,
    )

    ctx.actions.run_shell(
        inputs = [ctx.attr.embed[Plan].path, ctx.executable._plan],
        outputs = [out],
        command = cmd,
        mnemonic = "PlanBuild",
    )

    return [
        DefaultInfo(files = depset([out])),
    ]

plan_format = rule(
    _plan_format_impl,
    attrs = {
        "embed": attr.label(
            doc = "A Plan document.",
            mandatory = True,
            providers = [Plan],
        ),
        "_plan": attr.label(
            doc = "The Plan tool.",
            default = "//tools/plan:plan",
            executable = True,
            cfg = "exec",
        ),
    },
    doc = "Checks that a Plan document is correctly formatted.",
)
