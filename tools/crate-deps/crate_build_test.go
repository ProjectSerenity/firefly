// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestParseCratesBuild(t *testing.T) {
	name := filepath.Join("testdata", "crates.BUILD")
	got, err := ParseCratesBuild(name)
	if err != nil {
		t.Fatal(err)
	}

	// If the two match, we're all done.
	want := wantBuild
	if reflect.DeepEqual(got, want) {
		return
	}

	g := make([]string, 0, len(got))
	for key := range got {
		g = append(g, key)
	}

	w := make([]string, 0, len(want))
	for key := range want {
		w = append(w, key)
	}

	sort.Strings(g)
	sort.Strings(w)

	if len(g) != len(w) {
		t.Fatalf("alias list mismatch:\n  Got:  %q\n  Want: %q", g, w)
	}

	for i := range g {
		if g[i] != w[i] {
			t.Errorf("alias[%d] mismatch:\n  Got:  %q\n  Want: %q", i, g[i], w[i])
		}
	}
}

var wantBuild = map[string]string{
	"@crates//:bit_field":             "@crates__bit_field-0.10.1//:bit_field",
	"@crates//:bitflags":              "@crates__bitflags-1.3.2//:bitflags",
	"@crates//:byteorder":             "@crates__byteorder-1.4.3//:byteorder",
	"@crates//:chacha20":              "@crates__chacha20-0.9.0//:chacha20",
	"@crates//:cpufeatures":           "@crates__cpufeatures-0.2.2//:cpufeatures",
	"@crates//:digest":                "@crates__digest-0.10.3//:digest",
	"@crates//:fixedvec":              "@crates__fixedvec-0.2.4//:fixedvec",
	"@crates//:hex-literal":           "@crates__hex-literal-0.3.4//:hex_literal",
	"@crates//:lazy_static":           "@crates__lazy_static-1.4.0//:lazy_static",
	"@crates//:libc":                  "@crates__libc-0.2.121//:libc",
	"@crates//:linked_list_allocator": "@crates__linked_list_allocator-0.9.1//:linked_list_allocator",
	"@crates//:llvm-tools":            "@crates__llvm-tools-0.1.1//:llvm_tools",
	"@crates//:managed":               "@crates__managed-0.8.0//:managed",
	"@crates//:pic8259":               "@crates__pic8259-0.10.2//:pic8259",
	"@crates//:raw-cpuid":             "@crates__raw-cpuid-10.3.0//:raw_cpuid",
	"@crates//:rlibc":                 "@crates__rlibc-1.0.0//:rlibc",
	"@crates//:serde":                 "@crates__serde-1.0.136//:serde",
	"@crates//:sha2":                  "@crates__sha2-0.10.2//:sha2",
	"@crates//:smoltcp":               "@crates__smoltcp-0.8.0//:smoltcp",
	"@crates//:thiserror":             "@crates__thiserror-1.0.30//:thiserror",
	"@crates//:toml":                  "@crates__toml-0.5.8//:toml",
	"@crates//:uart_16550":            "@crates__uart_16550-0.2.16//:uart_16550",
	"@crates//:usize_conversions":     "@crates__usize_conversions-0.2.0//:usize_conversions",
	"@crates//:volatile":              "@crates__volatile-0.4.4//:volatile",
	"@crates//:x86_64":                "@crates__x86_64-0.14.8//:x86_64",
	"@crates//:xmas-elf":              "@crates__xmas-elf-0.8.0//:xmas_elf",
	"@crates//:zero":                  "@crates__zero-0.1.2//:zero",
	"@crates//:raw-cpuid__cpuid":      "@crates__raw-cpuid-10.3.0//:cpuid__bin",
	"@crates//:xmas-elf__xmas_elf":    "@crates__xmas-elf-0.8.0//:xmas_elf__bin",
}
