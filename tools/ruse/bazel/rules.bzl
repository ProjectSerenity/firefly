# Copyright 2023 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

# Rules for building Ruse programs.

load(":actions.bzl", "ruse_compile", "ruse_compile_stdlib", "ruse_link")
load(":providers.bzl", "RusePackageInfo")

def _ruse_internal_library_impl(ctx):
    rpkg = ctx.actions.declare_file("{name}_/{package_path}.rpkg".format(
        name = ctx.label.name,
        package_path = ctx.attr.package_path,
    ))

    ruse_compile(
        ctx,
        arch = ctx.attr.arch,
        package_path = ctx.attr.package_path,
        stdlib = None,
        srcs = ctx.files.srcs,
        deps = [dep[RusePackageInfo] for dep in ctx.attr.deps],
        out = rpkg,
    )

    return [
        DefaultInfo(files = depset([rpkg])),
        RusePackageInfo(
            info = struct(
                package_path = ctx.attr.package_path,
                rpkg = rpkg,
            ),
            deps = depset(
                direct = [dep[RusePackageInfo].info for dep in ctx.attr.deps],
                transitive = [dep[RusePackageInfo].deps for dep in ctx.attr.deps],
            ),
        ),
    ]

ruse_internal_library = rule(
    implementation = _ruse_internal_library_impl,
    attrs = {
        "arch": attr.string(
            mandatory = True,
            values = [
                "x86-64",
            ],
            doc = "The target architecture.",
        ),
        "deps": attr.label_list(
            providers = [RusePackageInfo],
            doc = "Direct dependencies of the package.",
        ),
        "package_path": attr.string(
            mandatory = True,
            doc = "The package's full package path.",
        ),
        "srcs": attr.label_list(
            allow_files = [".ruse"],
            doc = "Source files to compile.",
        ),
        "_ruse": attr.label(
            default = "//tools/ruse",
            executable = True,
            doc = "The Ruse tool.",
            cfg = "exec",
        ),
    },
    doc = "Compiles an internal Ruse rpkg from Ruse source code and dependencies.",
)

def _ruse_library_impl(ctx):
    rpkg = ctx.actions.declare_file("{name}_/{package_path}.rpkg".format(
        name = ctx.label.name,
        package_path = ctx.attr.package_path,
    ))

    ruse_compile(
        ctx,
        arch = ctx.attr.arch,
        package_path = ctx.attr.package_path,
        srcs = ctx.files.srcs,
        deps = [dep[RusePackageInfo] for dep in ctx.attr.deps],
        out = rpkg,
    )

    return [
        DefaultInfo(files = depset([rpkg])),
        RusePackageInfo(
            info = struct(
                package_path = ctx.attr.package_path,
                rpkg = rpkg,
            ),
            deps = depset(
                direct = [dep[RusePackageInfo].info for dep in ctx.attr.deps],
                transitive = [dep[RusePackageInfo].deps for dep in ctx.attr.deps],
            ),
        ),
    ]

ruse_library = rule(
    implementation = _ruse_library_impl,
    attrs = {
        "arch": attr.string(
            mandatory = True,
            values = [
                "x86-64",
            ],
            doc = "The target architecture.",
        ),
        "deps": attr.label_list(
            providers = [RusePackageInfo],
            doc = "Direct dependencies of the package.",
        ),
        "package_path": attr.string(
            mandatory = True,
            doc = "The package's full package path.",
        ),
        "srcs": attr.label_list(
            allow_files = [".ruse"],
            doc = "Source files to compile.",
        ),
        "_ruse": attr.label(
            default = "//tools/ruse",
            executable = True,
            doc = "The Ruse tool.",
            cfg = "exec",
        ),
        "_stdlib": attr.label(
            default = "//tools/ruse:stdlib",
            providers = [RusePackageInfo],
            doc = "The Ruse standard library",
        ),
    },
    doc = "Compiles a Ruse rpkg from Ruse source code and dependencies.",
)

def _ruse_standard_library_impl(ctx):
    rpkg = ctx.actions.declare_file("{name}_/{name}.rstd".format(
        name = ctx.label.name,
    ))

    ruse_compile_stdlib(
        ctx,
        arch = ctx.attr.arch,
        deps = [dep[RusePackageInfo] for dep in ctx.attr.deps],
        out = rpkg,
    )

    return [
        DefaultInfo(files = depset([rpkg])),
        RusePackageInfo(
            info = struct(
                package_path = ctx.attr.package_path,
                rpkg = rpkg,
            ),
            deps = depset(
                direct = [dep[RusePackageInfo].info for dep in ctx.attr.deps],
                transitive = [dep[RusePackageInfo].deps for dep in ctx.attr.deps],
            ),
        ),
    ]

ruse_standard_library = rule(
    implementation = _ruse_standard_library_impl,
    attrs = {
        "arch": attr.string(
            mandatory = True,
            values = [
                "x86-64",
            ],
            doc = "The target architecture.",
        ),
        "deps": attr.label_list(
            providers = [RusePackageInfo],
            doc = "Direct dependencies of the package.",
        ),
        "package_path": attr.string(
            mandatory = True,
            doc = "The package's full package path.",
        ),
        "_ruse": attr.label(
            default = "//tools/ruse",
            executable = True,
            doc = "The Ruse tool.",
            cfg = "exec",
        ),
    },
    doc = "Compiles the Ruse standard library from its rpkg files.",
)

def _ruse_binary_impl(ctx):
    executable = ctx.actions.declare_file("{name}_/{name}".format(
        name = ctx.label.name,
    ))

    ruse_link(
        ctx,
        format = ctx.attr.format,
        package = ctx.attr.package[RusePackageInfo].info.rpkg,
        provenance = ctx.attr.provenance,
        symbol_table = ctx.attr.symbol_table,
        deps = [ctx.attr.package[RusePackageInfo]],
        out = executable,
    )

    return [
        DefaultInfo(
            files = depset([executable]),
            executable = executable,
        ),
    ]

ruse_binary = rule(
    implementation = _ruse_binary_impl,
    attrs = {
        "format": attr.string(
            mandatory = True,
            values = [
                "elf",
            ],
            doc = "The binary encoding format.",
        ),
        "package": attr.label(
            mandatory = True,
            providers = [RusePackageInfo],
            doc = "The main package.",
        ),
        "provenance": attr.bool(
            default = True,
            doc = "Whether to include rpkg provenance data.",
        ),
        "symbol_table": attr.bool(
            default = True,
            doc = "Whether to include a symbol table.",
        ),
        "_ruse": attr.label(
            default = "//tools/ruse",
            executable = True,
            doc = "The Ruse tool.",
            cfg = "exec",
        ),
    },
    doc = "Links an executable binary from a Ruse rpkg and dependencies.",
    executable = True,
)
