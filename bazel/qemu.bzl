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
{drivecpy}

# Start Qemu.
echo {qemu} {args}
{qemu} {args}
"""

def _qemu_impl(ctx):
    # Construct the script.
    args = [
        "-drive",
        "format=raw,file=_image.bin",
    ]

    args.extend(ctx.attr.options)
    drivecpy = ""
    if ctx.attr.drive:
        args.extend(["-device", "virtio-blk-pci,drive=blk1,disable-legacy=on,disable-modern=off"])
        args.extend(["-drive", "file=drive.tar,id=blk1,if=none,readonly=on"])
        drivecpy = "cp " + ctx.file.drive.short_path + " drive.tar\nchmod +w drive.tar"

    script = ctx.actions.declare_file(ctx.label.name + ".sh")
    script_content = script_template.format(
        image = shell.quote(ctx.file.image.short_path),
        qemu = shell.quote(ctx.attr.qemu),
        drivecpy = drivecpy,
        args = " ".join([shell.quote(arg) for arg in args]),
    )

    ctx.actions.write(script, script_content, is_executable = True)

    # Make sure the image is available at runtime.
    files = [ctx.file.image]
    if ctx.attr.drive:
        files.append(ctx.file.drive)

    runfiles = ctx.runfiles(files = files)

    return [
        DefaultInfo(
            executable = script,
            runfiles = runfiles,
        ),
    ]

qemu = rule(
    _qemu_impl,
    attrs = {
        "drive": attr.label(
            doc = "Label of a file to serve as a Virtio block storage device.",
            allow_single_file = True,
        ),
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
