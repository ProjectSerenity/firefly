# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

def _copy_file_impl(ctx):
    outfile = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.run_shell(
        inputs = [ctx.file.src],
        outputs = [outfile],
        command = "cp %s %s" % (ctx.file.src.path, outfile.path),
    )

    return [DefaultInfo(
        files = depset([outfile]),
    )]

copy_file = rule(
    implementation = _copy_file_impl,
    attrs = {
        "src": attr.label(
            doc = "Label of the file to copy.",
            allow_single_file = True,
            mandatory = True,
        ),
    },
)
