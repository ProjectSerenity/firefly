# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@rules_rust//crate_universe:bootstrap.bzl", "crate_universe_bootstrap")
load("@rules_rust//crate_universe:defs.bzl", "crate", "crate_universe")
load("@rules_rust//rust:repositories.bzl", "rules_rust_dependencies", "rust_register_toolchains")

RUST_VERSION = "nightly"

RUST_ISO_DATE = "2022-02-01"

LLVM_TOOLS = struct(
    name = "llvm-tools-nightly-x86_64-unknown-linux-gnu",
    sum = "3eeba27c46ac7f2fd9092ed5baf8616c04021ac359f136a484b5942229e590fc",
)

RUST = struct(
    name = "rust-nightly-x86_64-unknown-linux-gnu",
    sum = "fe928a3f280355a1b87eb414ac9ab1333a38a3e5e6be1f1d6fa3e990527aec80",
)

RUST_SRC = struct(
    name = "rust-src-nightly",
    sum = "6177a62bd2c56dfeda4552d64d9f840ce3bbdef7206b9bcd7047c0b5af56f4a8",
)

RUST_STD = struct(
    name = "rust-std-nightly-x86_64-unknown-linux-gnu",
    sum = "882f458492f7efa8a9af5e5ffc8b70183107447fe4604a8c47a120b4f319e72e",
)

RUST_RUSTFMT = struct(
    name = "rustfmt-nightly-x86_64-unknown-linux-gnu",
    sum = "6cd904d0413a858a6073f1a553d2aa46e32124574da996dcd0d8aaeb706bd035",
)

RUST_CRATE_OVERRIDES = {
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
}

RUST_CRATES = [
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
        name = "libc",
        semver = "=0.2.117",
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
]

def rust_deps():
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

    rules_rust_dependencies()

    rust_register_toolchains(
        edition = "2018",
        include_rustc_srcs = True,
        iso_date = RUST_ISO_DATE,
        rustfmt_version = RUST_VERSION,
        sha256s = {
            RUST_ISO_DATE + "/" + LLVM_TOOLS.name: LLVM_TOOLS.sum,
            RUST_ISO_DATE + "/" + RUST.name: RUST.sum,
            RUST_ISO_DATE + "/" + RUST_SRC.name: RUST_SRC.sum,
            RUST_ISO_DATE + "/" + RUST_STD.name: RUST_STD.sum,
            RUST_ISO_DATE + "/" + RUST_RUSTFMT.name: RUST_RUSTFMT.sum,
        },
        version = RUST_VERSION,
    )

    crate_universe_bootstrap()

    # Specify and configure the crates we use.

    crate_universe(
        name = "crates",
        iso_date = RUST_ISO_DATE,
        overrides = RUST_CRATE_OVERRIDES,
        packages = RUST_CRATES,
        resolver = "@rules_rust_crate_universe_bootstrap//:crate_universe_resolver",
        supported_targets = [
            "x86_64-unknown-linux-gnu",
        ],
        version = RUST_VERSION,
    )
