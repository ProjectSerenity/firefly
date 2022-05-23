# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("bazel/library.bzl", "Plan")

def _plan_docs_impl(ctx):
    out = ctx.actions.declare_directory(ctx.label.name)
    cmd = "{plan} docs -arch {arch} -out {dst} {src}".format(
        plan = ctx.executable._plan.path,
        arch = ctx.attr.embed[Plan].arch,
        src = ctx.attr.embed[Plan].path.path,
        dst = out.path,
    )

    ctx.actions.run_shell(
        inputs = [ctx.attr.embed[Plan].path, ctx.executable._plan],
        outputs = [out],
        command = cmd,
        mnemonic = "PlanDocs",
    )

    return [
        DefaultInfo(files = depset([out])),
    ]

plan_docs = rule(
    _plan_docs_impl,
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
    doc = "Generates HTML documentation for a Plan document.",
)
