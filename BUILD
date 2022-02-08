# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@com_github_bazelbuild_buildtools//buildifier:def.bzl", "buildifier")

# Allow Buildifier to be run with `bazel run //:buildifier`.
buildifier(
    name = "buildifier",
    lint_mode = "fix",
    lint_warnings = ["all"],
    mode = "fix",
)

# Allow the bootable image to be built with `bazel build //:image`.
alias(
    name = "image",
    actual = "//bootimage:bootimage",
)
