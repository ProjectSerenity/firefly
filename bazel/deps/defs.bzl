# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load(":go.bzl", _go_deps = "go_deps")
load(":rust.bzl", _rust_deps = "rust_deps")

go_deps = _go_deps

rust_deps = _rust_deps
