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
            "bazel/patches/acpi.patch",
        ],
    ),
    crate(
        name = "aml",
        version = "0.16.2",
        edition = "2018",
        deps = [
            "bit_field",
            "byteorder",
            "spin",
        ],
        patch_args = ["-p1"],
        patches = [
            "bazel/patches/aml.patch",
        ],
    ),
    crate(
        name = "autocfg",
        version = "1.1.0",
        no_tests = True,  # Several tests assume Cargo's management.
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
        name = "byteorder",
        version = "1.4.3",
        edition = "2018",
        no_tests = True,  # The tests depend on quickcheck, which have lots of dependencies.
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
        version = "0.10.4",
        features = ["const_mut_refs"],
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
        name = "memoffset",
        version = "0.6.5",
        build_script = "build.rs",
        build_script_deps = [
            "autocfg",
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
        name = "raw-cpuid",
        version = "10.6.0",
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
            "bazel/patches/rsdp.patch",
        ],
    ),
    crate(
        name = "rustversion",
        version = "1.0.11",
        edition = "2018",
        proc_macro = True,
        build_script = "build/build.rs",
    ),
    crate(
        name = "smoltcp",
        version = "0.8.2",
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
        version = "0.4.6",
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
        version = "0.9.0",
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
        version = "v1.2.1",
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
        version = "v0.0.0-20230113180850-180a94ab3a3a",
        packages = [
            package(
                name = "github.com/bazelbuild/buildtools/build",
                build_file = "bazel/patches/github.com_bazelbuild_buildtools_build.BUILD",
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
        version = "v0.5.0",
        packages = [
            package(
                name = "golang.org/x/crypto/ed25519",
            ),
        ],
    ),
    module(
        name = "golang.org/x/mod",
        version = "v0.7.0",
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
        version = "v0.3.0",
        packages = [
            package(
                name = "golang.org/x/time/rate",
            ),
        ],
    ),
    module(
        name = "golang.org/x/tools",
        version = "v0.5.0",
        packages = [
            package(
                name = "golang.org/x/tools/txtar",
            ),
        ],
    ),
    module(
        name = "golang.org/x/vuln",
        version = "v0.0.0-20230118164824-4ec8867cc0e6",
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
        version = "v0.0.0-20220907171357-04be3eba64a2",
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
