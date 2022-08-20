# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

rust = [
    crate(
        name = "acpi",
        version = "4.1.1",
        edition = "2018",
        deps = [
            "bit_field",
            "rsdp",
        ],
        patch_args = ["-p1"],
        patches = [
            "third_party/acpi.patch",
        ],
    ),
    crate(
        name = "aml",
        version = "0.16.1",
        edition = "2018",
        deps = [
            "bit_field",
            "bitvec",
            "byteorder",
            "spinning_top",
        ],
        patch_args = ["-p1"],
        patches = [
            "third_party/aml.patch",
        ],
    ),
    crate(
        name = "autocfg",
        version = "1.1.0",
        no_tests = True,  # The tests run rustc and write to arbitrary directories.
    ),
    crate(
        name = "bit_field",
        version = "0.10.1",
    ),
    crate(
        name = "bitflags",
        version = "1.3.2",
        edition = "2018",
        no_tests = True,  # The tests have lots of dependencies.
    ),
    crate(
        name = "bitvec",
        version = "0.22.3",
        edition = "2018",
        deps = [
            "funty",
            "radium",
            "tap",
            "wyz",
        ],
        rustc_env = {
            "CARGO_PKG_REPOSITORY": "https://github.com/bitvecto-rs/bitvec",
        },
        no_tests = True,  # The tests have lots of dependencies.
    ),
    crate(
        name = "byteorder",
        version = "1.4.3",
        edition = "2018",
        no_tests = True,  # The tests depend on quickcheck, which have lots of dependencies.
    ),
    crate(
        name = "cfg-if",
        version = "1.0.0",
        edition = "2018",
    ),
    crate(
        name = "doc-comment",
        version = "0.3.3",
        build_script = "build.rs",
    ),
    crate(
        name = "fixedvec",
        version = "0.2.4",
    ),
    crate(
        name = "funty",
        version = "1.2.0",
        edition = "2018",
        test_deps = [
            "static_assertions",
        ],
    ),
    crate(
        name = "hex-literal",
        version = "0.3.4",
        edition = "2018",
        proc_macro = True,
    ),
    crate(
        name = "lazy_static",
        version = "1.4.0",
        features = ["spin_no_std"],
        deps = [
            "spin",
        ],
        test_deps = [
            "doc-comment",
        ],
    ),
    crate(
        name = "linked_list_allocator",
        version = "0.10.1",
        features = ["const_mut_refs"],
    ),
    crate(
        name = "lock_api",
        version = "0.4.7",
        edition = "2018",
        deps = [
            "scopeguard",
        ],
        build_script = "build.rs",
        build_script_deps = [
            "autocfg",
        ],
    ),
    crate(
        name = "log",
        version = "0.4.17",
        deps = [
            "cfg-if",
        ],
        build_script = "build.rs",
    ),
    crate(
        name = "managed",
        version = "0.8.0",
        features = [
            "alloc",
            "map",
        ],
    ),
    crate(
        name = "pic8259",
        version = "0.10.2",
        edition = "2018",
        deps = [
            "x86_64",
        ],
    ),
    crate(
        name = "radium",
        version = "0.7.0",
        edition = "2018",
        build_script = "build.rs",
        test_deps = [
            "static_assertions",
        ],
    ),
    crate(
        name = "raw-cpuid",
        version = "10.5.0",
        edition = "2018",
        deps = [
            "bitflags",
        ],
        no_tests = True,  # Tests depend on lots of dependencies.
    ),
    crate(
        name = "rsdp",
        version = "2.0.0",
        edition = "2018",
        patch_args = ["-p1"],
        patches = [
            "third_party/rsdp.patch",
        ],
    ),
    crate(
        name = "rustversion",
        version = "1.0.9",
        edition = "2018",
        proc_macro = True,
        build_script = "build/build.rs",
    ),
    crate(
        name = "scopeguard",
        version = "1.1.0",
    ),
    crate(
        name = "smoltcp",
        version = "0.8.1",
        edition = "2018",
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
        deps = [
            "bitflags",
            "byteorder",
            "managed",
        ],
        no_docs = True,  # The docs fail to build. See https://github.com/bazelbuild/rules_rust/issues/689#issuecomment-1132896493
        no_tests = True,  # The tests have lots of dependencies.
    ),
    crate(
        name = "spin",
        version = "0.9.4",
        features = [
            "mutex",
            "once",
            "spin_mutex",
        ],
    ),
    crate(
        name = "spinning_top",
        edition = "2018",
        version = "0.2.4",
        deps = [
            "lock_api",
        ],
    ),
    crate(
        name = "static_assertions",
        version = "1.1.0",
    ),
    crate(
        name = "tap",
        version = "1.0.1",
    ),
    crate(
        name = "uart_16550",
        version = "0.2.18",
        edition = "2018",
        deps = [
            "bitflags",
            "x86_64",
        ],
        proc_macro_deps = [
            "rustversion",
        ],
    ),
    crate(
        name = "volatile",
        version = "0.4.5",
    ),
    crate(
        name = "wyz",
        version = "0.5.0",
        edition = "2018",
        deps = [
            "tap",
        ],
    ),
    crate(
        name = "x86_64",
        version = "0.14.10",
        edition = "2018",
        features = [
            "abi_x86_interrupt",
            "const_fn",
            "doc_cfg",
            "instructions",
            "inline_asm",
            "nightly",
            "step_trait",
        ],
        deps = [
            "bit_field",
            "bitflags",
            "volatile",
        ],
        proc_macro_deps = [
            "rustversion",
        ],
    ),
    crate(
        name = "xmas-elf",
        version = "0.8.0",
        deps = [
            "zero",
        ],
    ),
    crate(
        name = "zero",
        version = "0.1.2",
    ),
]

go = [
    module(
        name = "github.com/BurntSushi/toml",
        version = "v1.2.0",
        packages = [
            package(
                name = "github.com/BurntSushi/toml",
                deps = [
                    "github.com/BurntSushi/toml/internal",
                ],
                test_deps = [
                    "github.com/BurntSushi/toml/internal/tag",
                    "github.com/BurntSushi/toml/internal/toml-test",
                ],
            ),
            package(
                name = "github.com/BurntSushi/toml/internal",
            ),
            package(
                name = "github.com/BurntSushi/toml/internal/tag",
                deps = [
                    "github.com/BurntSushi/toml/internal",
                ],
            ),
            package(
                name = "github.com/BurntSushi/toml/internal/toml-test",
                deps = [
                    "github.com/BurntSushi/toml",
                ],
                embed_globs = [
                    "tests/**",
                ],
            ),
        ],
    ),
    module(
        name = "github.com/bazelbuild/buildtools",
        version = "v0.0.0-20220817114000-5fa80af1e83e",
        packages = [
            package(
                name = "github.com/bazelbuild/buildtools/build",
                build_file = "third_party/github.com_bazelbuild_buildtools_build.BUILD",
            ),
            package(
                name = "github.com/bazelbuild/buildtools/tables",
                no_tests = True,  # The tests don't play nicely when vendored into another Bazel workspace.
            ),
            package(
                name = "github.com/bazelbuild/buildtools/testutils",
            ),
        ],
    ),
    module(
        name = "golang.org/x/crypto",
        version = "v0.0.0-20220817201139-bc19a97f63c8",
        packages = [
            package(
                name = "golang.org/x/crypto/ed25519",
            ),
        ],
    ),
    module(
        name = "golang.org/x/mod",
        version = "v0.6.0-dev.0.20220412012744-41445a152478",
        packages = [
            package(
                name = "golang.org/x/mod/internal/lazyregexp",
            ),
            package(
                name = "golang.org/x/mod/module",
                deps = [
                    "golang.org/x/mod/internal/lazyregexp",
                    "golang.org/x/mod/semver",
                    "golang.org/x/xerrors",
                ],
            ),
            package(
                name = "golang.org/x/mod/semver",
            ),
            package(
                name = "golang.org/x/mod/sumdb",
                deps = [
                    "golang.org/x/mod/internal/lazyregexp",
                    "golang.org/x/mod/module",
                    "golang.org/x/mod/sumdb/note",
                    "golang.org/x/mod/sumdb/tlog",
                ],
                test_deps = [
                    "golang.org/x/mod/sumdb/note",
                    "golang.org/x/mod/sumdb/tlog",
                ],
            ),
            package(
                name = "golang.org/x/mod/sumdb/dirhash",
            ),
            package(
                name = "golang.org/x/mod/sumdb/note",
                deps = [
                    "golang.org/x/crypto/ed25519",
                ],
                test_deps = [
                    "golang.org/x/crypto/ed25519",
                ],
            ),
            package(
                name = "golang.org/x/mod/sumdb/tlog",
            ),
            package(
                name = "golang.org/x/mod/zip",
                deps = [
                    "golang.org/x/mod/module",
                ],
                test_size = "medium",
                test_deps = [
                    "golang.org/x/mod/module",
                    "golang.org/x/mod/sumdb/dirhash",
                    "golang.org/x/tools/txtar",
                ],
            ),
        ],
    ),
    module(
        name = "golang.org/x/time",
        version = "v0.0.0-20220722155302-e5dcc9cfc0b9",
        packages = [
            package(
                name = "golang.org/x/time/rate",
            ),
        ],
    ),
    module(
        name = "golang.org/x/tools",
        version = "v0.1.12",
        packages = [
            package(
                name = "golang.org/x/tools/txtar",
            ),
        ],
    ),
    module(
        name = "golang.org/x/vuln",
        version = "v0.0.0-20220819162940-6faf8534b80b",
        packages = [
            package(
                name = "golang.org/x/vuln/internal/semver",
            ),
            package(
                name = "golang.org/x/vuln/osv",
                deps = [
                    "golang.org/x/mod/semver",
                    "golang.org/x/vuln/internal/semver",
                ],
            ),
        ],
    ),
    module(
        name = "golang.org/x/xerrors",
        version = "v0.0.0-20220609144429-65e65417b02f",
        packages = [
            package(
                name = "golang.org/x/xerrors",
                deps = [
                    "golang.org/x/xerrors/internal",
                ],
            ),
            package(
                name = "golang.org/x/xerrors/internal",
            ),
        ],
    ),
    module(
        name = "rsc.io/diff",
        version = "v0.0.0-20190621135850-fe3479844c3c",
        packages = [
            package(
                name = "rsc.io/diff",
            ),
        ],
    ),
]
