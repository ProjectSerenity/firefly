# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@bazel_skylib//lib:shell.bzl", "shell")

def _disk_image_impl(ctx):
    out = ctx.actions.declare_file(ctx.attr.out)
    cmd = "{bootimage} -bootloader={bootloader} -kernel={kernel} -user={user} -out={out}".format(
        bootimage = shell.quote(ctx.executable._bootimage.path),
        bootloader = shell.quote(ctx.file.bootloader.path),
        kernel = shell.quote(ctx.file.kernel.path),
        user = shell.quote(ctx.file.user.path),
        out = shell.quote(out.path),
    )

    ctx.actions.run_shell(
        inputs = [ctx.file.bootloader, ctx.file.kernel, ctx.file.user, ctx.executable._bootimage],
        outputs = [out],
        command = cmd,
        mnemonic = "DiskImageBuild",
    )

    return [
        DefaultInfo(files = depset([out])),
    ]

disk_image = rule(
    _disk_image_impl,
    attrs = {
        "bootloader": attr.label(
            doc = "The bootloader binary and MBR record.",
            allow_single_file = True,
            mandatory = True,
        ),
        "kernel": attr.label(
            doc = "The kernel binary.",
            allow_single_file = True,
            mandatory = True,
        ),
        "out": attr.string(
            doc = "The filename for the built disk image.",
            mandatory = True,
        ),
        "user": attr.label(
            doc = "The userspace disk image.",
            allow_single_file = True,
            mandatory = True,
        ),
        "_bootimage": attr.label(
            doc = "The bootimage tool.",
            default = "//tools/bootimage",
            executable = True,
            cfg = "exec",
        ),
    },
    doc = "Builds a bootable disk imaget.",
)
