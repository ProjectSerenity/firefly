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
    "notice",  # MIT from expression "MIT"
])

# Generated targets

# buildifier: leave-alone
rust_library(
    name = "uart_16550",
    deps = [
        "@crates__bitflags__1_3_2//:bitflags",
        "@crates__x86_64__0_14_8//:x86_64",
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
    version = "0.2.15",
    tags = [
        "cargo-raze",
        "manual",
    ],
    crate_features = [
        "default",
        "nightly",
    ],
    aliases = select({
        #  cfg(target_arch = "x86_64")
        "@rules_rust//rust/platform:x86_64-unknown-linux-gnu": {
        },
        # Default
        "//conditions:default": {
        },
    }),
)
