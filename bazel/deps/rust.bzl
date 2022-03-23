# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@rules_rust//crate_universe:crates.bzl", "crate_deps_repository")
load("@rules_rust//crate_universe:defs.bzl", "crate", "crates_repository")
load("@rules_rust//rust:repositories.bzl", "rules_rust_dependencies", "rust_register_toolchains")

RUST_VERSION = "nightly"

RUST_ISO_DATE = "2022-03-01"

LLVM_TOOLS = struct(
    name = "llvm-tools-nightly-x86_64-unknown-linux-gnu",
    sum = "da4fa8b7f33ac9c4ee4a7b88605883376ab142989a931485b0d66ccde017db29",
)

RUST = struct(
    name = "rust-nightly-x86_64-unknown-linux-gnu",
    sum = "d7e2aa9d5c8bb459f947fcae59b78d1b0590499eeb46eb57ddc9125a4bf24530",
)

RUST_SRC = struct(
    name = "rust-src-nightly",
    sum = "8f5126a9df3dcdc5b663e0697eef307d30bd5e65933e9372a6cf7096f0971b7e",
)

RUST_STD = struct(
    name = "rust-std-nightly-x86_64-unknown-linux-gnu",
    sum = "d0e68189692905dd7fa948b89b6616836079d4176116e15444687c5ebd258007",
)

RUST_RUSTFMT = struct(
    name = "rustfmt-nightly-x86_64-unknown-linux-gnu",
    sum = "9cf744dde9732d9b1039137acaa7e08aa7d05ca25e8039ecfffbd06d096a2b35",
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
        sha256 = "de78decc37247c7cfac5dbf3495c7298c6ac97cb355161caa7e15969c6648e6c",
        strip_prefix = "bootloader-0.9.22",  # Keep this in sync with its BUILD file.
        type = "tgz",
        urls = [
            "https://static.crates.io/crates/bootloader/bootloader-0.9.22.crate",
        ],
    )

    http_archive(
        name = "compiler_builtins",
        build_file = "//bazel/third_party:compiler_builtins.BUILD",
        sha256 = "80873f979f0a344a4ade87c2f70d9ccf5720b83b10c97ec7cd745895d021e85a",
        strip_prefix = "compiler_builtins-0.1.70",
        type = "tgz",
        urls = [
            "https://static.crates.io/crates/compiler_builtins/compiler_builtins-0.1.70.crate",
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
        edition = "2021",
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
