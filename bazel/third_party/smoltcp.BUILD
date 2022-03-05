# buildifier: disable=load
load("@bazel_skylib//lib:selects.bzl", "selects")

# buildifier: disable=load
load(
    "@rules_rust//rust:defs.bzl",
    "rust_binary",
    "rust_library",
    "rust_proc_macro",
    "rust_test",
)

package(default_visibility = [
    "//visibility:public",
])

licenses([
    "restricted",  # 0BSD from expression "0BSD"
])

# Generated targets
# Unsupported target "bench" with type "bench" omitted
# Unsupported target "benchmark" with type "example" omitted
# Unsupported target "client" with type "example" omitted
# Unsupported target "dhcp_client" with type "example" omitted
# Unsupported target "httpclient" with type "example" omitted
# Unsupported target "loopback" with type "example" omitted
# Unsupported target "multicast" with type "example" omitted
# Unsupported target "packet2pcap" with type "example" omitted
# Unsupported target "ping" with type "example" omitted
# Unsupported target "server" with type "example" omitted
# Unsupported target "sixlowpan" with type "example" omitted
# Unsupported target "tcpdump" with type "example" omitted

# buildifier: leave-alone
rust_library(
    name = "smoltcp",
    deps = [
        "@crates__bitflags__1_3_2//:bitflags",
        "@crates__byteorder__1_4_3//:byteorder",
        "@crates__libc__0_2_119//:libc",
        "@crates__managed__0_8_0//:managed",
        "@rand_core",
    ],
    srcs = glob(["**/*.rs"]),
    crate_root = "src/lib.rs",
    edition = "2018",
    rustc_flags = [
        "--cap-lints=allow",
    ],
    data = glob(
        ["**"],
        exclude = [
            # These can be manually added with overrides if needed.

            # If you run `cargo build` in this dir, the target dir can get very big very quick.
            "target/**",

            # These are not vendored from the crate - we exclude them to avoid busting caches
            # when we change how we generate BUILD files and such.
            "BUILD.bazel",
            "WORKSPACE.bazel",
            "WORKSPACE",
        ],
    ),
    version = "0.8.0",
    tags = [
        "cargo-raze",
        "manual",
    ],
    crate_features = [
        "alloc",
        "async",
        "default",
        "libc",
        "medium-ethernet",
        "proto-dhcpv4",
        "proto-ipv4",
        "rand_core",
        "socket",
        "socket-dhcpv4",
        "socket-raw",
        "socket-tcp",
        "socket-udp",
    ],
    aliases = select({
        # Default
        "//conditions:default": {
        },
    }),
)
