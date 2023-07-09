# Copyright 2023 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("//tools/ruse/bazel:providers.bzl", _RusePackageInfo = "RusePackageInfo")
load("//tools/ruse/bazel:rules.bzl", _ruse_binary = "ruse_binary", _ruse_library = "ruse_library")

ruse_binary = _ruse_binary
ruse_library = _ruse_library
RusePackageInfo = _RusePackageInfo
