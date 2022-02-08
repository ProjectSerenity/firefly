# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

workspace(name = "firefly")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Fetch external dependencies.

# Used by buildifier.
http_archive(
    name = "io_bazel_rules_go",
    sha256 = "2b1641428dff9018f9e85c0384f03ec6c10660d935b750e3fa1492a281a53b0f",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.29.0/rules_go-v0.29.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.29.0/rules_go-v0.29.0.zip",
    ],
)

# Used by buildifier.
http_archive(
    name = "com_google_protobuf",
    sha256 = "f94faa42d49c0450226d1e9700ab5f5c3d8e5b757df41bc741bd304fd353eb63",
    strip_prefix = "protobuf-3.15.5",
    urls = ["https://github.com/protocolbuffers/protobuf/archive/v3.15.5.zip"],
)

# For buildifier.
http_archive(
    name = "com_github_bazelbuild_buildtools",
    sha256 = "932160d5694e688cb7a05ac38efba4b9a90470c75f39716d85fb1d2f95eec96d",
    strip_prefix = "buildtools-4.0.1",
    urls = ["https://github.com/bazelbuild/buildtools/archive/4.0.1.zip"],
)

http_archive(
    name = "bazel_skylib",
    sha256 = "c6966ec828da198c5d9adbaa94c05e3a1c7f21bd012a0b29ba8ddbccb2c93b0d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.1.1/bazel-skylib-1.1.1.tar.gz",
        "https://github.com/bazelbuild/bazel-skylib/releases/download/1.1.1/bazel-skylib-1.1.1.tar.gz",
    ],
)

# Used to build our cross-compiling toolchain.
http_archive(
    name = "rules_cc",
    sha256 = "4dccbfd22c0def164c8f47458bd50e0c7148f3d92002cdb459c2a96a68498241",
    urls = ["https://github.com/bazelbuild/rules_cc/releases/download/0.0.1/rules_cc-0.0.1.tar.gz"],
)

http_archive(
    name = "rules_rust",
    sha256 = "391d6a7f34c89d475e03e02f71957663c9aff6dbd8b8c974945e604828eebe11",
    strip_prefix = "rules_rust-f569827113d0f1058f33da4a449ddd34be35a011",
    urls = [
        # `main` branch as of 2022-02-08
        "https://github.com/bazelbuild/rules_rust/archive/f569827113d0f1058f33da4a449ddd34be35a011.tar.gz",
    ],
)

# Rust crates where we use a custom BUILD file.

# Modify the binary build to mimic what `bootimage runner` does.
# We also patch the binary to fix a path issue.
http_archive(
    name = "bootloader",
    build_file = "//bazel/third_party:bootloader.BUILD",
    patch_args = ["-p1"],
    patches = [
        "//bazel/third_party:bootloader.patch",
    ],
    sha256 = "a62c8f6168cd106687ee36a2b71a46c4144d73399f72814104d33094b8092fd2",
    strip_prefix = "bootloader-0.9.21",
    type = "tgz",
    urls = [
        "https://static.crates.io/crates/bootloader/bootloader-0.9.21.crate",
    ],
)

http_archive(
    name = "compiler_builtins",
    build_file = "//bazel/third_party:compiler_builtins.BUILD",
    sha256 = "a68c69e9451f1df4b215c9588c621670c12286b53e60fb5ec4b59aaa1138d18e",
    strip_prefix = "compiler_builtins-0.1.67",
    type = "tgz",
    urls = [
        "https://static.crates.io/crates/compiler_builtins/compiler_builtins-0.1.67.crate",
    ],
)

# Patch out the optional dependency on getrandom.
http_archive(
    name = "rand_core",
    build_file = "//bazel/third_party:rand_core.BUILD",
    sha256 = "d34f1408f55294453790c48b2f1ebbb1c5b4b7563eb1f418bcfcfdbb06ebb4e7",
    strip_prefix = "rand_core-0.6.3",
    type = "tgz",
    urls = [
        "https://static.crates.io/crates/rand_core/rand_core-0.6.3.crate",
    ],
)

# Switch from @crates__rand_core__0_6_3 to @rand_core and
# patch out optional dependency on log.
http_archive(
    name = "smoltcp",
    build_file = "//bazel/third_party:smoltcp.BUILD",
    sha256 = "d2308a1657c8db1f5b4993bab4e620bdbe5623bd81f254cf60326767bb243237",
    strip_prefix = "smoltcp-0.8.0",
    type = "tgz",
    urls = [
        "https://static.crates.io/crates/smoltcp/smoltcp-0.8.0.crate",
    ],
)

# Patch in unconditional dependency on x86_64.
http_archive(
    name = "uart_16550",
    build_file = "//bazel/third_party:uart_16550.BUILD",
    sha256 = "65ad019480ef5ff8ffe66d6f6a259cd87cf317649481394981db1739d844f374",
    strip_prefix = "uart_16550-0.2.15",
    type = "tgz",
    urls = [
        "https://static.crates.io/crates/uart_16550/uart_16550-0.2.15.crate",
    ],
)

# Initialise external dependencies.

load("@bazel_skylib//:workspace.bzl", "bazel_skylib_workspace")

bazel_skylib_workspace()

load("@rules_cc//cc:repositories.bzl", "rules_cc_dependencies", "rules_cc_toolchains")

rules_cc_dependencies()

rules_cc_toolchains()

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(
    version = "1.17",
)

# Set up the Rust crates we depend on. Most of these are fetched
# using the experimental crate_universe functionality in rules_rust.
#
# Some crates require customisation beyond what crate_universe can
# give us. In particular, there doesn't seem to be support for:
#
# - Avoiding the use of optional dependencies.
# - Forcing the use of additional dependencies unconditionally.
#
# For the few crates this precludes, we instead fetch them using
# http_archive targets, injecting a custom build file. Those build
# files are all in //bazel/third_party.

load("@rules_rust//crate_universe:bootstrap.bzl", "crate_universe_bootstrap")
load("@rules_rust//crate_universe:defs.bzl", "crate", "crate_universe")
load("@rules_rust//rust:repositories.bzl", "rust_repositories")

RUST_VERSION = "nightly"

RUST_ISO_DATE = "2021-12-12"

rust_repositories(
    edition = "2018",
    include_rustc_srcs = True,
    iso_date = RUST_ISO_DATE,
    rustfmt_version = RUST_VERSION,
    sha256s = {
        # Update these from https://static.rust-lang.org/dist/YYYY-MM-DD/channel-rust-nightly.toml.
        "2021-12-12/llvm-tools-nightly-x86_64-unknown-linux-gnu": "b59434917226dc9c6b91311d0bad485c216575522336669107ecd689cad4ce9e",
        "2021-12-12/rust-nightly-x86_64-unknown-linux-gnu": "162fefb3d5b9f103d219ac937289c48b15e4cc2b1df744991c3dbcaf63868892",
        "2021-12-12/rust-src-nightly": "03954b83850cb28df3203577629bd2d33d41628a83020e986c37adb50c1a6d22",
        "2021-12-12/rust-std-nightly-x86_64-unknown-linux-gnu": "fefaa696829afabbc613d7afe395cd8f6560f9651e6a1cbc69e4b864fcf85133",
        "2021-12-12/rustfmt-nightly-x86_64-unknown-linux-gnu": "059bb8d1d4698de3a2b978194aa263e9b654be7ded868e9469ded3410207346d",
    },
    version = RUST_VERSION,
)

crate_universe_bootstrap()

# Specify and configure the crates we use.

crate_universe(
    name = "crates",
    iso_date = RUST_ISO_DATE,
    overrides = {
        "byteorder": crate.override(
            features_to_remove = ["std"],
        ),
        "crypto-common": crate.override(
            features_to_remove = ["std"],
        ),
        "digest": crate.override(
            features_to_remove = ["std"],
        ),
        "managed": crate.override(
            features_to_remove = ["std"],
        ),
        "rand": crate.override(
            features_to_remove = [
                "std",
                "std_rng",
            ],
        ),
        "serde": crate.override(
            features_to_remove = ["std"],
        ),
        "sha2": crate.override(
            features_to_remove = ["std"],
        ),
        "toml": crate.override(
            features_to_remove = ["indexmap"],
        ),
    },
    packages = [
        crate.spec(
            name = "bitflags",
            semver = "=1.3.2",
        ),
        crate.spec(
            name = "bit_field",
            semver = "=0.10.1",
        ),
        crate.spec(
            name = "byteorder",
            semver = "=1.4.3",
        ),
        crate.spec(
            name = "chacha20",
            semver = "=0.8.1",
        ),
        crate.spec(
            name = "fixedvec",
            semver = "=0.2.4",
        ),
        crate.spec(
            name = "hex-literal",
            semver = "=0.3.4",
        ),
        crate.spec(
            name = "lazy_static",
            semver = "=1.4.0",
            features = ["spin_no_std"],
        ),
        crate.spec(
            name = "linked_list_allocator",
            semver = "=0.9.0",
        ),
        crate.spec(
            name = "llvm-tools",
            semver = "=0.1.1",
        ),
        crate.spec(
            name = "managed",
            semver = "=0.8",
            features = [
                "alloc",
                "map",
            ],
        ),
        crate.spec(
            name = "pic8259",
            semver = "=0.10.1",
        ),
        crate.spec(
            name = "raw-cpuid",
            semver = "=10.2.0",
        ),
        crate.spec(
            name = "rlibc",
            semver = "=1.0.0",
        ),
        crate.spec(
            name = "serde",
            semver = "=1.0.116",
            features = ["alloc"],
        ),
        crate.spec(
            name = "sha2",
            semver = "=0.10.1",
            features = ["force-soft"],
        ),
        crate.spec(
            name = "spin",
            semver = "=0.9.2",
        ),
        crate.spec(
            name = "thiserror",
            semver = "=1.0.16",
        ),
        crate.spec(
            name = "toml",
            semver = "=0.5.6",
        ),
        crate.spec(
            name = "usize_conversions",
            semver = "=0.2.0",
        ),
        crate.spec(
            name = "volatile",
            semver = "=0.4.4",
        ),
        crate.spec(
            name = "x86_64",
            semver = "=0.14.7",
        ),
        crate.spec(
            name = "xmas-elf",
            semver = "=0.6.2",
        ),
        crate.spec(
            name = "zero",
            semver = "=0.1.2",
        ),
    ],
    resolver = "@rules_rust_crate_universe_bootstrap//:crate_universe_resolver",
    supported_targets = [
        "x86_64-unknown-linux-gnu",
    ],
    version = RUST_VERSION,
)

load("@crates//:defs.bzl", "pinned_rust_install")

pinned_rust_install()
