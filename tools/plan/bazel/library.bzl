# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

Plan = provider(
    doc = "A prepared Plan document and its context.",
    fields = {
        "arch": "The instruction set architecture this Plan uses.",
        "path": "The path to the Plan document.",
    },
)

def _plan_library_impl(ctx):
    prefix = ctx.label.name + "/"
    out = ctx.actions.declare_file(prefix + ctx.file.src.path)
    cmd = "{plan} build -arch {arch} {src} && cp {src} {dst}".format(
        plan = ctx.executable._plan.path,
        arch = ctx.attr.arch,
        src = ctx.file.src.path,
        dst = out.path,
    )

    ctx.actions.run_shell(
        inputs = [ctx.file.src, ctx.executable._plan],
        outputs = [out],
        command = cmd,
        mnemonic = "PlanBuild",
    )

    return [
        DefaultInfo(files = depset([out])),
        Plan(
            arch = ctx.attr.arch,
            path = out,
        ),
    ]

plan_library = rule(
    _plan_library_impl,
    attrs = {
        "arch": attr.string(
            doc = "The instruction set architecture to target.",
            mandatory = True,
            values = ["x86-64"],
        ),
        "src": attr.label(
            doc = "The Plan document to check.",
            allow_single_file = [".plan"],
            mandatory = True,
        ),
        "_plan": attr.label(
            doc = "The Plan tool.",
            default = "//tools/plan:plan",
            executable = True,
            cfg = "exec",
        ),
    },
    doc = "Checks a Plan document.",
)
