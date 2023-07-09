# Copyright 2023 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

# Common functionality for compiling and linking
# Ruse programs.

load(":providers.bzl", "RusePackageInfo")

def ruse_compile(ctx, arch, package_path, srcs, out, deps = []):
    """Compiles a single Ruse package from source code.

    Args:
        ctx: Analysis context.
        arch: The target architecture.
        package_path: The compiled package's full package path.
        srcs: The list of source files that will be compiled.
        out: The path to the resulting .rpkg file. The path
            should start with the package path. That is, the
            package "example.com/foo" should have the out path
            "example.com/foo.rpkg".
        deps: The list of RusePackageInfo objects for direct
            dependencies.
    """
    args = ctx.actions.args()
    args.add("compile")
    args.add("-arch", arch)
    args.add("-package", package_path)
    dep_paths = [dep.info.rpkg.path for dep in deps]
    args.add_all(dep_paths, before_each = "-rpkg")
    args.add("-o", out)
    args.add_all(srcs)

    inputs = (srcs +
              [dep.info.rpkg for dep in deps])

    ctx.actions.run(
        outputs = [out],
        inputs = inputs,
        executable = ctx.executable._ruse,
        arguments = [args],
        mnemonic = "RuseCompile",
    )

def ruse_link(ctx, format, package, out, deps = []):
    """Links a single executable binary from a Ruse package.

    Args:
        ctx: Analysis context.
        format: The executable binary format.
        package: The compiled package to link.
        out: The path to the resulting binary.
        deps: The list of RusePackageInfo objects for transitive
            dependencies.
    """
    transitive_deps = [dep.rpkg for dep in ctx.attr.package[RusePackageInfo].deps.to_list()]

    args = ctx.actions.args()
    args.add("link")
    args.add("-binary", format)
    dep_paths = [rpkg.path for rpkg in transitive_deps]
    args.add_all(dep_paths, before_each = "-rpkg")
    args.add("-o", out)
    args.add(package)

    inputs = ([package] +
              transitive_deps)

    ctx.actions.run(
        outputs = [out],
        inputs = inputs,
        executable = ctx.executable._ruse,
        arguments = [args],
        mnemonic = "RuseLink",
    )
