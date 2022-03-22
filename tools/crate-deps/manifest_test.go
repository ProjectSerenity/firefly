// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestParseManifest(t *testing.T) {
	name := filepath.Join("testdata", "Cargo.Bazel.lock")
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("failed to read %s: %v", name, err)
	}

	var got Manifest
	err = json.Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", name, err)
	}

	// If the two match, we're all done.
	want := wantManifest
	if reflect.DeepEqual(got, want) {
		return
	}

	// Work out what differed.

	if got.Checksum != want.Checksum {
		t.Errorf("Checksum mismatch:\n  Got:  %q\n  Want: %q", got.Checksum, want.Checksum)
	}

	g := make([]string, 0, len(got.Crates))
	for key := range got.Crates {
		g = append(g, key)
	}

	w := make([]string, 0, len(want.Crates))
	for key := range want.Crates {
		w = append(w, key)
	}

	sort.Strings(g)
	sort.Strings(w)

	if len(g) != len(w) {
		t.Fatalf("Crates list mismatch:\n  Got:  %q\n  Want: %q", g, w)
	}

	for i := range g {
		if g[i] != w[i] {
			t.Errorf("Crates[%d] mismatch:\n  Got:  %q\n  Want: %q", i, g[i], w[i])
			continue
		}

		g := got.Crates[g[i]]
		w := want.Crates[w[i]]
		if !reflect.DeepEqual(g, w) {
			t.Errorf("Crates[%d] mismatch:\n  Got:  %#v\n  Want: %#v", i, g, w)
		}
	}

	if t.Failed() {
		return
	}

	// Resort to the blunt instrument.
	t.Fatalf("Manifest mismatch:\n  Got:  %#v\n  Want: %#v", got, want)
}

var wantManifest = Manifest{
	Checksum: "14c845d163370d468cd89ba24e64a1a6bfed2ff69ad41fa835b7e2b0c3667007",
	Crates: map[string]*Crate{
		"bit_field 0.10.1": {
			Name:    "bit_field",
			Version: "0.10.1",
		},
		"bitflags 1.3.2": {
			Name:    "bitflags",
			Version: "1.3.2",
		},
		"block-buffer 0.10.2": {
			Name:    "block-buffer",
			Version: "0.10.2",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "generic-array 0.14.5",
							Target: "generic_array",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"byteorder 1.4.3": {
			Name:    "byteorder",
			Version: "1.4.3",
		},
		"cfg-if 1.0.0": {
			Name:    "cfg-if",
			Version: "1.0.0",
		},
		"chacha20 0.9.0": {
			Name:    "chacha20",
			Version: "0.9.0",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "cfg-if 1.0.0",
							Target: "cfg_if",
						},
						{
							Id:     "cipher 0.4.3",
							Target: "cipher",
						},
					},
					Selects: map[string][]BazelTarget{
						"cfg(any(target_arch = \"x86_64\", target_arch = \"x86\"))": []BazelTarget{
							{
								Id:     "cpufeatures 0.2.2",
								Target: "cpufeatures",
							},
						},
					},
				},
				ExtraDeps: []string{
					"@crates//:cpufeatures",
				},
			},
		},
		"cipher 0.4.3": {
			Name:    "cipher",
			Version: "0.4.3",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "crypto-common 0.1.3",
							Target: "crypto_common",
						},
						{
							Id:     "inout 0.1.2",
							Target: "inout",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"cpufeatures 0.2.2": {
			Name:    "cpufeatures",
			Version: "0.2.2",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{},
					Selects: map[string][]BazelTarget{
						"aarch64-apple-darwin": []BazelTarget{
							{
								Id:     "libc 0.2.121",
								Target: "libc",
							},
						},
						"aarch64-linux-android": []BazelTarget{
							{
								Id:     "libc 0.2.121",
								Target: "libc",
							},
						},
						"cfg(all(target_arch = \"aarch64\", target_os = \"linux\"))": []BazelTarget{
							{
								Id:     "libc 0.2.121",
								Target: "libc",
							},
						},
					},
				},
			},
		},
		"crypto-common 0.1.3": {
			Name:    "crypto-common",
			Version: "0.1.3",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "generic-array 0.14.5",
							Target: "generic_array",
						},
						{
							Id:     "typenum 1.15.0",
							Target: "typenum",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"digest 0.10.3": {
			Name:    "digest",
			Version: "0.10.3",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "block-buffer 0.10.2",
							Target: "block_buffer",
						},
						{
							Id:     "crypto-common 0.1.3",
							Target: "crypto_common",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"direct-cargo-bazel-deps 0.0.1": {
			Name:    "direct-cargo-bazel-deps",
			Version: "0.0.1",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "bit_field 0.10.1",
							Target: "bit_field",
						},
						{
							Id:     "bitflags 1.3.2",
							Target: "bitflags",
						},
						{
							Id:     "byteorder 1.4.3",
							Target: "byteorder",
						},
						{
							Id:     "chacha20 0.9.0",
							Target: "chacha20",
						},
						{
							Id:     "cpufeatures 0.2.2",
							Target: "cpufeatures",
						},
						{
							Id:     "digest 0.10.3",
							Target: "digest",
						},
						{
							Id:     "fixedvec 0.2.4",
							Target: "fixedvec",
						},
						{
							Id:     "lazy_static 1.4.0",
							Target: "lazy_static",
						},
						{
							Id:     "libc 0.2.121",
							Target: "libc",
						},
						{
							Id:     "linked_list_allocator 0.9.1",
							Target: "linked_list_allocator",
						},
						{
							Id:     "llvm-tools 0.1.1",
							Target: "llvm_tools",
						},
						{
							Id:     "managed 0.8.0",
							Target: "managed",
						},
						{
							Id:     "pic8259 0.10.2",
							Target: "pic8259",
						},
						{
							Id:     "rand 0.8.5",
							Target: "rand",
						},
						{
							Id:     "raw-cpuid 10.3.0",
							Target: "raw_cpuid",
						},
						{
							Id:     "rlibc 1.0.0",
							Target: "rlibc",
						},
						{
							Id:     "serde 1.0.136",
							Target: "serde",
						},
						{
							Id:     "sha2 0.10.2",
							Target: "sha2",
						},
						{
							Id:     "smoltcp 0.8.0",
							Target: "smoltcp",
						},
						{
							Id:     "spin 0.9.2",
							Target: "spin",
						},
						{
							Id:     "thiserror 1.0.30",
							Target: "thiserror",
						},
						{
							Id:     "toml 0.5.8",
							Target: "toml",
						},
						{
							Id:     "uart_16550 0.2.16",
							Target: "uart_16550",
						},
						{
							Id:     "usize_conversions 0.2.0",
							Target: "usize_conversions",
						},
						{
							Id:     "volatile 0.4.4",
							Target: "volatile",
						},
						{
							Id:     "x86_64 0.14.8",
							Target: "x86_64",
						},
						{
							Id:     "xmas-elf 0.8.0",
							Target: "xmas_elf",
						},
						{
							Id:     "zero 0.1.2",
							Target: "zero",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
				ProcMacroDeps: Deps{
					Common: []BazelTarget{
						{
							Id:     "hex-literal 0.3.4",
							Target: "hex_literal",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"fixedvec 0.2.4": {
			Name:    "fixedvec",
			Version: "0.2.4",
		},
		"generic-array 0.14.5": {
			Name:    "generic-array",
			Version: "0.14.5",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "generic-array 0.14.5",
							Target: "build_script_build",
						},
						{
							Id:     "typenum 1.15.0",
							Target: "typenum",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
			BuildScriptAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "version_check 0.9.4",
							Target: "version_check",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"hex-literal 0.3.4": {
			Name:    "hex-literal",
			Version: "0.3.4",
		},
		"inout 0.1.2": {
			Name:    "inout",
			Version: "0.1.2",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "generic-array 0.14.5",
							Target: "generic_array",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"lazy_static 1.4.0": {
			Name:    "lazy_static",
			Version: "1.4.0",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "spin 0.5.2",
							Target: "spin",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"libc 0.2.121": {
			Name:    "libc",
			Version: "0.2.121",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "libc 0.2.121",
							Target: "build_script_build",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"linked_list_allocator 0.9.1": {
			Name:    "linked_list_allocator",
			Version: "0.9.1",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "spinning_top 0.2.4",
							Target: "spinning_top",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"llvm-tools 0.1.1": {
			Name:    "llvm-tools",
			Version: "0.1.1",
		},
		"lock_api 0.4.6": {
			Name:    "lock_api",
			Version: "0.4.6",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "scopeguard 1.1.0",
							Target: "scopeguard",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"managed 0.8.0": {
			Name:    "managed",
			Version: "0.8.0",
		},
		"pic8259 0.10.2": {
			Name:    "pic8259",
			Version: "0.10.2",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "x86_64 0.14.8",
							Target: "x86_64",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"proc-macro2 1.0.36": {
			Name:    "proc-macro2",
			Version: "1.0.36",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "proc-macro2 1.0.36",
							Target: "build_script_build",
						},
						{
							Id:     "unicode-xid 0.2.2",
							Target: "unicode_xid",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"quote 1.0.16": {
			Name:    "quote",
			Version: "1.0.16",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "proc-macro2 1.0.36",
							Target: "proc_macro2",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"rand 0.8.5": {
			Name:    "rand",
			Version: "0.8.5",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "rand_core 0.6.3",
							Target: "rand_core",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"rand_core 0.6.3": {
			Name:    "rand_core",
			Version: "0.6.3",
		},
		"raw-cpuid 10.3.0": {
			Name:    "raw-cpuid",
			Version: "10.3.0",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "bitflags 1.3.2",
							Target: "bitflags",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"rlibc 1.0.0": {
			Name:    "rlibc",
			Version: "1.0.0",
		},
		"scopeguard 1.1.0": {
			Name:    "scopeguard",
			Version: "1.1.0",
		},
		"serde 1.0.136": {
			Name:    "serde",
			Version: "1.0.136",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "serde 1.0.136",
							Target: "build_script_build",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"sha2 0.10.2": {
			Name:    "sha2",
			Version: "0.10.2",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "cfg-if 1.0.0",
							Target: "cfg_if",
						},
						{
							Id:     "digest 0.10.3",
							Target: "digest",
						},
					},
					Selects: map[string][]BazelTarget{
						"cfg(any(target_arch = \"aarch64\", target_arch = \"x86_64\", target_arch = \"x86\"))": []BazelTarget{
							{
								Id:     "cpufeatures 0.2.2",
								Target: "cpufeatures",
							},
						},
					},
				},
			},
		},
		"smoltcp 0.8.0": {
			Name:    "smoltcp",
			Version: "0.8.0",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "bitflags 1.3.2",
							Target: "bitflags",
						},
						{
							Id:     "byteorder 1.4.3",
							Target: "byteorder",
						},
						{
							Id:     "libc 0.2.121",
							Target: "libc",
						},
						{
							Id:     "managed 0.8.0",
							Target: "managed",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"spin 0.5.2": {
			Name:    "spin",
			Version: "0.5.2",
		},
		"spin 0.9.2": {
			Name:    "spin",
			Version: "0.9.2",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "lock_api 0.4.6",
							Target: "lock_api",
							Alias:  "lock_api_crate",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"spinning_top 0.2.4": {
			Name:    "spinning_top",
			Version: "0.2.4",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "lock_api 0.4.6",
							Target: "lock_api",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"syn 1.0.89": {
			Name:    "syn",
			Version: "1.0.89",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "proc-macro2 1.0.36",
							Target: "proc_macro2",
						},
						{
							Id:     "quote 1.0.16",
							Target: "quote",
						},
						{
							Id:     "syn 1.0.89",
							Target: "build_script_build",
						},
						{
							Id:     "unicode-xid 0.2.2",
							Target: "unicode_xid",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"thiserror 1.0.30": {
			Name:    "thiserror",
			Version: "1.0.30",
			CommonAttributes: Attributes{
				ProcMacroDeps: Deps{
					Common: []BazelTarget{
						{
							Id:     "thiserror-impl 1.0.30",
							Target: "thiserror_impl",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"thiserror-impl 1.0.30": {
			Name:    "thiserror-impl",
			Version: "1.0.30",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "proc-macro2 1.0.36",
							Target: "proc_macro2",
						},
						{
							Id:     "quote 1.0.16",
							Target: "quote",
						},
						{
							Id:     "syn 1.0.89",
							Target: "syn",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"toml 0.5.8": {
			Name:    "toml",
			Version: "0.5.8",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "serde 1.0.136",
							Target: "serde",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"typenum 1.15.0": {
			Name:    "typenum",
			Version: "1.15.0",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "typenum 1.15.0",
							Target: "build_script_main",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"uart_16550 0.2.16": {
			Name:    "uart_16550",
			Version: "0.2.16",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "bitflags 1.3.2",
							Target: "bitflags",
						},
					},
					Selects: map[string][]BazelTarget{
						"cfg(target_arch = \"x86_64\")": []BazelTarget{
							{
								Id:     "x86_64 0.14.8",
								Target: "x86_64",
							},
						},
					},
				},
				ExtraDeps: []string{
					"@crates//:x86_64",
				},
			},
		},
		"unicode-xid 0.2.2": {
			Name:    "unicode-xid",
			Version: "0.2.2",
		},
		"usize_conversions 0.2.0": {
			Name:    "usize_conversions",
			Version: "0.2.0",
		},
		"version_check 0.9.4": {
			Name:    "version_check",
			Version: "0.9.4",
		},
		"volatile 0.4.4": {
			Name:    "volatile",
			Version: "0.4.4",
		},
		"x86_64 0.14.8": {
			Name:    "x86_64",
			Version: "0.14.8",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "bit_field 0.10.1",
							Target: "bit_field",
						},
						{
							Id:     "bitflags 1.3.2",
							Target: "bitflags",
						},
						{
							Id:     "volatile 0.4.4",
							Target: "volatile",
						},
						{
							Id:     "x86_64 0.14.8",
							Target: "build_script_build",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"xmas-elf 0.8.0": {
			Name:    "xmas-elf",
			Version: "0.8.0",
			CommonAttributes: Attributes{
				Deps: Deps{
					Common: []BazelTarget{
						{
							Id:     "zero 0.1.2",
							Target: "zero",
						},
					},
					Selects: map[string][]BazelTarget{},
				},
			},
		},
		"zero 0.1.2": {
			Name:    "zero",
			Version: "0.1.2",
		},
	},
}
