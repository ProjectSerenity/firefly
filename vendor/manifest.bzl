# Code generated by vendeps. DO NOT EDIT.

# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

rust = [
    crate(
        name = "acpi",
        version = "4.1.1",
        digest = "sha256:0da455b9606168cf46238320e44bb70abeb34d4d57aefce231808df3bada6af0",
    ),
    crate(
        name = "aml",
        version = "0.16.1",
        digest = "sha256:19f995f4af325e4fe1503560620f4075780d6b28f790d6b3ce474faae717bcf3",
    ),
    crate(
        name = "autocfg",
        version = "1.1.0",
        digest = "sha256:12f1f63c021f8e2912aa19a95c3e8f866ad98c9ea91017bb424114efc9184334",
    ),
    crate(
        name = "bit_field",
        version = "0.10.1",
        digest = "sha256:29b4061a841923f8aa6f215f81bda81f74c75b5fdd41f543d2ac51b8f5c8f316",
    ),
    crate(
        name = "bitflags",
        version = "1.3.2",
        digest = "sha256:620bbcf2a88ebee65366673cae085ea8c9945fcef1891d1c2b76a2b23f6b6aa1",
    ),
    crate(
        name = "bitvec",
        version = "0.22.3",
        digest = "sha256:efe6873f249e9c57c52efaad101fc6047a8961c89731658da3f56c13dee7cfc6",
    ),
    crate(
        name = "byteorder",
        version = "1.4.3",
        digest = "sha256:b44b7b8a858e8df3cf7ff4484609b0bf57ee2e0bd23c6a32aa8b9e4c212dbf81",
    ),
    crate(
        name = "cfg-if",
        version = "1.0.0",
        digest = "sha256:5154e65781591457f239de362aae282daf94061304c43e8dbfd64f83d577adb4",
    ),
    crate(
        name = "doc-comment",
        version = "0.3.3",
        digest = "sha256:04773b3cadd218cfcd1b522f3ba66abb649d860734ae7ae767a6434db92e5743",
    ),
    crate(
        name = "fixedvec",
        version = "0.2.4",
        digest = "sha256:9eef3fc63f4a7a3ef6670a4d4a01df6a19f09facbb059fb9dff6ce07bf93c4c2",
    ),
    crate(
        name = "funty",
        version = "1.2.0",
        digest = "sha256:652a9fc37a47c42afbb3348615711b469fca43decbd0025edacadff9e479b81e",
    ),
    crate(
        name = "hex-literal",
        version = "0.3.4",
        digest = "sha256:1b59e526cb4a7fcceb99b74c2393a7cd19284ca26521b9bae7277a164e87ace5",
    ),
    crate(
        name = "lazy_static",
        version = "1.4.0",
        digest = "sha256:19316bf6ec3f41e60a95ac7b66a9391df452051881a008765f854a1c911204f1",
    ),
    crate(
        name = "linked_list_allocator",
        version = "0.10.1",
        digest = "sha256:a937cbedec34cca9f72db51a04dd1654dc53ff3a8b3b5d94e1e1a39e4637d9d0",
    ),
    crate(
        name = "lock_api",
        version = "0.4.7",
        digest = "sha256:1e17f46359d88912e76f7748ce30a874e517d2d49bbe24bc19201f3319c9e519",
    ),
    crate(
        name = "log",
        version = "0.4.17",
        digest = "sha256:c04e87a51c74a6c1c38c8a64808edc0ce818c9f8291962a48ad6f6e91deabbfe",
    ),
    crate(
        name = "managed",
        version = "0.8.0",
        digest = "sha256:a4caf9de3ee73c274d82439238c0cbcf9fb50a8369b64f8c4d587cc541af5057",
    ),
    crate(
        name = "pic8259",
        version = "0.10.2",
        digest = "sha256:c6a1b19286fd77550b9d94ce44133596188c6d703150351c926fb992848e1910",
    ),
    crate(
        name = "radium",
        version = "0.7.0",
        digest = "sha256:4e2e29fe24658d34164ba3f389e1b85c9463aa25b4b8634c1c8c4d68c99992f6",
    ),
    crate(
        name = "raw-cpuid",
        version = "10.5.0",
        digest = "sha256:832eaa810b7987147fd31925205583b9d22ade2b2b48cb974ca8478b6184e470",
    ),
    crate(
        name = "rsdp",
        version = "2.0.0",
        digest = "sha256:4b963102cb1b3e01a489048851de8b21505ef8e4c7cdd7bc2db05b3d97dba8be",
        patch_args = [
            "-p1",
        ],
        patches = [
            "third_party/rsdp.patch",
        ],
        patch_digest = "sha256:633510063b7cb407b441b18e1ec4b2c85e20511a54a2217b562f82c5e5cf91c2",
    ),
    crate(
        name = "rustversion",
        version = "1.0.9",
        digest = "sha256:9b9d5802964a2d192be20052543cdcd5405248ff22738212d7de614c6a725cff",
    ),
    crate(
        name = "scopeguard",
        version = "1.1.0",
        digest = "sha256:69a679946cd73c39bd8cbed4f633c9b3012547bf6014a446c8ee5530a4056aae",
    ),
    crate(
        name = "smoltcp",
        version = "0.8.1",
        digest = "sha256:a2acd28879f77e694c9b2d90dfa969ee577214ca862900947427c8baf0eff2d4",
    ),
    crate(
        name = "spin",
        version = "0.9.4",
        digest = "sha256:950ded1cfe90f8fc09fb0260e3184ea50afb95a5f530ad4fe317fbe8ab1ecce8",
    ),
    crate(
        name = "spinning_top",
        version = "0.2.4",
        digest = "sha256:5ef1cb7be17d6c8fae84caf919eb065094625e18e9eeaaf1f76465d900ad43ac",
    ),
    crate(
        name = "static_assertions",
        version = "1.1.0",
        digest = "sha256:07527fab0fb8cecae9e3fcd0684883dc8b35ca8512e06ce795327674a63521d5",
    ),
    crate(
        name = "tap",
        version = "1.0.1",
        digest = "sha256:7b55bd4fe29a53c3480cbff8ef7b4b8d190302720f3c91c4bace0d48fc3836cb",
    ),
    crate(
        name = "uart_16550",
        version = "0.2.18",
        digest = "sha256:f58497bfb4bfba4fce95f5b518db8a162e9d033504eacae78033f1f2c8ada2a7",
    ),
    crate(
        name = "volatile",
        version = "0.4.5",
        digest = "sha256:f583a4d1ff9e541ba7ca79417a1656e96bcefb7afd9646fb437fb88d4018417a",
    ),
    crate(
        name = "wyz",
        version = "0.5.0",
        digest = "sha256:16e1ff51208f4ffc717bc9e3bb7642f04c09b59e8289c4e04a7a7ae6adc471a3",
    ),
    crate(
        name = "x86_64",
        version = "0.14.10",
        digest = "sha256:1aaa83c766b007375e43d85f17528241f22fb8b31b271168498fa99ef252756b",
    ),
    crate(
        name = "xmas-elf",
        version = "0.8.0",
        digest = "sha256:7ec4aaf04d8feb3fdb313654ddfcaaad78b0e24b6a15095fed5e6a91dff59396",
    ),
    crate(
        name = "zero",
        version = "0.1.2",
        digest = "sha256:7023c087cbadc9605b03585e687b06a5bb9b9b3e83fb00430c3850040d462596",
    ),
]

go = [
    module(
        name = "github.com/BurntSushi/toml",
        version = "v1.2.0",
        digest = "sha256:100ca6584b169504eb8625f39f923db07d4cb936e4cdcf272b1ef6c308cd70ff",
    ),
    module(
        name = "github.com/bazelbuild/buildtools",
        version = "v0.0.0-20220817114000-5fa80af1e83e",
        digest = "sha256:5fb158bdff540fc1c984065d1cef4acd4e6f94869a0b5b26f0bd29b4ed192509",
    ),
    module(
        name = "golang.org/x/crypto",
        version = "v0.0.0-20220817201139-bc19a97f63c8",
        digest = "sha256:9c1ca6648995078cbaa8083af18bec6ca7242b83a4c2a5a7b204559dff835a7d",
    ),
    module(
        name = "golang.org/x/mod",
        version = "v0.6.0-dev.0.20220412012744-41445a152478",
        digest = "sha256:ed2568b61fa50594b640142ec883d17ee5a7edd46fe7c2e40960ab9637adb5ba",
    ),
    module(
        name = "golang.org/x/time",
        version = "v0.0.0-20220722155302-e5dcc9cfc0b9",
        digest = "sha256:423a83bf899b93fd6004225201c9faa415872a7cd3813ab236a4f03d916d6fd6",
    ),
    module(
        name = "golang.org/x/tools",
        version = "v0.1.12",
        digest = "sha256:f9d4c420baf4a0fad2e22a02fee477ab2f450134a5e686e52baac19c7b8028ac",
    ),
    module(
        name = "golang.org/x/vuln",
        version = "v0.0.0-20220819162940-6faf8534b80b",
        digest = "sha256:994a4fa4620e8f298646a17b96f71d336ea0965fcb75bd61a196349df6fc84dd",
    ),
    module(
        name = "golang.org/x/xerrors",
        version = "v0.0.0-20220609144429-65e65417b02f",
        digest = "sha256:c0b0541a468b065940c635cb07ee30da788c8b01ecde1655caaa503ce05840d5",
    ),
    module(
        name = "rsc.io/diff",
        version = "v0.0.0-20190621135850-fe3479844c3c",
        digest = "sha256:2d08e81c4ae9aa1a306761dd6999b07ce470057ae3deca4c90ebc2072508127e",
    ),
]
