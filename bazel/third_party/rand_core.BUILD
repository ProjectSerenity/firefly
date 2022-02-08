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
    "notice",  # MIT from expression "MIT OR Apache-2.0"
])

# Generated targets

# buildifier: leave-alone
rust_library(
    name = "rand_core",
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
    version = "0.6.3",
    tags = [
        "cargo-raze",
        "manual",
    ],
    crate_features = [
        "alloc",
    ],
    aliases = select({
        # Default
        "//conditions:default": {
        },
    }),
)
