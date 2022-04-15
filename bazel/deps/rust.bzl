# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@rules_rust//crate_universe:defs.bzl", "crate", "crates_repository")
load("@rules_rust//crate_universe:repositories.bzl", "crate_universe_dependencies")
load("@rules_rust//rust:repositories.bzl", "rules_rust_dependencies", "rust_register_toolchains")

RUST_VERSION = "nightly"

RUST_ISO_DATE = "2022-04-10"

LLVM_TOOLS = struct(
    name = "llvm-tools-nightly-x86_64-unknown-linux-gnu",
    sum = "3ccc356d9d7a4415790db539aa1c449d77b75d249732bbe0cb3248a5f39e428d",
)

RUST = struct(
    name = "rust-nightly-x86_64-unknown-linux-gnu",
    sum = "05af4d844b308bfee0baa0f61a977a928b6b7eb27d4c859ececed5cab83a055d",
)

RUST_SRC = struct(
    name = "rust-src-nightly",
    sum = "19bd1a6030c98643ed270682b031997fb323fc90fefc72fe2cb313e256ab0016",
)

RUST_STD = struct(
    name = "rust-std-nightly-x86_64-unknown-linux-gnu",
    sum = "4166a60222de2c491847c3c925bcaf341afb19cad512f1c702b3b48e90867c90",
)

RUST_RUSTFMT = struct(
    name = "rustfmt-nightly-x86_64-unknown-linux-gnu",
    sum = "7fe3049fb4003f35539e622801cd62e1d20481915e4879aeb47965dafeb859bf",
)

RUST_NO_STD = struct(
    name = "rust-std-nightly-x86_64-unknown-none",
    sum = "35cd94ae9a6efc1839c227470041038e3c51f50db1f2c59ed7f5b32d03f4cd2f",
)

RUST_CRATE_ANNOTATIONS = {
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
    "linked_list_allocator": crate.spec(
        default_features = False,
        features = ["const_mut_refs"],
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
    "raw-cpuid": crate.spec(
        version = "=10.3.0",
    ),
    "rlibc": crate.spec(
        version = "=1.0.0",
    ),
    "serde": crate.spec(
        default_features = False,
        features = ["alloc"],
        version = "=1.0.136",
    ),
    "smoltcp": crate.spec(
        default_features = False,
        features = [
            "alloc",
            "async",
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
    "toml": crate.spec(
        version = "=0.5.8",
    ),
    "uart_16550": crate.spec(
        version = "=0.2.17",
    ),
    "usize_conversions": crate.spec(
        version = "=0.2.0",
    ),
    "volatile": crate.spec(
        version = "=0.4.4",
    ),
    "x86_64": crate.spec(
        version = "=0.14.9",
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
        sha256 = "de78decc37247c7cfac5dbf3495c7298c6ac97cb355161caa7e15969c6648e6c",
        strip_prefix = "bootloader-0.9.22",  # Keep this in sync with its BUILD file.
        type = "tgz",
        urls = [
            "https://static.crates.io/crates/bootloader/bootloader-0.9.22.crate",
        ],
    )

    # Fetch libcore, liballoc, and libcompiler-builtins
    # for the x86_64-unknown-none build target.

    http_archive(
        name = "rust_none_x86_64",
        build_file = "//bazel/third_party:no_std.BUILD",
        sha256 = RUST_NO_STD.sum,
        strip_prefix = "rust-std-nightly-x86_64-unknown-none",
        type = "tgz",
        urls = ["https://static.rust-lang.org/dist/" + RUST_ISO_DATE + "/" + RUST_NO_STD.name],
    )

    # Set up the Rust crates we depend on. Most of these are fetched
    # using the experimental crates_repository functionality in rules_rust.

    rules_rust_dependencies()

    rust_register_toolchains(
        edition = "2021",
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

    crate_universe_dependencies()

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
