# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@rules_rust//crate_universe:crates.bzl", "crate_deps_repository")
load("@rules_rust//crate_universe:defs.bzl", "crate", "crates_repository")
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

RUST_CRATE_ANNOTATIONS = {
    "chacha20": [crate.annotation(
        deps = ["@crates//:cpufeatures"],
    )],
    "uart_16550": [crate.annotation(
        deps = ["@crates//:x86_64"],
    )],
}

# After chaning any of these, the next build will
# need to be run with CARGO_BAZEL_REPIN=true.
RUST_CRATES = {
    "bit_field": crate.spec(
        version = "=0.10.1",
    ),
    "bitflags": crate.spec(
        version = "=1.3.2",
    ),
    "byteorder": crate.spec(
        default_features = False,
        version = "=1.4.3",
    ),
    "chacha20": crate.spec(
        version = "=0.9.0",
    ),
    "cpufeatures": crate.spec(
        version = "=0.2.1",
    ),
    "digest": crate.spec(
        #default_features = False,
        version = "=0.10.3",
    ),
    "fixedvec": crate.spec(
        version = "=0.2.4",
    ),
    "hex-literal": crate.spec(
        version = "=0.3.4",
    ),
    "lazy_static": crate.spec(
        features = ["spin_no_std"],
        version = "=1.4.0",
    ),
    "libc": crate.spec(
        version = "=0.2.119",
    ),
    "linked_list_allocator": crate.spec(
        version = "=0.9.1",
    ),
    "llvm-tools": crate.spec(
        version = "=0.1.1",
    ),
    "managed": crate.spec(
        default_features = False,
        features = [
            "alloc",
            "map",
        ],
        version = "=0.8",
    ),
    "pic8259": crate.spec(
        version = "=0.10.2",
    ),
    "rand": crate.spec(
        default_features = False,
        version = "=0.8.5",
    ),
    "raw-cpuid": crate.spec(
        version = "=10.2.0",
    ),
    "rlibc": crate.spec(
        version = "=1.0.0",
    ),
    "serde": crate.spec(
        default_features = False,
        features = ["alloc"],
        version = "=1.0.136",
    ),
    "sha2": crate.spec(
        default_features = False,
        features = ["force-soft"],
        version = "=0.10.2",
    ),
    "smoltcp": crate.spec(
        default_features = False,
        features = [
            "alloc",
            "async",
            "libc",
            "medium-ethernet",
            "proto-dhcpv4",
            "proto-ipv4",
            "socket",
            "socket-dhcpv4",
            "socket-raw",
            "socket-tcp",
            "socket-udp",
        ],
        version = "=0.8.0",
    ),
    "spin": crate.spec(
        version = "=0.9.2",
    ),
    "thiserror": crate.spec(
        version = "=1.0.30",
    ),
    "toml": crate.spec(
        version = "=0.5.8",
    ),
    "uart_16550": crate.spec(
        version = "=0.2.16",
    ),
    "usize_conversions": crate.spec(
        version = "=0.2.0",
    ),
    "volatile": crate.spec(
        version = "=0.4.4",
    ),
    "x86_64": crate.spec(
        version = "=0.14.8",
    ),
    "xmas-elf": crate.spec(
        version = "=0.8.0",
    ),
    "zero": crate.spec(
        version = "=0.1.2",
    ),
}

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

    # Specify and configure the crates we use.

    crate_deps_repository(
        bootstrap = True,
    )

    crates_repository(
        name = "crates",
        rust_version = RUST_VERSION + "-" + RUST_ISO_DATE,
        annotations = RUST_CRATE_ANNOTATIONS,
        generator = "@cargo_bazel_bootstrap//:cargo-bazel",
        lockfile = "//bazel/deps:Cargo.Bazel.lock",
        packages = RUST_CRATES,
        supported_platform_triples = [
            "x86_64-unknown-linux-gnu",
        ],
    )
