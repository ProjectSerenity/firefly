# Code generated by vendeps. DO NOT EDIT.

# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

go = [
    module(
        name = "github.com/bazelbuild/buildtools",
        version = "v0.0.0-20230113180850-180a94ab3a3a",
        digest = "sha256:0b78247e8d1abe1656e54459a05ca8c3b31b525914b3d6abbff5aff934fd78bf",
    ),
    module(
        name = "github.com/google/go-cmp",
        version = "v0.5.9",
        digest = "sha256:f124ca277a734000353d9dea2d5f72e095847ae0e460a9837014a4af84255cc7",
    ),
    module(
        name = "github.com/google/osv-scanner",
        version = "v1.2.0",
        digest = "sha256:2771f185633706b3197bb1606f81e2928028fa74c5a4bae998574d9c2b2226be",
        patch_args = [
            "-p1",
        ],
        patches = [
            "bazel/patches/github.com_google_osv-scanner_pkg_osv_osv.go",
        ],
        patch_digest = "sha256:a61855218ea9eb42f5ae7fb32635383c0566e3fa41d0e1feb29ed079b21c8643",
    ),
    module(
        name = "golang.org/x/crypto",
        version = "v0.5.0",
        digest = "sha256:518abb983d66814bf0bfaccc8cc903ff8dc692c964baa872e9bad4f1d6046c15",
    ),
    module(
        name = "golang.org/x/mod",
        version = "v0.8.0",
        digest = "sha256:8c8a1264537694d138af18364a3a023097c32abdd84a013e0be1968a24982c7c",
    ),
    module(
        name = "golang.org/x/time",
        version = "v0.3.0",
        digest = "sha256:095869f0370bf4377ec8a38b7ac04bc91c40642a6c85cc87c824b841839bad8f",
    ),
    module(
        name = "golang.org/x/tools",
        version = "v0.5.0",
        digest = "sha256:b7e1fe569092d058821a3c822529eb25a79d38a8fb8d041a622693b65e73144f",
    ),
    module(
        name = "golang.org/x/xerrors",
        version = "v0.0.0-20220907171357-04be3eba64a2",
        digest = "sha256:9df13449b145dfcfb97ea22dc88c0fdcb4d847a56de7a693724f62ebf2b23907",
    ),
    module(
        name = "rsc.io/diff",
        version = "v0.0.0-20190621135850-fe3479844c3c",
        digest = "sha256:2d08e81c4ae9aa1a306761dd6999b07ce470057ae3deca4c90ebc2072508127e",
    ),
    module(
        name = "rsc.io/pdf",
        version = "v0.1.1",
        digest = "sha256:dbcfced86e0476b66361055010722ad28f952e5745a43dd3d3fcd847d9f23733",
    ),
]
