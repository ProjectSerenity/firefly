# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("bazel/library.bzl", "Plan")

def _plan_rust_userspace_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".rs")
    cmd = "{plan} build -arch {arch} -rustfmt {rustfmt} -rust-user {dst} {src}".format(
        plan = ctx.executable._plan.path,
        arch = ctx.attr.embed[Plan].arch,
        rustfmt = ctx.executable.rustfmt.path,
        src = ctx.attr.embed[Plan].path.path,
        dst = out.path,
    )

    ctx.actions.run_shell(
        inputs = [ctx.attr.embed[Plan].path, ctx.executable.rustfmt, ctx.executable._plan],
        outputs = [out],
        command = cmd,
        mnemonic = "PlanBuild",
    )

    return [
        DefaultInfo(files = depset([out])),
    ]

plan_rust_userspace = rule(
    _plan_rust_userspace_impl,
    attrs = {
        "embed": attr.label(
            doc = "A Plan document.",
            mandatory = True,
            providers = [Plan],
        ),
        "rustfmt": attr.label(
            doc = "The rustfmt tool.",
            mandatory = True,
            executable = True,
            cfg = "exec",
        ),
        "_plan": attr.label(
            doc = "The Plan tool.",
            default = "//tools/plan:plan",
            executable = True,
            cfg = "exec",
        ),
    },
    doc = "Translates a Plan document to the equivalent Rust code for userspace.",
)

def _plan_rust_kernelspace_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".rs")
    cmd = "{plan} build -arch {arch} -rustfmt {rustfmt} -rust-kernel {dst} {src}".format(
        plan = ctx.executable._plan.path,
        arch = ctx.attr.embed[Plan].arch,
        rustfmt = ctx.executable.rustfmt.path,
        src = ctx.attr.embed[Plan].path.path,
        dst = out.path,
    )

    ctx.actions.run_shell(
        inputs = [ctx.attr.embed[Plan].path, ctx.executable.rustfmt, ctx.executable._plan],
        outputs = [out],
        command = cmd,
        mnemonic = "PlanBuild",
    )

    return [
        DefaultInfo(files = depset([out])),
    ]

plan_rust_kernelspace = rule(
    _plan_rust_kernelspace_impl,
    attrs = {
        "embed": attr.label(
            doc = "A Plan document.",
            mandatory = True,
            providers = [Plan],
        ),
        "rustfmt": attr.label(
            doc = "The rustfmt tool.",
            mandatory = True,
            executable = True,
            cfg = "exec",
        ),
        "_plan": attr.label(
            doc = "The Plan tool.",
            default = "//tools/plan:plan",
            executable = True,
            cfg = "exec",
        ),
    },
    doc = "Translates a Plan document to the equivalent Rust code for kernelspace.",
)
