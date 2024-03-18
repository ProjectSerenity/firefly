# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@bazel_skylib//lib:shell.bzl", "shell")

# script_template is a simple shell script
# to start qemu with the specified args.
script_template = """
#!/bin/sh

set -e

# The image is not writable (to prevent us interfering
# with other parts of the build). However, Qemu will
# not boot an image marked as writable for some reason,
# so we make a local copy and make it writable.
rm -f _image.bin  # Clean up any previous version.
cp {image} _image.bin
chmod +w _image.bin

# Start Qemu.
echo {qemu} {args}
{qemu} {args}
"""

def _qemu_impl(ctx):
    # Construct the script.
    args = ctx.attr.options + [
        "-drive",
        "format=raw,file=_image.bin,id=blk1",
    ]

    script = ctx.actions.declare_file(ctx.label.name + ".sh")
    script_content = script_template.format(
        image = shell.quote(ctx.file.image.short_path),
        qemu = shell.quote(ctx.attr.qemu),
        args = " ".join([shell.quote(arg) for arg in args]),
    )

    ctx.actions.write(script, script_content, is_executable = True)

    # Make sure the image is available at runtime.
    runfiles = ctx.runfiles(files = [ctx.file.image])

    return [
        DefaultInfo(
            executable = script,
            runfiles = runfiles,
        ),
    ]

qemu = rule(
    _qemu_impl,
    attrs = {
        "image": attr.label(
            mandatory = True,
            doc = "Label of the bootable image Qemu should start.",
            allow_single_file = True,
        ),
        "options": attr.string_list(
            mandatory = True,
            doc = "Other options to Qemu. The bootable image will have the name \"image.bin\".",
        ),
        "qemu": attr.string(
            mandatory = True,
            doc = "Name of the Qemu binary to execute (eg \"qemu-system-x86_64\").",
        ),
    },
    doc = "Invokes Qemu to boot Firefly.",
    executable = True,
)

# test_script_template is a simple shell
# script to start qemu with the specified
# args.
test_script_template = """
#!/bin/sh

set -e

# The image is not writable (to prevent us interfering
# with other parts of the build). However, Qemu will
# not boot an image marked as writable for some reason,
# so we make a local copy and make it writable.
rm -f _image.bin  # Clean up any previous version.
cp {image} _image.bin
chmod +w _image.bin

# Start Qemu.
echo {qemu} {args}
{qemu} {args} | tee $TEST_TMPDIR/out.log
grep -P "\\e\\[1;32mPASS\\e\\[0m" $TEST_TMPDIR/out.log #|| exit 1
"""

def _qemu_test_impl(ctx):
    # Construct the script.
    args = ctx.attr.options + [
        "-drive",
        "format=raw,file=_image.bin,id=blk1,if=none",
    ]

    script = ctx.actions.declare_file(ctx.label.name + ".sh")
    script_content = test_script_template.format(
        image = shell.quote(ctx.file.image.short_path),
        qemu = shell.quote(ctx.attr.qemu),
        args = " ".join([shell.quote(arg) for arg in args]),
    )

    ctx.actions.write(script, script_content, is_executable = True)

    # Make sure the image is available at runtime.
    runfiles = ctx.runfiles(files = [ctx.file.image])

    return [
        DefaultInfo(
            executable = script,
            runfiles = runfiles,
        ),
    ]

qemu_test = rule(
    _qemu_test_impl,
    attrs = {
        "image": attr.label(
            mandatory = True,
            doc = "Label of the bootable image Qemu should start.",
            allow_single_file = True,
        ),
        "options": attr.string_list(
            mandatory = True,
            doc = "Other options to Qemu. The bootable image will have the name \"image.bin\".",
        ),
        "qemu": attr.string(
            mandatory = True,
            doc = "Name of the Qemu binary to execute (eg \"qemu-system-x86_64\").",
        ),
    },
    doc = "Invokes Qemu to boot Firefly.",
    test = True,
)
